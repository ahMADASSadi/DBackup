<template>
  <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium" :class="styles">{{ status }}</span>
</template>

<script setup lang="ts">
const props = defineProps<{ status: string }>()
const styles = computed(() => {
  switch (props.status) {
    case 'completed': return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300'
    case 'running':
    case 'pending': return 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300'
    case 'failed': return 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300'
    default: return 'bg-zinc-200 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300'
  }
})
</script>
