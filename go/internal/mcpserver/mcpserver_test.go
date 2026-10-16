package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireToken(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	h := RequireToken(ok, func() string { return "s3cret" })
	try := func(remote, auth string) int {
		r := httptest.NewRequest("GET", "/mcp", nil)
		r.RemoteAddr = remote
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if try("127.0.0.1:5000", "") != 204 {
		t.Fatal("loopback must pass")
	}
	if try("192.168.1.9:5000", "") != 401 || try("192.168.1.9:5000", "Bearer nope") != 401 {
		t.Fatal("LAN without token must fail")
	}
	if try("192.168.1.9:5000", "Bearer s3cret") != 204 {
		t.Fatal("LAN with token must pass")
	}
	empty := RequireToken(ok, func() string { return "" })
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.2:1"
	w := httptest.NewRecorder()
	empty.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("no token configured must refuse the LAN")
	}
}
