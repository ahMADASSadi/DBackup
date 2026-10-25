# DBackup

DBackup is a single-administrator control plane for PostgreSQL, MySQL, and SQLite backups. It stores backup metadata in its own SQLite database, runs dumps with the native tools, and can upload finished backups to S3-compatible storage.

## Layout

```text
apps/backend     Go API, dump engine, scheduler
apps/frontend    Nuxt 3 interface
deployments      Compose files and Caddy
docs             Architecture, security, operations, API
sqlite-sources   Host directory mounted read-only for SQLite databases
```

## Run

```sh
cp .env.example .env
openssl rand -base64 32
```

Put that value in `APP_ENCRYPTION_KEY`. Set `ADMIN_PASSWORD` to at least 12 characters. Then:

```sh
make up
```

Open `http://127.0.0.1:8080`. Caddy is bound to loopback. The backend and frontend ports are not published.

Published images live on GitHub Container Registry. After they exist, start from those images instead of building locally:

```sh
make pull
make up-remote
```

`make up` still builds from this checkout. Images:

- `ghcr.io/ahmadassadi/dbackup-backend:latest`
- `ghcr.io/ahmadassadi/dbackup-frontend:latest`

If the repository or the packages are private, log in once before pulling:

```sh
docker login ghcr.io -u ahMADASSadi
```

`make down` stops the containers and keeps the `dbackup-data` volume. Do not use `down -v` unless you intend to delete backups and metadata.

For a local MinIO service used only as an upload target:

```sh
docker compose --env-file .env -f deployments/docker-compose.yml --profile s3 up -d
```

Set `S3_ALLOW_INSECURE=true` only for that local HTTP endpoint.

## Environment

| Variable | Purpose |
| --- | --- |
| `ADMIN_USERNAME`, `ADMIN_PASSWORD` | Bootstrap the first administrator. Ignored after the account exists. |
| `ADMIN_PASSWORD_RESET` | `true` replaces the password from `ADMIN_PASSWORD` once. |
| `APP_ENCRYPTION_KEY` | Base64 of 32 bytes. Required. Losing it makes stored secrets unreadable. |
| `SESSION_TTL` | Session lifetime. Default `12h`. |
| `COOKIE_SECURE` | Set `true` behind HTTPS. |
| `BACKUP_DIR`, `METADATA_DB_PATH` | Paths inside the container. Defaults live on `/data`. |
| `SQLITE_ALLOWED_ROOTS` | Colon is not used; separate directories with commas. |
| `HTTP_ADDR` | API listen address inside the container. Default `:8080`. |
| `TRUSTED_PROXIES` | CIDRs allowed to supply `X-Forwarded-For`. |
| `MAX_CONCURRENT_BACKUPS` | Hard cap. Settings can only lower it. |
| `BACKUP_TIMEOUT` | Per-dump timeout. Default `2h`. |
| `DB_HOST_ALLOWLIST` | Optional hostnames or CIDRs. Empty allows public and private hosts except link-local and metadata addresses. |
| `S3_ALLOW_INSECURE` | Permit `http://` object-storage endpoints. |
| `HTTP_PORT` | Loopback port published by Caddy. |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error`. |

## Tests

```sh
make test
make test-integration
```

`make test` runs the Go tests and the frontend validation tests. `make test-integration` starts disposable PostgreSQL, MySQL, and MinIO containers and checks that a dump can be restored or uploaded. It does not touch databases configured in the running panel.

## Development without Compose

```sh
cd apps/backend && go test ./...
# export the variables from .env, with BACKUP_DIR and METADATA_DB_PATH pointing at local directories
go run ./cmd/server
cd apps/frontend && npm install && npm run dev
```

The Nuxt dev server proxies `/api` to `127.0.0.1:8080`.

## Further reading

- [Architecture](docs/architecture.md)
- [Security](docs/security.md)
- [Operations and restore](docs/operations.md)
- [API](docs/api.md)
