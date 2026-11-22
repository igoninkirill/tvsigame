import { useState } from 'react'

import { useRoom } from '../lib/room'
import { useToast } from './common'
import { Cmd } from '../lib/types'

// Управление аукционом: фиксация ставок, пасы, ва-банк, завершение торгов.
export default function AuctionController() {
  const { snapshot, send } = useRoom()
  const { showToast } = useToast()
  const game = snapshot?.game
  const auc = game?.auction
  const cur = game?.current
  if (!game || !auc || !cur) return null

  const players = snapshot?.players.filter((p) => p.isActive) ?? []
  const [bidFor, setBidFor] = useState<string | null>(null)
  const [amount, setAmount] = useState('')

  async function run(type: string, payload: Record<string, unknown> = {}) {
    try {
      await send(type, payload)
      setBidFor(null)
      setAmount('')
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  return (
    <div className="host-section">
      <div className="host-section__title">
        Аукцион: {cur.themeName} (номинал {auc.nominal})
      </div>
      {auc.highestBid > 0 && (
        <div className="host-question__meta">
          <span className="host-question__badge">
            Текущая ставка: {auc.highestBid} —{' '}
            {players.find((p) => p.id === auc.highestBidderId)?.name}
          </span>
        </div>
      )}
      {players.map((p) => {
        const passed = (auc.passedIds ?? []).includes(p.id)
        const leader = auc.highestBidderId === p.id
        return (
          <div key={p.id} className="player-row">
            <span className="player-row__name">
              {p.name}
              {leader && ' (лидер)'}
              {passed && ' (пас)'}
            </span>
            <span className="player-row__score">{p.score}</span>
            <div className="player-row__btns">
              <button
                className="btn"
                style={{ minHeight: 44, padding: '0.4rem 0.8rem' }}
                disabled={passed || leader}
                onClick={() => setBidFor(bidFor === p.id ? null : p.id)}
              >
                Ставка
              </button>
              <button
                className="btn btn--ghost"
                style={{ minHeight: 44, padding: '0.4rem 0.8rem' }}
                disabled={passed || leader}
                onClick={() => run(Cmd.PassBid, { playerId: p.id })}
              >
                Пас
              </button>
              <button
                className="btn btn--gold"
                style={{ minHeight: 44, padding: '0.4rem 0.8rem' }}
                disabled={passed || leader || p.score <= Math.max(auc.highestBid, auc.nominal - 1)}
                onClick={() => run(Cmd.PlaceBid, { playerId: p.id, amount: p.score })}
              >
                Ва-банк
              </button>
            </div>
            {bidFor === p.id && (
              <div className="stake-row__input" style={{ gridColumn: '1 / -1' }}>
                <input
                  className="field"
                  type="number"
                  inputMode="numeric"
                  placeholder={`мин. ${Math.max(auc.nominal, auc.highestBid + 1)}`}
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                />
                <button
                  className="btn btn--gold"
                  onClick={() => run(Cmd.PlaceBid, { playerId: p.id, amount: Number(amount) })}
                >
                  Принять
                </button>
              </div>
            )}
          </div>
        )
      })}
      <div className="host-actions host-actions--single">
        <button
          className="btn btn--gold"
          disabled={!auc.highestBidderId}
          onClick={() => run(Cmd.EndAuction)}
        >
          Завершить торги
        </button>
      </div>
    </div>
  )
}
