<template>
  <div class="rounded-lg border border-dashed border-zinc-300 px-4 py-10 text-center text-sm text-zinc-500 dark:border-zinc-700">
    <p>{{ message }}</p>
    <slot />
  </div>
</template>

<script setup lang="ts">
defineProps<{ message: string }>()
</script>
