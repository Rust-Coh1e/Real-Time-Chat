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

type ChatService struct {
	hubs *internal.HubManager
	proto.UnimplementedChatServiceServer
}

func NewChatService(hubs *internal.HubManager) *ChatService {
	return &ChatService{
		hubs: hubs,
	}
}

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
	hub.Register <- currClient

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
	log.Println("New stream opened")
	proto.RegisterChatServiceServer(mGRPC, mChatService)

	listener, err := net.Listen("tcp", ":50051")
	log.Println("Chat Service listening on :50051")
	// Запустить server.Serve(listener)
	if err != nil {
		log.Println("Error listening port")
		return
	}

	mGRPC.Serve(listener)
}
