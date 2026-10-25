export default defineNuxtRouteMiddleware(async (to) => {
  const auth = useAuth()
  if (!auth.ready.value) {
    try {
      await auth.refresh()
    } catch {
      auth.user.value = null
      auth.ready.value = true
    }
  }
  if (to.path === '/login') {
    if (auth.user.value) return navigateTo('/')
    return
  }
  if (!auth.user.value) return navigateTo('/login')
})
