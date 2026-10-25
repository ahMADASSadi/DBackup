import type { ApiError } from '~/types/api'

export class ApiRequestError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export function useApi() {
  const config = useRuntimeConfig()

  async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
    const csrf = useState<string>('auth-csrf', () => '')
    const headers: Record<string, string> = {}
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    if (method !== 'GET' && method !== 'HEAD' && csrf.value) headers['X-CSRF-Token'] = csrf.value
    const response = await fetch(`${config.public.apiBase}${path}`, {
      method,
      credentials: 'include',
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    if (response.status === 204) return undefined as T
    const data = await response.json().catch(() => ({} as ApiError))
    if (!response.ok) {
      const payload = data as ApiError
      throw new ApiRequestError(response.status, payload.error?.code || 'request_failed', payload.error?.message || 'Request failed')
    }
    return data as T
  }

  return {
    get: <T>(path: string) => request<T>(path),
    post: <T>(path: string, body?: unknown) => request<T>(path, 'POST', body ?? {}),
    put: <T>(path: string, body?: unknown) => request<T>(path, 'PUT', body ?? {}),
    del: (path: string) => request<void>(path, 'DELETE'),
  }
}
