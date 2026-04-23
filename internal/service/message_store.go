package service

import (
	"context"
	"encoding/json"
	"real-time-chat/internal/model"
	"real-time-chat/internal/repository"

	"github.com/google/uuid"
)

type MessageStore struct {
	db    *repository.Postgres
	cache *repository.RedisClient
}

func NewMessageStore(db *repository.Postgres, cache *repository.RedisClient) *MessageStore {
	return &MessageStore{db: db, cache: cache}
}

func (ms *MessageStore) SaveMessage(ctx context.Context, roomID uuid.UUID, msg model.MessageRow) error {
	// 1. Postgres — надёжное хранилище
	err := ms.db.SaveMessage(ctx, roomID, msg)
	if err != nil {
		return err
	}

	// 2. Redis — кэш, ошибку игнорируем
	data, _ := json.Marshal(msg)
	ms.cache.SaveMessage(ctx, roomID.String(), data)

	return nil
}

func (ms *MessageStore) GetHistory(ctx context.Context, roomID uuid.UUID, limit int) ([]model.MessageRow, error) {
	// 1. Пробуем Redis
	cached, err := ms.cache.GetHistory(ctx, roomID.String())
	if err == nil && len(cached) > 0 {
		var messages []model.MessageRow
		for _, s := range cached {
			var m model.MessageRow
			if err := json.Unmarshal([]byte(s), &m); err == nil {
				messages = append(messages, m)
			}
		}
		return messages, nil
	}

	// 2. Fallback на Postgres
	messages, err := ms.db.GetHistory(ctx, roomID, limit)
	if err != nil {
		return nil, err
	}

	// 3. Прогреваем кэш
	for _, msg := range messages {
		data, _ := json.Marshal(msg)
		ms.cache.SaveMessage(ctx, roomID.String(), data)
	}

	return messages, nil
}
