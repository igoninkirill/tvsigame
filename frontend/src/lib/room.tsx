// React-контекст комнаты: владеет WebSocket-подключением и текущим снапшотом.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'

import { RoomConnection } from './connection'
import { sendCommand } from './api'
import { getClientId } from './session'
import type { ConnectionStatus, Snapshot } from './types'

interface RoomContextValue {
  snapshot: Snapshot | null
  status: ConnectionStatus
  authError: string | null
  signals: { kind: string; at: number } | null
  /** Отправить игровую команду (только для ведущего). Бросает ApiError. */
  send: (type: string, payload?: Record<string, unknown>) => Promise<void>
}

const RoomContext = createContext<RoomContextValue | null>(null)

export function RoomProvider(props: {
  roomId: string
  role: 'display' | 'host'
  token: string
  onAuthError?: (message: string) => void
  children: ReactNode
}) {
  const { roomId, role, token, onAuthError, children } = props
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [status, setStatus] = useState<ConnectionStatus>('connecting')
  const [authError, setAuthError] = useState<string | null>(null)
  const [signals, setSignals] = useState<{ kind: string; at: number } | null>(null)
  const versionRef = useRef(0)

  useEffect(() => {
    const conn = new RoomConnection(
      roomId,
      { role, token, clientId: role === 'host' ? getClientId() : undefined },
      {
        onState: (s) => {
          versionRef.current = s.version
          setSnapshot(s)
        },
        onStatus: setStatus,
        onAuthError: (m) => {
          setAuthError(m)
          onAuthError?.(m)
        },
        onSignal: (sig) => setSignals({ kind: sig.kind, at: Date.now() }),
      },
    )
    conn.start()
    return () => conn.stop()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [roomId, role, token])

  const send = useCallback(
    async (type: string, payload: Record<string, unknown> = {}) => {
      await sendCommand(roomId, token, type, payload, versionRef.current)
    },
    [roomId, token],
  )

  return (
    <RoomContext.Provider value={{ snapshot, status, authError, signals, send }}>
      {children}
    </RoomContext.Provider>
  )
}

export function useRoom(): RoomContextValue {
  const ctx = useContext(RoomContext)
  if (!ctx) throw new Error('useRoom вне RoomProvider')
  return ctx
}
