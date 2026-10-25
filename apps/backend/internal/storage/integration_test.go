//go:build integration

package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dbackup/internal/storage"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestS3UploadStatAndRetry(t *testing.T) {
	ctx := context.Background()
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Fatal("S3_ENDPOINT is required")
	}
	cfg := storage.S3Config{
		Endpoint: endpoint, Region: env("S3_REGION", "us-east-1"), Bucket: env("S3_BUCKET", "backups"),
		PathStyle: true, TLSVerify: true, AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"),
		AllowInsecure: true,
	}
	client := rawClient(t, cfg)
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.dump")
	if err := os.WriteFile(path, []byte("backup-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := storage.S3{}
	bad := cfg
	bad.Bucket = "missing-bucket-dbackup"
	if err := store.Upload(ctx, bad, "retry/object.dump", path, "abc"); err == nil {
		t.Fatal("expected the missing bucket to fail")
	}
	if err := store.Upload(ctx, cfg, "retry/object.dump", path, "abc"); err != nil {
		t.Fatal(err)
	}
	info, err := client.StatObject(ctx, cfg.Bucket, "retry/object.dump", minio.StatObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != int64(len("backup-bytes")) {
		t.Fatalf("size %d", info.Size)
	}
	if err := store.Delete(ctx, cfg, "retry/object.dump"); err != nil {
		t.Fatal(err)
	}
}

func rawClient(t *testing.T, cfg storage.S3Config) *minio.Client {
	t.Helper()
	client, err := minio.New("minio:9000", &minio.Options{
		Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: false, Region: cfg.Region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
