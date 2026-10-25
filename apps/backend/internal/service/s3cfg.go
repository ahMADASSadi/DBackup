package service

import (
	"context"

	"dbackup/internal/repository"
	"dbackup/internal/security"
	"dbackup/internal/storage"
)

func loadS3(ctx context.Context, store *repository.Store, cipher *security.Cipher, allowInsecure bool) (storage.S3Config, bool, error) {
	settings, err := store.GetSettings(ctx)
	if err != nil {
		return storage.S3Config{}, false, err
	}
	cfg := storage.S3Config{
		Endpoint:      settings.S3.Endpoint,
		Region:        settings.S3.Region,
		Bucket:        settings.S3.Bucket,
		Prefix:        settings.S3.Prefix,
		PathStyle:     settings.S3.PathStyle,
		TLSVerify:     settings.S3.TLSVerify,
		AllowInsecure: allowInsecure,
	}
	if !settings.S3.Configured {
		return cfg, false, nil
	}
	access, err := cipher.Decrypt("settings:s3_access_key", settings.S3.AccessCipher)
	if err != nil {
		return cfg, false, err
	}
	secret, err := cipher.Decrypt("settings:s3_secret_key", settings.S3.SecretCipher)
	if err != nil {
		return cfg, false, err
	}
	cfg.AccessKey = access
	cfg.SecretKey = secret
	return cfg, true, nil
}
