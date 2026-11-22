import { useEffect, useRef, useState } from 'react'

import { useRoom } from '../lib/room'
import {
  playAuction,
  playCat,
  playCorrect,
  playFinalDelete,
  playFinalQuestion,
  playForAll,
  playNoAnswers,
  playNoRisk,
  playResults,
  playRoundIntro,
  playWrong,
  setSoundEnabled,
  soundEnabled,
  stopFinalQuestion,
  stopRoundIntro,
} from './sounds'

// Следит за снапшотом и проигрывает озвучку ключевых моментов игры
// (аутентичные сэмплы телеигры, см. sounds.ts), плюс рендерит
// переключатель звука в углу ТВ.
export default function SoundManager() {
  const { snapshot } = useRoom()
  const [on, setOn] = useState(soundEnabled())
  // Снимок предыдущего состояния, чтобы звучать только на изменениях.
  const prev = useRef<{ version: number; stage?: string } | null>(null)

  useEffect(() => {
    const game = snapshot?.game
    if (!game || !snapshot) return
    const p = prev.current
    const first = !p
    const prevStage = p?.stage
    const prevVersion = p?.version ?? -1
    prev.current = { version: snapshot.version, stage: game.stage }
    if (first || snapshot.version === prevVersion) return // подключение/дубль — молчим

    // Смена стадий.
    if (game.stage !== prevStage) {
      if (prevStage === 'round_intro') stopRoundIntro()
      if (prevStage === 'final_question') stopFinalQuestion()
      switch (game.stage) {
        case 'round_intro':
          playRoundIntro()
          break
        case 'cat_transfer':
          playCat()
          break
        case 'auction':
          playAuction()
          break
        case 'for_all':
          playForAll()
          break
        case 'question':
          // Вопрос без риска озвучивается при открытии.
          if ((game.current as { type?: string } | null)?.type === 'noRisk') playNoRisk()
          break
        case 'final_question':
          playFinalQuestion()
          break
        case 'results':
          playResults()
          break
      }
    }

    // События последней команды.
    for (const ev of snapshot.events ?? []) {
      const payload = (ev.payload ?? {}) as { correct?: boolean; kind?: string }
      switch (ev.type) {
        case 'VERDICT':
          if (payload.correct === true) playCorrect()
          else if (payload.correct === false) playWrong()
          break
        case 'TIMER_EXPIRED':
          // «Время вышло, никто не ответил».
          if (payload.kind === 'answer_window') playNoAnswers()
          break
        case 'FINAL_THEME_EXCLUDED':
          playFinalDelete()
          break
        case 'FINAL_ANSWER_CHECKED':
          if (payload.correct === true) playCorrect()
          break
      }
    }
  }, [snapshot])

  const toggle = () => {
    const next = !on
    setOn(next)
    setSoundEnabled(next)
  }

  return (
    <button
      className={'tv-sound-toggle' + (on ? '' : ' tv-sound-toggle--off')}
      onClick={toggle}
      aria-label={on ? 'Выключить звук' : 'Включить звук'}
      title={on ? 'Выключить звук' : 'Включить звук'}
    >
      {on ? (
        <svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" fill="currentColor" stroke="none" />
          <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
          <path d="M19.07 4.93a10 10 0 0 1 0 14.14" />
        </svg>
      ) : (
        <svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" fill="currentColor" stroke="none" />
          <line x1="23" y1="9" x2="17" y2="15" />
          <line x1="17" y1="9" x2="23" y2="15" />
        </svg>
      )}
    </button>
  )
}
