import { useEffect, useState } from 'react'

import { useRoom } from '../lib/room'
import { api } from '../lib/api'
import { getHostToken } from '../lib/session'
import { Confirm, useToast } from './common'
import { Cmd, type HistoryEvent } from '../lib/types'
import { IconPause, IconPlay, IconUndo, IconClock, IconHistory, IconShield } from '../components/Icon'

// Раздел «Ещё»: пауза, отмена действия, таймеры, фальстарты, история событий.
export default function MorePanel() {
  const { snapshot, send, status } = useRoom()
  const { showToast } = useToast()
  const room = snapshot?.room
  const game = snapshot?.game
  const [confirmPause, setConfirmPause] = useState(false)
  const [confirmUndo, setConfirmUndo] = useState(false)
  const [events, setEvents] = useState<HistoryEvent[]>([])
  const [showHistory, setShowHistory] = useState(false)

  const token = room ? getHostToken(room.id) : null

  async function run(type: string, payload: Record<string, unknown> = {}) {
    try {
      await send(type, payload)
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  async function loadHistory() {
    if (!room || !token) return
    try {
      const res = await api.get<{ events: HistoryEvent[] }>(`/api/rooms/${room.id}/events`, token)
      setEvents(res.events)
      setShowHistory(true)
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  const paused = room?.status === 'paused'

  return (
    <>
      <div className="host-section">
        <div className="host-section__title">
          <IconPause width={18} height={18} /> Управление игрой
        </div>
        <div className="host-actions">
          <button className="btn" onClick={() => setConfirmPause(true)} disabled={room?.status === 'finished'}>
            {paused ? <IconPlay width={18} height={18} /> : <IconPause width={18} height={18} />}
            {paused ? 'Продолжить' : 'Пауза'}
          </button>
          <button className="btn btn--ghost" onClick={() => setConfirmUndo(true)}>
            <IconUndo width={18} height={18} /> Отменить действие
          </button>
        </div>
      </div>

      {game && <TimersPanel />}

      <div className="host-section">
        <div className="host-section__title">
          <IconShield width={18} height={18} /> Фальстарты
        </div>
        <button
          className="btn"
          onClick={() => run(Cmd.SetFalseStartGuard, { enabled: !game?.falseStartGuard })}
        >
          {game?.falseStartGuard ? 'Защита включена' : 'Защита выключена'} — переключить
        </button>
      </div>

      <div className="host-section">
        <div className="host-section__title">
          <IconHistory width={18} height={18} /> История событий
        </div>
        <button className="btn btn--ghost" onClick={loadHistory}>
          {showHistory ? 'Обновить историю' : 'Показать историю'}
        </button>
        {showHistory && (
          <div className="event-log">
            {events.length === 0 && <div className="event-log__item">Событий пока нет</div>}
            {events.map((e) => (
              <div key={e.seq} className="event-log__item">
                <span>
                  <b>#{e.seq}</b> {eventLabel(e)}
                </span>
                <span>{new Date(e.at).toLocaleTimeString('ru-RU')}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="host-section">
        <div className="host-section__title">Соединение</div>
        <span className={'conn' + (status === 'online' ? ' conn--online' : ' conn--offline')}>
          {status === 'online' ? 'Сервер на связи' : status === 'connecting' ? 'Подключение…' : 'Нет связи — переподключаемся'}
        </span>
      </div>

      {confirmPause && (
        <Confirm
          title={paused ? 'Возобновить игру?' : 'Поставить игру на паузу?'}
          confirmLabel={paused ? 'Продолжить' : 'Пауза'}
          onCancel={() => setConfirmPause(false)}
          onConfirm={() => {
            setConfirmPause(false)
            run(paused ? Cmd.ResumeGame : Cmd.PauseGame)
          }}
        />
      )}
      {confirmUndo && (
        <Confirm
          title="Отменить последнее игровое действие? Счёт и табло будут восстановлены."
          confirmLabel="Отменить действие"
          onCancel={() => setConfirmUndo(false)}
          onConfirm={() => {
            setConfirmUndo(false)
            run(Cmd.UndoLastAction)
          }}
        />
      )}
    </>
  )
}

// Настройка длительностей таймеров (серверные значения, мс).
function TimersPanel() {
  const { snapshot, send } = useRoom()
  const { showToast } = useToast()
  const timers = snapshot?.game?.timers
  const [values, setValues] = useState({
    answerWindowMs: 0,
    answeringMs: 0,
    finalThinkingMs: 0,
    forAllMs: 0,
  })

  useEffect(() => {
    if (timers) {
      setValues({
        answerWindowMs: timers.answerWindowMs / 1000,
        answeringMs: timers.answeringMs / 1000,
        finalThinkingMs: timers.finalThinkingMs / 1000,
        forAllMs: timers.forAllMs / 1000,
      })
    }
  }, [timers])

  if (!timers) return null

  async function save() {
    try {
      await send(Cmd.UpdateTimers, {
        answerWindowMs: values.answerWindowMs * 1000,
        answeringMs: values.answeringMs * 1000,
        finalThinkingMs: values.finalThinkingMs * 1000,
        forAllMs: values.forAllMs * 1000,
      })
      showToast('Таймеры обновлены')
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  const rows: Array<[keyof typeof values, string]> = [
    ['answerWindowMs', 'Окно ответа, сек'],
    ['answeringMs', 'Время на ответ, сек'],
    ['finalThinkingMs', 'Финал: размышление, сек'],
    ['forAllMs', 'Вопрос для всех, сек'],
  ]

  return (
    <div className="host-section">
      <div className="host-section__title">
        <IconClock width={18} height={18} /> Таймеры
      </div>
      {rows.map(([key, label]) => (
        <div key={key} className="stake-row">
          <span style={{ color: 'var(--text-dim)', fontSize: '0.9rem' }}>{label}</span>
          <input
            className="field"
            style={{ width: '6rem', textAlign: 'center' }}
            type="number"
            min={1}
            inputMode="numeric"
            value={values[key]}
            onChange={(e) => setValues({ ...values, [key]: Number(e.target.value) })}
          />
        </div>
      ))}
      <button className="btn" onClick={save}>
        Сохранить таймеры
      </button>
    </div>
  )
}

function eventLabel(e: HistoryEvent): string {
  const p = e.payload as Record<string, unknown>
  switch (e.type) {
    case 'GAME_STARTED':
      return 'Игра началась'
    case 'QUESTION_SELECTED':
      return `Выбран вопрос: ${p.theme} за ${p.price}`
    case 'RESPONDER_SELECTED':
      return 'Выбран отвечающий'
    case 'VERDICT':
      return p.correct ? 'Ответ засчитан' : 'Ответ не засчитан'
    case 'SCORE_CHANGED': {
      const delta = Number(p.delta)
      return `Счёт: ${delta >= 0 ? '+' : ''}${delta} (${p.reason})`
    }
    case 'ANSWER_SHOWN':
      return `Показан ответ: ${p.answer ?? ''}`
    case 'QUESTION_CLOSED':
      return 'Вопрос закрыт'
    case 'ROUND_STARTED':
      return `Раунд: ${p.name}`
    case 'BID_PLACED':
      return `Ставка: ${p.amount}`
    case 'BID_PASSED':
      return 'Пас'
    case 'AUCTION_ENDED':
      return p.burned ? 'Аукцион не состоялся' : `Аукцион: итог ${p.price}`
    case 'CAT_TRANSFERRED':
      return 'Кот передан'
    case 'FINAL_THEME_EXCLUDED':
      return `Финал: тема «${p.theme}» исключена`
    case 'FINAL_STAKE_SET':
      return p.revealed ? 'Ставки раскрыты' : 'Ставка принята'
    case 'FINAL_QUESTION_OPENED':
      return `Финальный вопрос: ${p.theme}`
    case 'FINAL_ANSWER_CHECKED':
      return 'Финал: ответ проверен'
    case 'GAME_FINISHED':
      return 'Игра завершена'
    case 'GAME_PAUSED':
      return 'Пауза'
    case 'GAME_RESUMED':
      return 'Игра продолжена'
    case 'ACTION_UNDONE':
      return `Отменено: ${p.commandType}`
    case 'TIMER_EXPIRED':
      return `Время истекло (${p.kind})`
    case 'TIMERS_UPDATED':
      return 'Таймеры перенастроены'
    case 'FALSE_START_UPDATED':
      return p.enabled ? 'Фальстарты: защита включена' : 'Фальстарты: защита выключена'
    case 'TIMER_STARTED':
      return 'Таймер запущен'
    case 'TIMER_PAUSED':
      return 'Таймер приостановлен'
    case 'ANSWER_WINDOW_OPENED':
      return 'Открыто окно ответа'
    default:
      return e.type
  }
}
