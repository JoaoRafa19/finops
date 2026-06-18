package app

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context, cfg Config) (*redis.Client, error) {
	var opts *redis.Options
	if cfg.RedisURL != "" {
		var err error
		opts, err = redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
		}
	} else {
		opts = &redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		}
	}
	client := redis.NewClient(opts)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("error connecting redis: %w", err)
	}

	return client, nil
}
