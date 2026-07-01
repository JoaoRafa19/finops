package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		xff        string
		xreal      string
		remoteAddr string
		want       string
	}{
		{"xff first hop", "1.2.3.4, 5.6.7.8", "", "10.0.0.1:1234", "1.2.3.4"},
		{"xff single", "1.2.3.4", "", "10.0.0.1:1234", "1.2.3.4"},
		{"x-real-ip fallback", "", "9.9.9.9", "10.0.0.1:1234", "9.9.9.9"},
		{"remote addr", "", "", "10.0.0.1:1234", "10.0.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/login", nil)
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xreal != "" {
				r.Header.Set("X-Real-IP", tc.xreal)
			}
			r.RemoteAddr = tc.remoteAddr
			if got := clientIP(r); got != tc.want {
				t.Errorf("clientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
