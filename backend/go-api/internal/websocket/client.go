package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"egobackend/internal/database"
	"egobackend/internal/engine"
	"egobackend/internal/models"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	db     *database.DB
	pyURL  string
	user   *models.User
	mu     sync.Mutex
	closed bool
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 20 * 1024 * 1024 // 20 MB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, user *models.User, db *database.DB, pyURL string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), user: user, db: db, pyURL: pyURL}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket read error for user %s: %v", c.user.Username, err)
			} else {
				log.Printf("Client %s disconnected.", c.user.Username)
			}
			break
		}
		go c.handleIncomingMessage(message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleIncomingMessage(message []byte) {
	var req models.StreamRequest
	if err := json.Unmarshal(message, &req); err != nil {
		c.sendEvent("error", map[string]string{"message": "Неверный формат запроса."})
		return
	}

	log.Printf("WS Запрос от %s (ID %d), Mode: %s -> делегируется Процессору", c.user.Username, c.user.ID, req.Mode)

	processor := engine.NewProcessor(c.db, c.pyURL)

	callback := func(eventType string, data interface{}) {
		c.sendEvent(eventType, data)
	}

	processor.ProcessRequest(req, c.user, callback)
}

func (c *Client) sendEvent(eventType string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}

	eventData := map[string]interface{}{"type": eventType, "data": data}
	jsonEvent, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("CRITICAL: Failed to marshal event to JSON: %v", err)
		return
	}

	select {
	case c.send <- jsonEvent:
	default:
		log.Printf("Warning: send channel is full for client %s. Message dropped.", c.user.Username)
	}
}
