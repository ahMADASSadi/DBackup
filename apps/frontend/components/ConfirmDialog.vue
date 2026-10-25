<template>
  <div v-if="open" class="fixed inset-0 z-20 flex items-end justify-center bg-zinc-950/50 p-4 sm:items-center" @keydown.esc="emit('cancel')">
    <div role="dialog" aria-modal="true" :aria-labelledby="titleId" class="w-full max-w-md rounded-lg border border-zinc-200 bg-white p-5 shadow-lg dark:border-zinc-700 dark:bg-zinc-900">
      <h2 :id="titleId" class="text-base font-semibold">{{ title }}</h2>
      <p class="mt-2 text-sm text-zinc-600 dark:text-zinc-300">{{ message }}</p>
      <div class="mt-5 flex justify-end gap-2">
        <button type="button" class="rounded-md px-3 py-2 text-sm" @click="emit('cancel')">Cancel</button>
        <button type="button" class="rounded-md px-3 py-2 text-sm text-white" :class="danger ? 'bg-red-700' : 'bg-emerald-700'" @click="emit('confirm')">{{ confirmLabel }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ open: boolean; title: string; message: string; confirmLabel: string; danger?: boolean }>()
const emit = defineEmits<{ confirm: []; cancel: [] }>()
const titleId = 'confirm-title'
</script>
