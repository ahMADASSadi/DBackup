<template>
  <section>
    <header class="mb-6">
      <h1 class="text-xl font-semibold">Backups</h1>
      <p class="text-sm text-zinc-500">Retained versions and upload state.</p>
    </header>
    <form class="mb-4 grid gap-3 md:grid-cols-4" @submit.prevent="load">
      <label class="text-sm">Source
        <select v-model="filters.source_id" class="field">
          <option value="">All</option>
          <option v-for="source in sources" :key="source.id" :value="source.id">{{ source.name }}</option>
        </select>
      </label>
      <label class="text-sm">Status
        <select v-model="filters.status" class="field">
          <option value="">All</option>
          <option v-for="status in statuses" :key="status" :value="status">{{ status }}</option>
        </select>
      </label>
      <label class="text-sm">From<input v-model="filters.from" type="datetime-local" class="field"></label>
      <label class="text-sm">To<input v-model="filters.to" type="datetime-local" class="field"></label>
      <button type="submit" class="rounded-md border border-zinc-300 px-3 py-2 text-sm dark:border-zinc-700 md:col-span-4 md:w-fit">Apply filters</button>
    </form>
    <p v-if="error" class="mb-4 rounded-md bg-red-50 px-3 py-2 text-sm text-red-800 dark:bg-red-950 dark:text-red-200" role="alert">{{ error }}</p>
    <p v-if="pending" class="text-sm text-zinc-500">Loading…</p>
    <EmptyState v-else-if="!page.items.length" message="No backups match these filters." />
    <div v-else class="overflow-x-auto rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <table class="w-full min-w-[720px] text-left text-sm">
        <thead class="text-xs uppercase text-zinc-500">
          <tr>
            <th class="px-3 py-2">Source</th>
            <th class="px-3 py-2">Status</th>
            <th class="px-3 py-2">Size</th>
            <th class="px-3 py-2">Completed</th>
            <th class="px-3 py-2">Upload</th>
            <th class="px-3 py-2">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in page.items" :key="item.id" class="border-t border-zinc-200 align-top dark:border-zinc-800">
            <td class="px-3 py-2">{{ item.source_name }}<p class="font-mono text-xs text-zinc-500">{{ item.id.slice(0, 8) }}</p></td>
            <td class="px-3 py-2"><StatusPill :status="item.status" /><p v-if="item.error_summary" class="mt-1 max-w-xs text-xs text-red-700 dark:text-red-300">{{ item.error_summary }}</p></td>
            <td class="px-3 py-2 tabular-nums">{{ formatBytes(item.size_bytes) }}</td>
            <td class="px-3 py-2 text-zinc-500">{{ formatTime(item.completed_at || item.created_at) }}</td>
            <td class="px-3 py-2"><StatusPill v-if="item.upload" :status="item.upload.status" /><span v-else class="text-zinc-400">—</span></td>
            <td class="px-3 py-2 space-x-2">
              <a v-if="item.status === 'completed'" :href="`/api/v1/backups/${item.id}/download`" class="text-emerald-700 dark:text-emerald-400">Download</a>
              <button v-if="item.status === 'failed'" type="button" class="text-emerald-700 dark:text-emerald-400" @click="retry(item.id)">Retry</button>
              <button v-if="item.upload?.status === 'failed'" type="button" class="text-emerald-700 dark:text-emerald-400" @click="retryUpload(item.id)">Retry upload</button>
              <button type="button" class="text-red-700 dark:text-red-300" @click="askDelete(item.id, item.status === 'completed')">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="page.total > page.limit" class="mt-4 flex gap-2 text-sm">
      <button type="button" class="rounded border border-zinc-300 px-2 py-1 dark:border-zinc-700" :disabled="page.offset === 0" @click="shift(-page.limit)">Previous</button>
      <button type="button" class="rounded border border-zinc-300 px-2 py-1 dark:border-zinc-700" :disabled="page.offset + page.limit >= page.total" @click="shift(page.limit)">Next</button>
    </div>
    <ConfirmDialog :open="Boolean(deleteId)" title="Delete backup" :message="deleteMessage" confirm-label="Delete backup" danger @cancel="deleteId = ''" @confirm="remove" />
  </section>
</template>

<script setup lang="ts">
import type { BackupPage, Source } from '~/types/api'
import { formatBytes, formatTime } from '~/utils/format'

const route = useRoute()
const api = useApi()
const statuses = ['pending', 'running', 'completed', 'failed', 'expired']
const sources = ref<Source[]>([])
const page = ref<BackupPage>({ items: [], total: 0, limit: 50, offset: 0 })
const filters = reactive({ source_id: String(route.query.source_id || ''), status: '', from: '', to: '' })
const pending = ref(true)
const error = ref('')
const deleteId = ref('')
const deleteCompleted = ref(false)
const deleteMessage = computed(() => deleteCompleted.value ? 'This may be the only completed backup. Deletion is refused unless it is safe or you confirm the last copy.' : 'Delete this backup record?')
let timer: ReturnType<typeof setInterval> | undefined

function query() {
  const params = new URLSearchParams()
  if (filters.source_id) params.set('source_id', filters.source_id)
  if (filters.status) params.set('status', filters.status)
  if (filters.from) params.set('from', new Date(filters.from).toISOString())
  if (filters.to) params.set('to', new Date(filters.to).toISOString())
  params.set('limit', String(page.value.limit))
  params.set('offset', String(page.value.offset))
  return `/backups?${params.toString()}`
}

async function load() {
  try {
    page.value = await api.get<BackupPage>(query())
    error.value = ''
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load backups'
  } finally {
    pending.value = false
  }
}

function shift(delta: number) {
  page.value.offset = Math.max(0, page.value.offset + delta)
  load()
}

async function retry(id: string) {
  try { await api.post(`/backups/${id}/retry`, {}); await load() } catch (err) { error.value = err instanceof Error ? err.message : 'Retry failed' }
}
async function retryUpload(id: string) {
  try { await api.post(`/backups/${id}/retry-upload`, {}); await load() } catch (err) { error.value = err instanceof Error ? err.message : 'Upload retry failed' }
}
function askDelete(id: string, completed: boolean) {
  deleteId.value = id
  deleteCompleted.value = completed
}
async function remove() {
  const id = deleteId.value
  deleteId.value = ''
  try {
    await api.del(`/backups/${id}${deleteCompleted.value ? '?force=true' : ''}`)
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Delete failed'
  }
}

onMounted(async () => {
  try {
    const body = await api.get<{ items: Source[] }>('/sources')
    sources.value = body.items
  } catch { sources.value = [] }
  await load()
  timer = setInterval(() => {
    if (page.value.items.some(item => item.status === 'pending' || item.status === 'running' || item.upload?.status === 'pending' || item.upload?.status === 'running')) load()
  }, 3000)
})
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.field { @apply mt-1 w-full rounded-md border border-zinc-300 bg-white px-3 py-2 dark:border-zinc-700 dark:bg-zinc-950; }
</style>
