package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"real-time-chat/internal"
	"real-time-chat/proto"

	"google.golang.org/grpc"
)

// Начни с Chat Service. Создай структуру ChatServer с HubManager внутри и встроенным UnimplementedChatServiceServer. Набросай — как она должна выглядеть

type ChatService struct {
	hubs *internal.HubManager
	proto.UnimplementedChatServiceServer
}

func NewChatService(hubs *internal.HubManager) *ChatService {
	return &ChatService{
		hubs: hubs,
	}
}

func (c *ChatService) WriteClient(stream grpc.BidiStreamingServer[proto.ChatMessage, proto.ChatMessage]) {
	// WritePump — читает из client.Chan и пишет в WebSocket
	for msg := range c.Chan {

		err := stream.Send(json.Unmarshal(msg))
		if err != nil {
			log.Println(err)
			c.currentHub.Unregister <- c
			return
		}
	}
}

// Первый stream.Recv() — достань room и sender
// Получи Hub через s.hubs.GetOrMake(room)
// Создай клиента, зарегистрируй в Hub
// Запусти горутину, которая читает из client.Chan и делает stream.Send()
// В цикле читай stream.Recv() и отправляй в hub.Broadcast

func (s *ChatService) Chat(stream grpc.BidiStreamingServer[proto.ChatMessage, proto.ChatMessage]) error {
	// First message
	msg, err := stream.Recv()

	if err == io.EOF {
		return err
	}

	// newClient := msg.Sender
	newHub := msg.Room

	hub := s.hubs.GetOrMake(newHub)
	currClient := internal.NewClient(hub, msg.Sender)

	go func() {
		for msg := range currClient.Chan {

			var chatMsg proto.ChatMessage

			err := json.Unmarshal(msg, &chatMsg)
			if err != nil {
				return
			}

			if err := stream.Send(&chatMsg); err != nil {
				return
			}

		}
	}()

	for {
		msg, err := stream.Recv()

		if err != nil {
			log.Println(err)
			currClient.CurrentHub.Unregister <- currClient
			return err
		}

		// currMsg := NewMessage(c.Name, string(msg), time.Now())
		data, err := json.Marshal(msg)

		if err != nil {
			log.Println(err)
			currClient.CurrentHub.Unregister <- currClient
			return err
		}
		// defer c.Conn.Close()
		currClient.CurrentHub.Broadcast <- data

	}
}

func main() {
	// Создать HubManager
	mHub := internal.NewHubManager()
	// Создать ChatService
	mChatService := NewChatService(mHub)
	// Создать gRPC сервер — grpc.NewServer()
	mGRPC := grpc.NewServer()
	// Зарегистрировать свой сервис — в сгенерированном коде есть функция RegisterChatServiceServer
	proto.RegisterChatServiceServer(mGRPC, mChatService)
	// Начать слушать TCP на порту :50051 — для этого net.Listen("tcp", ":50051")
	listener, err := net.Listen("tcp", ":50051")
	// Запустить server.Serve(listener)
	if err != nil {
		log.Println("Error listening port")
		return
	}

	mGRPC.Serve(listener)
}
