// REST-клиент API. Все запросы с токеном идут через Authorization: Bearer.

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message)
  }
}

async function request<T>(
  method: string,
  url: string,
  opts: { token?: string | null; body?: unknown; raw?: FormData } = {},
): Promise<T> {
  const headers: Record<string, string> = {}
  if (opts.token) headers['Authorization'] = `Bearer ${opts.token}`
  let body: BodyInit | undefined
  if (opts.raw) {
    body = opts.raw
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }
  const res = await fetch(url, { method, headers, body })
  const text = await res.text()
  let data: unknown = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    /* не-JSON ответ */
  }
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string } })?.error
    throw new ApiError(res.status, err?.code ?? 'HTTP_' + res.status, err?.message ?? res.statusText)
  }
  return data as T
}

export const api = {
  get: <T>(url: string, token?: string | null) => request<T>('GET', url, { token }),
  post: <T>(url: string, body?: unknown, token?: string | null) =>
    request<T>('POST', url, { body, token }),
  patch: <T>(url: string, body?: unknown, token?: string | null) =>
    request<T>('PATCH', url, { body, token }),
  del: <T>(url: string, token?: string | null) => request<T>('DELETE', url, { token }),
  upload: <T>(url: string, file: File, token: string) => {
    const fd = new FormData()
    fd.append('file', file)
    return request<T>('POST', url, { raw: fd, token })
  },
}

// Игровая команда с идемпотентным action_id и проверкой версии состояния.
export async function sendCommand(
  roomId: string,
  hostToken: string,
  commandType: string,
  payload: Record<string, unknown> = {},
  expectedVersion?: number,
): Promise<{ version: number; duplicated?: boolean }> {
  return api.post(`/api/rooms/${roomId}/actions`, {
    action_id: crypto.randomUUID(),
    command_type: commandType,
    payload,
    expected_state_version: expectedVersion ?? null,
  }, hostToken)
}
