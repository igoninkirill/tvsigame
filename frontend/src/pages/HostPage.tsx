import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { RoomProvider } from '../lib/room'
import { getHostToken, clearHostToken } from '../lib/session'
import HostLogin from '../host/HostLogin'
import HostDashboard from '../host/HostDashboard'
import { ToastProvider } from '../host/common'

export default function HostPage() {
  const { roomId: paramRoomId } = useParams()
  const navigate = useNavigate()
  // Комната может прийти из URL (переход после входа) или из восстановления сессии.
  const [roomId, setRoomId] = useState(paramRoomId ?? '')
  const [token, setToken] = useState(() => (paramRoomId ? getHostToken(paramRoomId) : null))
  const [authError, setAuthError] = useState<string | null>(null)

  if (!roomId || !token) {
    return (
      <HostLogin
        onJoined={(newRoomId) => {
          setRoomId(newRoomId)
          setToken(getHostToken(newRoomId))
          navigate(`/room/${newRoomId}/host`, { replace: true })
        }}
      />
    )
  }

  if (authError) {
    return (
      <div className="host-login">
        <div className="host-login__title">Нет доступа</div>
        <p style={{ color: 'var(--text-dim)', textAlign: 'center' }}>{authError}</p>
        <button
          className="btn btn--gold"
          onClick={() => {
            clearHostToken(roomId)
            setToken(null)
            setAuthError(null)
          }}
        >
          Войти по коду заново
        </button>
      </div>
    )
  }

  return (
    <ToastProvider>
      <RoomProvider roomId={roomId} role="host" token={token} onAuthError={setAuthError}>
        <HostDashboard />
      </RoomProvider>
    </ToastProvider>
  )
}
