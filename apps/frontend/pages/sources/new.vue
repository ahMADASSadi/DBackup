<template>
  <section class="max-w-3xl">
    <h1 class="mb-6 text-xl font-semibold">Add source</h1>
    <SourceForm :pending="pending" :error="error" @save="save" />
  </section>
</template>

<script setup lang="ts">
import type { Source } from '~/types/api'

const api = useApi()
const pending = ref(false)
const error = ref('')

async function save(payload: Record<string, unknown>) {
  pending.value = true
  error.value = ''
  try {
    const source = await api.post<Source>('/sources', payload)
    await navigateTo(`/sources/${source.id}`)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not save the source'
  } finally {
    pending.value = false
  }
}
</script>
