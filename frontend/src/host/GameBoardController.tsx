import { Fragment } from 'react'

import { useRoom } from '../lib/room'
import { useToast } from './common'
import { Cmd, type SkeletonQuestion } from '../lib/types'

// Мини-табло на пульте: темы строками слева, цены вправо; ведущий выбирает
// вопрос касанием ячейки.
export default function GameBoardController() {
  const { snapshot, send } = useRoom()
  const { showToast } = useToast()
  const game = snapshot?.game
  const pack = snapshot?.pack
  if (!game || !pack) return null

  const round = pack.rounds[game.roundIndex]
  if (!round) return null

  const qCount = Math.max(...round.themes.map((t) => t.questions.length), 0)

  async function select(themeIndex: number, questionIndex: number, q: SkeletonQuestion) {
    if (q.used) return
    try {
      await send(Cmd.SelectQuestion, { themeIndex, questionIndex })
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  return (
    <div className="host-section">
      <div className="host-section__title">{round.name}</div>
      <div
        className="host-board"
        style={{ gridTemplateColumns: `minmax(0, 1.35fr) repeat(${qCount}, minmax(0, 1fr))` }}
      >
        {round.themes.map((theme, ti) => (
          <Fragment key={ti}>
            <div className="host-board__theme">
              <span className="tv-theme__label">{theme.name}</span>
            </div>
            {Array.from({ length: qCount }, (_, qi) => {
              const q = theme.questions[qi]
              if (!q) return <div key={qi} />
              const special = q.type !== 'regular'
              return (
                <button
                  key={qi}
                  className={
                    'host-board__cell' +
                    (q.used ? ' host-board__cell--used' : '') +
                    (special && !q.used ? ' host-board__cell--special' : '')
                  }
                  disabled={q.used}
                  onClick={() => select(ti, qi, q)}
                >
                  {q.used ? ' ' : q.price}
                </button>
              )
            })}
          </Fragment>
        ))}
      </div>
    </div>
  )
}
