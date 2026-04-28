package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"real-time-chat/config"
	"real-time-chat/internal/hub"
	"real-time-chat/internal/repository"
	"real-time-chat/internal/service"
	"real-time-chat/proto"
	"time"

	"google.golang.org/grpc"
)

type ChatServer struct {
	chatService *service.ChatService
	hubs        *hub.HubManager
	rdb         *repository.RedisClient
	proto.UnimplementedChatServiceServer
}

// Конкструктор

func NewChatServer(cs *service.ChatService, hubs *hub.HubManager, rdb *repository.RedisClient) *ChatServer {
	return &ChatServer{
		chatService: cs,
		hubs:        hubs,
		rdb:         rdb,
	}
}

// ТРАНСПОРТ

func (s *ChatServer) Chat(stream grpc.BidiStreamingServer[proto.ChatMessage, proto.ChatMessage]) error {
	// First message
	msg, err := stream.Recv()
	ctx := stream.Context()

	log.Println("SenderId from stream:", msg.SenderId)

	if err == io.EOF {
		return err
	}

	Hubs, err := s.chatService.GetUserRooms(ctx, msg.SenderId)

	if err != nil {
		return err
	}

	log.Println("User connected:", msg.Sender, "ID:", msg.SenderId)
	log.Println("User rooms:", Hubs)

	clients := make(map[string]*hub.Client)

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

		currClient := hub.NewClient(currHub, msg.Sender)
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

		history, err := s.chatService.GetHistory(ctx, roomName, 50)
		if err != nil {
			log.Println("history error:", err)
			continue
		}
		for i := len(history) - 1; i >= 0; i-- {
			stream.Send(&proto.ChatMessage{
				Sender:    history[i].Sender,
				Text:      history[i].Text,
				FileUrl:   history[i].FileURL,
				SenderId:  history[i].SenderID.String(),
				MessageId: history[i].ID.String(),
				Timestamp: history[i].CreatedAt.UnixMilli(),
				Room:      roomName,
			})
		}
	}

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
			// Проверить на соответствие руме
			if _, ok := clients[msg.Room]; !ok {
				log.Println("user not in room:", msg.Room)
				continue
			}
			// бизнес логика
			data, err := s.chatService.SendMessage(ctx, msg.Room, msg.SenderId, msg.Sender, msg.Text, msg.FileUrl)
			if err != nil {
				log.Println(err)
				continue
			}
			// fallback
			hub := s.hubs.GetOrMake(msg.Room)
			if err := s.rdb.Publish(ctx, msg.Room, data); err != nil {
				hub.Broadcast <- data
			}

		case "edit":

			if _, ok := clients[msg.Room]; !ok {
				log.Println("user not in room:", msg.Room)
				continue
			}

			data, err := s.chatService.EditMessage(ctx, msg.Room, msg.MessageId, msg.SenderId, msg.Text)
			if err != nil {
				log.Println("edit denied:", err)
				continue
			}

			hub := s.hubs.GetOrMake(msg.Room)
			if err := s.rdb.Publish(ctx, msg.Room, data); err != nil {
				hub.Broadcast <- data
			}

		case "delete":

			if _, ok := clients[msg.Room]; !ok {
				log.Println("user not in room:", msg.Room)
				continue
			}

			data, err := s.chatService.DeleteMessage(ctx, msg.Room, msg.MessageId)
			if err != nil {
				log.Println("delete denied:", err)
				continue
			}

			hub := s.hubs.GetOrMake(msg.Room)
			if err := s.rdb.Publish(ctx, msg.Room, data); err != nil {
				hub.Broadcast <- data
			}

		case "react":

			if _, ok := clients[msg.Room]; !ok {
				log.Println("user not in room:", msg.Room)
				continue
			}

			data, err := s.chatService.ReactMessage(ctx, msg.Room, msg.MessageId, msg.SenderId, msg.Emoji)
			if err != nil {
				continue
			}

			hub := s.hubs.GetOrMake(msg.Room)
			if err := s.rdb.Publish(ctx, msg.Room, data); err != nil {
				hub.Broadcast <- data
			}

		case "join":
			if _, ok := clients[msg.Room]; ok {
				continue // уже в комнате
			}
			s.chatService.JoinRoom(ctx, msg.Room, msg.SenderId)

			// создание Hub, Client, горутина — остаётся тут
			currHub := s.hubs.GetOrMake(msg.Room)
			client := hub.NewClient(currHub, msg.Sender)
			currHub.Register <- client
			clients[msg.Room] = client

			go func() {
				for m := range client.Chan {
					var chatMsg proto.ChatMessage
					json.Unmarshal(m, &chatMsg)
					stream.Send(&chatMsg)
				}
			}()
		case "direct":
			roomName, err := s.chatService.DirectMessage(ctx, msg.Sender, msg.Room)
			if err != nil {
				log.Println("direct error:", err)
				continue
			}

			if _, ok := clients[roomName]; ok {
				continue // уже подписан
			}

			currHub := s.hubs.GetOrMake(roomName)
			client := hub.NewClient(currHub, msg.Sender)
			currHub.Register <- client
			clients[roomName] = client

			go func() {
				for m := range client.Chan {
					var chatMsg proto.ChatMessage
					json.Unmarshal(m, &chatMsg)
					stream.Send(&chatMsg)
				}
			}()

			stream.Send(&proto.ChatMessage{
				Action: "joined",
				Room:   roomName,
				Sender: msg.Sender,
			})

		case "leave":
			if _, ok := clients[msg.Room]; !ok {
				log.Println("user not in room:", msg.Room)
				continue
			}

			data, err := s.chatService.DeleteMessage(ctx, msg.Room, msg.MessageId)
			if err != nil {
				continue
			}

			hub := s.hubs.GetOrMake(msg.Room)
			if err := s.rdb.Publish(ctx, msg.Room, data); err != nil {
				hub.Broadcast <- data
			}

		case "history":
			log.Println("History request:", msg.Room, "timestamp:", msg.Timestamp)
			before := time.UnixMilli(msg.Timestamp)
			log.Println("Before:", before)

			history, err := s.chatService.GetHistoryBefore(ctx, msg.Room, before, 50)
			if err != nil {
				continue
			}
			log.Println("History result: count:", len(history), "err:", err)
			for i := len(history) - 1; i >= 0; i-- {
				stream.Send(&proto.ChatMessage{
					Action:    "history",
					Sender:    history[i].Sender,
					Text:      history[i].Text,
					FileUrl:   history[i].FileURL,
					SenderId:  history[i].SenderID.String(),
					MessageId: history[i].ID.String(),
					Timestamp: history[i].CreatedAt.UnixMilli(),
					Room:      msg.Room,
				})
			}
		case "typing":
			data, _ := json.Marshal(map[string]string{
				"action": "typing",
				"room":   msg.Room,
				"sender": msg.Sender,
			})
			if err := s.rdb.Publish(ctx, msg.Room, data); err != nil {
				hub := s.hubs.GetOrMake(msg.Room)
				hub.Broadcast <- data
			}
		}
	}
}

func main() {
	port := flag.String("port", "50051", "port")
	flag.Parse()

	// Создать HubManager

	// Создать ChatService

	// TODO env конфиг
	cfg, err := config.Load()

	// 1. Инфра
	db, _ := repository.NewPostgres(cfg)
	if err != nil {
		log.Println("Error: Cant connect to DB")
		return
	}

	rdb := repository.NewRedisClient(cfg.RedisPort, 50)
	hubs := hub.NewHubManager()

	// 2. Service
	chatService := service.NewChatService(hubs, rdb, db)

	// 3. Transport
	chatServer := NewChatServer(chatService, hubs, rdb)

	// 4. gRPC
	grpcServer := grpc.NewServer()
	proto.RegisterChatServiceServer(grpcServer, chatServer)

	listener, err := net.Listen("tcp", ":"+*port)
	log.Println("Chat Service listening on :" + *port)
	// Запустить server.Serve(listener)
	if err != nil {
		log.Println("Error listening port")
		return
	}

	grpcServer.Serve(listener)
}
