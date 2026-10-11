package wsrunner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"apiclient/internal/model"
	"github.com/coder/websocket"
)

func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"chat"}})
		if err != nil {
			return
		}
		defer c.CloseNow()
		// Greet with the header we received so the test can verify it.
		c.Write(r.Context(), websocket.MessageText, []byte("hello "+r.Header.Get("X-Test")+" "+r.URL.Query().Get("room")))
		for {
			typ, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if string(data) == "bye" {
				c.Close(websocket.StatusNormalClosure, "server bye")
				return
			}
			c.Write(r.Context(), typ, data)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

type collector struct {
	mu     sync.Mutex
	events []Event
	ch     chan Event
}

func newCollector() *collector { return &collector{ch: make(chan Event, 64)} }

func (c *collector) on(e Event) {
	c.mu.Lock()
	c.events = append(c.events, e)
	c.mu.Unlock()
	c.ch <- e
}

func (c *collector) wait(t *testing.T, typ string) Event {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case e := <-c.ch:
			if e.Type == typ {
				return e
			}
		case <-timeout:
			t.Fatalf("timed out waiting for %q event; got %+v", typ, c.events)
		}
	}
}

func wsURL(srv *httptest.Server) string { return "ws" + strings.TrimPrefix(srv.URL, "http") }

func TestConnectSendReceiveClose(t *testing.T) {
	srv := echoServer(t)
	col := newCollector()
	s, err := Connect(context.Background(), &model.Request{
		Kind:    model.KindWS,
		URL:     wsURL(srv),
		Params:  []model.KV{{Key: "room", Value: "lobby"}},
		Headers: []model.KV{{Key: "X-Test", Value: "abc"}},
		WS:      &model.WS{Subprotocols: []string{"chat"}},
	}, col.on)
	if err != nil {
		t.Fatal(err)
	}
	open := col.wait(t, EventOpen)
	if !strings.Contains(open.Data, "chat") {
		t.Errorf("open event should report negotiated subprotocol: %+v", open)
	}
	if greet := col.wait(t, EventMessage); greet.Data != "hello abc lobby" {
		t.Fatalf("greeting = %q", greet.Data)
	}
	if err := s.Send("ping"); err != nil {
		t.Fatal(err)
	}
	if e := col.wait(t, EventSent); e.Data != "ping" {
		t.Errorf("sent = %+v", e)
	}
	if e := col.wait(t, EventMessage); e.Data != "ping" {
		t.Errorf("echo = %+v", e)
	}
	s.Close()
	col.wait(t, EventClose)
	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed")
	}
	if err := s.Send("after close"); err == nil {
		t.Error("send after close should fail")
	}
	col.mu.Lock()
	defer col.mu.Unlock()
	for i, e := range col.events {
		if e.Seq != int64(i+1) {
			t.Errorf("event %d has seq %d; want 1..n in emission order", i, e.Seq)
		}
	}
}

func TestServerInitiatedClose(t *testing.T) {
	srv := echoServer(t)
	col := newCollector()
	s, err := Connect(context.Background(), &model.Request{URL: wsURL(srv)}, col.on)
	if err != nil {
		t.Fatal(err)
	}
	col.wait(t, EventMessage)
	s.Send("bye")
	e := col.wait(t, EventClose)
	if e.Code != int(websocket.StatusNormalClosure) || !strings.Contains(e.Data, "server bye") {
		t.Errorf("close = %+v", e)
	}
}

func TestContextCancelDisconnects(t *testing.T) {
	srv := echoServer(t)
	col := newCollector()
	ctx, cancel := context.WithCancel(context.Background())
	s, err := Connect(ctx, &model.Request{URL: wsURL(srv)}, col.on)
	if err != nil {
		t.Fatal(err)
	}
	col.wait(t, EventOpen)
	cancel()
	select {
	case <-s.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not disconnect")
	}
}

func TestDialError(t *testing.T) {
	if _, err := Connect(context.Background(), &model.Request{URL: "ws://127.0.0.1:1/nope"}, func(Event) {}); err == nil {
		t.Fatal("expected dial error")
	}
}
