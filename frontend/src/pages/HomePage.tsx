import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import { api } from '../lib/api'
import { setHostToken, setLastHostRoom, getLastHostRoom } from '../lib/session'
import { IconPhone, IconTv } from '../components/Icon'

export default function HomePage() {
  const navigate = useNavigate()
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const lastRoom = getLastHostRoom()

  async function joinAsHost() {
    if (!code.trim()) return
    setBusy(true)
    setError(null)
    try {
      const res = await api.post<{ roomId: string; hostToken: string }>('/api/rooms/join', {
        code: code.trim(),
      })
      setHostToken(res.roomId, res.hostToken)
      setLastHostRoom(res.roomId)
      navigate(`/room/${res.roomId}/host`)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось подключиться')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="home-root">
      <h1 className="home-title">Своя Игра</h1>
      <p className="home-subtitle">
        Телевизионная студия у вас дома: телевизор — игровое табло, смартфон ведущего — пульт
        управления.
      </p>

      <div className="home-cards">
        <Link className="home-card" to="/room/new">
          <span className="home-card__icon"><IconTv width={32} height={32} /></span>
          <span>
            <span className="home-card__title">Создать игру на ТВ</span>
            <br />
            <span className="home-card__sub">Откройте на телевизоре — появится код комнаты</span>
          </span>
        </Link>

        <div className="home-card" style={{ flexDirection: 'column', alignItems: 'stretch' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <span className="home-card__icon"><IconPhone width={32} height={32} /></span>
            <span>
              <span className="home-card__title">Я ведущий</span>
              <br />
              <span className="home-card__sub">Введите код с экрана телевизора</span>
            </span>
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              joinAsHost()
            }}
            style={{ display: 'flex', gap: '0.6rem', marginTop: '0.9rem' }}
          >
            <input
              className="field"
              style={{ textTransform: 'uppercase', letterSpacing: '0.2em', fontWeight: 700 }}
              placeholder="КОД"
              maxLength={6}
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase())}
              autoCapitalize="characters"
            />
            <button className="btn btn--gold" type="submit" disabled={busy || code.trim().length < 4}>
              Войти
            </button>
          </form>
          {error && <p style={{ color: 'var(--danger)', marginTop: '0.6rem' }}>{error}</p>}
          {lastRoom && (
            <Link to={`/room/${lastRoom}/host`} style={{ marginTop: '0.8rem', color: 'var(--gold-soft)', fontSize: '0.9rem' }}>
              Вернуться к последней игре
            </Link>
          )}
        </div>
      </div>
    </div>
  )
}
