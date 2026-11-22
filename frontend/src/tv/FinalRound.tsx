import { useRoom } from '../lib/room'
import { GameTimer } from './QuestionView'

// Финальный раунд на ТВ: исключение тем → ставки → вопрос → вердикты.
export default function FinalRound() {
  const { snapshot } = useRoom()
  const game = snapshot?.game
  const fin = game?.final
  if (!game || !fin) return null

  const stage = game.stage

  // Исключение тем / ставки
  if (stage === 'final_themes' || stage === 'final_stakes') {
    return (
      <div className="tv-special">
        <div className="tv-special__title" style={{ fontSize: '6.5vh' }}>Финальный раунд</div>
        <div className="tv-final__themes">
      {(fin.themeNames ?? []).map((name, i) => (
            <div
              key={i}
              className={
                'tv-final__theme' + ((fin.excluded ?? []).includes(i) ? ' tv-final__theme--excluded' : '')
              }
            >
              {name}
            </div>
          ))}
        </div>
        {stage === 'final_stakes' && (
          <>
            <div className="tv-special__sub">Игроки делают ставки…</div>
            {fin.stakesRevealed && fin.stakes && (
              <div className="tv-special__sub">
                {(fin.finalistIds ?? []).map((id) => {
                  const p = snapshot?.players.find((pl) => pl.id === id)
                  return (
                    <div key={id}>
                      {p?.name}: ставка {fin.stakes?.[id] ?? 0}
                    </div>
                  )
                })}
              </div>
            )}
          </>
        )}
      </div>
    )
  }

  // Финальный вопрос / проверка ответов
  const q = fin.question
  return (
    <div className="tv-question">
      {game.timer && <GameTimer timer={game.timer} />}
      <div className="tv-question__meta">
        <span>Финальный раунд</span>
        <span className="tv-question__price">{q?.themeName}</span>
      </div>
      {q?.scenario &&
        (q.scenario ?? [])
        .filter((a) => a.kind === 'text' || a.kind === 'say')
        .map((a, i) => (
          <div key={i} className="tv-question__text">{a.text}</div>
        ))}
      {q?.scenario &&
        (q.scenario ?? [])
        .filter((a) => a.kind === 'image')
        .map((a, i) =>
          a.mediaUrl ? (
            <img key={i} className="tv-question__image" src={a.mediaUrl} alt="Изображение" />
          ) : null,
        )}
      {stage === 'final_verdicts' && fin.answers && (
        <div className="tv-special__sub">
          {(fin.finalistIds ?? []).map((id) => {
            const p = snapshot?.players.find((pl) => pl.id === id)
            const ok = fin.answers?.[id]
            if (ok === undefined) return null
            return (
              <div key={id}>
                {p?.name}: {ok ? 'верно' : 'неверно'}
              </div>
            )
          })}
        </div>
      )}
      {q?.answerShown && q.answer && (
        <>
          <div className="tv-question__label">Правильный ответ</div>
          <div className="tv-question__answer">{q.answer}</div>
          {(q.answerScenario ?? [])
            .filter((a) => a.kind === 'image' && a.mediaUrl)
            .map((a, i) => (
              <img key={i} className="tv-question__image" src={a.mediaUrl} alt="Изображение к ответу" />
            ))}
        </>
      )}
    </div>
  )
}
