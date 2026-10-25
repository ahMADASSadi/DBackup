package storage

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"dbackup/internal/security"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config is a decrypted object-storage target.
type S3Config struct {
	Endpoint      string
	Region        string
	Bucket        string
	Prefix        string
	PathStyle     bool
	TLSVerify     bool
	AccessKey     string
	SecretKey     string
	AllowInsecure bool
}

// S3 uploads completed backups to an S3-compatible service.
type S3 struct {
	Resolver security.Resolver
}

// ObjectKey joins the configured prefix and the relative backup key.
func ObjectKey(prefix, rel string) string {
	prefix = strings.Trim(prefix, "/")
	rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	if prefix == "" {
		return rel
	}
	return prefix + "/" + rel
}

// Test checks that the bucket is reachable with the supplied credentials.
func (s S3) Test(ctx context.Context, cfg S3Config) error {
	client, err := s.client(ctx, cfg)
	if err != nil {
		return err
	}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return fmt.Errorf("object storage is unreachable")
	}
	if !exists {
		return fmt.Errorf("bucket was not found or is not accessible")
	}
	return nil
}

// Upload stores a local file and checks the remote size afterwards.
func (s S3) Upload(ctx context.Context, cfg S3Config, key, path, checksum string) error {
	client, err := s.client(ctx, cfg)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	_, err = client.PutObject(ctx, cfg.Bucket, key, file, info.Size(), minio.PutObjectOptions{
		ContentType:    "application/octet-stream",
		SendContentMd5: info.Size() > 0 && info.Size() <= 64*1024*1024,
		UserMetadata:   map[string]string{"Sha256": checksum},
	})
	if err != nil {
		return fmt.Errorf("upload failed")
	}
	remote, err := client.StatObject(ctx, cfg.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("upload could not be verified")
	}
	if remote.Size != info.Size() {
		_ = client.RemoveObject(ctx, cfg.Bucket, key, minio.RemoveObjectOptions{})
		return fmt.Errorf("uploaded object size did not match the local backup")
	}
	return nil
}

// Delete removes an object. A missing object is not an error.
func (s S3) Delete(ctx context.Context, cfg S3Config, key string) error {
	if key == "" {
		return nil
	}
	client, err := s.client(ctx, cfg)
	if err != nil {
		return err
	}
	err = client.RemoveObject(ctx, cfg.Bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		response := minio.ToErrorResponse(err)
		if response.Code == "NoSuchKey" || response.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("object delete failed")
	}
	return nil
}

func (s S3) client(ctx context.Context, cfg S3Config) (*minio.Client, error) {
	if cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("object storage is not configured")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	secure := true
	host := "s3.amazonaws.com"
	if endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" || parsed.User != nil {
			return nil, fmt.Errorf("object storage endpoint is invalid")
		}
		switch parsed.Scheme {
		case "https":
			secure = true
		case "http":
			if !cfg.AllowInsecure {
				return nil, fmt.Errorf("object storage endpoint must use https")
			}
			secure = false
		default:
			return nil, fmt.Errorf("object storage endpoint must use https")
		}
		host = parsed.Host
		if err := security.ResolveHost(ctx, s.Resolver, parsed.Hostname(), nil); err != nil {
			return nil, fmt.Errorf("object storage endpoint is not allowed")
		}
	}
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, fmt.Errorf("object storage region is required")
	}
	options := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	}
	if cfg.PathStyle {
		options.BucketLookup = minio.BucketLookupPath
	}
	if !cfg.TLSVerify {
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("object storage transport is unavailable")
		}
		transport := base.Clone()
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		options.Transport = transport
	}
	client, err := minio.New(host, options)
	if err != nil {
		return nil, fmt.Errorf("object storage client could not be created")
	}
	return client, nil
}
