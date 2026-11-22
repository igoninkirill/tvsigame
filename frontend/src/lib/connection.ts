// WebSocket-подключение к комнате с автопереподключением и heartbeat.
//
// Аутентификация — первым сообщением после установки соединения
// (токены не передаются в URL, см. backend/internal/ws).

import type { ConnectionStatus, Snapshot } from './types'

interface AuthMessage {
  role: 'display' | 'host'
  token: string
  clientId?: string
}

interface Handlers {
  onState: (s: Snapshot) => void
  onStatus: (s: ConnectionStatus) => void
  onAuthError?: (message: string) => void
  onSignal?: (payload: { kind: string }) => void
}

export class RoomConnection {
  private ws: WebSocket | null = null
  private closed = false
  private reconnectTimer: number | null = null
  private pingTimer: number | null = null
  private attempts = 0

  constructor(
    private roomId: string,
    private authMsg: AuthMessage,
    private handlers: Handlers,
  ) {}

  start() {
    this.closed = false
    this.connect()
  }

  stop() {
    this.closed = true
    if (this.reconnectTimer !== null) window.clearTimeout(this.reconnectTimer)
    if (this.pingTimer !== null) window.clearInterval(this.pingTimer)
    this.ws?.close()
    this.ws = null
  }

  private connect() {
    if (this.closed) return
    this.handlers.onStatus('connecting')
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${location.host}/ws/room/${this.roomId}`)
    this.ws = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'auth', ...this.authMsg }))
    }

    ws.onmessage = (ev) => {
      let msg: { type?: string; message?: string; kind?: string } & Record<string, unknown>
      try {
        msg = JSON.parse(ev.data as string)
      } catch {
        return
      }
      switch (msg.type) {
        case 'auth_ok':
          this.attempts = 0
          this.handlers.onStatus('online')
          this.startPing()
          break
        case 'auth_error':
          this.handlers.onAuthError?.(msg.message ?? 'ошибка авторизации')
          this.closed = true
          ws.close()
          break
        case 'state':
          this.handlers.onState(msg as unknown as Snapshot)
          break
        case 'signal':
          this.handlers.onSignal?.({ kind: String(msg.kind ?? '') })
          break
        case 'pong':
          break
      }
    }

    ws.onclose = () => {
      this.stopPing()
      if (this.closed) return
      this.handlers.onStatus('offline')
      // Экспоненциальный backoff: 0.5с, 1с, 2с, ... до 10с.
      const delay = Math.min(10000, 500 * 2 ** this.attempts)
      this.attempts++
      this.reconnectTimer = window.setTimeout(() => this.connect(), delay)
    }

    ws.onerror = () => ws.close()
  }

  private startPing() {
    this.stopPing()
    this.pingTimer = window.setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }))
      }
    }, 5000)
  }

  private stopPing() {
    if (this.pingTimer !== null) {
      window.clearInterval(this.pingTimer)
      this.pingTimer = null
    }
  }
}
