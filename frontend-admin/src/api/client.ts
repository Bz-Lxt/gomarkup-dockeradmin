import type { ApiError } from './types'

export class ApiRequestError extends Error {
  code: string
  status: number
  details?: { field: string; message: string }[]

  constructor(status: number, err: ApiError) {
    super(err.message)
    this.status = status
    this.code = err.code
    this.details = err.details
  }
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  let resp: Response
  try {
    resp = await fetch(url, {
      method,
      headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
  } catch {
    throw new ApiRequestError(0, { code: 'network_error', message: '无法连接后端服务' })
  }

  if (resp.status === 204) return undefined as T

  let payload: unknown
  try {
    payload = await resp.json()
  } catch {
    throw new ApiRequestError(resp.status, { code: 'bad_response', message: '响应不是合法 JSON' })
  }

  if (!resp.ok) {
    const err = (payload as { error?: ApiError })?.error
    throw new ApiRequestError(resp.status, err ?? { code: 'unknown', message: `HTTP ${resp.status}` })
  }
  return (payload as { data: T }).data
}

export const api = {
  get: <T>(url: string) => request<T>('GET', url),
  post: <T>(url: string, body?: unknown) => request<T>('POST', url, body),
  put: <T>(url: string, body?: unknown) => request<T>('PUT', url, body),
  del: <T>(url: string) => request<T>('DELETE', url),
}

export function openStream(path: string, onMessage: (data: unknown) => void, onError?: () => void): () => void {
  const es = new EventSource(path)
  es.onmessage = (ev) => {
    try {
      onMessage(JSON.parse(ev.data))
    } catch {
      /* 忽略单帧解析失败 */
    }
  }
  es.onerror = () => onError?.()
  return () => es.close()
}
