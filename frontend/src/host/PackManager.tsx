import { useEffect, useRef, useState } from 'react'

import { useRoom } from '../lib/room'
import { api } from '../lib/api'
import { getHostToken } from '../lib/session'
import { useToast } from './common'
import type { PackListItem } from '../lib/types'
import { IconPackage, IconUpload } from '../components/Icon'

// Управление пакетами: загрузка .siq и выбор из библиотеки.
export default function PackManager() {
  const { snapshot } = useRoom()
  const { showToast } = useToast()
  const room = snapshot?.room
  const [packs, setPacks] = useState<PackListItem[]>([])
  const [busy, setBusy] = useState(false)
  const [warnings, setWarnings] = useState<string[]>([])
  const fileRef = useRef<HTMLInputElement>(null)

  const token = room ? getHostToken(room.id) : null
  const gameActive = room?.status === 'active' || room?.status === 'paused'

  async function reload() {
    if (!room || !token) return
    try {
      const res = await api.get<{ packs: PackListItem[] }>(`/api/rooms/${room.id}/packs`, token)
      setPacks(res.packs)
    } catch {
      /* покажем при действии */
    }
  }

  useEffect(() => {
    reload()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [room?.id])

  async function upload(file: File) {
    if (!room || !token) return
    setBusy(true)
    setWarnings([])
    try {
      const res = await api.upload<{ name: string; warnings: string[] }>(
        `/api/rooms/${room.id}/pack/import`,
        file,
        token,
      )
      showToast(`Пакет «${res.name}» загружен`)
      setWarnings(res.warnings ?? [])
      reload()
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка импорта', true)
    } finally {
      setBusy(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  async function select(packId: string) {
    if (!room || !token) return
    try {
      await api.post(`/api/rooms/${room.id}/pack/select`, { packId }, token)
      showToast('Пакет выбран для игры')
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  return (
    <div className="host-section">
      <div className="host-section__title">
        <IconPackage width={18} height={18} /> Пакет вопросов
      </div>

      {snapshot?.pack && (
        <div className="host-question__meta">
          <span className="host-question__badge">Выбран: {snapshot.pack.name}</span>
        </div>
      )}

      <input
        ref={fileRef}
        type="file"
        accept=".siq,application/zip"
        style={{ display: 'none' }}
        onChange={(e) => e.target.files?.[0] && upload(e.target.files[0])}
      />
      <button
        className="btn btn--gold"
        disabled={busy || gameActive}
        onClick={() => fileRef.current?.click()}
      >
        <IconUpload width={18} height={18} />
        {busy ? 'Загрузка…' : 'Загрузить .siq'}
      </button>

      {warnings.length > 0 && (
        <div className="event-log">
          {warnings.map((w, i) => (
            <div key={i} className="event-log__item">{w}</div>
          ))}
        </div>
      )}

      {packs.length > 0 && (
        <>
          <div className="host-section__title" style={{ marginTop: '0.4rem' }}>Библиотека</div>
          {packs.map((p) => (
            <div key={p.id} className="player-row">
              <span className="player-row__name">
                {p.name}
                <br />
                <small style={{ color: 'var(--text-dim)' }}>
                  {p.author && `${p.author} · `}раундов: {p.rounds} · v{p.version}
                </small>
              </span>
              <button className="btn" disabled={gameActive} onClick={() => select(p.id)}>
                Выбрать
              </button>
            </div>
          ))}
        </>
      )}
    </div>
  )
}
