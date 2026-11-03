interface SessionResponse {
  authenticated: boolean
  username?: string
  csrf_token?: string
  encryption_unlocked?: boolean
}

export function useAuth() {
  const user = useState<string | null>('auth-user', () => null)
  const csrf = useState<string>('auth-csrf', () => '')
  const unlocked = useState('auth-unlocked', () => false)
  const ready = useState('auth-ready', () => false)
  const api = useApi()

  async function refresh() {
    const session = await api.get<SessionResponse>('/auth/session')
    user.value = session.authenticated ? session.username || 'admin' : null
    csrf.value = session.csrf_token || ''
    unlocked.value = Boolean(session.encryption_unlocked)
    ready.value = true
  }

  async function login(username: string, password: string) {
    const session = await api.post<SessionResponse>('/auth/login', { username, password })
    user.value = session.username || username
    csrf.value = session.csrf_token || ''
    unlocked.value = session.encryption_unlocked !== false
    ready.value = true
  }

  async function unlock(password: string) {
    const session = await api.post<SessionResponse>('/auth/unlock', { password })
    unlocked.value = Boolean(session.encryption_unlocked)
  }

  async function logout() {
    await api.post<void>('/auth/logout', {})
    user.value = null
    csrf.value = ''
    unlocked.value = false
  }

  return { user, csrf, unlocked, ready, refresh, login, logout, unlock }
}
