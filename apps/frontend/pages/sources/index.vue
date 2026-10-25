<template>
  <section>
    <header class="mb-6 flex items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold">Sources</h1>
        <p class="text-sm text-zinc-500">Databases this panel is allowed to back up.</p>
      </div>
      <NuxtLink to="/sources/new" class="rounded-md bg-emerald-700 px-3 py-2 text-sm text-white">Add source</NuxtLink>
    </header>
    <p v-if="error" class="mb-4 rounded-md bg-red-50 px-3 py-2 text-sm text-red-800 dark:bg-red-950 dark:text-red-200" role="alert">{{ error }}</p>
    <p v-if="pending" class="text-sm text-zinc-500">Loading…</p>
    <EmptyState v-else-if="!sources.length" message="No database sources yet." />
    <div v-else class="overflow-x-auto rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <table class="w-full text-left text-sm">
        <thead class="text-xs uppercase text-zinc-500">
          <tr><th class="px-3 py-2">Name</th><th class="px-3 py-2">Type</th><th class="px-3 py-2">State</th><th class="px-3 py-2">Next run</th><th class="px-3 py-2">Actions</th></tr>
        </thead>
        <tbody>
          <tr v-for="source in sources" :key="source.id" class="border-t border-zinc-200 dark:border-zinc-800">
            <td class="px-3 py-2"><NuxtLink :to="`/sources/${source.id}`" class="font-medium">{{ source.name }}</NuxtLink></td>
            <td class="px-3 py-2 uppercase">{{ source.type }}</td>
            <td class="px-3 py-2">{{ source.enabled ? 'Enabled' : 'Disabled' }}</td>
            <td class="px-3 py-2 text-zinc-500">{{ source.schedule.enabled ? formatTime(source.schedule.next_run_at) : '—' }}</td>
            <td class="px-3 py-2">
              <button type="button" class="mr-2 text-emerald-700 dark:text-emerald-400" @click="backup(source.id)">Backup</button>
              <button type="button" class="text-zinc-600 dark:text-zinc-300" @click="test(source.id)">Test</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-if="notice" class="mt-4 text-sm text-zinc-600 dark:text-zinc-300" role="status">{{ notice }}</p>
  </section>
</template>

<script setup lang="ts">
import type { Source } from '~/types/api'
import { formatTime } from '~/utils/format'

const api = useApi()
const sources = ref<Source[]>([])
const pending = ref(true)
const error = ref('')
const notice = ref('')

async function load() {
  try {
    const body = await api.get<{ items: Source[] }>('/sources')
    sources.value = body.items
    error.value = ''
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load sources'
  } finally {
    pending.value = false
  }
}

async function backup(id: string) {
  notice.value = ''
  try {
    await api.post(`/sources/${id}/backups`, {})
    notice.value = 'Backup queued.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Backup failed'
  }
}

async function test(id: string) {
  notice.value = ''
  try {
    await api.post(`/sources/${id}/test`, {})
    notice.value = 'Connection succeeded.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Connection failed'
  }
}

onMounted(load)
</script>
