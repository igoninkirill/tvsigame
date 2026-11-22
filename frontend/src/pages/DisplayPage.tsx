import { useMemo } from 'react'
import { Link, useParams } from 'react-router-dom'

import { RoomProvider, useRoom } from '../lib/room'
import { getDisplayToken } from '../lib/session'
import WaitingScreen from '../tv/WaitingScreen'
import LobbyScreen from '../tv/LobbyScreen'
import GameBoard from '../tv/GameBoard'
import RoundIntro from '../tv/RoundIntro'
import QuestionView from '../tv/QuestionView'
import SpecialScreen from '../tv/SpecialScreen'
import FinalRound from '../tv/FinalRound'
import ResultsScreen from '../tv/ResultsScreen'
import PlayerScoreboard from '../tv/PlayerScoreboard'
import ConnectionOverlay from '../tv/ConnectionOverlay'
import SoundManager from '../tv/SoundManager'

function DisplayContent() {
  const { snapshot } = useRoom()

  const stage = snapshot?.game?.stage ?? 'lobby'
  const status = snapshot?.room.status ?? 'waiting_for_host'
  const hasQuestion = !!snapshot?.game?.current
  const inFinal =
    stage === 'final_themes' || stage === 'final_stakes' || stage === 'final_question' || stage === 'final_verdicts'

  const main = useMemo(() => {
    if (!snapshot) return null
    if (status === 'waiting_for_host') return <WaitingScreen />
    if (status === 'configuring' || status === 'ready' || stage === 'lobby') return <LobbyScreen />
    if (stage === 'results') return <ResultsScreen />
    if (stage === 'round_intro') return <RoundIntro />
    if (inFinal) return <FinalRound />
    if (stage === 'auction' || stage === 'cat_transfer' || stage === 'for_all') return <SpecialScreen />
    if (hasQuestion && stage !== 'board') return <QuestionView />
    return <GameBoard />
  }, [snapshot, status, stage, hasQuestion, inFinal])

  if (!snapshot) {
    return (
      <div className="tv-root">
        <div className="tv-waiting">
          <div className="tv-waiting__title">Своя Игра</div>
          <div className="tv-waiting__hint">Подключение к серверу…</div>
        </div>
      </div>
    )
  }

  return (
    <div className="tv-root">
      {main}
      {status !== 'waiting_for_host' && <PlayerScoreboard />}
      <SoundManager />
      <ConnectionOverlay />
    </div>
  )
}

export default function DisplayPage() {
  const { roomId = '' } = useParams()
  const token = getDisplayToken(roomId)

  if (!token) {
    return (
      <div className="home-root">
        <h1 className="home-title">Своя Игра</h1>
        <p className="home-subtitle">
          Этот экран не привязан к комнате. Создайте новую игру на этом устройстве.
        </p>
        <Link className="btn btn--gold btn--lg" to="/room/new">
          Создать игру
        </Link>
      </div>
    )
  }

  return (
    <RoomProvider roomId={roomId} role="display" token={token}>
      <DisplayContent />
    </RoomProvider>
  )
}
