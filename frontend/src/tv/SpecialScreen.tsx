import { useRoom } from '../lib/room'
import { IconCat, IconGavel, IconUsers } from '../components/Icon'
import { GameTimer } from './QuestionView'

// Спецэкраны ТВ: аукцион, кот в мешке, вопрос для всех.
export default function SpecialScreen() {
  const { snapshot } = useRoom()
  const game = snapshot?.game
  if (!game?.current) return null
  const cur = game.current

  if (game.stage === 'auction') {
    const auc = game.auction
    return (
      <div className="tv-special">
        <span className="tv-special__icon"><IconGavel width={110} height={110} /></span>
        <div className="tv-special__title">Аукцион</div>
        <div className="tv-special__sub">
          {cur.themeName} — номинал {auc?.nominal}
        </div>
        {auc && auc.highestBid > 0 && (
          <>
            <div className="tv-auction__bid">{auc.highestBid}</div>
            <div className="tv-special__sub">
              {snapshot?.players.find((p) => p.id === auc.highestBidderId)?.name ?? ''}
            </div>
          </>
        )}
      </div>
    )
  }

  if (game.stage === 'cat_transfer') {
    return (
      <div className="tv-special">
        <span className="tv-special__icon"><IconCat width={110} height={110} /></span>
        <div className="tv-special__title">Кот в мешке</div>
        <div className="tv-special__sub">Вопрос передаётся другому игроку…</div>
      </div>
    )
  }

  // for_all
  return (
    <div className="tv-special">
      {game.timer && <GameTimer timer={game.timer} />}
      <span className="tv-special__icon"><IconUsers width={110} height={110} /></span>
      <div className="tv-special__title">Вопрос для всех</div>
      <div className="tv-special__sub">{cur.themeName}</div>
      {cur.scenario &&
        (cur.scenario ?? [])
        .filter((a) => a.kind === 'text' || a.kind === 'say')
        .map((a, i) => (
          <div key={i} className="tv-question__text">{a.text}</div>
        ))}
      {cur.answerShown && cur.answer && (
        <div className="tv-question__answer">{cur.answer}</div>
      )}
    </div>
  )
}
