package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDGeneratesAndStoresID(t *testing.T) {
	t.Parallel()

	var (
		requestID string
		loggerOK  bool
	)

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		requestID, ok = RequestIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected request id in context")
		}

		_, loggerOK = LoggerFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if requestID == "" {
		t.Fatal("expected generated request id")
	}

	if got := rec.Header().Get(RequestIDHeader); got != requestID {
		t.Fatalf("expected response request id %q, got %q", requestID, got)
	}

	if !loggerOK {
		t.Fatal("expected logger in context")
	}
}

func TestRequestIDUsesInboundHeader(t *testing.T) {
	t.Parallel()

	const inboundRequestID = "req-123"

	var requestID string

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		requestID, ok = RequestIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected request id in context")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(RequestIDHeader, inboundRequestID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if requestID != inboundRequestID {
		t.Fatalf("expected request id %q, got %q", inboundRequestID, requestID)
	}

	if got := rec.Header().Get(RequestIDHeader); got != inboundRequestID {
		t.Fatalf("expected response request id %q, got %q", inboundRequestID, got)
	}
}
