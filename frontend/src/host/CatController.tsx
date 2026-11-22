import { useRoom } from '../lib/room'
import { useToast } from './common'
import { Cmd } from '../lib/types'
import { IconCat } from '../components/Icon'

// Кот в мешке: ведущий выбирает, кому передаётся вопрос.
export default function CatController() {
  const { snapshot, send } = useRoom()
  const { showToast } = useToast()
  const cur = snapshot?.game?.current
  if (!cur) return null

  const players = snapshot?.players.filter((p) => p.isActive) ?? []

  async function transfer(playerId: string) {
    try {
      await send(Cmd.TransferCat, { playerId })
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  return (
    <div className="host-section">
      <div className="host-section__title">
        <IconCat width={18} height={18} /> Кот в мешке: кому передать?
      </div>
      {cur.typeParams?.theme && (
        <div className="host-question__meta">
          <span className="host-question__badge">Тема кота: {cur.typeParams.theme}</span>
        </div>
      )}
      <div className="host-actions">
        {players.map((p) => (
          <button key={p.id} className="btn btn--gold" onClick={() => transfer(p.id)}>
            {p.name}
          </button>
        ))}
      </div>
    </div>
  )
}
