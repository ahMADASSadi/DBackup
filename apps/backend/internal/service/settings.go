package service

import (
	"context"
	"net/url"
	"strings"

	"dbackup/internal/backup"
	"dbackup/internal/model"
	"dbackup/internal/repository"
	"dbackup/internal/security"
	"dbackup/internal/storage"
)

// SettingsUpdate is a partial settings write. Blank secrets keep the stored values.
type SettingsUpdate struct {
	S3            *S3Update        `json:"s3"`
	Retention     *model.Retention `json:"retention"`
	MaxConcurrent *int             `json:"max_concurrent_backups"`
}

// S3Update is the object-storage form.
type S3Update struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	PathStyle bool   `json:"path_style"`
	TLSVerify *bool  `json:"tls_verify"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// SettingsView is safe to return to the browser.
type SettingsView struct {
	BackupDir          string            `json:"backup_dir"`
	MetadataDB         string            `json:"metadata_db"`
	UsageBytes         int64             `json:"usage_bytes"`
	SQLiteAllowedRoots []string          `json:"sqlite_allowed_roots"`
	HardMaxConcurrent  int               `json:"hard_max_concurrent"`
	Retention          model.Retention   `json:"retention"`
	MaxConcurrent      int               `json:"max_concurrent_backups"`
	S3                 model.S3Settings  `json:"s3"`
	Tools              backup.ToolStatus `json:"tools"`
	Version            string            `json:"version"`
}

// Settings manages operational configuration.
type Settings struct {
	Store         *repository.Store
	Cipher        *security.Cipher
	Files         *storage.Local
	Objects       storage.S3
	AllowInsecure bool
	HardLimit     int
	BackupDir     string
	MetaPath      string
	Roots         []string
	Tools         backup.ToolStatus
	Version       string
	Resolver      security.Resolver
}

func (s *Settings) View(ctx context.Context) (SettingsView, error) {
	current, err := s.Store.GetSettings(ctx)
	if err != nil {
		return SettingsView{}, err
	}
	usage, err := s.Files.Usage()
	if err != nil {
		usage = 0
	}
	current.S3.AccessKey = ""
	current.S3.SecretKey = ""
	roots := append([]string(nil), s.Roots...)
	return SettingsView{
		BackupDir: s.BackupDir, MetadataDB: s.MetaPath, UsageBytes: usage,
		SQLiteAllowedRoots: roots, HardMaxConcurrent: s.HardLimit,
		Retention: current.Retention, MaxConcurrent: current.MaxConcurrentBackups,
		S3: current.S3, Tools: s.Tools, Version: s.Version,
	}, nil
}

func (s *Settings) Update(ctx context.Context, in SettingsUpdate, actor string) (SettingsView, error) {
	current, err := s.Store.GetSettings(ctx)
	if err != nil {
		return SettingsView{}, err
	}
	if in.Retention != nil {
		if err := validateRetention(*in.Retention); err != nil {
			return SettingsView{}, err
		}
		current.Retention = *in.Retention
	}
	if in.MaxConcurrent != nil {
		if *in.MaxConcurrent < 1 || *in.MaxConcurrent > s.HardLimit {
			return SettingsView{}, E(400, "validation_error", "max concurrent backups exceeds the configured limit")
		}
		current.MaxConcurrentBackups = *in.MaxConcurrent
	}
	if in.S3 != nil {
		if err := s.applyS3(ctx, &current, *in.S3); err != nil {
			return SettingsView{}, err
		}
	}
	if err := s.Store.UpdateSettings(ctx, current); err != nil {
		return SettingsView{}, err
	}
	_ = s.Store.InsertAudit(ctx, actor, "settings.update", "settings", "1", "{}")
	return s.View(ctx)
}

func (s *Settings) TestS3(ctx context.Context, in S3Update) error {
	current, err := s.Store.GetSettings(ctx)
	if err != nil {
		return err
	}
	if err := s.applyS3(ctx, &current, in); err != nil {
		return err
	}
	if !current.S3.Configured && (current.S3.Bucket == "" || current.S3.AccessCipher == "" || current.S3.SecretCipher == "") {
		return E(400, "validation_error", "object storage is incomplete")
	}
	access, err := s.Cipher.Decrypt("settings:s3_access_key", current.S3.AccessCipher)
	if err != nil {
		return E(500, "credential_error", "stored credential could not be read")
	}
	secret, err := s.Cipher.Decrypt("settings:s3_secret_key", current.S3.SecretCipher)
	if err != nil {
		return E(500, "credential_error", "stored credential could not be read")
	}
	cfg := storage.S3Config{
		Endpoint: current.S3.Endpoint, Region: current.S3.Region, Bucket: current.S3.Bucket,
		Prefix: current.S3.Prefix, PathStyle: current.S3.PathStyle, TLSVerify: current.S3.TLSVerify,
		AccessKey: access, SecretKey: secret, AllowInsecure: s.AllowInsecure,
	}
	if err := s.Objects.Test(ctx, cfg); err != nil {
		return E(422, "connection_failed", security.Truncate(security.Redact(err.Error(), access, secret), 300))
	}
	return nil
}

func (s *Settings) applyS3(ctx context.Context, current *model.Settings, in S3Update) error {
	endpoint := strings.TrimSpace(in.Endpoint)
	if endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" || parsed.User != nil {
			return E(400, "validation_error", "object storage endpoint is invalid")
		}
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && s.AllowInsecure) {
			return E(400, "validation_error", "object storage endpoint must use https")
		}
		if err := security.ResolveHost(ctx, s.Resolver, parsed.Hostname(), nil); err != nil {
			return E(400, "validation_error", "object storage endpoint is not allowed")
		}
	}
	bucket := strings.TrimSpace(in.Bucket)
	if err := security.ValidateBucketName(bucket); err != nil {
		return E(400, "validation_error", err.Error())
	}
	prefix := strings.Trim(strings.TrimSpace(in.Prefix), "/")
	if err := security.ValidatePrefix(prefix); err != nil {
		return E(400, "validation_error", err.Error())
	}
	region := strings.TrimSpace(in.Region)
	if bucket != "" && region == "" {
		return E(400, "validation_error", "object storage region is required")
	}
	if strings.ContainsAny(in.AccessKey+in.SecretKey, "\r\n\x00") || len(in.AccessKey) > 256 || len(in.SecretKey) > 256 {
		return E(400, "validation_error", "object storage credentials are invalid")
	}
	accessCipher := current.S3.AccessCipher
	secretCipher := current.S3.SecretCipher
	if in.AccessKey != "" {
		sealed, err := s.Cipher.Encrypt("settings:s3_access_key", in.AccessKey)
		if err != nil {
			return err
		}
		accessCipher = sealed
	}
	if in.SecretKey != "" {
		sealed, err := s.Cipher.Encrypt("settings:s3_secret_key", in.SecretKey)
		if err != nil {
			return err
		}
		secretCipher = sealed
	}
	if bucket != "" && (accessCipher == "" || secretCipher == "") {
		return E(400, "validation_error", "object storage credentials are required")
	}
	tlsVerify := current.S3.TLSVerify
	if in.TLSVerify != nil {
		tlsVerify = *in.TLSVerify
	}
	current.S3.Endpoint = endpoint
	current.S3.Region = region
	current.S3.Bucket = bucket
	current.S3.Prefix = prefix
	current.S3.PathStyle = in.PathStyle
	current.S3.TLSVerify = tlsVerify
	current.S3.AccessCipher = accessCipher
	current.S3.SecretCipher = secretCipher
	current.S3.AccessKeySet = accessCipher != ""
	current.S3.SecretKeySet = secretCipher != ""
	current.S3.Configured = bucket != "" && accessCipher != "" && secretCipher != ""
	return nil
}

func validateRetention(r model.Retention) error {
	for _, n := range []int{r.LocalCount, r.LocalDays, r.RemoteCount, r.RemoteDays} {
		if n < 0 || n > 10000 {
			return E(400, "validation_error", "retention values must be between 0 and 10000")
		}
	}
	return nil
}
