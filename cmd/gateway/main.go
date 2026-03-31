package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"real-time-chat/proto"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Теперь Gateway
// Chat Service готов. Нужен Gateway — cmd/gateway/main.go. Он делает:

// Подключается к Chat Service как gRPC клиент
// Поднимает HTTP сервер с WebSocket
// Когда приходит WebSocket клиент — открывает gRPC stream к Chat Service
// Перекидывает сообщения между WebSocket и gRPC stream

// Для gRPC клиента тебе понадобится: grpc.Dial("localhost:50051", ...) для соединения, потом proto.NewChatServiceClient(conn) для создания клиента, и client.Chat(ctx) для открытия stream.
// Попробуй набросать cmd/gateway/main.go — структуру и main функцию. Пока без wsHandler, просто подключение к gRPC и запуск HTTP сервера.

type Gateway struct {
	chatClient proto.ChatServiceClient
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(gateway *Gateway) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Upgrade до WebSocket (как раньше)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Error listening port")
			return
		}
		// Достать name и room из query параметров

		clientName := r.URL.Query().Get("name")
		clientRoom := r.URL.Query().Get("room")

		// Открыть gRPC stream: gateway.chatClient.Chat(context.Background())
		stream, err := gateway.chatClient.Chat(context.Background())
		if err != nil {
			log.Println("Error RPC open conn")
			return
		}
		defer stream.CloseSend()

		// у нас же нету в msg room
		// Отправить первое сообщение в stream с room и sender
		log.Println("New client:", clientName, " Hub ", clientRoom)

		stream.Send(&proto.ChatMessage{
			Sender: clientName,
			Room:   clientRoom,
		})

		// Запустить горутину: читать из stream.Recv() → писать в WebSocket
		go func() {
			for {
				msg, err := stream.Recv()

				if err != nil {
					log.Println(err)
					break
				}

				// currMsg := NewMessage(c.Name, string(msg), time.Now())
				data, err := json.Marshal(msg)

				if err != nil {
					log.Println(err)
					break
				}
				// defer c.Conn.Close()
				conn.WriteMessage(websocket.TextMessage, data)

			}
		}()

		// В основном цикле: читать из WebSocket → stream.Send()
		for {
			_, msg, err := conn.ReadMessage()

			if err != nil {
				log.Println(err)
				return
			}

			stream.Send(&proto.ChatMessage{
				Sender: clientName,
				Room:   clientRoom,
				Text:   string(msg),
			})
		}
	}
}

func main() {
	// Подключиться к gRPC серверу — grpc.Dial("localhost:50051", ...)
	log.Println("Start...")
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer conn.Close()

	log.Println("Connecting to GRPC...")

	chatClient := proto.NewChatServiceClient(conn)
	mainGateway := &Gateway{
		chatClient: chatClient,
	}

	log.Println("Making client..")

	http.HandleFunc("/ws", wsHandler(mainGateway))

	// go mainHub.Run()

	log.Println("Server started :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
	// Зарегистрировать wsHandler
	// Запустить HTTP сервер на :8080
}
