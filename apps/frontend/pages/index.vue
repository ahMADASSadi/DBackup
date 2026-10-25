<template>
  <section>
    <header class="mb-6">
      <h1 class="text-xl font-semibold">Dashboard</h1>
      <p class="text-sm text-zinc-500">Backup health for this instance.</p>
    </header>
    <p v-if="error" class="mb-4 rounded-md bg-red-50 px-3 py-2 text-sm text-red-800 dark:bg-red-950 dark:text-red-200" role="alert">{{ error }}</p>
    <p v-if="pending" class="text-sm text-zinc-500">Loading…</p>
    <template v-else-if="data">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <article v-for="card in cards" :key="card.label" class="rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900">
          <p class="text-xs uppercase tracking-wide text-zinc-500">{{ card.label }}</p>
          <p class="mt-2 text-2xl font-semibold tabular-nums">{{ card.value }}</p>
        </article>
      </div>
      <div class="mt-6 grid gap-6 lg:grid-cols-2">
        <section>
          <h2 class="mb-2 text-sm font-medium">Last success</h2>
          <EmptyState v-if="!data.last_success.length" message="No completed backups yet." />
          <ul v-else class="divide-y divide-zinc-200 rounded-lg border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900">
            <li v-for="item in data.last_success" :key="item.source_id" class="flex items-center justify-between px-3 py-2 text-sm">
              <span>{{ item.source_name }}</span>
              <span class="text-zinc-500">{{ item.completed_at ? formatTime(item.completed_at) : 'None' }}</span>
            </li>
          </ul>
        </section>
        <section>
          <h2 class="mb-2 text-sm font-medium">Next scheduled run</h2>
          <EmptyState v-if="!data.next_runs.length" message="No schedules are enabled." />
          <ul v-else class="divide-y divide-zinc-200 rounded-lg border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900">
            <li v-for="item in data.next_runs" :key="item.source_id" class="flex items-center justify-between px-3 py-2 text-sm">
              <span>{{ item.source_name }}</span>
              <span class="text-zinc-500">{{ formatTime(item.next_run_at) }}</span>
            </li>
          </ul>
        </section>
      </div>
      <section class="mt-6">
        <h2 class="mb-2 text-sm font-medium">Recent operations</h2>
        <EmptyState v-if="!data.recent.length" message="No backup operations yet." />
        <div v-else class="overflow-x-auto rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
          <table class="w-full text-left text-sm">
            <thead class="text-xs uppercase text-zinc-500"><tr><th class="px-3 py-2">Source</th><th class="px-3 py-2">Status</th><th class="px-3 py-2">When</th></tr></thead>
            <tbody>
              <tr v-for="item in data.recent" :key="item.id" class="border-t border-zinc-200 dark:border-zinc-800">
                <td class="px-3 py-2">{{ item.source_name }}</td>
                <td class="px-3 py-2"><StatusPill :status="item.status" /></td>
                <td class="px-3 py-2 text-zinc-500">{{ formatTime(item.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </section>
</template>

<script setup lang="ts">
import type { Dashboard } from '~/types/api'
import { formatBytes, formatTime } from '~/utils/format'

const api = useApi()
const data = ref<Dashboard | null>(null)
const error = ref('')
const pending = ref(true)
let timer: ReturnType<typeof setInterval> | undefined

const cards = computed(() => {
  if (!data.value) return []
  return [
    { label: 'Sources', value: String(data.value.sources) },
    { label: 'Local storage', value: formatBytes(data.value.usage_bytes) },
    { label: 'Failed jobs', value: String(data.value.failed_jobs) },
    { label: 'Pending uploads', value: String(data.value.pending_uploads) },
  ]
})

async function load() {
  try {
    data.value = await api.get<Dashboard>('/dashboard')
    error.value = ''
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load the dashboard'
  } finally {
    pending.value = false
  }
}

onMounted(() => {
  load()
  timer = setInterval(load, 4000)
})
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>
