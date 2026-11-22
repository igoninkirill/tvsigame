import { useEffect, useRef, useState } from 'react'

import { useRoom } from '../lib/room'
import type { Atom, Timer } from '../lib/types'

// Таймер обратного отсчёта: клиент лишь отображает серверное время.
export function GameTimer({ timer }: { timer: Timer }) {
  const [msLeft, setMsLeft] = useState(timer.remainingMs)

  useEffect(() => {
    if (!timer.running || !timer.endsAt) {
      setMsLeft(timer.remainingMs)
      return
    }
    const tick = () => setMsLeft(Math.max(0, timer.endsAt! - Date.now()))
    tick()
    const iv = window.setInterval(tick, 100)
    return () => window.clearInterval(iv)
  }, [timer.running, timer.endsAt, timer.remainingMs, timer.id])

  const seconds = Math.ceil(msLeft / 1000)
  const low = msLeft <= 3500 && timer.running
  return (
    <div className="tv-timer">
      <div className={'tv-timer__ring' + (low ? ' tv-timer__ring--low' : '')}>{seconds}</div>
    </div>
  )
}

function MediaAtom({ atom }: { atom: Atom }) {
  const { signals } = useRoom()
  const mediaRef = useRef<HTMLVideoElement | HTMLAudioElement | null>(null)
  // Браузеры блокируют автоплей со звуком до первого касания экрана ТВ —
  // в этом случае показываем кнопку включения звука.
  const [blocked, setBlocked] = useState(false)

  // Управление воспроизведением со стороны ведущего (сигналы через WS).
  useEffect(() => {
    if (!signals || !mediaRef.current) return
    if (signals.kind === 'media_pause') mediaRef.current.pause()
    if (signals.kind === 'media_play') void mediaRef.current.play().catch(() => {})
  }, [signals])

  const bindRef = (el: HTMLVideoElement | HTMLAudioElement | null) => {
    mediaRef.current = el
    if (el) {
      el.play().then(() => setBlocked(false)).catch(() => setBlocked(true))
    }
  }

  const unblock = () => {
    if (mediaRef.current) {
      void mediaRef.current.play().then(() => setBlocked(false)).catch(() => {})
    }
  }

  switch (atom.kind) {
    case 'image':
      return atom.mediaUrl ? (
        <img className="tv-question__image" src={atom.mediaUrl} alt="Изображение к вопросу" />
      ) : null
    case 'video':
      return atom.mediaUrl ? (
        <video
          ref={bindRef}
          className="tv-question__video"
          src={atom.mediaUrl}
          autoPlay
          controls
        />
      ) : null
    case 'audio':
      return atom.mediaUrl ? (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '1vh' }}>
          <div className="tv-question__label">Аудио</div>
          <audio
            ref={bindRef}
            src={atom.mediaUrl}
            autoPlay
            controls
            style={{ width: '40vw' }}
          />
          {blocked && (
            <button className="tv-sound-btn" onClick={unblock}>
              <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" fill="currentColor" stroke="none" />
                <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
                <path d="M19.07 4.93a10 10 0 0 1 0 14.14" />
              </svg>
              Включить звук
            </button>
          )}
        </div>
      ) : null
    default:
      return null
  }
}

// Экран вопроса: текст/медиа, таймер, правильный ответ после раскрытия.
export default function QuestionView() {
  const { snapshot } = useRoom()
  const game = snapshot?.game
  const cur = game?.current
  if (!game || !cur) return null

  const textAtoms = (cur.scenario ?? []).filter((a) => a.kind === 'text' || a.kind === 'say')
  const mediaAtoms = (cur.scenario ?? []).filter(
    (a) => a.kind === 'image' || a.kind === 'audio' || a.kind === 'video',
  )

  return (
    <div className="tv-question">
      {game.timer && <GameTimer timer={game.timer} />}
      <div className="tv-question__meta">
        <span>{cur.themeName}</span>
        <span className="tv-question__price">{cur.price}</span>
      </div>
      {textAtoms.map((a, i) => (
        <div key={i} className="tv-question__text">
          {a.text}
        </div>
      ))}
      {mediaAtoms.map((a, i) => (
        <MediaAtom key={i} atom={a} />
      ))}
      {cur.answerShown && cur.answer && (
        <>
          <div className="tv-question__label">Правильный ответ</div>
          <div className="tv-question__answer">{cur.answer}</div>
          {(cur.answerScenario ?? []).map((a, i) =>
            a.kind === 'text' || a.kind === 'say' ? (
              <div key={`at${i}`} className="tv-question__text">{a.text}</div>
            ) : (
              <MediaAtom key={`am${i}`} atom={a} />
            ),
          )}
        </>
      )}
    </div>
  )
}
