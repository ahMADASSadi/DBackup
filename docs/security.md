# Security

DBackup can read configured databases and write those dumps to disk or object storage. Treat the host and the administrator account as privileged.

## Controls

- One administrator, created from `ADMIN_USERNAME` and `ADMIN_PASSWORD` only when no account exists. There is no registration and no built-in password.
- Passwords are hashed with Argon2id. Database and object-storage secrets are encrypted with AES-256-GCM. The API returns booleans for stored secrets, never the secrets.
- Sessions use an `HttpOnly`, `SameSite=Lax` cookie. Set `COOKIE_SECURE=true` whenever the site is served over HTTPS. Logout and password changes delete server-side sessions. A second login replaces the previous session.
- State-changing requests require the `X-CSRF-Token` returned by login or session.
- Login failures are limited to 8 attempts per 15 minutes for each IP and username.
- JSON bodies are limited to 1 MiB. Identifiers are UUID version 4. Query values are validated.
- Dump tools are executed with argument arrays. Hosts, database names, and extra parameters are allowlisted before use. Passwords go in mode `0600` files that are removed after the dump.
- Backup downloads resolve a stored relative key and reject paths outside the backup directory.
- SQLite sources must stay inside `SQLITE_ALLOWED_ROOTS` after symlink resolution.
- Database hosts and S3 endpoints are checked after DNS resolution. Link-local and cloud metadata addresses are rejected. Private addresses remain allowed because databases normally live on a private network. `DB_HOST_ALLOWLIST` can narrow the hosts further.
- S3 endpoints must be `https` unless `S3_ALLOW_INSECURE=true`.
- Logs pass through a redacting handler. Backup errors stored in metadata are truncated and scrubbed.
- Caddy sends a restrictive Content Security Policy and the usual browser headers. The API adds `Cache-Control: no-store` and `default-src 'none'`.
- The metadata database and the backend port are not published. Caddy binds `127.0.0.1` unless `HTTP_PORT` publishing is changed. The Docker socket is not mounted.

## Checklist

- Replace every value in `.env`, especially `APP_ENCRYPTION_KEY` and `ADMIN_PASSWORD`.
- Keep `COOKIE_SECURE=true` behind HTTPS. Terminate TLS in front of Caddy for any host that is not loopback.
- Give database accounts only the rights needed to dump. PostgreSQL needs `CONNECT` and read access. MySQL needs `SELECT`, `SHOW VIEW`, and usually `TRIGGER`/`LOCK TABLES` depending on the engine. Do not use a superuser.
- Give the object-storage key access only to the configured bucket and prefix.
- Mount SQLite source directories read-only.
- Keep the backup volume and the encryption key in separate backups. Losing the key makes stored database passwords and S3 keys unreadable. The backup files themselves are not encrypted by the application.
- Run `make test` and review `govulncheck` and `npm audit` before upgrading.

## Limitations

Application code cannot replace network policy. An authenticated administrator can target any database host the process can reach, except blocked metadata and link-local addresses. Firewall the container so it can reach only intended databases and the object-storage endpoint.

The administrator is trusted to operate the panel. Controls against injection, path escape, and CSRF still apply because a stolen session or a malicious field must not become a shell command.

`pg_dump` cannot reliably dump a PostgreSQL server newer than the client in the image. Upgrade the client with the server. See `docs/operations.md`.

A lost `APP_ENCRYPTION_KEY` cannot be recovered from the metadata database. Rotate it only with a re-encryption procedure; this version does not migrate ciphertext between keys automatically.

`SameSite=Lax` and the CSRF header protect cookie-authenticated requests in normal browsers. The deployment must still use HTTPS outside local development.

`govulncheck` against Go 1.26.2 reports vulnerabilities in the standard library (`net/http`, `crypto/tls`, `crypto/x509`). They are fixed only by a newer Go toolchain, not by an application dependency bump. Rebuild the backend image when a patched `golang` and `postgres` base tag is available. `npm audit` reports upstream Nuxt 3 and Nitro issues, including `node-forge` through the Nuxt CLI listener. The panel does not use that code for password hashing or backup encryption. Stay on Nuxt 3 as specified, and move to a patched 3.x release when one is published.

The panel does not scan backup contents for malware and does not encrypt backup files at rest. Protect the volume with disk encryption if the dumps are sensitive.
