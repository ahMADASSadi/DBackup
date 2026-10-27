<template>
  <div class="mx-auto flex min-h-screen max-w-md flex-col justify-center px-4">
    <div class="rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
    <p class="text-xs font-medium uppercase tracking-[0.16em] text-emerald-700 dark:text-emerald-400">DBackup</p>
    <h1 class="mt-2 text-2xl font-semibold">Sign in</h1>
    <p class="mt-2 text-sm text-zinc-600 dark:text-zinc-400">One administrator, provisioned by deployment configuration.</p>
    <form class="mt-6 space-y-4" @submit.prevent="submit">
      <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-800 dark:bg-red-950 dark:text-red-200" role="alert">{{ error }}</p>
      <label class="block text-sm">Username
        <input v-model="username" required autocomplete="username" class="mt-1 w-full rounded-md border border-zinc-300 bg-white px-3 py-2 dark:border-zinc-700 dark:bg-zinc-950">
      </label>
      <label class="block text-sm">Password
        <input v-model="password" required type="password" autocomplete="current-password" class="mt-1 w-full rounded-md border border-zinc-300 bg-white px-3 py-2 dark:border-zinc-700 dark:bg-zinc-950">
      </label>
      <button type="submit" class="w-full rounded-md bg-emerald-700 px-3 py-2 text-sm text-white disabled:opacity-60" :disabled="pending">{{ pending ? 'Signing in…' : 'Sign in' }}</button>
    </form>
    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const auth = useAuth()
const username = ref('')
const password = ref('')
const error = ref('')
const pending = ref(false)

async function submit() {
  error.value = ''
  pending.value = true
  try {
    await auth.login(username.value, password.value)
    await navigateTo('/')
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Sign-in failed'
  } finally {
    pending.value = false
  }
}
</script>
