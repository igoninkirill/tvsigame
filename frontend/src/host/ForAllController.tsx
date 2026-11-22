import { useRoom } from '../lib/room'
import { useToast } from './common'
import { Cmd } from '../lib/types'
import { IconCheck, IconX } from '../components/Icon'

// Вопрос для всех: после таймера ведущий фиксирует вердикт по каждому игроку.
export default function ForAllController() {
  const { snapshot, send } = useRoom()
  const { showToast } = useToast()
  const cur = snapshot?.game?.current
  if (!cur) return null

  const players = snapshot?.players.filter((p) => p.isActive) ?? []
  const verdicts = cur.verdicts ?? {}

  async function verdict(playerId: string, correct: boolean) {
    try {
      await send(Cmd.SubmitVerdict, { playerId, correct })
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  return (
    <div className="host-section">
      <div className="host-section__title">Вопрос для всех — вердикты</div>
      <div className="host-question__text">
        {(cur.scenario ?? [])
          .filter((a) => a.kind === 'text' || a.kind === 'say')
          .map((a) => a.text)
          .join(' ')}
      </div>
      {cur.answer && <div className="host-question__answer">Ответ: {cur.answer}</div>}
      {players.map((p) => {
        const v = verdicts[p.id]
        return (
          <div key={p.id} className="player-row">
            <span className="player-row__name">{p.name}</span>
            {v === undefined ? (
              <div className="player-row__btns">
                <button
                  className="verdict-btn verdict-btn--ok"
                  style={{ minHeight: 44, padding: '0.3rem 0.9rem', fontSize: '1rem' }}
                  onClick={() => verdict(p.id, true)}
                >
                  <IconCheck width={18} height={18} /> Верно
                </button>
                <button
                  className="verdict-btn verdict-btn--no"
                  style={{ minHeight: 44, padding: '0.3rem 0.9rem', fontSize: '1rem' }}
                  onClick={() => verdict(p.id, false)}
                >
                  <IconX width={18} height={18} /> Неверно
                </button>
              </div>
            ) : (
              <span style={{ color: v ? 'var(--success)' : 'var(--danger)', fontWeight: 700 }}>
                {v ? 'Верно' : 'Неверно'}
              </span>
            )}
          </div>
        )
      })}
    </div>
  )
}
