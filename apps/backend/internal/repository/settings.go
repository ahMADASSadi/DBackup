package repository

import (
	"context"
	"time"

	"dbackup/internal/model"
)

func (s *Store) GetSettings(ctx context.Context) (model.Settings, error) {
	var out model.Settings
	var pathStyle, tlsVerify int
	var updated string
	err := s.DB.QueryRowContext(ctx, `SELECT s3_endpoint, s3_region, s3_bucket, s3_prefix, s3_path_style, s3_tls_verify,
		s3_access_key_ciphertext, s3_secret_key_ciphertext,
		local_retention_count, local_retention_days, remote_retention_count, remote_retention_days,
		max_concurrent_backups, updated_at FROM settings WHERE id = 1`).Scan(
		&out.S3.Endpoint, &out.S3.Region, &out.S3.Bucket, &out.S3.Prefix, &pathStyle, &tlsVerify,
		&out.S3.AccessCipher, &out.S3.SecretCipher,
		&out.Retention.LocalCount, &out.Retention.LocalDays, &out.Retention.RemoteCount, &out.Retention.RemoteDays,
		&out.MaxConcurrentBackups, &updated,
	)
	if err != nil {
		return model.Settings{}, err
	}
	out.S3.PathStyle = pathStyle == 1
	out.S3.TLSVerify = tlsVerify == 1
	out.S3.AccessKeySet = out.S3.AccessCipher != ""
	out.S3.SecretKeySet = out.S3.SecretCipher != ""
	out.S3.Configured = out.S3.Bucket != "" && out.S3.AccessKeySet && out.S3.SecretKeySet
	out.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return model.Settings{}, err
	}
	return out, nil
}

func (s *Store) UpdateSettings(ctx context.Context, in model.Settings) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE settings SET
		s3_endpoint = ?, s3_region = ?, s3_bucket = ?, s3_prefix = ?, s3_path_style = ?, s3_tls_verify = ?,
		s3_access_key_ciphertext = ?, s3_secret_key_ciphertext = ?,
		local_retention_count = ?, local_retention_days = ?, remote_retention_count = ?, remote_retention_days = ?,
		max_concurrent_backups = ?, updated_at = ? WHERE id = 1`,
		in.S3.Endpoint, in.S3.Region, in.S3.Bucket, in.S3.Prefix, boolInt(in.S3.PathStyle), boolInt(in.S3.TLSVerify),
		in.S3.AccessCipher, in.S3.SecretCipher,
		in.Retention.LocalCount, in.Retention.LocalDays, in.Retention.RemoteCount, in.Retention.RemoteDays,
		in.MaxConcurrentBackups, formatTime(time.Now()))
	return err
}
