# API

All routes are under `/api/v1`. Protected routes require the `dbackup_session` cookie. State-changing routes also require `X-CSRF-Token`.

Errors use `{"error":{"code":"...","message":"..."}}`.

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/health` | Public. `{"status":"ok"}` or 503. No diagnostics. |
| POST | `/auth/login` | Sets the session cookie and returns `csrf_token`. |
| POST | `/auth/logout` | Deletes the session. |
| GET | `/auth/session` | `{authenticated, username, csrf_token}`. |
| PUT | `/auth/password` | Requires the current password. Clears sessions. |
| GET | `/dashboard` | Counts, recent jobs, last success, next run, usage. |
| GET | `/sources` | `{items:[...]}`. Passwords are never returned. |
| POST | `/sources` | Creates a source. |
| GET | `/sources/{id}` | One source. |
| PUT | `/sources/{id}` | Replaces editable fields. A blank password keeps the stored one. |
| DELETE | `/sources/{id}?delete_backups=true` | Refuses when backups exist unless the query flag is set. |
| POST | `/sources/{id}/test` | Connects and runs `SELECT 1`. |
| POST | `/sources/{id}/backups` | Returns `202` and a backup record. Optional `{"upload":true}`. |
| GET | `/backups?source_id&status&from&to&limit&offset` | Page of history. `from` and `to` are RFC3339. |
| GET | `/backups/{id}` | One record, including upload status when present. |
| GET | `/backups/{id}/download` | The completed file. |
| DELETE | `/backups/{id}?force=true` | Refuses to delete the only completed backup unless `force=true`. |
| POST | `/backups/{id}/retry` | Re-queues a failed dump that published no file. |
| POST | `/backups/{id}/retry-upload` | Re-queues a failed upload. |
| GET | `/settings` | Non-secret configuration and `access_key_set` / `secret_key_set`. |
| PUT | `/settings` | Partial update of retention, concurrency, and S3. Blank secrets are kept. |
| POST | `/settings/s3/test` | Checks the bucket with the submitted or stored credentials. |

Backup creation does not wait for `pg_dump` or `mysqldump`. Poll `GET /backups/{id}` until the status is `completed` or `failed`.
