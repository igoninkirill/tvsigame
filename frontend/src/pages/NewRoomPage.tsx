import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { api } from '../lib/api'
import { setDisplayToken } from '../lib/session'

// Создание комнаты с ТВ: запрашиваем комнату и сразу переходим на экран табло.
export default function NewRoomPage() {
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return
    started.current = true
    api
      .post<{ roomId: string; displayToken: string }>('/api/rooms')
      .then((res) => {
        setDisplayToken(res.roomId, res.displayToken)
        navigate(`/room/${res.roomId}/display`, { replace: true })
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Ошибка создания комнаты'))
  }, [navigate])

  return (
    <div className="home-root">
      <h1 className="home-title">Своя Игра</h1>
      <p className="home-subtitle">{error ?? 'Создаём комнату…'}</p>
      {error && (
        <button className="btn btn--gold" onClick={() => location.reload()}>
          Попробовать снова
        </button>
      )}
    </div>
  )
}
