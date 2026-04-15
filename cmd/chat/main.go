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
	rdb  *internal.RedisClient
	proto.UnimplementedChatServiceServer
}

func NewChatService(hubs *internal.HubManager, url string, capacity int) *ChatService {

	rdb := internal.NewRedisClient(url, capacity)

	return &ChatService{
		hubs: hubs,
		rdb:  rdb,
	}
}

func (s *ChatService) Chat(stream grpc.BidiStreamingServer[proto.ChatMessage, proto.ChatMessage]) error {
	// First message
	msg, err := stream.Recv()
	ctx := stream.Context()

	if err == io.EOF {
		return err
	}

	// newClient := msg.Sender
	newHub := msg.Room

	hub := s.hubs.GetOrMake(newHub)

	hub.SubscribeOnce.Do(func() {
		sub := s.rdb.Subscribe(ctx, newHub)
		go func() {
			for msg := range sub.Channel() {
				hub.Broadcast <- []byte(msg.Payload)
			}
		}()
	})

	currClient := internal.NewClient(hub, msg.Sender)
	hub.Register <- currClient

	// Тут нужно добавить клиента в Redis

	err = s.rdb.AddMember(ctx, newHub, currClient.Name)

	if err != nil {
		return err
	}

	// Нужно подтянуть историю
	history, err := s.rdb.GetHistory(ctx, newHub)

	if err != nil {
		return err
	}

	var chatMsg proto.ChatMessage
	for i := len(history) - 1; i >= 0; i-- {
		// так мы получили history нужно его конвертнуть
		history_freq := []byte(history[i])

		// Преобразование и отправка в API
		err = json.Unmarshal(history_freq, &chatMsg)
		if err != nil {
			return err
		}

		if err := stream.Send(&chatMsg); err != nil {
			return err
		}
	}
	// историю получили

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

		defer func() {
			// нужно удалить клиента из redis и тут
			s.rdb.RemoveMember(ctx, newHub, currClient.Name)
			//
			currClient.CurrentHub.Unregister <- currClient
		}()

		if err != nil {
			log.Println(err)
			return err
		}

		// currMsg := NewMessage(c.Name, string(msg), time.Now())
		data, err := json.Marshal(msg)

		if err != nil {
			log.Println(err)
			return err
		}

		// Тут нужно запушить сообщение в redis
		s.rdb.SaveMessage(ctx, newHub, data)

		// defer c.Conn.Close()

		// currClient.CurrentHub.Broadcast <- data

		// тут надо запушить сообщение в Sub
		s.rdb.Publish(ctx, newHub, data)

	}
}

func main() {
	port := flag.String("port", "50051", "port")
	flag.Parse()

	// Создать HubManager
	mHub := internal.NewHubManager()
	// Создать ChatService

	// TODO env конфиг

	redisURL := "redis://localhost:6379/0"

	mChatService := NewChatService(mHub, redisURL, 50)

	// Создать gRPC сервер — grpc.NewServer()
	mGRPC := grpc.NewServer()
	log.Println("New stream opened")
	proto.RegisterChatServiceServer(mGRPC, mChatService)

	listener, err := net.Listen("tcp", ":"+*port)
	log.Println("Chat Service listening on :" + *port)
	// Запустить server.Serve(listener)
	if err != nil {
		log.Println("Error listening port")
		return
	}

	mGRPC.Serve(listener)
}
