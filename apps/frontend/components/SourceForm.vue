<template>
  <form class="space-y-6" @submit.prevent="submit">
    <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-800 dark:bg-red-950 dark:text-red-200" role="alert">{{ error }}</p>
    <div class="grid gap-4 md:grid-cols-2">
      <label class="block text-sm">Name
        <input v-model="draft.name" required class="field" autocomplete="off">
      </label>
      <label class="block text-sm">Type
        <select v-model="draft.type" class="field" :disabled="lockedType">
          <option value="postgres">PostgreSQL</option>
          <option value="mysql">MySQL</option>
          <option value="sqlite">SQLite</option>
        </select>
      </label>
      <label class="flex items-center gap-2 text-sm md:col-span-2">
        <input v-model="draft.enabled" type="checkbox"> Enabled
      </label>
    </div>

    <fieldset v-if="draft.type === 'sqlite'" class="space-y-3">
      <legend class="text-sm font-medium">SQLite file</legend>
      <label class="block text-sm">Absolute path
        <input v-model="draft.path" class="field" placeholder="/sqlite-sources/app.db">
      </label>
    </fieldset>

    <fieldset v-else class="grid gap-4 md:grid-cols-2">
      <legend class="col-span-full text-sm font-medium">Connection</legend>
      <label class="text-sm">Host<input v-model="draft.host" class="field" autocomplete="off"></label>
      <label class="text-sm">Port<input v-model="draft.port" class="field" inputmode="numeric"></label>
      <label class="text-sm">Database<input v-model="draft.database" class="field" autocomplete="off"></label>
      <label class="text-sm">Username<input v-model="draft.username" class="field" autocomplete="off"></label>
      <label class="text-sm">Password
        <input v-model="draft.password" type="password" class="field" autocomplete="new-password" :placeholder="passwordSet ? 'Leave blank to keep the stored password' : ''">
      </label>
      <label class="text-sm">TLS
        <select v-model="draft.tlsMode" class="field">
          <option value="require">Require encryption</option>
          <option value="verify-ca">Verify CA</option>
          <option value="disable">Disable</option>
        </select>
      </label>
      <label v-if="draft.tlsMode === 'verify-ca'" class="text-sm md:col-span-2">CA bundle
        <textarea v-model="draft.ca" rows="4" class="field font-mono"></textarea>
      </label>
    </fieldset>

    <fieldset class="grid gap-4 md:grid-cols-2">
      <legend class="col-span-full text-sm font-medium">Schedule</legend>
      <label class="flex items-center gap-2 text-sm md:col-span-2"><input v-model="draft.scheduleEnabled" type="checkbox"> Run on a schedule</label>
      <label class="text-sm">Mode
        <select v-model="draft.mode" class="field">
          <option value="cron">Cron</option>
          <option value="interval">Interval</option>
        </select>
      </label>
      <label v-if="draft.mode === 'cron'" class="text-sm">Cron<input v-model="draft.cron" class="field font-mono" placeholder="0 2 * * *"></label>
      <label v-else class="text-sm">Interval seconds<input v-model="draft.interval" class="field" inputmode="numeric"></label>
      <label class="text-sm">Timezone<input v-model="draft.timezone" class="field" placeholder="UTC"></label>
      <label class="flex items-center gap-2 text-sm"><input v-model="draft.upload" type="checkbox"> Upload to object storage</label>
    </fieldset>

    <div class="flex flex-wrap gap-2">
      <button type="submit" class="rounded-md bg-emerald-700 px-3 py-2 text-sm text-white disabled:opacity-60" :disabled="pending">{{ pending ? 'Saving…' : 'Save source' }}</button>
      <button v-if="sourceId" type="button" class="rounded-md border border-zinc-300 px-3 py-2 text-sm dark:border-zinc-700" :disabled="pending" @click="emit('test')">Test connection</button>
    </div>
  </form>
</template>

<script setup lang="ts">
import type { Source } from '~/types/api'
import { sourceErrors } from '~/utils/validate'

const props = defineProps<{ source?: Source | null; pending?: boolean; error?: string }>()
const emit = defineEmits<{ save: [payload: Record<string, unknown>]; test: [] }>()
const lockedType = computed(() => Boolean(props.source?.id))
const sourceId = computed(() => props.source?.id)
const passwordSet = computed(() => props.source?.password_set)

const draft = reactive({
  name: '',
  type: 'postgres' as 'postgres' | 'mysql' | 'sqlite',
  enabled: true,
  host: '',
  port: '5432',
  database: '',
  username: '',
  password: '',
  tlsMode: 'require' as 'disable' | 'require' | 'verify-ca',
  ca: '',
  path: '',
  scheduleEnabled: false,
  mode: 'cron' as 'interval' | 'cron',
  interval: '86400',
  cron: '0 2 * * *',
  timezone: 'UTC',
  upload: false,
})

watch(() => props.source, (source) => {
  if (!source) return
  draft.name = source.name
  draft.type = source.type
  draft.enabled = source.enabled
  draft.host = source.config.host || ''
  draft.port = String(source.config.port || (source.type === 'mysql' ? 3306 : 5432))
  draft.database = source.config.database || ''
  draft.username = source.config.username || ''
  draft.tlsMode = source.config.tls_mode || 'require'
  draft.ca = source.config.tls_ca_pem || ''
  draft.path = source.config.path || ''
  draft.scheduleEnabled = source.schedule.enabled
  draft.mode = source.schedule.mode
  draft.interval = String(source.schedule.interval_seconds || 86400)
  draft.cron = source.schedule.cron || '0 2 * * *'
  draft.timezone = source.schedule.timezone || 'UTC'
  draft.upload = source.schedule.upload
  draft.password = ''
}, { immediate: true })

watch(() => draft.type, (type) => {
  if (!props.source && type === 'mysql' && draft.port === '5432') draft.port = '3306'
  if (!props.source && type === 'postgres' && draft.port === '3306') draft.port = '5432'
})

const localError = ref('')
const error = computed(() => props.error || localError.value)

function submit() {
  localError.value = ''
  const problems = sourceErrors({
    name: draft.name,
    type: draft.type,
    host: draft.host,
    port: draft.port,
    database: draft.database,
    username: draft.username,
    path: draft.path,
    scheduleEnabled: draft.scheduleEnabled,
    mode: draft.mode,
    interval: draft.interval,
    cron: draft.cron,
    timezone: draft.timezone,
  })
  if (!props.source && draft.type !== 'sqlite' && !draft.password) problems.push('Password is required.')
  if (problems.length) {
    localError.value = problems[0]
    return
  }
  const config = draft.type === 'sqlite'
    ? { path: draft.path }
    : {
        host: draft.host.trim(),
        port: Number(draft.port),
        database: draft.database,
        username: draft.username,
        tls_mode: draft.tlsMode,
        tls_ca_pem: draft.tlsMode === 'verify-ca' ? draft.ca : '',
      }
  emit('save', {
    name: draft.name.trim(),
    type: draft.type,
    enabled: draft.enabled,
    password: draft.password,
    config,
    schedule: {
      enabled: draft.scheduleEnabled,
      mode: draft.mode,
      interval_seconds: draft.mode === 'interval' ? Number(draft.interval) : 0,
      cron: draft.cron.trim(),
      timezone: draft.timezone.trim(),
      upload: draft.upload,
      retention: {},
    },
  })
}
</script>

<style scoped>
.field { @apply mt-1 w-full rounded-md border border-zinc-300 bg-white px-3 py-2 dark:border-zinc-700 dark:bg-zinc-950; }
</style>
