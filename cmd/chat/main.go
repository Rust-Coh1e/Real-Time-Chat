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

	log.Println("SenderId from stream:", msg.SenderId)

	if err == io.EOF {
		return err
	}

	// newClient := msg.Sender
	// newHub := msg.Room
	currUser, _ := uuid.Parse(msg.SenderId)

	Hubs, err := s.db.GetUserRooms(ctx, currUser)

	if err != nil {
		return err
	}

	log.Println("User connected:", msg.Sender, "ID:", msg.SenderId)
	log.Println("User rooms:", Hubs)

	clients := make(map[string]*internal.Client)

	defer func() {
		for roomName, client := range clients {
			s.rdb.RemoveMember(ctx, roomName, client.Name)
			client.CurrentHub.Unregister <- client
		}
	}()

	for _, newHub := range Hubs {
		currHub := s.hubs.GetOrMake(newHub)

		currHub.SubscribeOnce.Do(func() {
			sub := s.rdb.Subscribe(ctx, newHub)
			go func() {
				for msg := range sub.Channel() {
					currHub.Broadcast <- []byte(msg.Payload)
				}
			}()
		})

		currClient := internal.NewClient(currHub, msg.Sender)
		currHub.Register <- currClient

		clients[currHub.Name] = currClient

		if err := s.rdb.AddMember(ctx, newHub, currClient.Name); err != nil {
			log.Println("Redis AddMember failed:", err)
		}

		go func() {
			// ________этот for отвечает зо CHANEL -> GATEAWAY __________________
			for msg := range currClient.Chan {

				log.Println("Sending back to client, room:", newHub)
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
	}

	for _, roomName := range Hubs {
		stream.Send(&proto.ChatMessage{
			Action: "joined",
			Room:   roomName,
			Sender: msg.Sender,
		})
	}

	// currClient := internal.NewClient(hub, msg.Sender)
	// hub.Register <- currClient

	// Тут нужно добавить клиента в Redis

	// Нужно подтянуть историю

	// историю получили

	// ________этот for отвечает зо GateAway -> BD/Redis __________________
	for {
		msg, err := stream.Recv()

		if err != nil {
			log.Println(err)
			return err
		}

		log.Println("Received:", msg.Action, "room:", msg.Room, "text:", msg.Text)

		switch msg.Action {
		case "", "send":
			// currMsg := NewMessage(c.Name, string(msg), time.Now())

			hub := s.hubs.GetOrMake(msg.Room)
			roomID, _ := s.db.GetOrCreateRoom(ctx, msg.Room)

			newMsgID := uuid.New()

			msg.MessageId = newMsgID.String()

			if _, ok := clients[msg.Room]; !ok {
				log.Println("user not in room:", msg.Room)
				continue
			}
			// hubID, _ := uuid.Parse(msg.Room)

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
				ID:        newMsgID,
				SenderID:  msgID, // из JWT или первого сообщения
				Sender:    msg.Sender,
				Text:      msg.Text,
				FileURL:   msg.FileUrl,
				CreatedAt: time.Now(),
			}

			s.ms.SaveMessage(ctx, roomID, msgRow)

			// defer c.Conn.Close()

			// currClient.CurrentHub.Broadcast <- data

			// тут надо запушить сообщение в Sub
			err = s.rdb.Publish(ctx, msg.Room, data)
			if err != nil {
				hub.Broadcast <- data
			}

		case "edit":
			hub := s.hubs.GetOrMake(msg.Room)

			log.Println("Edit:", msg.MessageId, "sender:", msg.SenderId)

			msgUuid, _ := uuid.Parse(msg.MessageId)
			senderUuid, _ := uuid.Parse(msg.SenderId)

			err := s.db.EditMessage(ctx, msgUuid, senderUuid, msg.Text)
			if err != nil {
				log.Println("edit denied:", err)
				continue
			}
			data, _ := json.Marshal(msg)
			if err := s.rdb.Publish(ctx, hub.Name, data); err != nil {
				hub.Broadcast <- data
			}

		case "delete":

			hub := s.hubs.GetOrMake(msg.Room)
			msgUuid, err := uuid.Parse(msg.MessageId)
			if err != nil {
				return err
			}

			s.db.RemoveMessage(ctx, msgUuid)

			data, _ := json.Marshal(msg)
			if err := s.rdb.Publish(ctx, hub.Name, data); err != nil {
				hub.Broadcast <- data
			}

		case "react":
			hub := s.hubs.GetOrMake(msg.Room)

			msgUuid, err := uuid.Parse(msg.MessageId)
			if err != nil {
				return err
			}

			userUuid, err := uuid.Parse(msg.SenderId)
			if err != nil {
				return err
			}

			s.db.ToggleReaction(ctx, msgUuid, userUuid, msg.Emoji)

			data, _ := json.Marshal(msg)
			if err := s.rdb.Publish(ctx, hub.Name, data); err != nil {
				hub.Broadcast <- data
			}
		case "join":
			if _, ok := clients[msg.Room]; ok {
				log.Println("already in room:", msg.Room)
				continue
			}

			roomID, _ := s.db.GetOrCreateRoom(ctx, msg.Room)
			senderID, _ := uuid.Parse(msg.SenderId)
			s.db.JoinRoom(ctx, roomID, senderID)

			hub := s.hubs.GetOrMake(msg.Room)
			client := internal.NewClient(hub, msg.Sender)
			hub.Register <- client
			clients[msg.Room] = client

			go func() {
				for m := range client.Chan {
					var chatMsg proto.ChatMessage
					json.Unmarshal(m, &chatMsg)
					stream.Send(&chatMsg)
				}
			}()

		case "leave":
			roomID, _ := s.db.GetOrCreateRoom(ctx, msg.Room)
			senderID, _ := uuid.Parse(msg.SenderId)
			s.db.LeaveRoom(ctx, roomID, senderID)

			if client, ok := clients[msg.Room]; ok {
				client.CurrentHub.Unregister <- client
				delete(clients, msg.Room)
			}
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
