import { useRoom } from '../lib/room'
import { IconCrown } from '../components/Icon'

// Итоговая таблица и победитель.
export default function ResultsScreen() {
  const { snapshot } = useRoom()
  const players = [...(snapshot?.players ?? [])].sort((a, b) => b.score - a.score)
  const winnerId = snapshot?.game?.winnerId

  return (
    <div className="tv-results">
      <span className="tv-special__icon"><IconCrown width={120} height={120} /></span>
      <div className="tv-special__title" style={{ fontSize: '7vh' }}>Игра окончена</div>
      <div>
        {players.map((p, i) => (
          <div
            key={p.id}
            className={'tv-results__row' + (p.id === winnerId ? ' tv-results__row--winner' : '')}
          >
            <span>{i + 1}</span>
            <span style={{ textAlign: 'left' }}>{p.name}</span>
            <span style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{p.score}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
