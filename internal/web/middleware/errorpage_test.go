package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotFoundInterceptor_RewritesPlainTextTo404Page(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	rr := httptest.NewRecorder()
	NotFoundInterceptor(next).ServeHTTP(rr, httptest.NewRequest("GET", "/nowhere", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Página não encontrada") {
		t.Errorf("body should contain styled page, got %q", rr.Body.String())
	}
}

func TestNotFoundInterceptor_PassesThroughSuccess(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<h1>ok</h1>"))
	})
	rr := httptest.NewRecorder()
	NotFoundInterceptor(next).ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "<h1>ok</h1>" {
		t.Errorf("body mismatch: %q", rr.Body.String())
	}
}

func TestPanicRecover_ReturnsStyled500(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	rr := httptest.NewRecorder()
	PanicRecover(next).ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Algo deu errado") {
		t.Errorf("body should contain styled page")
	}
}
