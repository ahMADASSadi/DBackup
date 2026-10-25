import { describe, expect, it } from 'vitest'
import { passwordErrors, sourceErrors, type SourceDraft } from './validate'

function draft(overrides: Partial<SourceDraft> = {}): SourceDraft {
  return {
    name: 'Billing',
    type: 'postgres',
    host: 'db.internal',
    port: '5432',
    database: 'billing',
    username: 'backup',
    path: '',
    scheduleEnabled: false,
    mode: 'cron',
    interval: '86400',
    cron: '0 2 * * *',
    timezone: 'UTC',
    ...overrides,
  }
}

describe('sourceErrors', () => {
  it('accepts a postgres source', () => {
    expect(sourceErrors(draft())).toEqual([])
  })

  it('requires an absolute sqlite path', () => {
    expect(sourceErrors(draft({ type: 'sqlite', path: 'app.db' }))).toContain('SQLite path must be absolute.')
  })

  it('rejects a short backup interval', () => {
    expect(sourceErrors(draft({ scheduleEnabled: true, mode: 'interval', interval: '10' }))).toContain('Interval must be at least 60 seconds.')
  })

  it('rejects shell-like database names', () => {
    expect(sourceErrors(draft({ database: 'app;drop' })).length).toBeGreaterThan(0)
  })
})

describe('passwordErrors', () => {
  it('requires a long replacement password', () => {
    expect(passwordErrors('correct-horse', 'short', 'short').length).toBeGreaterThan(0)
  })

  it('accepts a matching replacement', () => {
    expect(passwordErrors('correct-horse-battery', 'another-strong-password', 'another-strong-password')).toEqual([])
  })
})
