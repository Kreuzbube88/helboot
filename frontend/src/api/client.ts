// Minimal typed API client. All backend communication goes through this
// module (ADR-0010): session cookie + CSRF header handling live here.

export class ApiError extends Error {
  /** Stable error code from the backend, used as the i18n key. */
  readonly code: string
  readonly status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.code = code
    this.status = status
  }

  /** i18n key for this error, e.g. "errors.auth.invalid_credentials". */
  get i18nKey(): string {
    return `errors.${this.code}`
  }
}

let csrfToken = ''

/** Remember the CSRF token returned by login / auth/me. */
export function setCsrfToken(token: string) {
  csrfToken = token
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET' && csrfToken) headers['X-CSRF-Token'] = csrfToken

  const resp = await fetch(`/api/v1${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: 'same-origin',
  })

  if (resp.status === 204) return undefined as T

  const data = await resp.json().catch(() => null)
  if (!resp.ok) {
    const code = data?.error?.code ?? 'internal'
    const message = data?.error?.message ?? resp.statusText
    throw new ApiError(resp.status, code, message)
  }
  return data as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  delete: <T = void>(path: string) => request<T>('DELETE', path),
}
