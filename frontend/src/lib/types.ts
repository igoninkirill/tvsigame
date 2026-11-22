// Общие типы протокола сервер ↔ клиент (снапшот состояния комнаты).

export type RoomStatus =
  | 'waiting_for_host'
  | 'configuring'
  | 'ready'
  | 'active'
  | 'paused'
  | 'finished'
  | 'expired'

export interface Player {
  id: string
  name: string
  score: number
  position: number
  isActive: boolean
}

export interface SkeletonQuestion {
  price: number
  type: string
  used: boolean
}

export interface SkeletonTheme {
  name: string
  questions: SkeletonQuestion[]
}

export interface SkeletonRound {
  name: string
  type: 'standard' | 'final'
  themes: SkeletonTheme[]
}

export interface PackMeta {
  name: string
  author?: string
  logoUrl?: string
  rounds: SkeletonRound[]
}

export interface Atom {
  kind: 'text' | 'image' | 'audio' | 'video' | 'say' | string
  text?: string
  mediaUrl?: string
  time?: number
}

export interface Timer {
  id: number
  kind: string
  durationMs: number
  remainingMs: number
  endsAt?: number
  running: boolean
}

export interface TimersConfig {
  answerWindowMs: number
  answeringMs: number
  finalThinkingMs: number
  forAllMs: number
}

export interface CurrentQuestion {
  roundIndex: number
  themeIndex: number
  questionIndex: number
  themeName: string
  basePrice: number
  price: number
  type: string
  typeParams?: Record<string, string>
  scenario: Atom[]
  answerScenario?: Atom[]
  answer?: string
  answerShown: boolean
  answeredIds: string[]
  answeringId?: string
  forcedId?: string
  windowLeftMs?: number
  verdicts?: Record<string, boolean>
}

export interface AuctionState {
  highestBid: number
  highestBidderId?: string
  passedIds: string[]
  nominal: number
}

export interface FinalState {
  themeNames: string[]
  excluded: number[]
  finalistIds: string[]
  stakes?: Record<string, number>
  stakesRevealed: boolean
  question?: CurrentQuestion
  answers?: Record<string, boolean>
}

export interface GameState {
  stage: string
  roundIndex: number
  usedCells: Record<string, boolean>
  current?: CurrentQuestion
  intro?: { roundName: string; themeNames: string[] }
  auction?: AuctionState
  final?: FinalState
  timer?: Timer
  timers: TimersConfig
  falseStartGuard: boolean
  answerWindowOpen: boolean
  winnerId?: string
}

export interface GameEvent {
  type: string
  payload: Record<string, unknown>
}

export interface Snapshot {
  type: 'state'
  version: number
  room: { id: string; code: string; status: RoomStatus; hostConnected: boolean }
  players: Player[]
  pack: PackMeta | null
  game: GameState | null
  events: GameEvent[]
}

export interface HistoryEvent {
  seq: number
  type: string
  actor: string
  payload: Record<string, unknown>
  stateVersion: number
  at: string
}

export interface PackListItem {
  id: string
  name: string
  author: string
  version: string
  createdAt: string
  logoUrl?: string
  rounds: number
}

export type ConnectionStatus = 'connecting' | 'online' | 'offline'

// Типы команд игрового движка (см. backend/internal/game).
export const Cmd = {
  StartGame: 'START_GAME',
  BeginRound: 'BEGIN_ROUND',
  SelectQuestion: 'SELECT_QUESTION',
  AllowAnswers: 'ALLOW_ANSWERS',
  SelectResponder: 'SELECT_RESPONDER',
  SubmitVerdict: 'SUBMIT_VERDICT',
  ShowAnswer: 'SHOW_ANSWER',
  CloseQuestion: 'CLOSE_QUESTION',
  NextRound: 'NEXT_ROUND',
  StartTimer: 'START_TIMER',
  PauseTimer: 'PAUSE_TIMER',
  ResumeTimer: 'RESUME_TIMER',
  PlaceBid: 'PLACE_BID',
  PassBid: 'PASS_BID',
  EndAuction: 'END_AUCTION',
  TransferCat: 'TRANSFER_CAT',
  ExcludeFinalTheme: 'EXCLUDE_FINAL_THEME',
  SetFinalStake: 'SET_FINAL_STAKE',
  RevealFinalStakes: 'REVEAL_FINAL_STAKES',
  OpenFinalQuestion: 'OPEN_FINAL_QUESTION',
  SubmitFinalAnswer: 'SUBMIT_FINAL_ANSWER',
  FinishFinal: 'FINISH_FINAL',
  SetScore: 'SET_SCORE',
  UpdateTimers: 'UPDATE_TIMERS',
  SetFalseStartGuard: 'SET_FALSE_START_GUARD',
  PauseGame: 'PAUSE_GAME',
  ResumeGame: 'RESUME_GAME',
  UndoLastAction: 'UNDO_LAST_ACTION',
} as const
