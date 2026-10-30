package events

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSE(t *testing.T) {
	b := New()
	b.Publish("pose", map[string]any{"x": 1})
	b.Publish("alert", map[string]any{"kind": "obstacle"})
	srv := httptest.NewServer(http.HandlerFunc(b.Handler))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"?types=alert,reply", nil)
	req.Header.Set("Last-Event-ID", "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	go func() {
		time.Sleep(100 * time.Millisecond)
		b.Publish("pose", nil) // filtered out
		b.Publish("reply", map[string]any{"text": "bip"})
	}()
	sc := bufio.NewScanner(res.Body)
	var evs []string
	for sc.Scan() && len(evs) < 2 {
		if l := sc.Text(); strings.HasPrefix(l, "event: ") {
			evs = append(evs, strings.TrimPrefix(l, "event: "))
		}
	}
	// Last-Event-ID 1 replays the alert (id 2); the pose is filtered out; then the live reply.
	if strings.Join(evs, ",") != "alert,reply" {
		t.Fatal(evs)
	}
}

func TestReplay(t *testing.T) {
	b := New()
	for i := 0; i < keep+10; i++ {
		b.Publish("pose", i)
	}
	if got := b.Since(0); len(got) != keep || got[0].ID != 11 {
		t.Fatal(len(got), got[0].ID)
	}
	if got := b.Since(keep + 5); len(got) != 5 {
		t.Fatal(len(got))
	}
}
