export interface SourceDraft {
  name: string
  type: 'postgres' | 'mysql' | 'sqlite'
  host: string
  port: string
  database: string
  username: string
  path: string
  scheduleEnabled: boolean
  mode: 'interval' | 'cron'
  interval: string
  cron: string
  timezone: string
}

const ident = /^[A-Za-z0-9_][A-Za-z0-9_-]{0,62}$/
const user = /^[A-Za-z0-9_][A-Za-z0-9._-]{0,62}$/

export function sourceErrors(draft: SourceDraft): string[] {
  const errors: string[] = []
  const name = draft.name.trim()
  if (!name || name.length > 80) errors.push('Name must be 1–80 characters.')
  if (draft.type === 'sqlite') {
    if (!draft.path.startsWith('/')) errors.push('SQLite path must be absolute.')
  } else {
    if (!draft.host.trim()) errors.push('Host is required.')
    const port = Number(draft.port)
    if (!Number.isInteger(port) || port < 1 || port > 65535) errors.push('Port must be between 1 and 65535.')
    if (!ident.test(draft.database)) errors.push('Database name contains unsupported characters.')
    if (!user.test(draft.username)) errors.push('Username contains unsupported characters.')
  }
  if (!draft.timezone.trim()) errors.push('Timezone is required.')
  if (draft.scheduleEnabled && draft.mode === 'interval') {
    const interval = Number(draft.interval)
    if (!Number.isInteger(interval) || interval < 60) errors.push('Interval must be at least 60 seconds.')
  }
  if (draft.scheduleEnabled && draft.mode === 'cron' && !draft.cron.trim()) errors.push('Cron expression is required.')
  return errors
}

export function passwordErrors(current: string, next: string, confirm: string): string[] {
  const errors: string[] = []
  if (!current) errors.push('Current password is required.')
  if (next.length < 12 || next.length > 200) errors.push('New password must be 12–200 characters.')
  if (next !== confirm) errors.push('New passwords do not match.')
  if (current && next && current === next) errors.push('New password must be different.')
  return errors
}
