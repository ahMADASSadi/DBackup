// Package model holds the metadata the control plane stores and returns.
package model

import "time"

const (
	TypePostgres = "postgres"
	TypeMySQL    = "mysql"
	TypeSQLite   = "sqlite"

	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusExpired   = "expired"

	TriggerManual   = "manual"
	TriggerSchedule = "schedule"

	ModeInterval = "interval"
	ModeCron     = "cron"

	FormatCustom = "custom"
	FormatSQL    = "sql"
	FormatSQLite = "sqlite"
)

// ConnConfig is the non-secret connection configuration for a source.
type ConnConfig struct {
	Host     string            `json:"host,omitempty"`
	Port     int               `json:"port,omitempty"`
	Database string            `json:"database,omitempty"`
	Username string            `json:"username,omitempty"`
	TLSMode  string            `json:"tls_mode,omitempty"`
	TLSCAPem string            `json:"tls_ca_pem,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
	Path     string            `json:"path,omitempty"`
}

// Retention is a local and remote keep policy.
// Zero means that dimension is disabled.
type Retention struct {
	LocalCount  int `json:"local_count"`
	LocalDays   int `json:"local_days"`
	RemoteCount int `json:"remote_count"`
	RemoteDays  int `json:"remote_days"`
}

// RetentionOverride uses nil to inherit the global policy.
type RetentionOverride struct {
	LocalCount  *int `json:"local_count"`
	LocalDays   *int `json:"local_days"`
	RemoteCount *int `json:"remote_count"`
	RemoteDays  *int `json:"remote_days"`
}

// Schedule is the periodic backup policy for one source.
type Schedule struct {
	Enabled         bool              `json:"enabled"`
	Mode            string            `json:"mode"`
	IntervalSeconds int               `json:"interval_seconds,omitempty"`
	Cron            string            `json:"cron,omitempty"`
	Timezone        string            `json:"timezone"`
	Upload          bool              `json:"upload"`
	Retention       RetentionOverride `json:"retention"`
	LastRunAt       *time.Time        `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time        `json:"next_run_at,omitempty"`
}

// Source is a configured database to back up.
type Source struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Type           string     `json:"type"`
	Enabled        bool       `json:"enabled"`
	Config         ConnConfig `json:"config"`
	PasswordCipher string     `json:"-"`
	PasswordSet    bool       `json:"password_set"`
	Schedule       Schedule   `json:"schedule"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Upload is the object-storage state for one backup.
type Upload struct {
	ID           string     `json:"id"`
	BackupID     string     `json:"backup_id"`
	Status       string     `json:"status"`
	ObjectKey    string     `json:"object_key,omitempty"`
	Attempts     int        `json:"attempts"`
	ErrorSummary string     `json:"error_summary,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Backup is one dump attempt and its optional upload.
type Backup struct {
	ID              string     `json:"id"`
	SourceID        string     `json:"source_id"`
	SourceName      string     `json:"source_name,omitempty"`
	DBType          string     `json:"db_type"`
	Status          string     `json:"status"`
	Format          string     `json:"format"`
	ToolVersion     string     `json:"tool_version,omitempty"`
	RelativePath    string     `json:"relative_path,omitempty"`
	SizeBytes       int64      `json:"size_bytes"`
	ChecksumSHA256  string     `json:"checksum_sha256,omitempty"`
	ErrorSummary    string     `json:"error_summary,omitempty"`
	Trigger         string     `json:"trigger"`
	UploadRequested bool       `json:"upload_requested"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Upload          *Upload    `json:"upload,omitempty"`
}

// S3Settings is object storage configuration with secrets kept out of JSON.
type S3Settings struct {
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	PathStyle    bool   `json:"path_style"`
	TLSVerify    bool   `json:"tls_verify"`
	AccessKeySet bool   `json:"access_key_set"`
	SecretKeySet bool   `json:"secret_key_set"`
	Configured   bool   `json:"configured"`
	AccessKey    string `json:"-"`
	SecretKey    string `json:"-"`
	AccessCipher string `json:"-"`
	SecretCipher string `json:"-"`
}

// Settings is the singleton operational configuration.
type Settings struct {
	S3                   S3Settings `json:"s3"`
	Retention            Retention  `json:"retention"`
	MaxConcurrentBackups int        `json:"max_concurrent_backups"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// AuditEvent is a redacted record of an administrator action.
type AuditEvent struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// BackupFilter selects a page of backup history.
type BackupFilter struct {
	SourceID string
	Status   string
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

// BackupPage is a page of backup history.
type BackupPage struct {
	Items  []Backup `json:"items"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

// EncryptedExt is the published suffix after gzip and encryption.
const EncryptedExt = ".dbk"

// Extension returns the dump suffix before compression and encryption.
func Extension(dbType string) string {
	switch dbType {
	case TypePostgres:
		return ".dump"
	case TypeMySQL:
		return ".sql"
	case TypeSQLite:
		return ".db"
	default:
		return ".bin"
	}
}

// FormatFor returns the backup format name for a database type.
func FormatFor(dbType string) string {
	switch dbType {
	case TypePostgres:
		return FormatCustom
	case TypeMySQL:
		return FormatSQL
	default:
		return FormatSQLite
	}
}
