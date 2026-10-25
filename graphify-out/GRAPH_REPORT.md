# Graph Report - DBackup  (2026-10-25)

## Corpus Check
- 85 files · ~32,888 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 11 file(s) not represented in the graph (top: (none) 8, .example 1, .test 1)

## Summary
- 718 nodes · 2026 edges · 43 communities (29 shown, 14 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 161 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- app.go
- testing.T
- net/http.ResponseWriter
- E
- dumpPostgres
- ResolveHost
- Jobs
- Operations
- package.json
- context.Context
- config.go
- time.Time
- Local
- SourceForm.vue
- backups/index.vue
- api.ts
- settings.vue
- [id].vue
- insertSchedule
- Source
- formatTime
- parseTime
- Backup
- pages/index.vue
- sources/index.vue
- default.vue
- login.vue
- useAuth
- useTheme
- new.vue
- StatusPill.vue
- tsconfig.json
- limitWriter
- .NewBackup
- docker-entrypoint.sh
- ConfirmDialog.vue
- dbackup

## God Nodes (most connected - your core abstractions)
1. `Store` - 30 edges
2. `E()` - 29 edges
3. `formatTime()` - 28 edges
4. `writeErr()` - 25 edges
5. `Jobs` - 25 edges
6. `Store` - 23 edges
7. `Backup` - 22 edges
8. `Source` - 21 edges
9. `writeJSON()` - 20 edges
10. `Local` - 19 edges

## Surprising Connections (you probably didn't know these)
- `TestSQLiteBackupSeesWAL()` --calls--> `dumpSQLite()`  [INFERRED]
  apps/backend/internal/backup/backup_test.go → apps/backend/internal/backup/sqlite.go
- `backupWhere()` --calls--> `formatTime()`  [INFERRED]
  apps/backend/internal/repository/backups.go → apps/backend/internal/repository/repository.go
- `scanBackup()` --calls--> `parseTime()`  [INFERRED]
  apps/backend/internal/repository/backups.go → apps/backend/internal/repository/repository.go
- `scanBackup()` --calls--> `parseTimePtr()`  [INFERRED]
  apps/backend/internal/repository/backups.go → apps/backend/internal/repository/repository.go
- `scanSource()` --calls--> `parseTime()`  [INFERRED]
  apps/backend/internal/repository/sources.go → apps/backend/internal/repository/repository.go

## Import Cycles
- None detected.

## Communities (43 total, 14 thin omitted)

### Community 0 - "app.go"
Cohesion: 0.13
Nodes (4): leakError, backupRequest, loginRequest, passwordRequest

### Community 1 - "testing.T"
Cohesion: 0.07
Nodes (39): App, New(), allowRootFromSettings(), decode(), do(), freshRoot(), newApp(), sqliteRoot() (+31 more)

### Community 2 - "net/http.ResponseWriter"
Cohesion: 0.13
Nodes (19): actor(), Handler, Handler, tokenOK(), bytesReader(), decode(), decodeOptional(), writeErr() (+11 more)

### Community 3 - "E"
Cohesion: 0.10
Nodes (18): FormatFor(), NextRun(), TestNextRun(), Cipher, ValidID(), ValidateCAPem(), ValidateIdent(), ValidateParams() (+10 more)

### Community 4 - "dumpPostgres"
Cohesion: 0.08
Nodes (26): pingMySQL(), pingPostgres(), pingSQL(), pingSQLite(), Engine, firstLine(), stripEnv(), toolMissing() (+18 more)

### Community 5 - "ResolveHost"
Cohesion: 0.09
Nodes (24): ToolStatus, Retention, RetentionOverride, S3Settings, Settings, Store, EffectiveRetention(), TestRejectsShellHost() (+16 more)

### Community 6 - "Jobs"
Cohesion: 0.09
Nodes (14): healthcheck(), main(), Extension(), NewRedactHandler(), Redact(), redactAttr(), contains(), stringIndex() (+6 more)

### Community 7 - "Operations"
Cohesion: 0.06
Nodes (26): API, Architecture, Dependencies, Dump formats, Job states, Layers, Metadata, Clean install and recreation (+18 more)

### Community 8 - "package.json"
Cohesion: 0.07
Nodes (25): dependencies, nuxt, vue, devDependencies, happy-dom, @nuxtjs/tailwindcss, tailwindcss, typescript (+17 more)

### Community 9 - "context.Context"
Cohesion: 0.15
Nodes (3): dumpSQLite(), Store, Truncate()

### Community 10 - "config.go"
Cohesion: 0.14
Nodes (18): boolEnv(), decodeKey(), durationEnv(), envDefault(), Config, intEnv(), Load(), splitCSV() (+10 more)

### Community 11 - "time.Time"
Cohesion: 0.15
Nodes (15): parseTimeQuery(), AuditEvent, Overview, exceeds(), later(), SelectFailedDeletions(), SelectLocalDeletions(), SelectRemoteDeletions() (+7 more)

### Community 12 - "Local"
Cohesion: 0.13
Nodes (5): Local, hashFile(), syncFile(), validExt(), Download

### Community 13 - "SourceForm.vue"
Cohesion: 0.14
Nodes (14): draft, emit, error, localError, lockedType, passwordSet, props, sourceId (+6 more)

### Community 14 - "backups/index.vue"
Cohesion: 0.13
Nodes (17): api, deleteCompleted, deleteId, deleteMessage, error, filters, load(), page (+9 more)

### Community 15 - "api.ts"
Cohesion: 0.14
Nodes (13): ApiRequestError, useApi(), request(), ApiError, Backup, BackupPage, BackupStatus, ConnConfig (+5 more)

### Community 16 - "settings.vue"
Cohesion: 0.17
Nodes (15): api, error, fill(), load(), notice, passwords, pending, retention (+7 more)

### Community 17 - "[id].vue"
Cohesion: 0.13
Nodes (9): api, confirmDelete, error, id, loading, notice, pending, route (+1 more)

### Community 18 - "insertSchedule"
Cohesion: 0.27
Nodes (9): Schedule, marshalConfig(), nullableInt(), nullableTime(), boolInt(), insertSchedule(), nullableString(), upsertSchedule() (+1 more)

### Community 19 - "Source"
Cohesion: 0.27
Nodes (6): Source, Store, nullIntPtr(), scanSource(), sourceSelect(), scanner

### Community 20 - "formatTime"
Cohesion: 0.27
Nodes (3): formatTime(), Store, NewID()

### Community 21 - "parseTime"
Cohesion: 0.20
Nodes (7): Upload, scanUpload(), Session, parseTime(), parseTimePtr(), HashToken(), Admin

### Community 22 - "Backup"
Cohesion: 0.36
Nodes (6): Backup, BackupFilter, BackupPage, backupSelect(), backupWhere(), scanBackup()

### Community 23 - "pages/index.vue"
Cohesion: 0.27
Nodes (7): api, cards, data, error, pending, formatBytes(), formatTime()

### Community 24 - "sources/index.vue"
Cohesion: 0.22
Nodes (5): api, error, notice, pending, sources

### Community 25 - "default.vue"
Cohesion: 0.29
Nodes (4): auth, items, route, theme

### Community 26 - "login.vue"
Cohesion: 0.29
Nodes (5): auth, error, password, pending, username

### Community 28 - "useTheme"
Cohesion: 0.47
Nodes (5): ThemeName, useTheme(), apply(), init(), toggle()

### Community 29 - "new.vue"
Cohesion: 0.33
Nodes (4): api, error, pending, Source

## Knowledge Gaps
- **8 isolated node(s):** `dbackup`, `nuxt`, `vue`, `@nuxtjs/tailwindcss`, `@vue/test-utils` (+3 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 188 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **14 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Jobs` connect `E` to `app.go`, `testing.T`, `dumpPostgres`, `ResolveHost`, `context.Context`, `config.go`, `Local`, `formatTime`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._
- **Are the 23 inferred relationships involving `E()` (e.g. with `normalizeSchedule()` and `validateRetention()`) actually correct?**
  _`E()` has 23 INFERRED edges - model-reasoned connections that need verification._
- **What connects `dbackup`, `nuxt`, `vue` to the rest of the system?**
  _8 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `app.go` be split into smaller, more focused modules?**
  _Cohesion score 0.12534332084893882 - nodes in this community are weakly interconnected._
- **Why does `Local` connect `Local` to `app.go`, `testing.T`, `E`, `ResolveHost`, `time.Time`?**
  _High betweenness centrality (0.025) - this node is a cross-community bridge._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.06966618287373004 - nodes in this community are weakly interconnected._
- **Why does `E()` connect `E` to `app.go`, `net/http.ResponseWriter`, `ResolveHost`, `Jobs`, `context.Context`, `config.go`, `parseTime`?**
  _High betweenness centrality (0.016) - this node is a cross-community bridge._