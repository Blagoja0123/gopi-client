// Package webhook provides a local receiver/inspector for incoming webhooks,
// replay of captured requests, and HMAC signature helpers.
package webhook

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"apiclient/internal/model"
	"apiclient/internal/runner/httprunner"
	"github.com/google/uuid"
)

const maxCaptureBody = 10 << 20

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Capture struct {
	ID         string    `json:"id"`
	Time       time.Time `json:"time"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Query      string    `json:"query"`
	Headers    []Header  `json:"headers"`
	Body       string    `json:"body"`
	BodyBase64 bool      `json:"bodyBase64"`
	Size       int64     `json:"size"`
	RemoteAddr string    `json:"remoteAddr"`
}

// Header returns the first value of a header (case-insensitive).
func (c Capture) Header(key string) string {
	for _, h := range c.Headers {
		if strings.EqualFold(h.Key, key) {
			return h.Value
		}
	}
	return ""
}

// Response is what the receiver answers to every request.
type Response struct {
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Body        string `json:"body"`
}

type Receiver struct {
	mu        sync.Mutex
	capacity  int
	captures  []Capture
	resp      Response
	srv       *http.Server
	addr      string
	onCapture func(Capture)
}

func NewReceiver(capacity int, onCapture func(Capture)) *Receiver {
	if capacity <= 0 {
		capacity = 200
	}
	return &Receiver{capacity: capacity, onCapture: onCapture,
		resp: Response{Status: 200, ContentType: "application/json", Body: `{"received":true}`}}
}

func (r *Receiver) SetResponse(resp Response) {
	if resp.Status == 0 {
		resp.Status = 200
	}
	r.mu.Lock()
	r.resp = resp
	r.mu.Unlock()
}

// Start listens on addr (e.g. "127.0.0.1:8090"; port 0 picks a free one) and
// returns the actual address.
func (r *Receiver) Start(addr string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.srv != nil {
		return "", errors.New("receiver already running on " + r.addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	r.srv = &http.Server{Handler: http.HandlerFunc(r.handle), ReadHeaderTimeout: 10 * time.Second}
	r.addr = ln.Addr().String()
	go r.srv.Serve(ln)
	return r.addr, nil
}

func (r *Receiver) Stop() error {
	r.mu.Lock()
	srv := r.srv
	r.srv, r.addr = nil, ""
	r.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func (r *Receiver) Status() (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.srv != nil, r.addr
}

func (r *Receiver) handle(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(req.Body, maxCaptureBody))
	rest, _ := io.Copy(io.Discard, req.Body)
	c := Capture{
		ID:         uuid.NewString(),
		Time:       time.Now(),
		Method:     req.Method,
		Path:       req.URL.Path,
		Query:      req.URL.RawQuery,
		Size:       int64(len(body)) + rest,
		RemoteAddr: req.RemoteAddr,
	}
	keys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if req.Host != "" {
		c.Headers = append(c.Headers, Header{"Host", req.Host})
	}
	for _, k := range keys {
		for _, v := range req.Header[k] {
			c.Headers = append(c.Headers, Header{k, v})
		}
	}
	if utf8.Valid(body) {
		c.Body = string(body)
	} else {
		c.Body = base64.StdEncoding.EncodeToString(body)
		c.BodyBase64 = true
	}

	r.mu.Lock()
	r.captures = append(r.captures, c)
	if over := len(r.captures) - r.capacity; over > 0 {
		r.captures = append([]Capture(nil), r.captures[over:]...)
	}
	resp := r.resp
	cb := r.onCapture
	r.mu.Unlock()
	if cb != nil {
		cb(c)
	}

	if resp.ContentType != "" {
		w.Header().Set("Content-Type", resp.ContentType)
	}
	w.WriteHeader(resp.Status)
	io.WriteString(w, resp.Body)
}

// Captures returns captured requests, oldest first.
func (r *Receiver) Captures() []Capture {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Capture(nil), r.captures...)
}

func (r *Receiver) Get(id string) (Capture, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.captures {
		if c.ID == id {
			return c, true
		}
	}
	return Capture{}, false
}

func (r *Receiver) Clear() {
	r.mu.Lock()
	r.captures = nil
	r.mu.Unlock()
}

// skipOnReplay lists headers that describe the original hop, not the payload.
var skipOnReplay = map[string]bool{
	"host": true, "content-length": true, "connection": true, "accept-encoding": true,
	"transfer-encoding": true, "user-agent": true, "x-forwarded-for": true,
	"x-forwarded-host": true, "x-forwarded-proto": true,
}

// ToRequest converts a capture into a request aimed at baseURL (the capture's
// path and query are appended). Used for replay and "save as request".
func ToRequest(c Capture, baseURL string) *model.Request {
	u := strings.TrimRight(baseURL, "/") + c.Path
	if c.Query != "" {
		u += "?" + c.Query
	}
	name := c.Method + " " + c.Path
	if ev := c.Header("X-GitHub-Event"); ev != "" {
		name = "GitHub " + ev
	}
	req := &model.Request{Name: name, Kind: model.KindHTTP, Method: c.Method, URL: u}
	for _, h := range c.Headers {
		if !skipOnReplay[strings.ToLower(h.Key)] {
			req.Headers = append(req.Headers, model.KV{Key: h.Key, Value: h.Value})
		}
	}
	if c.Body != "" && !c.BodyBase64 {
		bt := model.BodyText
		if strings.Contains(c.Header("Content-Type"), "json") {
			bt = model.BodyJSON
		}
		req.Body = &model.Body{Type: bt, Content: c.Body}
	}
	return req
}

// Replay re-sends a captured webhook to target. Captured headers (including
// Content-Type and signatures) are sent unchanged.
func Replay(ctx context.Context, c Capture, target string) (*httprunner.Response, error) {
	if c.BodyBase64 {
		return nil, errors.New("replaying binary webhook bodies is not supported")
	}
	return httprunner.Execute(ctx, ToRequest(c, target), httprunner.Options{})
}
