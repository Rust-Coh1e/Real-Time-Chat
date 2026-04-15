package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Sender    string    `json:"sender"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdat"`
}

type Client struct {
	Name       string
	Conn       *websocket.Conn
	Chan       chan []byte
	currentHub *Hub
}

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

func NewMessage(send string, text string, created time.Time) *Message {
	return &Message{
		Sender:    send,
		Text:      text,
		CreatedAt: created,
	}
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

func NewClient(cn *websocket.Conn, h *Hub, name string) *Client {
	return &Client{
		Name:       name,
		Conn:       cn,
		Chan:       make(chan []byte),
		currentHub: h,
	}
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
		// тут надо читать из broadcast
		case msg := <-h.Broadcast:
			for client := range h.Clients {
				client.Chan <- msg
			}
		}
	}
}

func (c *Client) ReadPump() {
	// читает сообщения из WebSocket и отправляет их в Hub.Broadcast
	for {
		_, msg, err := c.Conn.ReadMessage()

		if err != nil {
			log.Println(err)
			c.currentHub.Unregister <- c
			return
		}

		currMsg := NewMessage(c.Name, string(msg), time.Now())
		data, err := json.Marshal(currMsg)

		if err != nil {
			log.Println(err)
			c.currentHub.Unregister <- c
			return
		}
		// defer c.Conn.Close()
		c.currentHub.Broadcast <- data

	}

}

func (c *Client) WritePump() {
	// WritePump — читает из client.Chan и пишет в WebSocket
	for msg := range c.Chan {

		err := c.Conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println(err)
			c.currentHub.Unregister <- c
			return
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

func main() {

	mainHubManager := NewHubManager()

	http.HandleFunc("/ws", wsHandler(mainHubManager))

	// go mainHub.Run()

	log.Println("Server started :8080")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
