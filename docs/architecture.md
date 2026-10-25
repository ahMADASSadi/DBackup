# Architecture

DBackup is a single-instance control plane. A browser talks to Caddy. Caddy serves the Nuxt interface and reverse-proxies `/api/*` to the Go API. The API stores its own metadata in SQLite and runs backup jobs in-process.

```text
Browser → Caddy → Nuxt
                → Go API → metadata SQLite
                         → pg_dump / mysqldump / SQLite backup API
                         → local backup volume
                         → optional S3-compatible storage
```

## Layers

- `handler` validates HTTP input and writes JSON.
- `service` decides when a backup, upload, or retention deletion is allowed.
- `repository` reads and writes metadata.
- `backup`, `storage`, and `scheduler` talk to dump tools, the filesystem, object storage, and cron.

Long-running work is not tied to the HTTP request. Creating a backup inserts a `pending` row and returns `202`. A dispatcher claims the row, runs the dump, and publishes the file only after it is complete.

## Job states

`pending` → `running` → `completed` or `failed`.

Retention later marks a completed backup `expired` after its file is removed. An explicit delete removes the row. A process restart marks `running` backups and uploads as `failed` with the reason `interrupted by process restart`. `pending` work is resumed. A failed dump that never published a file can be retried. A failed upload can be retried without creating another dump.

Only one active backup is allowed per source. The global concurrency limit is the smaller of `MAX_CONCURRENT_BACKUPS` and the value saved in settings.

## Dump formats

PostgreSQL uses `pg_dump --format=custom`. The file is compressed and restored with `pg_restore`. It is not plain SQL, and the client major version must be at least the server major version. The image ships the PostgreSQL 17 client.

MySQL uses `mysqldump` with `--single-transaction`, routines, triggers, and hex blobs. The result is plain SQL for one database and is restored with the `mysql` client. Credentials are passed in a `0600` defaults file, not on the command line.

SQLite uses the driver's online backup API, which copies a consistent snapshot, including WAL pages. The result is a restorable database file.

## Metadata

The metadata database is separate from every source. It uses WAL, `busy_timeout=5000`, foreign keys, and a single writer connection. Schema changes are embedded SQL files applied once.

Passwords and object-storage keys are AES-256-GCM ciphertext bound to the record. The administrator password is an Argon2id hash. Session tokens are stored only as SHA-256 digests.

## Dependencies

| Module | Why it is here |
| --- | --- |
| `modernc.org/sqlite` | Metadata database and consistent SQLite snapshots, without CGO |
| `github.com/jackc/pgx/v5` | PostgreSQL connection tests |
| `github.com/go-sql-driver/mysql` | MySQL connection tests |
| `golang.org/x/crypto` | Argon2id |
| `github.com/minio/minio-go/v7` | S3-compatible uploads |
| `github.com/robfig/cron/v3` | Timezone-aware cron expressions |

The frontend uses Nuxt 3, Vue 3, TypeScript, and Tailwind CSS.
