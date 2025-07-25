package websocket

import "log"

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("Client %s connected. Total clients: %d", client.user.Username, len(h.clients))
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				client.mu.Lock()
				client.closed = true
				client.mu.Unlock()
				delete(h.clients, client)
				close(client.send)

				log.Printf("Client %s unregistered. Total clients: %d", client.user.Username, len(h.clients))
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					client.mu.Lock()
					client.closed = true
					client.mu.Unlock()
					delete(h.clients, client)
					close(client.send)
				}
			}
		}
	}
}
