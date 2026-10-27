<template>
  <div class="min-h-screen md:grid md:grid-cols-[15rem_1fr]">
    <aside
      class="border-b border-zinc-800 bg-zinc-950 text-zinc-100 md:min-h-screen md:border-b-0 md:border-r"
    >
      <div class="flex items-center justify-between px-4 py-4">
        <NuxtLink to="/" class="text-sm font-semibold tracking-wide"
          >DBackup</NuxtLink
        >
        <div class="flex items-center gap-2">
          <select
            v-if="colors.schemes.value.length"
            :value="colors.scheme.value"
            class="max-w-[7rem] bg-transparent text-xs text-zinc-400"
            aria-label="Primary color"
            @change="colors.apply(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="item in colors.schemes.value"
              :key="item.id"
              :value="item.id"
              class="text-zinc-900"
            >
              {{ item.label }}
            </option>
          </select>
          <button
            type="button"
            class="text-xs text-zinc-400 hover:text-white"
            @click="theme.toggle()"
          >
            {{ theme.theme.value === 'dark' ? 'Light' : 'Dark' }}
          </button>
        </div>
      </div>
      <nav
        class="flex gap-1 overflow-x-auto px-3 pb-3 md:block md:space-y-1 md:px-3"
        aria-label="Primary"
      >
        <NuxtLink
          v-for="item in items"
          :key="item.to"
          :to="item.to"
          class="block whitespace-nowrap rounded-md px-3 py-2 text-sm text-zinc-300 hover:bg-zinc-800 hover:text-white"
          :class="active(item.to) ? 'bg-zinc-800 text-white' : ''"
        >
          {{ item.label }}
        </NuxtLink>
      </nav>
      <div class="hidden px-4 py-4 md:block">
        <p class="text-xs text-zinc-500">{{ auth.user.value }}</p>
        <button
          type="button"
          class="mt-2 text-sm text-zinc-300 hover:text-white"
          @click="signOut"
        >
          Sign out
        </button>
      </div>
    </aside>
    <div class="min-w-0">
      <div
        class="flex items-center justify-between border-b border-zinc-200 px-4 py-3 dark:border-zinc-800 md:hidden"
      >
        <p class="text-xs text-zinc-500">{{ auth.user.value }}</p>
        <button type="button" class="text-sm" @click="signOut">Sign out</button>
      </div>
      <main class="mx-auto max-w-6xl px-4 py-6 md:px-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
const auth = useAuth()
const theme = useTheme()
const colors = useColorScheme()
const route = useRoute()
const items = [
  { to: '/', label: 'Dashboard' },
  { to: '/sources', label: 'Sources' },
  { to: '/backups', label: 'Backups' },
  { to: '/settings', label: 'Settings' },
]

function active(path: string) {
  if (path === '/') return route.path === '/'
  return route.path === path || route.path.startsWith(path + '/')
}

async function signOut() {
  await auth.logout()
  await navigateTo('/login')
}
</script>
