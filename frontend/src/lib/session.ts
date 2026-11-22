// Хранение сессий в localStorage: токены не попадают в URL.
// Ключи: sigame:display:{roomId}, sigame:host:{roomId}, sigame:clientId.

export function getDisplayToken(roomId: string): string | null {
  return localStorage.getItem(`sigame:display:${roomId}`)
}

export function setDisplayToken(roomId: string, token: string) {
  localStorage.setItem(`sigame:display:${roomId}`, token)
}

export function getHostToken(roomId: string): string | null {
  return localStorage.getItem(`sigame:host:${roomId}`)
}

export function setHostToken(roomId: string, token: string) {
  localStorage.setItem(`sigame:host:${roomId}`, token)
}

export function clearHostToken(roomId: string) {
  localStorage.removeItem(`sigame:host:${roomId}`)
}

// Стабильный идентификатор устройства ведущего (для правила «один ведущий»).
export function getClientId(): string {
  let id = localStorage.getItem('sigame:clientId')
  if (!id) {
    id = crypto.randomUUID()
    localStorage.setItem('sigame:clientId', id)
  }
  return id
}

// Последняя комната хоста — для быстрого переподключения со стартовой страницы.
export function setLastHostRoom(roomId: string) {
  localStorage.setItem('sigame:lastHostRoom', roomId)
}

export function getLastHostRoom(): string | null {
  return localStorage.getItem('sigame:lastHostRoom')
}
