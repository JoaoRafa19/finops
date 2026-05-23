package observability

import (
	"context"
	"testing"
)

func TestWithRequestContextStoresRequestIDAndLogger(t *testing.T) {
	t.Parallel()

	ctx := WithRequestContext(context.Background(), "req-123")

	requestID, ok := RequestID(ctx)
	if !ok {
		t.Fatal("expected request id in context")
	}

	if requestID != "req-123" {
		t.Fatalf("expected request id %q, got %q", "req-123", requestID)
	}

	if logger := Logger(ctx); logger == nil {
		t.Fatal("expected logger in context")
	}
}

func TestLoggerFallsBackToDefault(t *testing.T) {
	t.Parallel()

	if logger := Logger(context.Background()); logger == nil {
		t.Fatal("expected default logger")
	}
}
