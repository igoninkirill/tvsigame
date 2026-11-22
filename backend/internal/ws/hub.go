// Package ws — WebSocket-хаб комнат.
//
// Протокол:
//   - клиент подключается к /ws/room/{id} и ПЕРВЫМ сообщением отправляет
//     {"type":"auth","role":"display|host","token":"...","clientId":"..."}
//     (токены не передаются в URL, чтобы не попадать в логи);
//   - сервер отвечает auth_ok/auth_error и далее шлёт {"type":"state",...};
//   - клиент отправляет {"type":"ping"} — сервер отвечает {"type":"pong"};
//   - сервер шлёт websocket ping-фреймы для контроля heartbeat.
package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 30 * time.Second
	pingPeriod     = 15 * time.Second
	authTimeout    = 5 * time.Second
	maxMessageSize = 4096
)

type Role string

const (
	RoleDisplay Role = "display"
	RoleHost    Role = "host"
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	RoomID   string
	Role     Role
	ClientID string
	send     chan []byte
	done     chan struct{}
	once     sync.Once
}

type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[*Client]bool
	// onHostGone вызывается, когда у комнаты не осталось host-подключений.
	onHostGone func(roomID, clientID string)
	// onHostMessage — ping и прочие сообщения от ведущего (обновление last_seen).
	onHostAlive func(roomID, clientID string)
}

func NewHub() *Hub {
	return &Hub{rooms: map[string]map[*Client]bool{}}
}

// SetHostHandlers подключает обработчики потери/активности ведущего
// (вызывается httpapi при связывании слоёв).
func (h *Hub) SetHostHandlers(onGone, onAlive func(roomID, clientID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onHostGone = onGone
	h.onHostAlive = onAlive
}

// Broadcast отправляет снапшот: hostMsg — хостам, displayMsg — ТВ-клиентам.
func (h *Hub) Broadcast(roomID string, hostMsg, displayMsg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.rooms[roomID] {
		msg := displayMsg
		if c.Role == RoleHost {
			msg = hostMsg
		}
		select {
		case c.send <- msg:
		default:
			// Клиент не успевает читать — закрываем, он переподключится.
			h.removeLocked(c)
		}
	}
}

// HostOnline — есть ли живое host-подключение у комнаты.
func (h *Hub) HostOnline(roomID string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.rooms[roomID] {
		if c.Role == RoleHost {
			return c.ClientID, true
		}
	}
	return "", false
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[c.RoomID] == nil {
		h.rooms[c.RoomID] = map[*Client]bool{}
	}
	h.rooms[c.RoomID][c] = true
}

func (h *Hub) removeLocked(c *Client) {
	if room, ok := h.rooms[c.RoomID]; ok {
		if _, ok := room[c]; ok {
			delete(room, c)
			if len(room) == 0 {
				delete(h.rooms, c.RoomID)
			}
			c.closeLocked()
		}
	}
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	h.removeLocked(c)
	h.mu.Unlock()
	if c.Role == RoleHost && h.onHostGone != nil {
		h.onHostGone(c.RoomID, c.ClientID)
	}
}

func (c *Client) closeLocked() {
	c.once.Do(func() {
		close(c.done)
		c.conn.Close()
	})
}

// Serve обслуживает одно WebSocket-подключение. auth возвращает роль и
// clientID либо ошибку; onAuthed вызывается после успешной регистрации.
func Serve(h *Hub, conn *websocket.Conn, roomID string,
	auth func(msg []byte) (Role, string, error),
	initialHost, initialDisplay []byte,
	onAuthed func(role Role, clientID string),
) {
	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(authTimeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return
	}
	role, clientID, err := auth(msg)
	if err != nil {
		_ = conn.WriteJSON(map[string]any{"type": "auth_error", "message": err.Error()})
		conn.Close()
		return
	}
	_ = conn.WriteJSON(map[string]any{"type": "auth_ok", "role": string(role)})

	c := &Client{
		hub: h, conn: conn, RoomID: roomID, Role: role, ClientID: clientID,
		send: make(chan []byte, 32), done: make(chan struct{}),
	}
	h.register(c)
	if onAuthed != nil {
		onAuthed(role, clientID)
	}

	// Первичный снапшот сразу после подключения.
	initial := initialDisplay
	if role == RoleHost {
		initial = initialHost
	}
	if len(initial) > 0 {
		c.send <- initial
	}

	go c.writePump()
	c.readPump() // блокируется до разрыва соединения
}

func (c *Client) readPump() {
	defer c.hub.unregister(c)
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		// Простейший heartbeat от клиента.
		if string(msg) == `{"type":"ping"}` {
			if c.Role == RoleHost && c.hub.onHostAlive != nil {
				c.hub.onHostAlive(c.RoomID, c.ClientID)
			}
			select {
			case c.send <- []byte(`{"type":"pong"}`):
			default:
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.hub.mu.Lock()
		c.hub.removeLocked(c)
		c.hub.mu.Unlock()
	}()
	for {
		select {
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}
