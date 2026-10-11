// Package wsrunner manages interactive WebSocket sessions.
package wsrunner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"apiclient/internal/model"
	"apiclient/internal/runner/httprunner"
	"github.com/coder/websocket"
)

const (
	EventOpen    = "open"
	EventMessage = "message"
	EventSent    = "sent"
	EventClose   = "close"
	EventError   = "error"
)

// Event is one item of a session's timeline. Seq orders events, since UI
// event delivery may reorder them.
type Event struct {
	Seq    int64     `json:"seq"`
	Type   string    `json:"type"`
	Data   string    `json:"data"`
	Binary bool      `json:"binary,omitempty"`
	Code   int       `json:"code,omitempty"`
	Time   time.Time `json:"time"`
}

type Session struct {
	conn    *websocket.Conn
	ctx     context.Context
	cancel  context.CancelFunc
	onEvent func(Event)
	done    chan struct{}
	once    sync.Once
	seq     atomic.Int64
}

func (s *Session) emit(e Event) {
	e.Seq = s.seq.Add(1)
	s.onEvent(e)
}

// Connect dials the (resolved) request's URL with its headers, params and
// auth, and starts a read loop that reports every frame through onEvent.
func Connect(ctx context.Context, r *model.Request, onEvent func(Event)) (*Session, error) {
	httpReq := r.Clone()
	httpReq.Body = nil
	// Build accepts any scheme, so reuse it for URL/params/headers/auth.
	if !strings.Contains(httpReq.URL, "://") {
		httpReq.URL = "ws://" + httpReq.URL
	}
	req, err := httprunner.Build(ctx, httpReq)
	if err != nil {
		return nil, err
	}
	opts := &websocket.DialOptions{HTTPHeader: req.Header}
	if r.WS != nil {
		opts.Subprotocols = r.WS.Subprotocols
	}
	if r.Settings != nil && r.Settings.InsecureSkipVerify {
		opts.HTTPClient = &http.Client{Transport: insecureTransport()}
	}
	dialCtx, dialCancel := context.WithTimeout(ctx, 30*time.Second)
	defer dialCancel()
	conn, resp, err := websocket.Dial(dialCtx, req.URL.String(), opts)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("websocket handshake failed: %s: %w", resp.Status, err)
		}
		return nil, err
	}
	conn.SetReadLimit(32 << 20)

	sctx, cancel := context.WithCancel(ctx)
	s := &Session{conn: conn, ctx: sctx, cancel: cancel, onEvent: onEvent, done: make(chan struct{})}
	info := "connected"
	if p := conn.Subprotocol(); p != "" {
		info += " (subprotocol " + p + ")"
	}
	s.emit(Event{Type: EventOpen, Data: info, Time: time.Now()})
	go s.readLoop()
	return s, nil
}

func (s *Session) readLoop() {
	defer s.finish()
	for {
		typ, data, err := s.conn.Read(s.ctx)
		if err != nil {
			e := Event{Type: EventClose, Time: time.Now()}
			var ce websocket.CloseError
			switch {
			case errors.As(err, &ce):
				e.Code = int(ce.Code)
				e.Data = fmt.Sprintf("closed: %d %s", ce.Code, ce.Reason)
			case s.ctx.Err() != nil:
				e.Data = "disconnected"
			default:
				e.Data = err.Error()
			}
			s.emit(e)
			return
		}
		s.emit(Event{Type: EventMessage, Data: string(data), Binary: typ == websocket.MessageBinary, Time: time.Now()})
	}
}

func (s *Session) finish() {
	s.once.Do(func() {
		s.cancel()
		s.conn.CloseNow()
		close(s.done)
	})
}

func (s *Session) Send(text string) error {
	select {
	case <-s.done:
		return errors.New("connection is closed")
	default:
	}
	if err := s.conn.Write(s.ctx, websocket.MessageText, []byte(text)); err != nil {
		return err
	}
	s.emit(Event{Type: EventSent, Data: text, Time: time.Now()})
	return nil
}

// Close performs a normal closing handshake (bounded by a short timeout).
func (s *Session) Close() error {
	go func() {
		select {
		case <-s.done:
		case <-time.After(3 * time.Second):
			s.finish()
		}
	}()
	return s.conn.Close(websocket.StatusNormalClosure, "client closed")
}

func (s *Session) Done() <-chan struct{} { return s.done }
