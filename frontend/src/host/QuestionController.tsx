import { useRoom } from '../lib/room'
import { api } from '../lib/api'
import { getHostToken } from '../lib/session'
import { useToast } from './common'
import { Cmd } from '../lib/types'
import { IconCheck, IconEye, IconPause, IconPlay, IconSkipForward, IconX } from '../components/Icon'

// Управление текущим вопросом: окно ответа, выбор отвечающего, вердикт,
// показ ответа, закрытие вопроса, управление медиа и таймером.
export default function QuestionController() {
  const { snapshot, send } = useRoom()
  const { showToast } = useToast()
  const game = snapshot?.game
  const cur = game?.current
  if (!game || !cur) return null

  const stage = game.stage
  const players = snapshot?.players.filter((p) => p.isActive) ?? []

  async function run(type: string, payload: Record<string, unknown> = {}) {
    try {
      await send(type, payload)
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка команды', true)
    }
  }

  async function signal(kind: string) {
    try {
      await api.post(`/api/rooms/${snapshot!.room.id}/signal`, { kind }, getHostToken(snapshot!.room.id))
    } catch {
      /* некритично */
    }
  }

  const hasMedia = (cur.scenario ?? []).some((a) => a.kind === 'audio' || a.kind === 'video')

  return (
    <>
      <div className="host-section">
        <div className="host-section__title">Текущий вопрос</div>
        <div className="host-question">
          <div className="host-question__meta">
            <span className="host-question__badge">{cur.themeName}</span>
            <span className="host-question__badge">{cur.price}</span>
            {cur.type !== 'regular' && (
              <span className="host-question__badge">{typeLabel(cur.type)}</span>
            )}
          </div>
          <div className="host-question__text">
            {(cur.scenario ?? [])
              .filter((a) => a.kind === 'text' || a.kind === 'say')
              .map((a) => a.text)
              .join(' ')}
          </div>
          {cur.answer && (
            <div className="host-question__answer">Ответ: {cur.answer}</div>
          )}
          {(cur.scenario ?? []).some((a) => a.kind === 'image' && a.mediaUrl) && (
            <div className="host-question__media">
              {(cur.scenario ?? [])
                .filter((a) => a.kind === 'image' && a.mediaUrl)
                .map((a, i) => (
                  <img key={i} src={a.mediaUrl} alt="Медиа вопроса" />
                ))}
            </div>
          )}
          {cur.answerShown && (cur.answerScenario ?? []).some((a) => a.kind === 'image' && a.mediaUrl) && (
            <div className="host-question__media">
              {(cur.answerScenario ?? [])
                .filter((a) => a.kind === 'image' && a.mediaUrl)
                .map((a, i) => (
                  <img key={i} src={a.mediaUrl} alt="Медиа ответа" />
                ))}
            </div>
          )}
          {cur.type.startsWith('unknown:') && (
            <div className="host-question__meta" style={{ color: 'var(--gold-soft)' }}>
              Неподдерживаемый тип «{cur.type.slice(8)}» — проведите вручную, баллы
              скорректируйте через раздел «Игроки»
            </div>
          )}
        </div>
        {hasMedia && (
          <div className="host-actions">
            <button className="btn" onClick={() => signal('media_play')}>
              <IconPlay width={18} height={18} /> Медиа
            </button>
            <button className="btn" onClick={() => signal('media_pause')}>
              <IconPause width={18} height={18} /> Пауза
            </button>
          </div>
        )}
      </div>

      {stage === 'question' && (
        <div className="host-section">
          <div className="host-section__title">Окно ответа</div>
          <div className="host-actions host-actions--single">
            <button className="btn btn--gold" onClick={() => run(Cmd.AllowAnswers)}>
              Разрешить отвечать
            </button>
          </div>
        </div>
      )}

      {stage === 'answer_window' && (
        <div className="host-section">
          <div className="host-section__title">Кто отвечает?</div>
          <div className="host-actions">
            {players.map((p) => {
              const answeredIds = cur.answeredIds ?? []
              const disabled =
                answeredIds.includes(p.id) || (!!cur.forcedId && cur.forcedId !== p.id)
              return (
                <button
                  key={p.id}
                  className="btn"
                  disabled={disabled}
                  onClick={() => run(Cmd.SelectResponder, { playerId: p.id })}
                >
                  {p.name}
                </button>
              )
            })}
          </div>
        </div>
      )}

      {stage === 'answering' && (
        <div className="host-section">
          <div className="host-section__title">
            Отвечает: {players.find((p) => p.id === cur.answeringId)?.name ?? '…'}
          </div>
          <div className="verdict-row">
            <button className="verdict-btn verdict-btn--ok" onClick={() => run(Cmd.SubmitVerdict, { correct: true })}>
              <IconCheck width={22} height={22} /> Верно
            </button>
            <button className="verdict-btn verdict-btn--no" onClick={() => run(Cmd.SubmitVerdict, { correct: false })}>
              <IconX width={22} height={22} /> Неверно
            </button>
          </div>
        </div>
      )}

      {(stage === 'question' || stage === 'answer_window' || stage === 'answering') && (
        <div className="host-section">
          <div className="host-actions">
            <button className="btn btn--ghost" onClick={() => run(Cmd.ShowAnswer)}>
              <IconEye width={18} height={18} /> Показать ответ
            </button>
            {game.timer && (
              <button
                className="btn btn--ghost"
                onClick={() => run(game.timer!.running ? Cmd.PauseTimer : Cmd.ResumeTimer)}
              >
                {game.timer.running ? <IconPause width={18} height={18} /> : <IconPlay width={18} height={18} />}
                Таймер
              </button>
            )}
          </div>
        </div>
      )}

      {stage === 'answer_shown' && (
        <div className="host-section">
          <div className="host-actions host-actions--single">
            <button className="btn btn--gold" onClick={() => run(Cmd.CloseQuestion)}>
              <IconSkipForward width={18} height={18} /> К табло
            </button>
          </div>
        </div>
      )}
    </>
  )
}

export function typeLabel(t: string): string {
  switch (t) {
    case 'auction':
      return 'Аукцион'
    case 'cat':
      return 'Кот в мешке'
    case 'noRisk':
      return 'Без риска'
    case 'forAll':
      return 'Для всех'
    case 'regular':
      return 'Обычный'
    default:
      return t.startsWith('unknown:') ? `Особый (${t.slice(8)})` : t
  }
}
