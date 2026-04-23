package hub

import "sync"

type Hub struct {
	Name          string
	Clients       map[*Client]bool
	Register      chan *Client
	Unregister    chan *Client
	Broadcast     chan []byte
	SubscribeOnce sync.Once
}

func NewHub(name string) *Hub {
	// cli := make(map[*Client]bool)
	return &Hub{
		Name:       name,
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan []byte),
	}
}

func (h *Hub) Run() {

	for {

		select {
		case client := <-h.Register:
			h.Clients[client] = true
		case client := <-h.Unregister:
			delete(h.Clients, client)
		case msg := <-h.Broadcast:
			for client := range h.Clients {
				client.Chan <- msg
			}
		}
	}
}
