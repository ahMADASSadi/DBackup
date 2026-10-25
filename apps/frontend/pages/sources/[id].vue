<template>
  <section class="max-w-3xl">
    <p v-if="loading" class="text-sm text-zinc-500">Loading…</p>
    <template v-else-if="source">
      <header class="mb-6 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold">{{ source.name }}</h1>
          <p class="text-sm text-zinc-500">Created {{ formatTime(source.created_at) }}</p>
        </div>
        <div class="flex gap-2">
          <button type="button" class="rounded-md bg-emerald-700 px-3 py-2 text-sm text-white" @click="backup">Backup now</button>
          <button type="button" class="rounded-md border border-red-300 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:text-red-300" @click="confirmDelete = true">Delete</button>
        </div>
      </header>
      <p v-if="notice" class="mb-4 text-sm" role="status">{{ notice }}</p>
      <SourceForm :source="source" :pending="pending" :error="error" @save="save" @test="test" />
      <p class="mt-6 text-sm"><NuxtLink :to="`/backups?source_id=${source.id}`" class="text-emerald-700 dark:text-emerald-400">View backup history</NuxtLink></p>
    </template>
    <ConfirmDialog :open="confirmDelete" title="Delete source" message="Delete this source and every retained backup? This cannot be undone." confirm-label="Delete source and backups" danger @cancel="confirmDelete = false" @confirm="remove" />
  </section>
</template>

<script setup lang="ts">
import type { Source } from '~/types/api'
import { formatTime } from '~/utils/format'

const route = useRoute()
const api = useApi()
const source = ref<Source | null>(null)
const loading = ref(true)
const pending = ref(false)
const error = ref('')
const notice = ref('')
const confirmDelete = ref(false)
const id = computed(() => String(route.params.id))

async function load() {
  try {
    source.value = await api.get<Source>(`/sources/${id.value}`)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load the source'
  } finally {
    loading.value = false
  }
}

async function save(payload: Record<string, unknown>) {
  pending.value = true
  error.value = ''
  try {
    source.value = await api.put<Source>(`/sources/${id.value}`, payload)
    notice.value = 'Source saved.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not save the source'
  } finally {
    pending.value = false
  }
}

async function test() {
  notice.value = ''
  try {
    await api.post(`/sources/${id.value}/test`, {})
    notice.value = 'Connection succeeded.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Connection failed'
  }
}

async function backup() {
  notice.value = ''
  try {
    await api.post(`/sources/${id.value}/backups`, {})
    notice.value = 'Backup queued.'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Backup failed'
  }
}

async function remove() {
  confirmDelete.value = false
  try {
    await api.del(`/sources/${id.value}?delete_backups=true`)
    await navigateTo('/sources')
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not delete the source'
  }
}

onMounted(load)
</script>
