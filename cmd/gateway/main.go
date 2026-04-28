package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"real-time-chat/config"
	"real-time-chat/internal/jwt"
	"real-time-chat/proto"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	chatClient proto.ChatServiceClient
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// var wsMsg struct {
// 	Text      string `json:"text"`
// 	FileURL   string `json:"file_url"`
// 	Action    string `json:"action"`
// 	MessageID string `json:"message_id"`
// 	Emoji     string `json:"emoji"`
// }

func wsHandler(gateway *Gateway, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("WS request received")
		clientToken := r.URL.Query().Get("token")
		// clientRoom := r.URL.Query().Get("room")
		clientClaims, err := jwt.ParseToken(clientToken, cfg.Secret)

		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Upgrade до WebSocket (как раньше)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Error listening port")
			return
		}
		// Достать name и room из query параметров
		clientName := clientClaims.Username
		clientRoom := r.URL.Query().Get("room")
		clientID := clientClaims.UserID

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
			// Room:     clientRoom,
			SenderId: clientID,
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

		// // В основном цикле: читать из WebSocket → stream.Send()
		// for {
		// 	_, msg, err := conn.ReadMessage()

		// 	if err != nil {
		// 		log.Println(err)
		// 		return
		// 	}

		// 	stream.Send(&proto.ChatMessage{
		// 		Sender:   clientName,
		// 		Room:     clientRoom,
		// 		Text:     string(msg),
		// 		SenderId: clientID,
		// 	})
		// }

		for {
			_, msg, err := conn.ReadMessage()

			if err != nil {
				log.Println(err)
				return
			}

			var wsMsg struct {
				Text      string `json:"text"`
				FileURL   string `json:"file_url"`
				Action    string `json:"action"`
				MessageID string `json:"message_id"`
				Emoji     string `json:"emoji"`
				Room      string `json:"room"`
				Timestamp int64  `json:"timestamp"`
			}

			json.Unmarshal(msg, &wsMsg)

			stream.Send(&proto.ChatMessage{
				Sender:    clientName,
				SenderId:  clientID,
				Room:      wsMsg.Room,
				Text:      wsMsg.Text,
				FileUrl:   wsMsg.FileURL,
				Action:    wsMsg.Action,
				MessageId: wsMsg.MessageID,
				Emoji:     wsMsg.Emoji,
				Timestamp: wsMsg.Timestamp,
			})
		}
	}
}

func main() {
	port := flag.String("port", "8080", "port")
	server_port := flag.String("chat", "50051", "chat")

	cfg, status := config.Load()

	if status != nil {
		log.Fatal("Failed to open env:", status)
	}

	flag.Parse()
	// Подключиться к gRPC серверу — grpc.Dial("localhost:50051", ...)
	log.Println("Start...")
	conn, err := grpc.NewClient("localhost:"+*server_port, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	http.HandleFunc("/ws", wsHandler(mainGateway, cfg))

	// go mainHub.Run()

	log.Println("Server started :" + *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
	// Зарегистрировать wsHandler
	// Запустить HTTP сервер на :8080
}
