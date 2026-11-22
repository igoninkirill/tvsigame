// Package game — игровой движок «Своей Игры».
//
// Движок реализован как конечный автомат состояний поверх PostgreSQL:
// каждая команда проходит валидацию, атомарно изменяет состояние (JSONB),
// счёт игроков и журнал событий, после чего версия состояния увеличивается.
//
// Клиенты НЕ хранят и НЕ вычисляют критическое состояние — сервер
// является единственным источником истины.
package game

import (
	"github.com/krutokir/svoya-igra/server/internal/model"
)

// Стадии игрового автомата.
const (
	StageLobby         = "lobby"           // игра не начата (настройка)
	StageRoundIntro    = "round_intro"     // заставка раунда: объявление тем
	StageBoard         = "board"           // игровое табло, выбор вопроса
	StageQuestion      = "question"        // вопрос предъявляется
	StageAnswerWindow  = "answer_window"   // окно возможности ответа открыто
	StageAnswering     = "answering"       // выбран отвечающий, идёт время на ответ
	StageAnswerShown   = "answer_shown"    // правильный ответ показан, вопрос завершён
	StageAuction       = "auction"         // торги аукциона
	StageCatTransfer   = "cat_transfer"    // выбор получателя «кота в мешке»
	StageForAll        = "for_all"         // вопрос для всех: обдумывание
	StageRoundEnd      = "round_end"       // раунд завершён, ожидание NEXT_ROUND
	StageFinalThemes   = "final_themes"    // финал: исключение тем
	StageFinalStakes   = "final_stakes"    // финал: ставки
	StageFinalQuestion = "final_question"  // финал: вопрос задан, таймер размышления
	StageFinalVerdicts = "final_verdicts"  // финал: проверка ответов
	StageResults       = "results"         // игра завершена, итоги
)

// Типы таймеров.
const (
	TimerAnswerWindow  = "answer_window"
	TimerAnswering     = "answering"
	TimerFinalThinking = "final_thinking"
	TimerForAll        = "for_all"
)

// Типы команд (command_type в API).
const (
	CmdStartGame          = "START_GAME"
	CmdBeginRound         = "BEGIN_ROUND" // переход от заставки раунда к табло
	CmdSelectQuestion     = "SELECT_QUESTION"
	CmdOpenQuestion       = "OPEN_QUESTION" // алиас повторного показа текущего вопроса
	CmdAllowAnswers       = "ALLOW_ANSWERS"
	CmdSelectResponder    = "SELECT_RESPONDER"
	CmdSubmitVerdict      = "SUBMIT_VERDICT"
	CmdShowAnswer         = "SHOW_ANSWER"
	CmdCloseQuestion      = "CLOSE_QUESTION"
	CmdNextRound          = "NEXT_ROUND"
	CmdStartTimer         = "START_TIMER"
	CmdPauseTimer         = "PAUSE_TIMER"
	CmdResumeTimer        = "RESUME_TIMER"
	CmdPlaceBid           = "PLACE_BID"
	CmdPassBid            = "PASS_BID"
	CmdEndAuction         = "END_AUCTION"
	CmdTransferCat        = "TRANSFER_CAT"
	CmdExcludeFinalTheme  = "EXCLUDE_FINAL_THEME"
	CmdSetFinalStake      = "SET_FINAL_STAKE"
	CmdRevealFinalStakes  = "REVEAL_FINAL_STAKES"
	CmdOpenFinalQuestion  = "OPEN_FINAL_QUESTION"
	CmdSubmitFinalAnswer  = "SUBMIT_FINAL_ANSWER"
	CmdFinishFinal        = "FINISH_FINAL"
	CmdSetScore           = "SET_SCORE"
	CmdUpdateTimers       = "UPDATE_TIMERS"
	CmdSetFalseStartGuard = "SET_FALSE_START_GUARD"
	CmdPauseGame          = "PAUSE_GAME"
	CmdResumeGame         = "RESUME_GAME"
	CmdUndoLastAction     = "UNDO_LAST_ACTION"
	CmdTimerExpired       = "TIMER_EXPIRED" // внутренняя команда движка
)

// Типы событий журнала.
const (
	EvGameStarted        = "GAME_STARTED"
	EvQuestionSelected   = "QUESTION_SELECTED"
	EvAnswerWindowOpened = "ANSWER_WINDOW_OPENED"
	EvResponderSelected  = "RESPONDER_SELECTED"
	EvVerdict            = "VERDICT"
	EvAnswerShown        = "ANSWER_SHOWN"
	EvQuestionClosed     = "QUESTION_CLOSED"
	EvScoreChanged       = "SCORE_CHANGED"
	EvTimerStarted       = "TIMER_STARTED"
	EvTimerPaused        = "TIMER_PAUSED"
	EvTimerExpired       = "TIMER_EXPIRED"
	EvRoundStarted       = "ROUND_STARTED"
	EvBidPlaced          = "BID_PLACED"
	EvBidPassed          = "BID_PASSED"
	EvAuctionEnded       = "AUCTION_ENDED"
	EvCatTransferred     = "CAT_TRANSFERRED"
	EvFinalThemeExcluded = "FINAL_THEME_EXCLUDED"
	EvFinalStakeSet      = "FINAL_STAKE_SET"
	EvFinalQuestionOpen  = "FINAL_QUESTION_OPENED"
	EvFinalAnswerChecked = "FINAL_ANSWER_CHECKED"
	EvGameFinished       = "GAME_FINISHED"
	EvPaused             = "GAME_PAUSED"
	EvResumed            = "GAME_RESUMED"
	EvUndone             = "ACTION_UNDONE"
	EvTimersUpdated      = "TIMERS_UPDATED"
	EvFalseStartUpdated  = "FALSE_START_UPDATED"
)

// Timer — серверный таймер. Сервер — источник истины: клиенты лишь
// отображают обратный отсчёт по EndsAt/RemainingMs.
type Timer struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	DurationMs  int64  `json:"durationMs"`
	RemainingMs int64  `json:"remainingMs"` // актуально когда Running=false
	EndsAt      int64  `json:"endsAt,omitempty"` // unix ms, когда Running=true
	Running     bool   `json:"running"`
}

// TimersConfig — настраиваемые длительности (мс). Значения не зашиты в код:
// хранятся в состоянии игры и меняются командой UPDATE_TIMERS.
type TimersConfig struct {
	AnswerWindowMs  int64 `json:"answerWindowMs"`
	AnsweringMs     int64 `json:"answeringMs"`
	FinalThinkingMs int64 `json:"finalThinkingMs"`
	ForAllMs        int64 `json:"forAllMs"`
}

func DefaultTimersConfig() TimersConfig {
	return TimersConfig{
		AnswerWindowMs:  7000,
		AnsweringMs:     15000,
		FinalThinkingMs: 60000,
		ForAllMs:        60000,
	}
}

// CurrentQuestion — активный вопрос.
type CurrentQuestion struct {
	RoundIndex      int               `json:"roundIndex"`
	ThemeIndex      int               `json:"themeIndex"`
	QuestionIndex   int               `json:"questionIndex"`
	ThemeName       string            `json:"themeName"`
	BasePrice       int64             `json:"basePrice"`
	Price           int64             `json:"price"` // эффективная стоимость (ставка аукциона и т.п.)
	Type            string            `json:"type"`
	TypeParams      map[string]string `json:"typeParams,omitempty"`
	Scenario        []model.Atom      `json:"scenario"`
	AnswerScenario  []model.Atom      `json:"answerScenario,omitempty"` // показывается при раскрытии ответа
	Answer          string            `json:"answer,omitempty"`
	AnswerShown     bool              `json:"answerShown"`
	AnsweredIDs     []string          `json:"answeredIds"`               // ошибшиеся на этом вопросе
	AnsweringID     string            `json:"answeringId,omitempty"`     // кто отвечает сейчас
	ForcedID        string            `json:"forcedId,omitempty"`        // единственный допустимый отвечающий (аукцион/кот/без риска)
	OpenedByID      string            `json:"openedById,omitempty"`      // кто открыл (для кота/без риска)
	WindowLeftMs    int64             `json:"windowLeftMs,omitempty"`    // остаток окна ответа на момент выбора отвечающего
	Verdicts        map[string]bool   `json:"verdicts"`                  // для вопроса «для всех» (nil только вне forAll)
}

// AuctionState — состояние торгов.
type AuctionState struct {
	HighestBid       int64    `json:"highestBid"`
	HighestBidderID  string   `json:"highestBidderId,omitempty"`
	PassedIDs        []string `json:"passedIds"`
	Nominal          int64    `json:"nominal"`
}

// IntroState — заставка раунда: объявление тем перед табло.
type IntroState struct {
	RoundName  string   `json:"roundName"`
	ThemeNames []string `json:"themeNames"`
}

// FinalState — состояние финального раунда.
type FinalState struct {
	ThemeNames     []string          `json:"themeNames"`
	Excluded       []int             `json:"excluded"`      // индексы исключённых тем
	FinalistIDs    []string          `json:"finalistIds"`
	Stakes         map[string]int64  `json:"stakes"`        // playerId -> ставка (скрыто до раскрытия)
	StakesRevealed bool              `json:"stakesRevealed"`
	Question       *CurrentQuestion  `json:"question,omitempty"`
	Answers        map[string]bool   `json:"answers"` // playerId -> верно/неверно
}

// State — полное игровое состояние комнаты (хранится в game_states.state).
type State struct {
	Stage            string         `json:"stage"`
	RoundIndex       int            `json:"roundIndex"`
	UsedCells        map[string]bool `json:"usedCells"` // question.ID -> использован
	Current          *CurrentQuestion `json:"current,omitempty"`
	Intro            *IntroState    `json:"intro,omitempty"`
	Auction          *AuctionState  `json:"auction,omitempty"`
	Final            *FinalState    `json:"final,omitempty"`
	Timer            *Timer         `json:"timer,omitempty"`
	Timers           TimersConfig   `json:"timers"`
	FalseStartGuard  bool           `json:"falseStartGuard"` // true: отвечать можно только после ALLOW_ANSWERS
	AnswerWindowOpen bool           `json:"answerWindowOpen"`
	WinnerID         string         `json:"winnerId,omitempty"`
}

func InitialState() State {
	return State{
		Stage:           StageLobby,
		UsedCells:       map[string]bool{},
		Timers:          DefaultTimersConfig(),
		FalseStartGuard: true,
	}
}

// QuestionID формирует стабильный идентификатор ячейки вопроса.
func QuestionID(round, theme, question int) string {
	return qid(round, theme, question)
}

func qid(r, t, q int) string {
	return "r" + itoa(r) + "/t" + itoa(t) + "/q" + itoa(q)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
