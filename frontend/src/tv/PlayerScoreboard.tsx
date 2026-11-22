import { useRoom } from '../lib/room'

// Табло игроков внизу ТВ-экрана: имена, баллы, подсветка отвечающего.
export default function PlayerScoreboard() {
  const { snapshot } = useRoom()
  const players = snapshot?.players ?? []
  const game = snapshot?.game
  if (players.length === 0) return null

  const activeId =
    game?.current?.answeringId ||
    (game?.stage === 'auction' ? game.auction?.highestBidderId : undefined) ||
    (game?.stage === 'cat_transfer' ? undefined : game?.current?.forcedId)

  return (
    <div className="tv-scoreboard">
      {players.map((p) => (
        <div key={p.id} className={'tv-player' + (p.id === activeId ? ' tv-player--active' : '')}>
          {p.id === activeId && <div className="tv-player__badge">отвечает</div>}
          <div className="tv-player__name">{p.name}</div>
          <div className={'tv-player__score' + (p.score < 0 ? ' tv-player__score--negative' : '')}>
            {p.score}
          </div>
        </div>
      ))}
    </div>
  )
}
