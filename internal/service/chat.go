package service

import (
	"context"
	"encoding/json"
	"log"
	"real-time-chat/internal/hub"
	"real-time-chat/internal/model"
	"real-time-chat/internal/repository"
	"time"

	"github.com/google/uuid"
)

type ChatService struct {
	hubs *hub.HubManager
	rdb  *repository.RedisClient
	ms   *MessageStore
	db   *repository.Postgres
}

func NewChatService(hubs *hub.HubManager, rdb *repository.RedisClient, db *repository.Postgres) *ChatService {
	ms := NewMessageStore(db, rdb)
	return &ChatService{
		hubs: hubs,
		rdb:  rdb,
		ms:   ms,
		db:   db,
	}
}

func (chat *ChatService) SendMessage(ctx context.Context, room, senderID, sender, text, fileURL string) ([]byte, error) {

	// Создаем hub и руму в бд
	roomID, _ := chat.db.GetOrCreateRoom(ctx, room)

	// Генерируем новый ID для сообщения

	newMsgID := uuid.New()
	senderUUID, err := uuid.Parse(senderID)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Теперь тут надо сделать Cache aside
	msgRow := model.MessageRow{
		ID:        newMsgID,
		SenderID:  senderUUID, // из JWT или первого сообщения
		Sender:    sender,
		Text:      text,
		FileURL:   fileURL,
		CreatedAt: time.Now(),
	}

	chat.ms.SaveMessage(ctx, roomID, msgRow)

	msgData := map[string]interface{}{
		"action":     "send",
		"sender":     sender,
		"sender_id":  senderID,
		"text":       text,
		"file_url":   fileURL,
		"message_id": newMsgID.String(),
		"room":       room,
		"timestamp":  msgRow.CreatedAt.UnixMilli(),
	}

	data, err := json.Marshal(msgData)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return data, nil
}

func (chat *ChatService) EditMessage(ctx context.Context, room, msgID, senderID, text string) ([]byte, error) {

	msgUuid, _ := uuid.Parse(msgID)
	senderUuid, _ := uuid.Parse(senderID)

	err := chat.db.EditMessage(ctx, msgUuid, senderUuid, text)
	if err != nil {
		log.Println("edit denied:", err)
		return nil, err
	}
	msgData := map[string]interface{}{
		"action":     "edit",
		"sender_id":  senderID,
		"text":       text,
		"message_id": msgID,
		"room":       room,
	}

	data, err := json.Marshal(msgData)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (chat *ChatService) DeleteMessage(ctx context.Context, room, msgID string) ([]byte, error) {

	msgUuid, err := uuid.Parse(msgID)
	if err != nil {
		return nil, err
	}

	if err = chat.db.RemoveMessage(ctx, msgUuid); err != nil {
		return nil, err
	}

	msgData := map[string]interface{}{
		"action":     "delete",
		"message_id": msgID,
		"room":       room,
	}

	data, err := json.Marshal(msgData)

	if err != nil {
		return nil, err
	}

	return data, nil
}

func (chat *ChatService) ReactMessage(ctx context.Context, room, msgID, senderID, emoji string) ([]byte, error) {

	msgUuid, err := uuid.Parse(msgID)
	senderUuid, _ := uuid.Parse(senderID)
	if err != nil {
		return nil, err
	}

	if err = chat.db.ToggleReaction(ctx, msgUuid, senderUuid, emoji); err != nil {
		return nil, err
	}

	msgData := map[string]interface{}{
		"action":     "react",
		"message_id": msgID,
		"room":       room,
		"sender_id":  senderID,
		"emoji":      emoji,
	}

	data, err := json.Marshal(msgData)

	if err != nil {
		return nil, err
	}

	return data, nil
}

func (chat *ChatService) JoinRoom(ctx context.Context, room, senderID string) error {
	roomID, err := chat.db.GetOrCreateRoom(ctx, room)
	if err != nil {
		return err
	}

	senderUUID, err := uuid.Parse(senderID)
	if err != nil {
		return err
	}

	return chat.db.JoinRoom(ctx, roomID, senderUUID)
}

func (chat *ChatService) LeaveRoom(ctx context.Context, room, senderID string) error {
	roomID, err := chat.db.GetOrCreateRoom(ctx, room)
	if err != nil {
		return err
	}

	senderUUID, err := uuid.Parse(senderID)
	if err != nil {
		return err
	}

	return chat.db.LeaveRoom(ctx, roomID, senderUUID)
}

func (chat *ChatService) GetUserRooms(ctx context.Context, userID string) ([]string, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	return chat.db.GetUserRooms(ctx, id)
}

// func (chat *ChatService) GetHistory(ctx context.Context, room string, limit int) ([]model.MessageRow, error) {
// 	roomID, err := chat.db.GetOrCreateRoom(ctx, room)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return chat.ms.GetHistory(ctx, roomID, limit)
// }

func (chat *ChatService) GetHistory(ctx context.Context, room string, limit int) ([]model.MessageRow, error) {
	roomID, err := chat.db.GetOrCreateRoom(ctx, room)
	if err != nil {
		return nil, err
	}
	return chat.db.GetHistory(ctx, roomID, limit)
}

func (chat *ChatService) GetHistoryBefore(ctx context.Context, room string, before time.Time, limit int) ([]model.MessageRow, error) {

	roomID, err := chat.db.GetOrCreateRoom(ctx, room)
	log.Println("GetHistoryBefore roomID:", roomID, "before:", before, "limit:", limit)
	if err != nil {
		return nil, err
	}
	return chat.ms.db.GetHistoryBefore(ctx, roomID, before, limit)
}
