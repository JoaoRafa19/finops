package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context, cfg Config) (*redis.Client, error) {
	// Aceita URL vindo em REDIS_URL ou REDIS_ADDR (defesa contra config manual do dashboard).
	url := firstURL(cfg.RedisURL, cfg.RedisAddr)

	var opts *redis.Options
	if url != "" {
		slog.Info("redis_connect_attempt", "url", maskDSN(url))
		var err error
		opts, err = redis.ParseURL(url)
		if err != nil {
			return nil, fmt.Errorf("invalid redis URL %q: %w", maskDSN(url), err)
		}
	} else {
		slog.Info("redis_connect_attempt", "addr", cfg.RedisAddr)
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

// firstURL retorna o primeiro dos valores que parece uma URL (contém "://").
// Cobre o caso de operador ter setado a URL do managed em REDIS_ADDR por engano.
func firstURL(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if strings.Contains(c, "://") {
			return c
		}
	}
	return ""
}
