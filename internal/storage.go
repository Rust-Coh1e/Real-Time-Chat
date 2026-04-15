package internal

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client   *redis.Client
	capacity int
}

func NewRedisClient(url string, capacity int) *RedisClient {

	opt, err := redis.ParseURL(url) // "redis://<user>:<pass>@localhost:6379/<db>"
	if err != nil {
		panic(err)
	}

	return &RedisClient{
		client:   redis.NewClient(opt),
		capacity: capacity,
	}
}

// SaveMessage, GetHistory, AddMember

func (rdb *RedisClient) SaveMessage(ctx context.Context, room string, msg []byte) error {
	key := "room:" + room + ":messages"

	if err := rdb.client.LPush(ctx, key, string(msg)).Err(); err != nil {
		return err
	}

	if err := rdb.client.LTrim(ctx, key, 0, int64(rdb.capacity-1)).Err(); err != nil {
		return err
	}
	return nil
}

func (rdb *RedisClient) GetHistory(ctx context.Context, room string) ([]string, error) {
	key := "room:" + room + ":messages"

	res, err := rdb.client.LRange(ctx, key, 0, int64(rdb.capacity-1)).Result()

	if err != nil {
		return nil, err
	}

	return res, err
}

func (rdb *RedisClient) AddMember(ctx context.Context, room string, client string) error {
	key := "room:" + room + ":members"
	_, err := rdb.client.SAdd(ctx, key, client).Result()

	if err != nil {
		return err
	}

	return nil
}

func (rdb *RedisClient) RemoveMember(ctx context.Context, room string, client string) error {

	key := "room:" + room + ":members"
	_, err := rdb.client.SRem(ctx, key, client).Result()

	if err != nil {
		return err
	}

	return nil
}

func (rdb *RedisClient) GetMembers(ctx context.Context, room string) ([]string, error) {

	key := "room:" + room + ":members"
	res, err := rdb.client.SMembers(ctx, key).Result()

	if err != nil {
		return res, err
	}

	return res, nil
}

func (rdb *RedisClient) Publish(ctx context.Context, room string, msg []byte) error {
	key := "room:" + room

	err := rdb.client.Publish(ctx, key, msg).Err()

	if err != nil {
		return err.Err()
	}

	return nil
}

func (rdb *RedisClient) Subscribe(ctx context.Context, room string) *redis.PubSub {
	key := "room:" + room

	sub := rdb.client.Subscribe(ctx, key)

	return sub
}
