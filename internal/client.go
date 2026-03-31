package internal

import (
	"github.com/gorilla/websocket"
)

type Client struct {
	Name       string
	Conn       *websocket.Conn
	Chan       chan []byte
	CurrentHub *Hub
}

// type Message struct {
// 	Sender    string    `json:"sender"`
// 	Text      string    `json:"text"`
// 	CreatedAt time.Time `json:"createdat"`
// }

// func NewMessage(send string, text string, created time.Time) *Message {
// 	return &Message{
// 		Sender:    send,
// 		Text:      text,
// 		CreatedAt: created,
// 	}
// }

func NewClient(h *Hub, name string) *Client {
	return &Client{
		Name:       name,
		Chan:       make(chan []byte),
		CurrentHub: h,
	}
}
