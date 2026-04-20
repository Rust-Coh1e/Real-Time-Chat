package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"real-time-chat/config"
	"real-time-chat/internal"
	"real-time-chat/proto"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type ChatService struct {
	hubs *internal.HubManager
	rdb  *internal.RedisClient
	ms   *internal.MessageStore
	db   *internal.Database
	proto.UnimplementedChatServiceServer
}

func NewChatService(hubs *internal.HubManager, rdb *internal.RedisClient, db *internal.Database) *ChatService {
	ms := internal.NewMessageStore(db, rdb)
	return &ChatService{
		hubs: hubs,
		rdb:  rdb,
		ms:   ms,
		db:   db,
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

	defer func() {
		// нужно удалить клиента из redis и тут
		if err := s.rdb.RemoveMember(ctx, newHub, currClient.Name); err != nil {
			log.Println("Redis RemoveMember failed:", err)
		}
		//
		currClient.CurrentHub.Unregister <- currClient
	}()

	// Тут нужно добавить клиента в Redis

	if err := s.rdb.AddMember(ctx, newHub, currClient.Name); err != nil {
		log.Println("Redis AddMember failed:", err)
	}

	// uuidRoomID, status := uuid.Parse(newHub)

	// if status != nil {
	// 	return status
	// }

	hubID, ok := s.db.GetOrCreateRoom(ctx, newHub)
	if ok != nil {
		return ok
	}

	// Нужно подтянуть историю
	history, err := s.ms.GetHistory(ctx, hubID, 50)

	log.Println("History:", len(history), "err:", err)
	if err != nil {
		return err
	}

	for i := len(history) - 1; i >= 0; i-- {
		err := stream.Send(&proto.ChatMessage{
			Sender:    history[i].Sender,
			Text:      history[i].Text,
			SenderId:  history[i].SenderID.String(),
			Timestamp: history[i].CreatedAt.Unix(),
		})
		if err != nil {
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
		// s.rdb.SaveMessage(ctx, newHub, data)
		msgID, err := uuid.Parse(msg.SenderId)

		if err != nil {
			log.Println(err)
			return err
		}

		// Теперь тут надо сделать Cache aside
		msgRow := internal.MessageRow{
			ID:        uuid.New(),
			SenderID:  msgID, // из JWT или первого сообщения
			Sender:    msg.Sender,
			Text:      msg.Text,
			CreatedAt: time.Now(),
		}

		s.ms.SaveMessage(ctx, hubID, msgRow)

		// defer c.Conn.Close()

		// currClient.CurrentHub.Broadcast <- data

		// тут надо запушить сообщение в Sub
		err = s.rdb.Publish(ctx, newHub, data)
		if err != nil {
			hub.Broadcast <- data
		}

	}
}

func main() {
	port := flag.String("port", "50051", "port")
	flag.Parse()

	// Создать HubManager
	mHub := internal.NewHubManager()
	// Создать ChatService

	// TODO env конфиг
	cfg, err := config.Load()
	// redisURL := "redis://localhost:6379/0"
	if err != nil {
		log.Println("Error: Dont find env file")
		return
	}

	db, err := internal.NewDatabase(cfg)

	if err != nil {
		log.Println("Error: Cant connect to DB")
		return
	}

	rdb := internal.NewRedisClient(cfg.RedisPort, 50)

	mChatService := NewChatService(mHub, rdb, db)

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
