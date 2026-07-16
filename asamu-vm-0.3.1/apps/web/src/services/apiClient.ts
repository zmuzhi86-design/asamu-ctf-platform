import { randomUUID } from '../utils/random'

export class ApiError extends Error {
  constructor(public status: number, message: string, public code = 'API_ERROR', public details?: unknown) { super(message) }
}

type Envelope<T> = { success: boolean; data?: T; error?: { code: string; message: string; details?: unknown }; requestId: string }
type RequestOptions = RequestInit & { skipAuthRefresh?: boolean }
export type DownloadedFile = { blob: Blob; filename?: string }

const API_BASE = import.meta.env.VITE_API_BASE || '/api/v1'
let accessToken: string | undefined
let refreshPromise: Promise<boolean> | null = null

export const mockApiEnabled = import.meta.env.DEV && import.meta.env.VITE_ENABLE_MOCK_API === 'true'
export function getAccessToken() { return accessToken }
export function setAccessToken(token?: string) { accessToken = token }

function normalizeResponseData<T>(data: T): T {
  if (data && typeof data === 'object' && 'items' in data && (data as { items?: unknown }).items == null) {
    return { ...data, items: [] } as T
  }
  return data
}

async function refreshAccessToken() {
  if (!refreshPromise) refreshPromise = fetch(`${API_BASE}/auth/refresh`, { method: 'POST', credentials: 'include' })
    .then(async (response) => {
      if (!response.ok) return false
      const body = await response.json() as Envelope<{ accessToken: string }>
      if (!body.success || !body.data?.accessToken) return false
      setAccessToken(body.data.accessToken)
      return true
    })
    .catch(() => false)
    .finally(() => { refreshPromise = null })
  return refreshPromise
}

export async function apiRequest<T>(path: string, init: RequestOptions = {}): Promise<T> {
  const headers = new Headers(init.headers)
  const token = getAccessToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include' })
  const refreshableAuthPath = path === '/auth/me'
  if (response.status === 401 && !init.skipAuthRefresh && (!path.startsWith('/auth/') || refreshableAuthPath)) {
    if (await refreshAccessToken()) return apiRequest<T>(path, { ...init, skipAuthRefresh: true })
  }
  if (response.status === 204) return undefined as T
  const body = await response.json().catch(() => ({ success: false, error: { code: 'INVALID_RESPONSE', message: response.statusText } })) as Envelope<T>
  if (!response.ok || !body.success) throw new ApiError(response.status, body.error?.message ?? '请求失败', body.error?.code, body.error?.details)
  return normalizeResponseData(body.data as T)
}

function apiURL(path: string) {
  if (path.startsWith(`${API_BASE}/`) || path === API_BASE) return path
  return `${API_BASE}${path.startsWith('/') ? path : `/${path}`}`
}

function responseFilename(response: Response) {
  const disposition = response.headers.get('Content-Disposition') || ''
  const encoded = disposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  if (encoded) {
    try { return decodeURIComponent(encoded) } catch { return encoded }
  }
  return disposition.match(/filename="([^"]+)"/i)?.[1] || disposition.match(/filename=([^;]+)/i)?.[1]?.trim()
}

/** Download a protected API file with the same token refresh behavior as JSON requests. */
export async function apiDownload(path: string, init: RequestOptions = {}): Promise<DownloadedFile> {
  const headers = new Headers(init.headers)
  const token = getAccessToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(apiURL(path), { ...init, headers, credentials: 'include' })
  if (response.status === 401 && !init.skipAuthRefresh && await refreshAccessToken()) {
    return apiDownload(path, { ...init, skipAuthRefresh: true })
  }
  if (!response.ok) {
    const body = await response.clone().json().catch(() => undefined) as Envelope<unknown> | undefined
    throw new ApiError(response.status, body?.error?.message || response.statusText || '文件下载失败', body?.error?.code, body?.error?.details)
  }
  return { blob: await response.blob(), filename: responseFilename(response) }
}

export function createIdempotencyKey(operation: string) { return `${operation}-${randomUUID()}` }
