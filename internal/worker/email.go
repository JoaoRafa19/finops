package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"finops/internal/models"
	service "finops/internal/services"

	"github.com/redis/go-redis/v9"
)

// RunEmailWorker consome as filas de e-mail (reset e verify) até ctx cancelar.
// Roda embutido no monolito (goroutine) ou standalone via cmd/email-worker.
func RunEmailWorker(ctx context.Context, rdb *redis.Client, sender service.EmailService) {
	slog.Info("email_worker_started")
	for {
		res, err := rdb.BRPop(ctx, 30*time.Second, models.EmailResetKey, models.EmailVerifyKey).Result()
		if ctx.Err() != nil {
			slog.Info("email_worker_stopped")
			return
		}
		if err == redis.Nil {
			continue // timeout sem mensagem: volta a esperar
		}
		if err != nil {
			slog.Error("email_worker_brpop", "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		key, raw := res[0], res[1]

		var msg models.VerifyResetEmailBody
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			slog.Error("email_worker_payload_invalido", "key", key, "err", err)
			continue
		}

		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		switch key {
		case models.EmailResetKey:
			err = sender.SendPasswordReset(sendCtx, msg.Email, msg.ResetUrl)
		case models.EmailVerifyKey:
			err = sender.SendVerifyEmail(sendCtx, msg.Email, msg.VerifyUrl)
		}
		cancel()

		if err != nil {
			// ponytail: log-and-drop; usuário reenvia pelo fluxo normal. DLQ se virar problema.
			slog.Error("email_worker_send_falhou", "key", key, "to", msg.Email, "err", err)
		}
	}
}
