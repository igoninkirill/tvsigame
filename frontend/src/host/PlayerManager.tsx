import { useState } from 'react'

import { useRoom } from '../lib/room'
import { api } from '../lib/api'
import { getHostToken } from '../lib/session'
import { Confirm, useToast } from './common'
import { Cmd } from '../lib/types'
import { IconCheck, IconEdit, IconMinus, IconPlus, IconTrash, IconUsers, IconX } from '../components/Icon'

// Управление игроками: добавление, переименование, удаление,
// ручная корректировка счёта (через игровую команду SET_SCORE).
export default function PlayerManager() {
  const { snapshot, send } = useRoom()
  const { showToast } = useToast()
  const room = snapshot?.room
  const players = snapshot?.players ?? []
  const [newName, setNewName] = useState('')
  const [editId, setEditId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')
  const [scoreEdit, setScoreEdit] = useState<{ id: string; value: string } | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)

  if (!room) return null
  const token = getHostToken(room.id)
  const gameActive = room.status === 'active' || room.status === 'paused'

  async function addPlayer() {
    if (!newName.trim() || !token) return
    try {
      await api.post(`/api/rooms/${room!.id}/players`, { name: newName.trim() }, token)
      setNewName('')
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  async function rename(id: string) {
    try {
      await api.patch(`/api/rooms/${room!.id}/players/${id}`, { name: editName.trim() }, token)
      setEditId(null)
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  async function remove(id: string) {
    try {
      await api.del(`/api/rooms/${room!.id}/players/${id}`, token)
      setConfirmDelete(null)
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  async function setScore(id: string, value: number) {
    try {
      await send(Cmd.SetScore, { playerId: id, score: value, reason: 'manual_correction' })
      setScoreEdit(null)
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  return (
    <div className="host-section">
      <div className="host-section__title">
        <IconUsers width={18} height={18} /> Игроки и счёт
      </div>

      {players.map((p) => (
        <div key={p.id} className="player-row" style={{ flexWrap: 'wrap' }}>
          {editId === p.id ? (
            <>
              <input
                className="field"
                style={{ flex: 1 }}
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                autoFocus
              />
              <button className="icon-btn" onClick={() => rename(p.id)}>
                <IconCheck width={18} height={18} />
              </button>
              <button className="icon-btn" onClick={() => setEditId(null)}>
                <IconX width={18} height={18} />
              </button>
            </>
          ) : (
            <>
              <span className="player-row__name">{p.name}</span>
              <span className={'player-row__score' + (p.score < 0 ? ' player-row__score--negative' : '')}>
                {p.score}
              </span>
              <div className="player-row__btns">
                <button
                  className="icon-btn"
                  title="Изменить счёт"
                  onClick={() => setScoreEdit({ id: p.id, value: String(p.score) })}
                >
                  <IconMinus width={16} height={16} />
                  <IconPlus width={16} height={16} style={{ marginLeft: -10 }} />
                </button>
                <button
                  className="icon-btn"
                  title="Переименовать"
                  onClick={() => {
                    setEditId(p.id)
                    setEditName(p.name)
                  }}
                >
                  <IconEdit width={16} height={16} />
                </button>
                <button
                  className="icon-btn icon-btn--danger"
                  title="Удалить"
                  disabled={gameActive}
                  onClick={() => setConfirmDelete(p.id)}
                >
                  <IconTrash width={16} height={16} />
                </button>
              </div>
            </>
          )}
          {scoreEdit?.id === p.id && (
            <div className="stake-row__input" style={{ width: '100%' }}>
              <input
                className="field"
                type="number"
                inputMode="numeric"
                value={scoreEdit.value}
                onChange={(e) => setScoreEdit({ id: p.id, value: e.target.value })}
              />
              <button className="btn btn--gold" onClick={() => setScore(p.id, Number(scoreEdit.value))}>
                Установить
              </button>
            </div>
          )}
        </div>
      ))}

      <div style={{ display: 'flex', gap: '0.5rem' }}>
        <input
          className="field"
          placeholder="Имя нового игрока"
          value={newName}
          maxLength={40}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && addPlayer()}
        />
        <button className="btn btn--gold" onClick={addPlayer} disabled={!newName.trim()}>
          <IconPlus width={18} height={18} />
        </button>
      </div>

      {confirmDelete && (
        <Confirm
          title={`Удалить игрока «${players.find((p) => p.id === confirmDelete)?.name}»?`}
          confirmLabel="Удалить"
          onCancel={() => setConfirmDelete(null)}
          onConfirm={() => remove(confirmDelete)}
        />
      )}
    </div>
  )
}
