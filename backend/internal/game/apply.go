package game

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/krutokir/svoya-igra/server/internal/model"
)

// applyCtx — контекст применения одной команды внутри транзакции.
type applyCtx struct {
	ctx        context.Context
	tx         pgx.Tx
	roomID     string
	cmd        Command
	state      *State
	pack       *model.Pack
	scores     map[string]int64 // playerID -> score (до команды)
	roomStatus string

	newRoomStatus string
	events        []map[string]any
}

func (a *applyCtx) ev(eventType string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	a.events = append(a.events, map[string]any{"type": eventType, "payload": payload})
}

func (a *applyCtx) decode(v any) error {
	if len(a.cmd.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(a.cmd.Payload, v); err != nil {
		return fail("BAD_PAYLOAD", "некорректный payload: %v", err)
	}
	return nil
}

// apply — диспетчер команд конечного автомата.
func (a *applyCtx) apply() error {
	// Глобальные стражи статуса комнаты.
	switch a.cmd.Type {
	case CmdResumeGame:
		if a.roomStatus != "paused" {
			return fail("BAD_STATE", "игра не на паузе")
		}
	case CmdStartGame, CmdSetScore, CmdUpdateTimers, CmdSetFalseStartGuard:
		// допустимы в configuring/ready/active
		if a.roomStatus == "finished" || a.roomStatus == "expired" || a.roomStatus == "waiting_for_host" {
			return fail("BAD_STATE", "команда недопустима в статусе комнаты %s", a.roomStatus)
		}
	case CmdUndoLastAction:
		if a.roomStatus == "expired" || a.roomStatus == "waiting_for_host" {
			return fail("BAD_STATE", "команда недопустима в статусе комнаты %s", a.roomStatus)
		}
	default:
		if a.roomStatus != "active" {
			return fail("BAD_STATE", "игра не активна (статус: %s)", a.roomStatus)
		}
	}

	switch a.cmd.Type {
	case CmdStartGame:
		return a.startGame()
	case CmdBeginRound:
		return a.beginRound()
	case CmdSelectQuestion, CmdOpenQuestion:
		return a.selectQuestion()
	case CmdAllowAnswers:
		return a.allowAnswers()
	case CmdSelectResponder:
		return a.selectResponder()
	case CmdSubmitVerdict:
		return a.submitVerdict()
	case CmdShowAnswer:
		return a.showAnswer()
	case CmdCloseQuestion:
		return a.closeQuestion()
	case CmdNextRound:
		return a.nextRound()
	case CmdStartTimer:
		return a.timerControl("start")
	case CmdPauseTimer:
		return a.timerControl("pause")
	case CmdResumeTimer:
		return a.timerControl("resume")
	case CmdPlaceBid:
		return a.placeBid()
	case CmdPassBid:
		return a.passBid()
	case CmdEndAuction:
		return a.endAuction()
	case CmdTransferCat:
		return a.transferCat()
	case CmdExcludeFinalTheme:
		return a.excludeFinalTheme()
	case CmdSetFinalStake:
		return a.setFinalStake()
	case CmdRevealFinalStakes:
		return a.revealFinalStakes()
	case CmdOpenFinalQuestion:
		return a.openFinalQuestion()
	case CmdSubmitFinalAnswer:
		return a.submitFinalAnswer()
	case CmdFinishFinal:
		return a.finishFinal()
	case CmdSetScore:
		return a.setScore()
	case CmdUpdateTimers:
		return a.updateTimers()
	case CmdSetFalseStartGuard:
		return a.setFalseStartGuard()
	case CmdPauseGame:
		return a.pauseGame()
	case CmdResumeGame:
		return a.resumeGame()
	case CmdUndoLastAction:
		return a.undoLastAction()
	case CmdTimerExpired:
		return a.timerExpired()
	default:
		return fail("UNKNOWN_COMMAND", "неизвестный тип команды: %s", a.cmd.Type)
	}
}

// --- вспомогательные операции ---

// addScore атомарно изменяет счёт игрока и пишет событие.
func (a *applyCtx) addScore(playerID string, delta int64, reason string) error {
	old := a.scores[playerID]
	newScore := old + delta
	if _, err := a.tx.Exec(a.ctx,
		`UPDATE players SET score = $2 WHERE id = $1 AND room_id = $3`, playerID, newScore, a.roomID); err != nil {
		return err
	}
	a.scores[playerID] = newScore
	a.ev(EvScoreChanged, map[string]any{
		"playerId": playerID, "delta": delta, "oldScore": old, "newScore": newScore, "reason": reason,
	})
	return nil
}

func (a *applyCtx) setTimer(kind string, durationMs int64, running bool) {
	t := &Timer{
		ID:          time.Now().UnixNano(),
		Kind:        kind,
		DurationMs:  durationMs,
		RemainingMs: durationMs,
	}
	if running {
		startTimer(t)
		a.ev(EvTimerStarted, map[string]any{"kind": kind, "durationMs": durationMs})
	}
	a.state.Timer = t
}

func (a *applyCtx) clearTimer() {
	a.state.Timer = nil
}

// requireStage проверяет текущую стадию автомата.
func (a *applyCtx) requireStage(stages ...string) error {
	cur := a.state.Stage
	for _, s := range stages {
		if cur == s {
			return nil
		}
	}
	return fail("BAD_STAGE", "команда недопустима на стадии %q", cur)
}

// --- игровые команды ---

func (a *applyCtx) startGame() error {
	if err := a.requireStage(StageLobby); err != nil {
		return err
	}
	if a.pack == nil {
		return fail("NO_PACK", "пак вопросов не загружен")
	}
	if len(a.pack.Rounds) == 0 {
		return fail("NO_PACK", "в пакете нет раундов")
	}
	ids, err := activePlayerIDs(a.ctx, a.tx, a.roomID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fail("NO_PLAYERS", "добавьте хотя бы одного игрока")
	}
	a.state.RoundIndex = 0
	a.state.UsedCells = map[string]bool{}
	a.newRoomStatus = "active"
	a.ev(EvGameStarted, map[string]any{"rounds": len(a.pack.Rounds)})
	a.ev(EvRoundStarted, map[string]any{"roundIndex": 0, "name": a.pack.Rounds[0].Name})
	if a.pack.Rounds[0].Type == "final" {
		return a.initFinal(a.pack.Rounds[0])
	}
	a.startRoundIntro(a.pack.Rounds[0])
	return nil
}

// startRoundIntro переводит игру в заставку раунда с объявлением тем.
func (a *applyCtx) startRoundIntro(round model.Round) {
	names := make([]string, 0, len(round.Themes))
	for _, t := range round.Themes {
		names = append(names, t.Name)
	}
	a.state.Stage = StageRoundIntro
	a.state.Intro = &IntroState{RoundName: round.Name, ThemeNames: names}
}

// beginRound завершает заставку и открывает табло (можно и досрочно).
func (a *applyCtx) beginRound() error {
	if err := a.requireStage(StageRoundIntro); err != nil {
		return err
	}
	a.state.Intro = nil
	a.state.Stage = StageBoard
	return nil
}

type pSelectQuestion struct {
	ThemeIndex    int `json:"themeIndex"`
	QuestionIndex int `json:"questionIndex"`
}

func (a *applyCtx) selectQuestion() error {
	if a.cmd.Type == CmdOpenQuestion {
		// Повторный показ уже выбранного вопроса (например, после сбоя на ТВ).
		if a.state.Current == nil {
			return fail("NO_QUESTION", "нет активного вопроса")
		}
		return nil
	}
	if err := a.requireStage(StageBoard); err != nil {
		return err
	}
	if a.pack == nil {
		return fail("NO_PACK", "пак не загружен")
	}
	var p pSelectQuestion
	if err := a.decode(&p); err != nil {
		return err
	}
	q, themeName, err := a.questionAt(a.state.RoundIndex, p.ThemeIndex, p.QuestionIndex)
	if err != nil {
		return err
	}
	if a.state.UsedCells[q.ID] {
		return fail("CELL_USED", "вопрос уже использован")
	}

	cur := &CurrentQuestion{
		RoundIndex:    a.state.RoundIndex,
		ThemeIndex:    p.ThemeIndex,
		QuestionIndex: p.QuestionIndex,
		ThemeName:     themeName,
		BasePrice:     q.Price,
		Price:         q.Price,
		Type:          q.Type,
		TypeParams:    q.TypeParams,
		Scenario:      q.Scenario,
		AnswerScenario: q.AnswerScenario,
		Answer:        q.RightAnswer(),
		AnsweredIDs:   []string{},
	}
	if cur.Scenario == nil {
		cur.Scenario = []model.Atom{}
	}
	a.state.Current = cur
	a.state.AnswerWindowOpen = false

	payload := map[string]any{
		"theme": themeName, "price": q.Price, "type": q.Type,
		"themeIndex": p.ThemeIndex, "questionIndex": p.QuestionIndex,
	}
	if isUnknown(q.Type) {
		payload["warning"] = "неподдерживаемый тип вопроса — проведите вручную"
		payload["typeParams"] = q.TypeParams
	}

	switch q.Type {
	case "auction":
		a.state.Stage = StageAuction
		a.state.Auction = &AuctionState{Nominal: q.Price, PassedIDs: []string{}}
	case "cat":
		a.state.Stage = StageCatTransfer
	case "forAll":
		a.state.Stage = StageForAll
		cur.Verdicts = map[string]bool{}
		a.setTimer(TimerForAll, a.state.Timers.ForAllMs, true)
	default: // regular, noRisk, unknown:*
		a.state.Stage = StageQuestion
	}
	a.ev(EvQuestionSelected, payload)
	return nil
}

func (a *applyCtx) allowAnswers() error {
	if err := a.requireStage(StageQuestion); err != nil {
		return err
	}
	a.state.Stage = StageAnswerWindow
	a.state.AnswerWindowOpen = true
	a.setTimer(TimerAnswerWindow, a.state.Timers.AnswerWindowMs, true)
	a.ev(EvAnswerWindowOpened, nil)
	return nil
}

type pSelectResponder struct {
	PlayerID string `json:"playerId"`
}

func (a *applyCtx) selectResponder() error {
	cur := a.state.Current
	if cur == nil {
		return fail("NO_QUESTION", "нет активного вопроса")
	}
	// Фальстарты: при включённой защите отвечать можно только после ALLOW_ANSWERS.
	if a.state.Stage == StageQuestion {
		if a.state.FalseStartGuard {
			return fail("FALSE_START", "сначала откройте окно ответа (ALLOW_ANSWERS)")
		}
		a.state.Stage = StageAnswerWindow
		a.state.AnswerWindowOpen = true
		a.setTimer(TimerAnswerWindow, a.state.Timers.AnswerWindowMs, true)
	}
	if err := a.requireStage(StageAnswerWindow); err != nil {
		return err
	}
	var p pSelectResponder
	if err := a.decode(&p); err != nil {
		return err
	}
	if _, ok := a.scores[p.PlayerID]; !ok {
		return fail("NO_PLAYER", "игрок не найден")
	}
	for _, id := range cur.AnsweredIDs {
		if id == p.PlayerID {
			return fail("ALREADY_ANSWERED", "игрок уже отвечал на этот вопрос")
		}
	}
	if cur.ForcedID != "" && cur.ForcedID != p.PlayerID {
		return fail("NOT_ALLOWED", "на этот вопрос может отвечать только выбранный игрок")
	}
	// Останавливаем окно ответа, сохраняя остаток для продолжения после неверного ответа.
	if a.state.Timer != nil && a.state.Timer.Kind == TimerAnswerWindow {
		pauseTimer(a.state.Timer)
		cur.WindowLeftMs = a.state.Timer.RemainingMs
	}
	cur.AnsweringID = p.PlayerID
	a.state.Stage = StageAnswering
	a.setTimer(TimerAnswering, a.state.Timers.AnsweringMs, true)
	a.ev(EvResponderSelected, map[string]any{"playerId": p.PlayerID})
	return nil
}

type pVerdict struct {
	PlayerID string `json:"playerId"`
	Correct  bool   `json:"correct"`
}

func (a *applyCtx) submitVerdict() error {
	cur := a.state.Current
	if cur == nil {
		return fail("NO_QUESTION", "нет активного вопроса")
	}
	var p pVerdict
	if err := a.decode(&p); err != nil {
		return err
	}

	// Вопрос «для всех»: вердикт по конкретному игроку.
	if a.state.Stage == StageForAll {
		if _, ok := a.scores[p.PlayerID]; !ok {
			return fail("NO_PLAYER", "игрок не найден")
		}
		if cur.Verdicts == nil {
			cur.Verdicts = map[string]bool{}
		}
		cur.Verdicts[p.PlayerID] = p.Correct
		a.ev(EvVerdict, map[string]any{"playerId": p.PlayerID, "correct": p.Correct})
		// Когда все активные игроки получили вердикт — закрываем вопрос.
		ids, err := activePlayerIDs(a.ctx, a.tx, a.roomID)
		if err != nil {
			return err
		}
		all := true
		for _, id := range ids {
			if _, ok := cur.Verdicts[id]; !ok {
				all = false
				break
			}
		}
		if all {
			for id, ok := range cur.Verdicts {
				delta := cur.Price
				if !ok {
					delta = -delta
				}
				if err := a.addScore(id, delta, "for_all"); err != nil {
					return err
				}
			}
			a.finishQuestion()
		}
		return nil
	}

	if err := a.requireStage(StageAnswering); err != nil {
		return err
	}
	playerID := cur.AnsweringID
	a.ev(EvVerdict, map[string]any{"playerId": playerID, "correct": p.Correct})

	if p.Correct {
		if err := a.addScore(playerID, cur.Price, "correct_answer"); err != nil {
			return err
		}
		a.finishQuestion()
		return nil
	}

	// Неверный ответ.
	if cur.Type != "noRisk" {
		if err := a.addScore(playerID, -cur.Price, "wrong_answer"); err != nil {
			return err
		}
	}
	cur.AnsweredIDs = append(cur.AnsweredIDs, playerID)
	cur.AnsweringID = ""

	// Вопросы с единственным отвечающим завершаются после ошибки.
	if cur.ForcedID != "" || cur.Type == "noRisk" {
		a.finishQuestion()
		return nil
	}

	// Возвращаемся к окну ответа: остальные могут попробовать.
	windowLeft := cur.WindowLeftMs
	if windowLeft <= 0 {
		windowLeft = a.state.Timers.AnswerWindowMs / 2 // запасной минимум
	}
	a.state.Stage = StageAnswerWindow
	a.setTimer(TimerAnswerWindow, windowLeft, true)
	return nil
}

func (a *applyCtx) finishQuestion() {
	cur := a.state.Current
	if cur == nil {
		return
	}
	cur.AnswerShown = true
	a.state.UsedCells[qid(cur.RoundIndex, cur.ThemeIndex, cur.QuestionIndex)] = true
	a.state.Stage = StageAnswerShown
	a.clearTimer()
	a.ev(EvAnswerShown, map[string]any{"answer": cur.Answer})
}

func (a *applyCtx) showAnswer() error {
	if err := a.requireStage(StageQuestion, StageAnswerWindow, StageAnswering, StageAnswerShown); err != nil {
		return err
	}
	if !a.state.Current.AnswerShown {
		a.finishQuestion()
	}
	return nil
}

func (a *applyCtx) closeQuestion() error {
	if err := a.requireStage(StageAnswerShown); err != nil {
		return err
	}
	cur := a.state.Current
	a.ev(EvQuestionClosed, map[string]any{
		"themeIndex": cur.ThemeIndex, "questionIndex": cur.QuestionIndex,
	})
	a.state.Current = nil
	a.state.Auction = nil
	a.state.AnswerWindowOpen = false
	a.clearTimer()
	if a.roundComplete(a.state.RoundIndex) {
		a.state.Stage = StageRoundEnd
	} else {
		a.state.Stage = StageBoard
	}
	return nil
}

func (a *applyCtx) nextRound() error {
	if err := a.requireStage(StageRoundEnd, StageBoard); err != nil {
		return err
	}
	if a.state.Stage == StageBoard && !a.roundComplete(a.state.RoundIndex) {
		return fail("ROUND_INCOMPLETE", "в раунде остались неразыгранные вопросы")
	}
	if a.pack == nil {
		return fail("NO_PACK", "пак не загружен")
	}
	next := a.state.RoundIndex + 1
	if next >= len(a.pack.Rounds) {
		// Раундов больше нет — завершаем игру без финала.
		return a.finishGameNoFinal()
	}
	a.state.RoundIndex = next
	round := a.pack.Rounds[next]
	a.ev(EvRoundStarted, map[string]any{"roundIndex": next, "name": round.Name, "type": round.Type})
	if round.Type == "final" {
		return a.initFinal(round)
	}
	a.startRoundIntro(round)
	return nil
}

// --- таймеры ---

func (a *applyCtx) timerControl(op string) error {
	t := a.state.Timer
	if t == nil {
		return fail("NO_TIMER", "нет активного таймера")
	}
	switch op {
	case "start":
		if t.Running {
			return fail("TIMER_RUNNING", "таймер уже запущен")
		}
		startTimer(t)
		a.ev(EvTimerStarted, map[string]any{"kind": t.Kind})
	case "pause":
		if !t.Running {
			return fail("TIMER_NOT_RUNNING", "таймер не запущен")
		}
		pauseTimer(t)
		a.ev(EvTimerPaused, map[string]any{"kind": t.Kind, "remainingMs": t.RemainingMs})
	case "resume":
		if t.Running {
			return fail("TIMER_RUNNING", "таймер уже запущен")
		}
		startTimer(t)
		a.ev(EvTimerStarted, map[string]any{"kind": t.Kind})
	}
	return nil
}

type pTimerExpired struct {
	TimerID int64 `json:"timerId"`
}

func (a *applyCtx) timerExpired() error {
	var p pTimerExpired
	if err := a.decode(&p); err != nil {
		return err
	}
	t := a.state.Timer
	if t == nil || t.ID != p.TimerID || !t.Running {
		// Таймер уже был остановлен/заменён — событие устарело.
		return fail("STALE_TIMER", "таймер уже неактивен")
	}
	a.ev(EvTimerExpired, map[string]any{"kind": t.Kind})

	switch t.Kind {
	case TimerAnswerWindow:
		// Время вышло: новых попыток нет, раскрываем ответ.
		a.finishQuestion()
	case TimerAnswering:
		cur := a.state.Current
		if cur == nil || a.state.Stage != StageAnswering {
			return fail("BAD_STAGE", "некорректная стадия для таймера ответа")
		}
		playerID := cur.AnsweringID
		a.ev(EvVerdict, map[string]any{"playerId": playerID, "correct": false, "reason": "timeout"})
		if cur.Type != "noRisk" {
			if err := a.addScore(playerID, -cur.Price, "answer_timeout"); err != nil {
				return err
			}
		}
		cur.AnsweredIDs = append(cur.AnsweredIDs, playerID)
		cur.AnsweringID = ""
		if cur.ForcedID != "" || cur.Type == "noRisk" {
			a.finishQuestion()
			return nil
		}
		a.state.Stage = StageAnswerWindow
		left := cur.WindowLeftMs
		if left <= 0 {
			left = a.state.Timers.AnswerWindowMs / 2
		}
		a.setTimer(TimerAnswerWindow, left, true)
	case TimerFinalThinking:
		if a.state.Stage != StageFinalQuestion {
			return fail("BAD_STAGE", "некорректная стадия для финального таймера")
		}
		a.state.Stage = StageFinalVerdicts
		a.clearTimer()
	case TimerForAll:
		// Время обдумывания истекло — ведущий фиксирует ответы вручную.
		a.clearTimer()
	}
	return nil
}

// --- аукцион ---

type pBid struct {
	PlayerID string `json:"playerId"`
	Amount   int64  `json:"amount"`
}

func (a *applyCtx) placeBid() error {
	if err := a.requireStage(StageAuction); err != nil {
		return err
	}
	var p pBid
	if err := a.decode(&p); err != nil {
		return err
	}
	auc := a.state.Auction
	score, ok := a.scores[p.PlayerID]
	if !ok {
		return fail("NO_PLAYER", "игрок не найден")
	}
	for _, id := range auc.PassedIDs {
		if id == p.PlayerID {
			return fail("ALREADY_PASSED", "игрок отказался от торгов")
		}
	}
	if auc.HighestBidderID == p.PlayerID {
		return fail("ALREADY_HIGHEST", "ставка этого игрока уже наивысшая")
	}
	if p.Amount > score {
		return fail("BID_TOO_HIGH", "ставка превышает счёт игрока (%d)", score)
	}
	if auc.HighestBid == 0 && p.Amount < auc.Nominal {
		return fail("BID_TOO_LOW", "минимальная ставка — номинал вопроса (%d)", auc.Nominal)
	}
	if p.Amount <= auc.HighestBid {
		return fail("BID_TOO_LOW", "ставка должна перекрывать текущую (%d)", auc.HighestBid)
	}
	auc.HighestBid = p.Amount
	auc.HighestBidderID = p.PlayerID
	a.ev(EvBidPlaced, map[string]any{"playerId": p.PlayerID, "amount": p.Amount, "allIn": p.Amount == score})
	return nil
}

func (a *applyCtx) passBid() error {
	if err := a.requireStage(StageAuction); err != nil {
		return err
	}
	var p pBid
	if err := a.decode(&p); err != nil {
		return err
	}
	auc := a.state.Auction
	if _, ok := a.scores[p.PlayerID]; !ok {
		return fail("NO_PLAYER", "игрок не найден")
	}
	for _, id := range auc.PassedIDs {
		if id == p.PlayerID {
			return fail("ALREADY_PASSED", "игрок уже спасовал")
		}
	}
	auc.PassedIDs = append(auc.PassedIDs, p.PlayerID)
	a.ev(EvBidPassed, map[string]any{"playerId": p.PlayerID})

	// Если спасовали все активные игроки, кроме возможного лидера — торги кончены.
	ids, err := activePlayerIDs(a.ctx, a.tx, a.roomID)
	if err != nil {
		return err
	}
	remaining := 0
	for _, id := range ids {
		passed := false
		for _, pid := range auc.PassedIDs {
			if pid == id {
				passed = true
				break
			}
		}
		if !passed && id != auc.HighestBidderID {
			remaining++
		}
	}
	if remaining == 0 {
		if auc.HighestBidderID == "" {
			// Никто не сделал ставку — вопрос сгорает.
			a.ev(EvAuctionEnded, map[string]any{"burned": true})
			a.finishQuestion()
			return nil
		}
		return a.finishAuction()
	}
	return nil
}

func (a *applyCtx) endAuction() error {
	if err := a.requireStage(StageAuction); err != nil {
		return err
	}
	return a.finishAuction()
}

func (a *applyCtx) finishAuction() error {
	auc := a.state.Auction
	if auc == nil || auc.HighestBidderID == "" {
		return fail("NO_BIDS", "нет ни одной ставки")
	}
	cur := a.state.Current
	cur.Price = auc.HighestBid
	cur.ForcedID = auc.HighestBidderID
	a.state.Stage = StageQuestion
	a.ev(EvAuctionEnded, map[string]any{
		"winnerId": auc.HighestBidderID, "price": auc.HighestBid,
	})
	return nil
}

// --- кот в мешке ---

type pTransferCat struct {
	PlayerID string `json:"playerId"`
}

func (a *applyCtx) transferCat() error {
	if err := a.requireStage(StageCatTransfer); err != nil {
		return err
	}
	var p pTransferCat
	if err := a.decode(&p); err != nil {
		return err
	}
	if _, ok := a.scores[p.PlayerID]; !ok {
		return fail("NO_PLAYER", "игрок не найден")
	}
	cur := a.state.Current
	// Настройки из пакета: фиксированная стоимость кота.
	if v, ok := cur.TypeParams["price"]; ok {
		var price int64
		if _, err := fmt.Sscanf(v, "%d", &price); err == nil && price > 0 {
			cur.Price = price
		}
	}
	cur.ForcedID = p.PlayerID
	a.state.Stage = StageQuestion
	a.ev(EvCatTransferred, map[string]any{"toPlayerId": p.PlayerID, "price": cur.Price})
	return nil
}

// --- финальный раунд ---

func (a *applyCtx) initFinal(round model.Round) error {
	ids, err := activePlayerIDs(a.ctx, a.tx, a.roomID)
	if err != nil {
		return err
	}
	// К финалу допускаются игроки с положительным счётом;
	// если таких нет — все активные (крайний случай).
	finalists := []string{}
	for _, id := range ids {
		if a.scores[id] > 0 {
			finalists = append(finalists, id)
		}
	}
	if len(finalists) == 0 {
		finalists = ids
	}
	themes := make([]string, 0, len(round.Themes))
	for _, t := range round.Themes {
		themes = append(themes, t.Name)
	}
	a.state.Final = &FinalState{
		ThemeNames:  themes,
		Excluded:    []int{},
		FinalistIDs: finalists,
		Stakes:      map[string]int64{},
		Answers:     map[string]bool{},
	}
	a.state.Stage = StageFinalThemes
	return nil
}

type pExcludeTheme struct {
	ThemeIndex int `json:"themeIndex"`
}

func (a *applyCtx) excludeFinalTheme() error {
	if err := a.requireStage(StageFinalThemes); err != nil {
		return err
	}
	var p pExcludeTheme
	if err := a.decode(&p); err != nil {
		return err
	}
	fin := a.state.Final
	if p.ThemeIndex < 0 || p.ThemeIndex >= len(fin.ThemeNames) {
		return fail("BAD_THEME", "некорректный индекс темы")
	}
	for _, i := range fin.Excluded {
		if i == p.ThemeIndex {
			return fail("ALREADY_EXCLUDED", "тема уже исключена")
		}
	}
	if len(fin.Excluded) >= len(fin.ThemeNames)-1 {
		return fail("LAST_THEME", "нельзя исключить последнюю тему")
	}
	fin.Excluded = append(fin.Excluded, p.ThemeIndex)
	a.ev(EvFinalThemeExcluded, map[string]any{
		"themeIndex": p.ThemeIndex, "theme": fin.ThemeNames[p.ThemeIndex],
	})
	if len(fin.Excluded) == len(fin.ThemeNames)-1 {
		a.state.Stage = StageFinalStakes
	}
	return nil
}

type pFinalStake struct {
	PlayerID string `json:"playerId"`
	Amount   int64  `json:"amount"`
}

func (a *applyCtx) setFinalStake() error {
	if err := a.requireStage(StageFinalStakes); err != nil {
		return err
	}
	var p pFinalStake
	if err := a.decode(&p); err != nil {
		return err
	}
	fin := a.state.Final
	if !containsStr(fin.FinalistIDs, p.PlayerID) {
		return fail("NOT_FINALIST", "игрок не участвует в финале")
	}
	score := a.scores[p.PlayerID]
	maxStake := score
	if maxStake < 0 {
		maxStake = 0
	}
	if p.Amount < 0 || p.Amount > maxStake {
		return fail("BAD_STAKE", "ставка должна быть от 0 до %d", maxStake)
	}
	fin.Stakes[p.PlayerID] = p.Amount
	a.ev(EvFinalStakeSet, map[string]any{"playerId": p.PlayerID}) // сумма скрыта до раскрытия
	return nil
}

func (a *applyCtx) revealFinalStakes() error {
	if err := a.requireStage(StageFinalStakes); err != nil {
		return err
	}
	fin := a.state.Final
	for _, id := range fin.FinalistIDs {
		if _, ok := fin.Stakes[id]; !ok {
			return fail("STAKES_INCOMPLETE", "не все участники сделали ставки")
		}
	}
	fin.StakesRevealed = true
	a.ev(EvFinalStakeSet, map[string]any{"revealed": true})
	return nil
}

func (a *applyCtx) openFinalQuestion() error {
	if err := a.requireStage(StageFinalStakes); err != nil {
		return err
	}
	fin := a.state.Final
	if !fin.StakesRevealed {
		return fail("STAKES_HIDDEN", "сначала раскройте ставки")
	}
	if a.pack == nil {
		return fail("NO_PACK", "пак не загружен")
	}
	// Оставшаяся тема финального раунда.
	round := a.pack.Rounds[a.state.RoundIndex]
	remaining := -1
	for i := range fin.ThemeNames {
		excluded := false
		for _, e := range fin.Excluded {
			if e == i {
				excluded = true
				break
			}
		}
		if !excluded {
			remaining = i
			break
		}
	}
	if remaining < 0 || remaining >= len(round.Themes) || len(round.Themes[remaining].Questions) == 0 {
		return fail("NO_QUESTION", "в финальной теме нет вопроса")
	}
	q := round.Themes[remaining].Questions[0]
	cur := &CurrentQuestion{
		RoundIndex:    a.state.RoundIndex,
		ThemeIndex:    remaining,
		QuestionIndex: 0,
		ThemeName:     fin.ThemeNames[remaining],
		Type:          q.Type,
		TypeParams:    q.TypeParams,
		Scenario:      q.Scenario,
		AnswerScenario: q.AnswerScenario,
		Answer:        q.RightAnswer(),
		AnsweredIDs:   []string{},
	}
	if cur.Scenario == nil {
		cur.Scenario = []model.Atom{}
	}
	fin.Question = cur
	a.state.Stage = StageFinalQuestion
	a.setTimer(TimerFinalThinking, a.state.Timers.FinalThinkingMs, true)
	a.ev(EvFinalQuestionOpen, map[string]any{"theme": cur.ThemeName})
	return nil
}

type pFinalAnswer struct {
	PlayerID string `json:"playerId"`
	Correct  bool   `json:"correct"`
}

func (a *applyCtx) submitFinalAnswer() error {
	if err := a.requireStage(StageFinalQuestion, StageFinalVerdicts); err != nil {
		return err
	}
	var p pFinalAnswer
	if err := a.decode(&p); err != nil {
		return err
	}
	fin := a.state.Final
	if !containsStr(fin.FinalistIDs, p.PlayerID) {
		return fail("NOT_FINALIST", "игрок не участвует в финале")
	}
	// Проверка ответов возможна после истечения таймера или досрочно ведущим.
	if a.state.Stage == StageFinalQuestion {
		a.state.Stage = StageFinalVerdicts
		a.clearTimer()
	}
	if fin.Answers == nil {
		fin.Answers = map[string]bool{}
	}
	fin.Answers[p.PlayerID] = p.Correct
	a.ev(EvFinalAnswerChecked, map[string]any{"playerId": p.PlayerID, "correct": p.Correct})

	// Когда все ответы проверены — завершаем автоматически.
	for _, id := range fin.FinalistIDs {
		if _, ok := fin.Answers[id]; !ok {
			return nil
		}
	}
	return a.finishFinalInternal()
}

func (a *applyCtx) finishFinal() error {
	if err := a.requireStage(StageFinalVerdicts); err != nil {
		return err
	}
	fin := a.state.Final
	for _, id := range fin.FinalistIDs {
		if _, ok := fin.Answers[id]; !ok {
			return fail("ANSWERS_INCOMPLETE", "не все ответы проверены")
		}
	}
	return a.finishFinalInternal()
}

func (a *applyCtx) finishFinalInternal() error {
	fin := a.state.Final
	for _, id := range fin.FinalistIDs {
		stake := fin.Stakes[id]
		correct := fin.Answers[id]
		if stake == 0 {
			continue // нулевая ставка — счёт не меняется
		}
		delta := stake
		if !correct {
			delta = -delta
		}
		if err := a.addScore(id, delta, "final"); err != nil {
			return err
		}
	}
	if fin.Question != nil && !fin.Question.AnswerShown {
		fin.Question.AnswerShown = true
		a.ev(EvAnswerShown, map[string]any{"answer": fin.Question.Answer})
	}
	return a.finishGameNoFinal()
}

func (a *applyCtx) finishGameNoFinal() error {
	// Победитель — максимальный счёт среди активных игроков.
	best := ""
	var bestScore int64
	first := true
	for id, s := range a.scores {
		if first || s > bestScore {
			first = false
			best = id
			bestScore = s
		}
	}
	a.state.WinnerID = best
	a.state.Stage = StageResults
	a.state.Current = nil
	a.clearTimer()
	a.newRoomStatus = "finished"
	a.ev(EvGameFinished, map[string]any{"winnerId": best})
	return nil
}

// --- служебные команды ---

type pSetScore struct {
	PlayerID string `json:"playerId"`
	Score    int64  `json:"score"`
	Reason   string `json:"reason"`
}

func (a *applyCtx) setScore() error {
	var p pSetScore
	if err := a.decode(&p); err != nil {
		return err
	}
	old, ok := a.scores[p.PlayerID]
	if !ok {
		return fail("NO_PLAYER", "игрок не найден")
	}
	if p.Reason == "" {
		p.Reason = "manual"
	}
	if _, err := a.tx.Exec(a.ctx,
		`UPDATE players SET score = $2 WHERE id = $1 AND room_id = $3`, p.PlayerID, p.Score, a.roomID); err != nil {
		return err
	}
	a.scores[p.PlayerID] = p.Score
	a.ev(EvScoreChanged, map[string]any{
		"playerId": p.PlayerID, "delta": p.Score - old, "oldScore": old, "newScore": p.Score,
		"reason": p.Reason, "manual": true,
	})
	return nil
}

type pUpdateTimers struct {
	AnswerWindowMs  *int64 `json:"answerWindowMs"`
	AnsweringMs     *int64 `json:"answeringMs"`
	FinalThinkingMs *int64 `json:"finalThinkingMs"`
	ForAllMs        *int64 `json:"forAllMs"`
}

func (a *applyCtx) updateTimers() error {
	var p pUpdateTimers
	if err := a.decode(&p); err != nil {
		return err
	}
	cfg := &a.state.Timers
	if p.AnswerWindowMs != nil && *p.AnswerWindowMs >= 1000 {
		cfg.AnswerWindowMs = *p.AnswerWindowMs
	}
	if p.AnsweringMs != nil && *p.AnsweringMs >= 1000 {
		cfg.AnsweringMs = *p.AnsweringMs
	}
	if p.FinalThinkingMs != nil && *p.FinalThinkingMs >= 1000 {
		cfg.FinalThinkingMs = *p.FinalThinkingMs
	}
	if p.ForAllMs != nil && *p.ForAllMs >= 1000 {
		cfg.ForAllMs = *p.ForAllMs
	}
	a.ev(EvTimersUpdated, map[string]any{
		"answerWindowMs": cfg.AnswerWindowMs, "answeringMs": cfg.AnsweringMs,
		"finalThinkingMs": cfg.FinalThinkingMs, "forAllMs": cfg.ForAllMs,
	})
	return nil
}

type pSetGuard struct {
	Enabled bool `json:"enabled"`
}

func (a *applyCtx) setFalseStartGuard() error {
	var p pSetGuard
	if err := a.decode(&p); err != nil {
		return err
	}
	a.state.FalseStartGuard = p.Enabled
	a.ev(EvFalseStartUpdated, map[string]any{"enabled": p.Enabled})
	return nil
}

type pPause struct {
	Reason string `json:"reason"`
}

func (a *applyCtx) pauseGame() error {
	if a.roomStatus != "active" {
		return fail("BAD_STATE", "игра не активна")
	}
	// Таймер останавливается с сохранением оставшегося времени.
	if a.state.Timer != nil {
		pauseTimer(a.state.Timer)
	}
	a.newRoomStatus = "paused"
	var p pPause
	_ = a.decode(&p)
	a.ev(EvPaused, map[string]any{"reason": p.Reason})
	return nil
}

func (a *applyCtx) resumeGame() error {
	// Таймер продолжается с сохранённого остатка.
	if a.state.Timer != nil && !a.state.Timer.Running && a.state.Timer.RemainingMs > 0 {
		startTimer(a.state.Timer)
	}
	a.newRoomStatus = "active"
	a.ev(EvResumed, nil)
	return nil
}

func (a *applyCtx) undoLastAction() error {
	// Последняя отменяемая команда: не UNDO, не внутренний таймер.
	var (
		actionID, commandType string
		prevState, prevScores []byte
		prevVersion           int64
	)
	err := a.tx.QueryRow(a.ctx, `
		SELECT action_id, command_type, prev_state, prev_scores, prev_version
		FROM game_actions
		WHERE room_id = $1 AND NOT undone AND prev_state IS NOT NULL
		  AND command_type NOT IN ('UNDO_LAST_ACTION', 'TIMER_EXPIRED')
		ORDER BY id DESC LIMIT 1`, a.roomID,
	).Scan(&actionID, &commandType, &prevState, &prevScores, &prevVersion)
	if err != nil {
		return fail("NOTHING_TO_UNDO", "нет действий для отмены")
	}

	var restored State
	if err := json.Unmarshal(prevState, &restored); err != nil {
		return fmt.Errorf("corrupt undo snapshot: %w", err)
	}
	// Откат счетов игроков.
	var scores map[string]int64
	if err := json.Unmarshal(prevScores, &scores); err != nil {
		return fmt.Errorf("corrupt undo scores: %w", err)
	}
	for id, score := range scores {
		if _, err := a.tx.Exec(a.ctx,
			`UPDATE players SET score = $2 WHERE id = $1 AND room_id = $3`, id, score, a.roomID); err != nil {
			return err
		}
		a.scores[id] = score
	}
	// Откат статуса комнаты к актуальному для восстановленной стадии.
	restoredStatus := a.roomStatus
	if restored.Stage == StageResults {
		restoredStatus = "finished"
	} else if a.roomStatus == "finished" {
		restoredStatus = "active"
	}
	a.newRoomStatus = restoredStatus

	if _, err := a.tx.Exec(a.ctx,
		`UPDATE game_actions SET undone = TRUE WHERE room_id = $1 AND action_id = $2`,
		a.roomID, actionID); err != nil {
		return err
	}
	*a.state = restored
	a.ev(EvUndone, map[string]any{"undoneActionId": actionID, "commandType": commandType})
	return nil
}

// --- помощники пакета ---

func (a *applyCtx) questionAt(round, theme, question int) (model.Question, string, error) {
	if round < 0 || round >= len(a.pack.Rounds) {
		return model.Question{}, "", fail("BAD_CELL", "некорректный раунд")
	}
	r := a.pack.Rounds[round]
	if theme < 0 || theme >= len(r.Themes) {
		return model.Question{}, "", fail("BAD_CELL", "некорректная тема")
	}
	t := r.Themes[theme]
	if question < 0 || question >= len(t.Questions) {
		return model.Question{}, "", fail("BAD_CELL", "некорректный вопрос")
	}
	return t.Questions[question], t.Name, nil
}

func (a *applyCtx) roundComplete(round int) bool {
	if a.pack == nil || round < 0 || round >= len(a.pack.Rounds) {
		return false
	}
	r := a.pack.Rounds[round]
	for ti, t := range r.Themes {
		for qi := range t.Questions {
			if !a.state.UsedCells[qid(round, ti, qi)] {
				return false
			}
		}
	}
	return true
}

func isUnknown(t string) bool {
	return len(t) > 8 && t[:8] == "unknown:"
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
