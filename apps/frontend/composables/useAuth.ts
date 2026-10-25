interface SessionResponse {
  authenticated: boolean
  username?: string
  csrf_token?: string
}

export function useAuth() {
  const user = useState<string | null>('auth-user', () => null)
  const csrf = useState<string>('auth-csrf', () => '')
  const ready = useState('auth-ready', () => false)
  const api = useApi()

  async function refresh() {
    const session = await api.get<SessionResponse>('/auth/session')
    user.value = session.authenticated ? session.username || 'admin' : null
    csrf.value = session.csrf_token || ''
    ready.value = true
  }

  async function login(username: string, password: string) {
    const session = await api.post<SessionResponse>('/auth/login', { username, password })
    user.value = session.username || username
    csrf.value = session.csrf_token || ''
    ready.value = true
  }

  async function logout() {
    await api.post<void>('/auth/logout', {})
    user.value = null
    csrf.value = ''
  }

  return { user, csrf, ready, refresh, login, logout }
}
