package relay

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRelay(t *testing.T) {
	srv := &Server{Token: func() string { return "dev-token" }}
	mux := http.NewServeMux()
	mux.Handle("/robot", srv.RobotHandler())
	mux.Handle("/", srv)
	web := httptest.NewServer(mux)
	defer web.Close()

	// Offline: a clear 503.
	r, _ := http.Get(web.URL + "/api/hello")
	if r.StatusCode != 503 {
		t.Fatal("offline should be 503", r.StatusCode)
	}

	var cancelled atomic.Bool
	robot := http.NewServeMux()
	robot.HandleFunc("GET /api/hello", func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr != "127.0.0.1:0" || r.Header.Get("Authorization") != "" {
			t.Errorf("relay requests count as local and carry no credentials: %q %q", r.RemoteAddr, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"q":%q}`, r.URL.Query().Get("q"))
	})
	robot.HandleFunc("POST /api/echo", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.WriteHeader(201)
		w.Write(b)
	})
	robot.HandleFunc("GET /api/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; ; i++ {
			fmt.Fprintf(w, "data: %d\n\n", i)
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				cancelled.Store(true)
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
	})
	ws := "ws" + strings.TrimPrefix(web.URL, "http") + "/robot"

	// A wrong device token never gets in.
	bad := &Client{URL: ws, Token: func() string { return "nope" }, Handler: robot}
	ctxBad, stopBad := context.WithTimeout(context.Background(), 300*time.Millisecond)
	bad.Run(ctxBad)
	stopBad()
	if up, _ := srv.Connected(); up {
		t.Fatal("bad token connected")
	}

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	up := make(chan bool, 4)
	c := &Client{URL: ws, Token: func() string { return "dev-token" }, Handler: robot, OnState: func(b bool) { up <- b }}
	go c.Run(ctx)
	if !<-up {
		t.Fatal("not up")
	}
	for i := 0; i < 50; i++ { // the server side registers right after the handshake
		if ok, _ := srv.Connected(); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	r, err := http.Get(web.URL + "/api/hello?q=hola")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != 200 || string(b) != `{"q":"hola"}` || r.Header.Get("Content-Type") != "application/json" {
		t.Fatal(r.StatusCode, string(b), r.Header)
	}
	req, _ := http.NewRequest("POST", web.URL+"/api/echo", strings.NewReader(`{"x":1}`))
	req.Header.Set("Authorization", "Bearer page-secret")
	r, _ = http.DefaultClient.Do(req)
	b, _ = io.ReadAll(r.Body)
	if r.StatusCode != 201 || string(b) != `{"x":1}` {
		t.Fatal(r.StatusCode, string(b))
	}

	// A stream arrives event by event, and closing it stops the robot's handler.
	sctx, sstop := context.WithCancel(context.Background())
	req, _ = http.NewRequestWithContext(sctx, "GET", web.URL+"/api/stream", nil)
	r, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	rd := bufio.NewReader(r.Body)
	t0 := time.Now()
	got := 0
	for got < 3 {
		line, err := rd.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "data:") {
			got++
		}
	}
	if time.Since(t0) > 2*time.Second {
		t.Fatal("stream was not live")
	}
	sstop()
	r.Body.Close()
	for i := 0; i < 100 && !cancelled.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if !cancelled.Load() {
		t.Fatal("closing the page did not cancel the robot's handler")
	}

	// The robot leaves: back to 503.
	stop()
	for i := 0; i < 100; i++ {
		if ok, _ := srv.Connected(); !ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, _ = http.Get(web.URL + "/api/hello")
	if r.StatusCode != 503 {
		t.Fatal("robot gone should be 503", r.StatusCode)
	}
}
