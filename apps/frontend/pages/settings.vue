<template>
  <section class="max-w-3xl space-y-8">
    <header>
      <h1 class="text-xl font-semibold">Settings</h1>
      <p class="text-sm text-zinc-500">Credentials, retention, and object storage. Secrets already stored are not shown.</p>
    </header>
    <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-800 dark:bg-red-950 dark:text-red-200" role="alert">{{ error }}</p>
    <p v-if="notice" class="text-sm" role="status">{{ notice }}</p>
    <p v-if="pending" class="text-sm text-zinc-500">Loading…</p>
    <template v-else-if="settings">
      <section class="rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900">
        <h2 class="text-sm font-medium">Health</h2>
        <dl class="mt-3 grid gap-2 text-sm md:grid-cols-2">
          <div><dt class="text-zinc-500">Version</dt><dd>{{ settings.version }}</dd></div>
          <div><dt class="text-zinc-500">Local usage</dt><dd>{{ formatBytes(settings.usage_bytes) }}</dd></div>
          <div><dt class="text-zinc-500">Backup directory</dt><dd class="font-mono text-xs">{{ settings.backup_dir }}</dd></div>
          <div><dt class="text-zinc-500">Metadata database</dt><dd class="font-mono text-xs">{{ settings.metadata_db }}</dd></div>
          <div class="md:col-span-2"><dt class="text-zinc-500">pg_dump</dt><dd>{{ settings.tools.pg_dump || settings.tools.pg_dump_error || 'Unavailable' }}</dd></div>
          <div class="md:col-span-2"><dt class="text-zinc-500">mysqldump</dt><dd>{{ settings.tools.mysqldump || settings.tools.mysqldump_error || 'Unavailable' }}</dd></div>
        </dl>
      </section>

      <form class="space-y-3" @submit.prevent="changePassword">
        <h2 class="text-sm font-medium">Administrator password</h2>
        <label class="block text-sm">Current<input v-model="passwords.current" type="password" autocomplete="current-password" class="field"></label>
        <label class="block text-sm">New<input v-model="passwords.next" type="password" autocomplete="new-password" class="field"></label>
        <label class="block text-sm">Confirm<input v-model="passwords.confirm" type="password" autocomplete="new-password" class="field"></label>
        <button class="rounded-md bg-zinc-900 px-3 py-2 text-sm text-white dark:bg-zinc-100 dark:text-zinc-900" type="submit">Update password</button>
      </form>

      <form class="grid gap-3 md:grid-cols-2" @submit.prevent="saveRetention">
        <h2 class="text-sm font-medium md:col-span-2">Retention</h2>
        <label class="text-sm">Local copies<input v-model.number="retention.local_count" type="number" min="0" class="field"></label>
        <label class="text-sm">Local days<input v-model.number="retention.local_days" type="number" min="0" class="field"></label>
        <label class="text-sm">Remote copies<input v-model.number="retention.remote_count" type="number" min="0" class="field"></label>
        <label class="text-sm">Remote days<input v-model.number="retention.remote_days" type="number" min="0" class="field"></label>
        <label class="text-sm md:col-span-2">Max concurrent jobs
          <input v-model.number="retention.max" type="number" min="1" :max="settings.hard_max_concurrent" class="field">
        </label>
        <p class="text-xs text-zinc-500 md:col-span-2">A backup is removed only when it exceeds both the count and the age, and the newest successful copy is always kept. Zero disables that limit.</p>
        <button class="w-fit rounded-md bg-emerald-700 px-3 py-2 text-sm text-white" type="submit">Save retention</button>
      </form>

      <form class="grid gap-3 md:grid-cols-2" @submit.prevent="saveS3">
        <h2 class="text-sm font-medium md:col-span-2">Object storage</h2>
        <label class="text-sm md:col-span-2">Endpoint<input v-model="s3.endpoint" class="field" placeholder="https://s3.amazonaws.com"></label>
        <label class="text-sm">Region<input v-model="s3.region" class="field"></label>
        <label class="text-sm">Bucket<input v-model="s3.bucket" class="field"></label>
        <label class="text-sm">Prefix<input v-model="s3.prefix" class="field"></label>
        <label class="flex items-center gap-2 text-sm"><input v-model="s3.path_style" type="checkbox"> Path-style URLs</label>
        <label class="flex items-center gap-2 text-sm"><input v-model="s3.tls_verify" type="checkbox"> Verify TLS certificate</label>
        <label class="text-sm">Access key<input v-model="s3.access_key" type="password" autocomplete="off" class="field" :placeholder="settings.s3.access_key_set ? 'Stored key will be kept if blank' : ''"></label>
        <label class="text-sm">Secret key<input v-model="s3.secret_key" type="password" autocomplete="off" class="field" :placeholder="settings.s3.secret_key_set ? 'Stored secret will be kept if blank' : ''"></label>
        <p class="text-xs text-zinc-500 md:col-span-2">{{ settings.s3.configured ? 'Object storage is configured.' : 'Object storage is not configured.' }}</p>
        <div class="flex gap-2 md:col-span-2">
          <button class="rounded-md bg-emerald-700 px-3 py-2 text-sm text-white" type="submit">Save storage</button>
          <button class="rounded-md border border-zinc-300 px-3 py-2 text-sm dark:border-zinc-700" type="button" @click="testS3">Test connection</button>
        </div>
      </form>
    </template>
  </section>
</template>

<script setup lang="ts">
import type { Settings } from '~/types/api'
import { formatBytes } from '~/utils/format'
import { passwordErrors } from '~/utils/validate'

const api = useApi()
const settings = ref<Settings | null>(null)
const pending = ref(true)
const error = ref('')
const notice = ref('')
const passwords = reactive({ current: '', next: '', confirm: '' })
const retention = reactive({ local_count: 7, local_days: 30, remote_count: 14, remote_days: 90, max: 2 })
const s3 = reactive({ endpoint: '', region: '', bucket: '', prefix: '', path_style: false, tls_verify: true, access_key: '', secret_key: '' })

function fill(value: Settings) {
  settings.value = value
  retention.local_count = value.retention.local_count
  retention.local_days = value.retention.local_days
  retention.remote_count = value.retention.remote_count
  retention.remote_days = value.retention.remote_days
  retention.max = value.max_concurrent_backups
  s3.endpoint = value.s3.endpoint
  s3.region = value.s3.region
  s3.bucket = value.s3.bucket
  s3.prefix = value.s3.prefix
  s3.path_style = value.s3.path_style
  s3.tls_verify = value.s3.tls_verify
  s3.access_key = ''
  s3.secret_key = ''
}

async function load() {
  try {
    fill(await api.get<Settings>('/settings'))
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load settings'
  } finally {
    pending.value = false
  }
}

async function changePassword() {
  error.value = ''
  const problems = passwordErrors(passwords.current, passwords.next, passwords.confirm)
  if (problems.length) { error.value = problems[0]; return }
  try {
    await api.put('/auth/password', { current_password: passwords.current, new_password: passwords.next })
    await navigateTo('/login')
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Password change failed'
  }
}

async function saveRetention() {
  error.value = ''
  try {
    fill(await api.put<Settings>('/settings', {
      retention: { local_count: retention.local_count, local_days: retention.local_days, remote_count: retention.remote_count, remote_days: retention.remote_days },
      max_concurrent_backups: retention.max,
    }))
    notice.value = 'Retention saved.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not save retention'
  }
}

function s3Payload() {
  return {
    endpoint: s3.endpoint, region: s3.region, bucket: s3.bucket, prefix: s3.prefix,
    path_style: s3.path_style, tls_verify: s3.tls_verify, access_key: s3.access_key, secret_key: s3.secret_key,
  }
}

async function saveS3() {
  error.value = ''
  try {
    fill(await api.put<Settings>('/settings', { s3: s3Payload() }))
    notice.value = 'Object storage saved.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not save object storage'
  }
}

async function testS3() {
  error.value = ''
  try {
    await api.post('/settings/s3/test', s3Payload())
    notice.value = 'Object storage connection succeeded.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Object storage test failed'
  }
}

onMounted(load)
</script>

<style scoped>
.field { @apply mt-1 w-full rounded-md border border-zinc-300 bg-white px-3 py-2 dark:border-zinc-700 dark:bg-zinc-950; }
</style>
