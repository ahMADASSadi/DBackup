# Operations

## Restore a backup

New backups are stored as gzip-compressed files encrypted with the administrator password that was in memory when the dump finished. Download keeps that sealed `.dbk` file. Backups created before this protection still use `.dump`, `.sql`, or `.db` and can be restored directly.

Decrypt a new file outside the panel. The password is the administrator password from the time of that backup, read from the terminal and not from an argument:

```sh
docker compose --env-file .env -f deployments/docker-compose.yml exec backend decrypt -in /data/backups/<source-id>/<backup-id>.dbk -out /tmp/backup.dump
```

`decrypt` writes the original dump, already uncompressed. Restore that file. If the administrator password was changed after the backup, use the password from the time of the backup.

PostgreSQL custom format:

```sh
pg_restore --no-owner --clean --if-exists --dbname 'postgresql://USER@HOST:5432/DATABASE' backup.dump
```

The restore client should be at least as new as the server that created the dump. A custom-format dump is not a SQL script.

MySQL:

```sh
mysql --host HOST --user USER --database DATABASE < backup.sql
```

The dump contains one database and does not include a `CREATE DATABASE` statement. Create the empty database first. `--single-transaction` produces a consistent InnoDB snapshot; MyISAM tables are not transactional.

SQLite:

Stop writers, then replace the database file with the snapshot, or open the snapshot as a new file. The snapshot includes data that was only in the WAL at backup time. Do not copy the live database file yourself while it is being written.

## Interrupted jobs

On startup, backups and uploads left in `running` are marked `failed` and temporary files are deleted. Pending backups are picked up again once backup encryption is unlocked. Until an administrator logs in or unlocks the panel, new backups stay pending. Use Retry for a failed dump that has no file, and Retry upload for a completed file whose upload failed. A failed upload does not delete the local file.

## Retention

Local and remote retention are independent. A backup is eligible for removal only when it is beyond the configured count and older than the configured age, when both are set. The newest successful local backup and the newest successful remote object are never removed by retention. Set a limit to `0` to disable that dimension. Deleting the only completed backup from the UI requires the explicit force confirmation.

## Metadata recovery

The metadata database is `/data/meta/dbackup.db` on the `dbackup-data` volume, with `-wal` and `-shm` companions. To back it up, stop the backend and copy all three files, or use SQLite's backup API. Restoring those files restores sources, schedules, history, and encrypted secrets. It does not restore the backup files; those live under `/data/backups`.

If the encryption key is lost, source passwords and S3 keys cannot be decrypted. Create the sources again after choosing a new key. Backup files on disk remain usable.

## Credential rotation

Change the administrator password in Settings. That invalidates sessions and does not re-encrypt existing backups. To reset a forgotten password, set `ADMIN_PASSWORD` to the new value and `ADMIN_PASSWORD_RESET=true`, start the container once, then set the flag back to `false`. Files encrypted with the forgotten password cannot be opened.

To rotate database or S3 secrets, submit the new secret in the existing form. A blank secret field keeps the stored value.

## Upgrades

1. Copy `.env` and confirm the encryption key is unchanged.
2. `make down` is not required before a rebuild, but stop writers if you are also moving the volume.
3. `make up` rebuilds the images and runs migrations on startup.
4. Confirm `GET /api/v1/health` and a test backup.

The backend image is based on `postgres:${PG_MAJOR}-bookworm` (default `18`) and also installs PostgreSQL clients 15–17. At dump time the app reads `server_version_num` and picks a `pg_dump` that is at least that major. When you need to dump PostgreSQL 19+, set `PG_MAJOR=19` in `.env`, rebuild (`make up`), and rerun `make test-integration`.

## Clean install and recreation

`docker compose up` creates the named volume `dbackup-data`. `docker compose down` keeps it. `docker compose down -v` deletes backup files and metadata. Recreating the containers without `-v` keeps both.

SQLite databases to be backed up belong in `sqlite-sources/` on the host, mounted read-only at `/sqlite-sources`.
