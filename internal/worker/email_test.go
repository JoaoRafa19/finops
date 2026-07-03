package worker

import (
	"context"
	"testing"
	"time"

	"finops/internal/models"
	service "finops/internal/services"

	"github.com/redis/go-redis/v9"
)

// Integração: exige Redis local (docker compose); pula se indisponível.
func TestEmailWorkerDrainsQueues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis indisponível: %v", err)
	}

	q := service.NewQueueEmailService(rdb)
	if err := q.SendPasswordReset(ctx, "smoke@test.dev", "https://x/reset"); err != nil {
		t.Fatal(err)
	}
	if err := q.SendVerifyEmail(ctx, "smoke@test.dev", "https://x/verify"); err != nil {
		t.Fatal(err)
	}

	wctx, wcancel := context.WithCancel(ctx)
	defer wcancel()
	go RunEmailWorker(wctx, rdb, service.NewNoopEmailService())

	deadline := time.Now().Add(5 * time.Second)
	for {
		nReset, _ := rdb.LLen(ctx, models.EmailResetKey).Result()
		nVerify, _ := rdb.LLen(ctx, models.EmailVerifyKey).Result()
		if nReset == 0 && nVerify == 0 {
			return // filas drenadas
		}
		if time.Now().After(deadline) {
			t.Fatalf("filas não drenadas: reset=%d verify=%d", nReset, nVerify)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
