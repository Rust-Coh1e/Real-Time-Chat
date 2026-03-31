package internal

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	Name       string
	Clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan []byte
}

type HubManager struct {
	hubs map[string]*Hub
	mu   sync.RWMutex
}

func NewHubManager() *HubManager {
	return &HubManager{
		hubs: make(map[string]*Hub),
	}
}

func (h *HubManager) GetOrMake(name string) *Hub {
	h.mu.RLock()
	res, ok := h.hubs[name]
	if ok == true {
		h.mu.RUnlock()
		return res
	}
	h.mu.RUnlock()
	h.mu.Lock()
	defer h.mu.Unlock()
	res, ok = h.hubs[name]
	if ok == true {
		return res
	} else {
		h.hubs[name] = NewHub(name)
		go h.hubs[name].Run()
		res = h.hubs[name]
	}
	return res
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

// func (h *Hub) Add(c *Client) {
// 	h.Clients[c] = true
// }

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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(hubMan *HubManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)

		clientName := r.URL.Query().Get("name")
		clientRoom := r.URL.Query().Get("room")

		if err != nil {
			log.Println(err)
			return
		}

		defer conn.Close()

		hub := hubMan.GetOrMake(clientRoom)

		currClient := NewClient(conn, hub, clientName)

		hub.Register <- currClient

		go currClient.WritePump()
		currClient.ReadPump()

	}
}
