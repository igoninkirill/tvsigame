import { Fragment } from 'react'

import { useRoom } from '../lib/room'

// Игровое табло: темы — строки слева, стоимости растут вправо. Классика НТВ.
export default function GameBoard() {
  const { snapshot } = useRoom()
  const game = snapshot?.game
  const pack = snapshot?.pack
  if (!game || !pack) return null

  const round = pack.rounds[game.roundIndex]
  if (!round) return null

  const qCount = Math.max(...round.themes.map((t) => t.questions.length), 0)

  return (
    <div className="tv-board">
      <div className="tv-board__round">{round.name}</div>
      <div
        className="tv-board__grid"
        style={{ gridTemplateColumns: `minmax(0, 1.7fr) repeat(${qCount}, minmax(0, 1fr))` }}
      >
        {round.themes.map((theme, ti) => (
          <Fragment key={ti}>
            <div className="tv-theme tv-theme--row">
              <span className="tv-theme__label">{theme.name}</span>
            </div>
            {Array.from({ length: qCount }, (_, qi) => {
              const q = theme.questions[qi]
              if (!q) return <div key={qi} />
              const used = q.used
              const special = q.type !== 'regular'
              return (
                <div
                  key={qi}
                  className={
                    'tv-cell' + (used ? ' tv-cell--used' : '') + (special && !used ? ' tv-cell--special' : '')
                  }
                >
                  {used ? ' ' : q.price}
                </div>
              )
            })}
          </Fragment>
        ))}
      </div>
    </div>
  )
}
