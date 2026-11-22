import { useState } from 'react'

import { useRoom } from '../lib/room'
import { useToast } from './common'
import { Cmd } from '../lib/types'
import GameBoardController from './GameBoardController'
import QuestionController from './QuestionController'
import AuctionController from './AuctionController'
import CatController from './CatController'
import ForAllController from './ForAllController'
import FinalRoundController from './FinalRoundController'
import PlayerManager from './PlayerManager'
import PackManager from './PackManager'
import MorePanel from './MorePanel'
import {
  IconCrown,
  IconPackage,
  IconPlay,
  IconSettings,
  IconSkipForward,
  IconTv,
  IconUsers,
  IconWifi,
  IconWifiOff,
} from '../components/Icon'

type Tab = 'game' | 'players' | 'pack' | 'more'

export default function HostDashboard() {
  const { snapshot, status, send } = useRoom()
  const { showToast } = useToast()
  const [tab, setTab] = useState<Tab>('game')

  const room = snapshot?.room
  const game = snapshot?.game
  const stage = game?.stage ?? 'lobby'

  async function run(type: string) {
    try {
      await send(type)
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Ошибка', true)
    }
  }

  return (
    <div className="host-root">
      <div className="host-header">
        <span className="host-header__title">
          <IconTv width={20} height={20} /> Комната {room?.code ?? '…'}
        </span>
        <span className={'conn' + (status === 'online' ? ' conn--online' : ' conn--offline')}>
          {status === 'online' ? <IconWifi width={16} height={16} /> : <IconWifiOff width={16} height={16} />}
          {status === 'online' ? 'онлайн' : 'нет связи'}
        </span>
      </div>

      {room?.status === 'paused' && (
        <div className="host-section" style={{ borderColor: 'var(--gold)' }}>
          <div className="host-section__title">Игра на паузе</div>
          <button className="btn btn--gold" onClick={() => run(Cmd.ResumeGame)}>
            <IconPlay width={18} height={18} /> Продолжить игру
          </button>
        </div>
      )}

      {tab === 'game' && <GameTab stage={stage} onStartGame={() => run(Cmd.StartGame)} onNextRound={() => run(Cmd.NextRound)} onBeginRound={() => run(Cmd.BeginRound)} />}
      {tab === 'players' && <PlayerManager />}
      {tab === 'pack' && <PackManager />}
      {tab === 'more' && <MorePanel />}

      <div className="host-nav">
        <button className={'host-nav__btn' + (tab === 'game' ? ' host-nav__btn--active' : '')} onClick={() => setTab('game')}>
          <IconPlay width={20} height={20} />
          Игра
        </button>
        <button className={'host-nav__btn' + (tab === 'players' ? ' host-nav__btn--active' : '')} onClick={() => setTab('players')}>
          <IconUsers width={20} height={20} />
          Игроки
        </button>
        <button className={'host-nav__btn' + (tab === 'pack' ? ' host-nav__btn--active' : '')} onClick={() => setTab('pack')}>
          <IconPackage width={20} height={20} />
          Пакет
        </button>
        <button className={'host-nav__btn' + (tab === 'more' ? ' host-nav__btn--active' : '')} onClick={() => setTab('more')}>
          <IconSettings width={20} height={20} />
          Ещё
        </button>
      </div>
    </div>
  )
}

function GameTab(props: { stage: string; onStartGame: () => void; onNextRound: () => void; onBeginRound: () => void }) {
  const { snapshot } = useRoom()
  const { stage } = props
  const pack = snapshot?.pack
  const players = snapshot?.players ?? []

  switch (stage) {
    case 'lobby':
      return (
        <div className="host-section">
          <div className="host-section__title">Подготовка</div>
          <p style={{ color: 'var(--text-dim)', fontSize: '0.9rem' }}>
            {!pack && '1. Загрузите пакет вопросов (вкладка «Пакет»). '}
            {players.length === 0 && '2. Добавьте игроков (вкладка «Игроки»).'}
          </p>
          <button
            className="btn btn--gold btn--lg"
            disabled={!pack || players.length === 0}
            onClick={props.onStartGame}
          >
            <IconPlay width={20} height={20} /> Начать игру
          </button>
        </div>
      )
    case 'round_intro': {
      const intro = snapshot?.game?.intro
      return (
        <div className="host-section">
          <div className="host-section__title">{intro?.roundName ?? 'Новый раунд'}</div>
          <p style={{ color: 'var(--text-dim)', fontSize: '0.9rem' }}>
            На ТВ объявляются темы раунда:
          </p>
          {(intro?.themeNames ?? []).map((name, i) => (
            <div key={i} className="player-row">
              <span className="player-row__name">{name}</span>
            </div>
          ))}
          <button className="btn btn--gold btn--lg" onClick={() => props.onBeginRound()}>
            <IconPlay width={20} height={20} /> К табло
          </button>
        </div>
      )
    }
    case 'board':
      return <GameBoardController />
    case 'round_end':
      return (
        <div className="host-section">
          <div className="host-section__title">Раунд завершён</div>
          <button className="btn btn--gold btn--lg" onClick={props.onNextRound}>
            <IconSkipForward width={20} height={20} /> Следующий раунд
          </button>
        </div>
      )
    case 'question':
    case 'answer_window':
    case 'answering':
    case 'answer_shown':
      return <QuestionController />
    case 'auction':
      return <AuctionController />
    case 'cat_transfer':
      return <CatController />
    case 'for_all':
      return <ForAllController />
    case 'final_themes':
    case 'final_stakes':
    case 'final_question':
    case 'final_verdicts':
      return <FinalRoundController />
    case 'results': {
      const sorted = [...players].sort((a, b) => b.score - a.score)
      const winner = snapshot?.game?.winnerId
      return (
        <div className="host-section">
          <div className="host-section__title">
            <IconCrown width={18} height={18} /> Итоги игры
          </div>
          {sorted.map((p, i) => (
            <div key={p.id} className="player-row">
              <span className="player-row__name">
                {i + 1}. {p.name} {p.id === winner ? '— победитель' : ''}
              </span>
              <span className="player-row__score">{p.score}</span>
            </div>
          ))}
        </div>
      )
    }
    default:
      return null
  }
}
