export type DatabaseType = 'postgres' | 'mysql' | 'sqlite'
export type BackupStatus = 'pending' | 'running' | 'completed' | 'failed' | 'expired'
export type UploadStatus = 'pending' | 'running' | 'completed' | 'failed' | 'expired'

export interface Schedule {
  enabled: boolean
  mode: 'interval' | 'cron'
  interval_seconds?: number
  cron?: string
  timezone: string
  upload: boolean
  retention: {
    local_count?: number | null
    local_days?: number | null
    remote_count?: number | null
    remote_days?: number | null
  }
  last_run_at?: string | null
  next_run_at?: string | null
}

export interface ConnConfig {
  host?: string
  port?: number
  database?: string
  username?: string
  tls_mode?: 'disable' | 'require' | 'verify-ca'
  tls_ca_pem?: string
  path?: string
}

export interface Source {
  id: string
  name: string
  type: DatabaseType
  enabled: boolean
  config: ConnConfig
  password_set: boolean
  schedule: Schedule
  created_at: string
  updated_at: string
}

export interface Upload {
  id: string
  status: UploadStatus
  object_key?: string
  attempts: number
  error_summary?: string
  completed_at?: string | null
}

export interface Backup {
  id: string
  source_id: string
  source_name?: string
  db_type: DatabaseType
  status: BackupStatus
  format: string
  tool_version?: string
  relative_path?: string
  size_bytes: number
  checksum_sha256?: string
  error_summary?: string
  trigger: 'manual' | 'schedule'
  upload_requested: boolean
  created_at: string
  started_at?: string | null
  completed_at?: string | null
  upload?: Upload | null
}

export interface BackupPage {
  items: Backup[]
  total: number
  limit: number
  offset: number
}

export interface Dashboard {
  sources: number
  recent: Backup[]
  last_success: { source_id: string; source_name: string; backup_id?: string; completed_at?: string | null }[]
  next_runs: { source_id: string; source_name: string; next_run_at: string }[]
  usage_bytes: number
  failed_jobs: number
  pending_uploads: number
  failed_uploads: number
  audit: { id: string; actor: string; action: string; target_type: string; target_id?: string; created_at: string }[]
}

export interface Settings {
  backup_dir: string
  metadata_db: string
  usage_bytes: number
  sqlite_allowed_roots: string[]
  hard_max_concurrent: number
  retention: { local_count: number; local_days: number; remote_count: number; remote_days: number }
  max_concurrent_backups: number
  s3: {
    endpoint: string
    region: string
    bucket: string
    prefix: string
    path_style: boolean
    tls_verify: boolean
    access_key_set: boolean
    secret_key_set: boolean
    configured: boolean
  }
  tools: { pg_dump: string; mysqldump: string; pg_dump_error: string; mysqldump_error: string }
  version: string
}

export interface ApiError {
  error?: { code?: string; message?: string }
}
