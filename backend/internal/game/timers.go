package game

import (
	"sync"
	"time"
)

// TimerManager управляет серверными таймерами комнат.
// Истечение обрабатывается движком ровно один раз — через идемпотентную
// внутреннюю команду TIMER_EXPIRED с action_id вида "timer-{id}".
type TimerManager struct {
	mu       sync.Mutex
	timers   map[string]*time.Timer // roomID -> активный таймер
	onExpire func(roomID string, timerID int64)
}

func NewTimerManager(onExpire func(roomID string, timerID int64)) *TimerManager {
	return &TimerManager{timers: map[string]*time.Timer{}, onExpire: onExpire}
}

// Arm (пере)запускает таймер комнаты по его состоянию.
func (m *TimerManager) Arm(roomID string, t *Timer) {
	m.Cancel(roomID)
	if t == nil || !t.Running {
		return
	}
	delay := time.Until(time.UnixMilli(t.EndsAt))
	if delay < 0 {
		delay = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id := t.ID
	m.timers[roomID] = time.AfterFunc(delay, func() { m.onExpire(roomID, id) })
}

func (m *TimerManager) Cancel(roomID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.timers[roomID]; ok {
		t.Stop()
		delete(m.timers, roomID)
	}
}

// SnapshotNow вычисляет актуальный RemainingMs таймера (для паузы/снапшотов).
func SnapshotNow(t *Timer) {
	if t == nil {
		return
	}
	if t.Running {
		rem := time.UnixMilli(t.EndsAt).Sub(time.Now()).Milliseconds()
		if rem < 0 {
			rem = 0
		}
		t.RemainingMs = rem
	}
}

func startTimer(t *Timer) {
	t.Running = true
	t.EndsAt = time.Now().Add(time.Duration(t.RemainingMs) * time.Millisecond).UnixMilli()
}

func pauseTimer(t *Timer) {
	if t == nil || !t.Running {
		return
	}
	SnapshotNow(t)
	t.Running = false
	t.EndsAt = 0
}
