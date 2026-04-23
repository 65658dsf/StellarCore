class HTTPError extends Error {
  status: number
  statusText: string

  constructor(status: number, statusText: string, message?: string) {
    super(message || statusText)
    this.status = status
    this.statusText = statusText
  }
}

const defaultOptions: RequestInit = {
  credentials: 'include',
}

function normalizeBody(body: unknown): BodyInit | undefined {
  if (body == null) {
    return undefined
  }
  if (
    typeof body === 'string' ||
    body instanceof FormData ||
    body instanceof URLSearchParams ||
    body instanceof Blob ||
    body instanceof ArrayBuffer
  ) {
    return body as BodyInit
  }
  return JSON.stringify(body)
}

async function ensureOK(response: Response): Promise<Response> {
  if (response.ok) {
    return response
  }

  let message = `HTTP ${response.status}`
  try {
    const text = await response.text()
    if (text) {
      message = text
    }
  } catch {
    // ignore response parsing failures and keep fallback message
  }

  throw new HTTPError(response.status, response.statusText, message)
}

async function requestJSON<T>(url: string, options: RequestInit = {}): Promise<T> {
  const response = await ensureOK(await fetch(url, { ...defaultOptions, ...options }))
  if (response.status === 204) {
    return {} as T
  }
  const text = await response.text()
  if (!text) {
    return {} as T
  }
  return JSON.parse(text) as T
}

async function requestText(url: string, options: RequestInit = {}): Promise<string> {
  const response = await ensureOK(await fetch(url, { ...defaultOptions, ...options }))
  return response.text()
}

export const http = {
  get: <T>(url: string, options?: RequestInit) =>
    requestJSON<T>(url, { ...options, method: 'GET' }),
  getText: (url: string, options?: RequestInit) =>
    requestText(url, { ...options, method: 'GET' }),
  post: <T>(url: string, body?: unknown, options?: RequestInit) =>
    requestJSON<T>(url, {
      ...options,
      method: 'POST',
      headers:
        body == null || typeof body === 'string' || body instanceof FormData
          ? options?.headers
          : { 'Content-Type': 'application/json', ...options?.headers },
      body: normalizeBody(body),
    }),
  put: <T>(url: string, body?: unknown, options?: RequestInit) =>
    requestJSON<T>(url, {
      ...options,
      method: 'PUT',
      headers:
        body == null || typeof body === 'string' || body instanceof FormData
          ? options?.headers
          : { 'Content-Type': 'application/json', ...options?.headers },
      body: normalizeBody(body),
    }),
  delete: <T>(url: string, options?: RequestInit) =>
    requestJSON<T>(url, { ...options, method: 'DELETE' }),
}

export { HTTPError }
