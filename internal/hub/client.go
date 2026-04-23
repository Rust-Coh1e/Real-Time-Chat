package hub

import (
	"github.com/gorilla/websocket"
)

type Client struct {
	Name       string
	Conn       *websocket.Conn
	Chan       chan []byte
	CurrentHub *Hub
}

func NewClient(h *Hub, name string) *Client {
	return &Client{
		Name:       name,
		Chan:       make(chan []byte),
		CurrentHub: h,
	}
}
