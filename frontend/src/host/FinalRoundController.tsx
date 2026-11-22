import { useState } from 'react'

import { useRoom } from '../lib/room'
import { useToast } from './common'
import { Cmd } from '../lib/types'
import { IconCheck, IconEye, IconFlag, IconX } from '../components/Icon'

// Управление финальным раундом: исключение тем, ставки, вопрос, вердикты.
export default function FinalRoundController() {
  const { snapshot, send } = useRoom()
  const { showToast } = useToast()
  const game = snapshot?.game
  const fin = game?.final
  if (!game || !fin) return null

  const players = snapshot?.players ?? []
  const finalists = players.filter((p) => (fin.finalistIds ?? []).includes(p.id))
  const stage = game.stage

  return (
    <>
      {stage === 'final_themes' && <ThemeExcluder />}
      {stage === 'final_stakes' && <StakesPanel />}
      {(stage === 'final_question' || stage === 'final_verdicts') && <FinalQuestionPanel />}
    </>
  )

  function ThemeExcluder() {
    async function exclude(themeIndex: number) {
      try {
        await send(Cmd.ExcludeFinalTheme, { themeIndex })
      } catch (e) {
        showToast(e instanceof Error ? e.message : 'Ошибка', true)
      }
    }
    return (
      <div className="host-section">
        <div className="host-section__title">Финал: исключение тем</div>
      {fin!.themeNames && (fin!.themeNames ?? []).map((name, i) => (
          <button
            key={i}
            className="btn"
            style={{ minHeight: 52, justifyContent: 'space-between' }}
            disabled={(fin!.excluded ?? []).includes(i)}
            onClick={() => exclude(i)}
          >
            <span>{name}</span>
            {!(fin!.excluded ?? []).includes(i) && <IconX width={18} height={18} />}
          </button>
        ))}
      </div>
    )
  }

  function StakesPanel() {
    const [stakes, setStakes] = useState<Record<string, string>>({})

    async function setStake(playerId: string, amount: number) {
      try {
        await send(Cmd.SetFinalStake, { playerId, amount })
      } catch (e) {
        showToast(e instanceof Error ? e.message : 'Ошибка', true)
      }
    }

    async function reveal() {
      try {
        await send(Cmd.RevealFinalStakes)
      } catch (e) {
        showToast(e instanceof Error ? e.message : 'Ошибка', true)
      }
    }

    async function openQuestion() {
      try {
        await send(Cmd.OpenFinalQuestion)
      } catch (e) {
        showToast(e instanceof Error ? e.message : 'Ошибка', true)
      }
    }

    return (
      <div className="host-section">
        <div className="host-section__title">Финал: ставки</div>
        {finalists.map((p) => {
          const saved = fin!.stakes && p.id in fin!.stakes
          return (
            <div key={p.id} className="stake-row">
              <span className="player-row__name">
                {p.name} (счёт {p.score})
                {saved && <span style={{ color: 'var(--success)' }}> — ставка принята</span>}
              </span>
              <div className="stake-row__input">
                <input
                  className="field"
                  type="number"
                  inputMode="numeric"
                  min={0}
                  max={Math.max(0, p.score)}
                  placeholder={`0–${Math.max(0, p.score)}`}
                  value={stakes[p.id] ?? ''}
                  onChange={(e) => setStakes({ ...stakes, [p.id]: e.target.value })}
                />
                <button
                  className="btn"
                  onClick={() => setStake(p.id, Number(stakes[p.id] ?? 0))}
                  disabled={stakes[p.id] === undefined || stakes[p.id] === ''}
                >
                  ОК
                </button>
                <button
                  className="btn btn--gold"
                  disabled={p.score <= 0}
                  onClick={() => setStake(p.id, p.score)}
                >
                  Ва-банк
                </button>
              </div>
            </div>
          )
        })}
        <div className="host-actions">
          <button className="btn" onClick={reveal} disabled={fin!.stakesRevealed}>
            <IconEye width={18} height={18} /> Раскрыть ставки
          </button>
          <button className="btn btn--gold" onClick={openQuestion} disabled={!fin!.stakesRevealed}>
            <IconFlag width={18} height={18} /> Открыть вопрос
          </button>
        </div>
      </div>
    )
  }

  function FinalQuestionPanel() {
    const q = fin!.question
    const answers = fin!.answers ?? {}

    async function verdict(playerId: string, correct: boolean) {
      try {
        await send(Cmd.SubmitFinalAnswer, { playerId, correct })
      } catch (e) {
        showToast(e instanceof Error ? e.message : 'Ошибка', true)
      }
    }

    async function finish() {
      try {
        await send(Cmd.FinishFinal)
      } catch (e) {
        showToast(e instanceof Error ? e.message : 'Ошибка', true)
      }
    }

    return (
      <>
        <div className="host-section">
          <div className="host-section__title">Финальный вопрос: {q?.themeName}</div>
          <div className="host-question__text">
            {(q?.scenario ?? [])
              .filter((a) => a.kind === 'text' || a.kind === 'say')
              .map((a) => a.text)
              .join(' ')}
          </div>
          {q?.answer && <div className="host-question__answer">Ответ: {q.answer}</div>}
        </div>
        <div className="host-section">
          <div className="host-section__title">Вердикты по ответам</div>
          {finalists.map((p) => {
            const v = answers[p.id]
            return (
              <div key={p.id} className="player-row">
                <span className="player-row__name">{p.name}</span>
                {v === undefined ? (
                  <div className="player-row__btns">
                    <button
                      className="verdict-btn verdict-btn--ok"
                      style={{ minHeight: 44, padding: '0.3rem 0.9rem', fontSize: '1rem' }}
                      onClick={() => verdict(p.id, true)}
                    >
                      <IconCheck width={18} height={18} /> Верно
                    </button>
                    <button
                      className="verdict-btn verdict-btn--no"
                      style={{ minHeight: 44, padding: '0.3rem 0.9rem', fontSize: '1rem' }}
                      onClick={() => verdict(p.id, false)}
                    >
                      <IconX width={18} height={18} /> Неверно
                    </button>
                  </div>
                ) : (
                  <span style={{ color: v ? 'var(--success)' : 'var(--danger)', fontWeight: 700 }}>
                    {v ? 'Верно' : 'Неверно'}
                  </span>
                )}
              </div>
            )
          })}
          {stage === 'final_verdicts' && (
            <button className="btn btn--gold btn--lg" onClick={finish}>
              Завершить игру
            </button>
          )}
        </div>
      </>
    )
  }
}
