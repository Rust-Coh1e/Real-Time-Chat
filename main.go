package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type Client struct {
	Conn       *websocket.Conn
	Chan       chan []byte
	currentHub *Hub
}

type Hub struct {
	Clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan []byte
}

func NewClient(cn *websocket.Conn, h *Hub) *Client {
	return &Client{
		Conn:       cn,
		Chan:       make(chan []byte),
		currentHub: h,
	}
}

func NewHub() *Hub {
	// cli := make(map[*Client]bool)
	return &Hub{
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

func (c *Client) ReadPump() {
	// читает сообщения из WebSocket и отправляет их в Hub.Broadcast
	for {
		_, msg, err := c.Conn.ReadMessage()

		if err != nil {
			log.Println(err)
			c.currentHub.Unregister <- c
			return
		}
		// defer c.Conn.Close()
		c.currentHub.Broadcast <- msg

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

func wsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)

		if err != nil {
			log.Println(err)
			return
		}

		defer conn.Close()

		currClient := NewClient(conn, hub)

		hub.Register <- currClient

		go currClient.WritePump()
		currClient.ReadPump()

	}
}

func main() {

	mainHub := NewHub()

	http.HandleFunc("/ws", wsHandler(mainHub))

	go mainHub.Run()

	log.Println("Server started :8080")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
