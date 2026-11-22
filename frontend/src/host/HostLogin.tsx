import { useState } from 'react'

import { api } from '../lib/api'
import { setHostToken, setLastHostRoom } from '../lib/session'
import { IconPhone } from '../components/Icon'

// Вход ведущего по коду комнаты. Токен сохраняется в localStorage,
// сессия переживает обновление страницы и перезапуск браузера.
export default function HostLogin(props: { onJoined: (roomId: string) => void }) {
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function join() {
    setBusy(true)
    setError(null)
    try {
      const res = await api.post<{ roomId: string; hostToken: string }>('/api/rooms/join', {
        code: code.trim(),
      })
      setHostToken(res.roomId, res.hostToken)
      setLastHostRoom(res.roomId)
      props.onJoined(res.roomId)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось подключиться')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="host-login">
      <span style={{ color: 'var(--gold)' }}>
        <IconPhone width={56} height={56} />
      </span>
      <div className="host-login__title">Пульт ведущего</div>
      <p style={{ color: 'var(--text-dim)', textAlign: 'center' }}>
        Введите код комнаты с экрана телевизора
      </p>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          join()
        }}
        style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem', width: '100%' }}
      >
        <input
          className="field host-login__input"
          maxLength={6}
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          placeholder="—————"
          autoCapitalize="characters"
          autoComplete="off"
        />
        <button className="btn btn--gold btn--lg" type="submit" disabled={busy || code.trim().length < 4}>
          Подключиться
        </button>
      </form>
      {error && <p style={{ color: 'var(--danger)' }}>{error}</p>}
    </div>
  )
}
