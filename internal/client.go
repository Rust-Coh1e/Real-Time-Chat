package internal

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	Name       string
	Conn       *websocket.Conn
	Chan       chan []byte
	CurrentHub *Hub
}

type Message struct {
	Sender    string    `json:"sender"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdat"`
}

func NewMessage(send string, text string, created time.Time) *Message {
	return &Message{
		Sender:    send,
		Text:      text,
		CreatedAt: created,
	}
}

func NewClient(h *Hub, name string) *Client {
	return &Client{
		Name:       name,
		Chan:       make(chan []byte),
		CurrentHub: h,
	}
}

func (c *Client) ReadPump() {
	// читает сообщения из WebSocket и отправляет их в Hub.Broadcast
	for {
		_, msg, err := c.Conn.ReadMessage()

		if err != nil {
			log.Println(err)
			c.CurrentHub.Unregister <- c
			return
		}

		currMsg := NewMessage(c.Name, string(msg), time.Now())
		data, err := json.Marshal(currMsg)

		if err != nil {
			log.Println(err)
			c.CurrentHub.Unregister <- c
			return
		}
		// defer c.Conn.Close()
		c.CurrentHub.Broadcast <- data

	}

}

func (c *Client) WritePump() {
	// WritePump — читает из client.Chan и пишет в WebSocket
	for msg := range c.Chan {

		err := c.Conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println(err)
			c.CurrentHub.Unregister <- c
			return
		}
	}
}
