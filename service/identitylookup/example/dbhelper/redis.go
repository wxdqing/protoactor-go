package dbhelper

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string
	User     string
	Password string
	DB       int
}

func NewRedisClient() redis.UniversalClient {
	cfg := &RedisConfig{
		Addr:     "192.168.0.138:7000",
		Password: "123456",
		DB:       0,
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		OnConnect: func(ctx context.Context, conn *redis.Conn) error {
			return nil
		},
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		panic(err)
	}

	return client
}
