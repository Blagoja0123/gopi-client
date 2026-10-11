package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"apiclient/internal/importer/curl"
	"apiclient/internal/model"
	"apiclient/internal/runner/grpcrunner"
	"apiclient/internal/runner/httprunner"
	"apiclient/internal/runner/wsrunner"
	"apiclient/internal/vars"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"google.golang.org/grpc"
)

type RunResult struct {
	Response *httprunner.Response `json:"response"`
	Missing  []string             `json:"missing"`
}

type Preview struct {
	URL     string   `json:"url"`
	Missing []string `json:"missing"`
	Error   string   `json:"error"`
}

// PreviewRequest shows the resolved URL (secrets masked) and any
// unresolved variables, for live feedback while editing.
func (a *App) PreviewRequest(rel string, req *model.Request) Preview {
	r, missing, secrets, err := a.resolve(rel, req)
	if err != nil {
		return Preview{Error: err.Error()}
	}
	u := r.URL
	var q []string
	for _, p := range r.Params {
		if !p.Disabled && p.Key != "" {
			q = append(q, p.Key+"="+p.Value)
		}
	}
	if len(q) > 0 {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + strings.Join(q, "&")
	}
	return Preview{URL: vars.Mask(u, secrets), Missing: missing}
}

// SendHTTP executes a request. runID lets the UI cancel it with CancelRun.
func (a *App) SendHTTP(runID, rel string, req *model.Request) (*RunResult, error) {
	r, missing, secrets, err := a.resolve(rel, req)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.runs[runID] = cancel
	a.mu.Unlock()
	defer func() {
		cancel()
		a.mu.Lock()
		delete(a.runs, runID)
		a.mu.Unlock()
	}()
	resp, err := httprunner.Execute(ctx, r, httprunner.Options{Jar: a.cookieJar()})
	if err != nil {
		return nil, fmt.Errorf("%s", vars.Mask(err.Error(), secrets))
	}
	resp.URL = vars.Mask(resp.URL, secrets)
	for i := range resp.RequestHeaders {
		resp.RequestHeaders[i].Value = vars.Mask(resp.RequestHeaders[i].Value, secrets)
	}
	return &RunResult{Response: resp, Missing: missing}, nil
}

func (a *App) CancelRun(runID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.runs[runID]; ok {
		c()
	}
}

// ExportCurl renders the request as cURL. With resolved=true, variables are
// substituted (including secrets — the user asked for a runnable command).
func (a *App) ExportCurl(rel string, req *model.Request, resolved bool) (string, error) {
	r := req
	if resolved {
		var err error
		r, _, _, err = a.resolve(rel, req)
		if err != nil {
			return "", err
		}
	} else if ws, err := a.workspace(); err == nil && rel != "" {
		if r, err = ws.Effective(rel, req); err != nil {
			return "", err
		}
	}
	return curl.Export(r), nil
}

func (a *App) ImportCurl(cmd string) (*model.Request, error) {
	return curl.Parse(cmd)
}

// ---- WebSocket ----

// WSConnect opens a session; frames arrive as "ws:<sessionID>" events.
func (a *App) WSConnect(sessionID, rel string, req *model.Request) error {
	r, _, _, err := a.resolve(rel, req)
	if err != nil {
		return err
	}
	event := "ws:" + sessionID
	s, err := wsrunner.Connect(a.ctx, r, func(e wsrunner.Event) { runtime.EventsEmit(a.ctx, event, e) })
	if err != nil {
		return err
	}
	a.mu.Lock()
	if old, ok := a.wsConns[sessionID]; ok {
		old.Close()
	}
	a.wsConns[sessionID] = s
	a.mu.Unlock()
	go func() {
		<-s.Done()
		a.mu.Lock()
		if a.wsConns[sessionID] == s {
			delete(a.wsConns, sessionID)
		}
		a.mu.Unlock()
	}()
	return nil
}

// WSSend sends a text frame. Variables in the message are resolved.
func (a *App) WSSend(sessionID, text string) error {
	a.mu.Lock()
	s := a.wsConns[sessionID]
	a.mu.Unlock()
	if s == nil {
		return errInvalid("not connected")
	}
	r, _, _, err := a.resolve("", &model.Request{Body: &model.Body{Content: text}})
	if err != nil {
		return err
	}
	return s.Send(r.Body.Content)
}

func (a *App) WSClose(sessionID string) {
	a.mu.Lock()
	s := a.wsConns[sessionID]
	a.mu.Unlock()
	if s != nil {
		s.Close()
	}
}

// ---- gRPC ----

type grpcCall struct {
	stream *grpcrunner.Stream
	conn   *grpc.ClientConn
	src    grpcrunner.Source
}

// grpcSource dials and picks a descriptor source: .proto files if the
// request lists any (paths relative to the workspace root), else reflection.
func (a *App) grpcSource(rel string, req *model.Request) (*model.Request, *grpc.ClientConn, grpcrunner.Source, error) {
	r, _, _, err := a.resolve(rel, req)
	if err != nil {
		return nil, nil, nil, err
	}
	conn, err := grpcrunner.Dial(r)
	if err != nil {
		return nil, nil, nil, err
	}
	if r.GRPC != nil && len(r.GRPC.ProtoFiles) > 0 {
		base := ""
		if ws, err := a.workspace(); err == nil {
			base = ws.Root
		}
		abs := func(ps []string) []string {
			out := make([]string, len(ps))
			for i, p := range ps {
				if filepath.IsAbs(p) || base == "" {
					out[i] = p
				} else {
					out[i] = filepath.Join(base, p)
				}
			}
			return out
		}
		src, err := grpcrunner.NewFileSource(abs(r.GRPC.ImportPaths), abs(r.GRPC.ProtoFiles))
		if err != nil {
			conn.Close()
			return nil, nil, nil, err
		}
		return r, conn, src, nil
	}
	return r, conn, grpcrunner.NewReflectionSource(a.ctx, conn), nil
}

// GRPCDescribe lists services and methods with JSON templates.
func (a *App) GRPCDescribe(rel string, req *model.Request) ([]grpcrunner.ServiceInfo, error) {
	_, conn, src, err := a.grpcSource(rel, req)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer src.Close()
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	return grpcrunner.Describe(ctx, src)
}

// GRPCStart invokes the method; events arrive as "grpc:<callID>". For
// client/bidi streaming methods the stream stays open for GRPCSend.
func (a *App) GRPCStart(callID, rel string, req *model.Request) error {
	r, conn, src, err := a.grpcSource(rel, req)
	if err != nil {
		return err
	}
	event := "grpc:" + callID
	st, err := grpcrunner.Run(a.ctx, conn, src, r, func(e grpcrunner.Event) { runtime.EventsEmit(a.ctx, event, e) })
	if err != nil {
		conn.Close()
		src.Close()
		return err
	}
	call := &grpcCall{stream: st, conn: conn, src: src}
	a.mu.Lock()
	a.grpcCalls[callID] = call
	a.mu.Unlock()
	go func() {
		<-st.Done()
		src.Close()
		conn.Close()
		a.mu.Lock()
		if a.grpcCalls[callID] == call {
			delete(a.grpcCalls, callID)
		}
		a.mu.Unlock()
	}()
	return nil
}

func (a *App) grpcCall(callID string) (*grpcCall, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c := a.grpcCalls[callID]
	if c == nil {
		return nil, errInvalid("call is not active")
	}
	return c, nil
}

func (a *App) GRPCSend(callID, body string) error {
	c, err := a.grpcCall(callID)
	if err != nil {
		return err
	}
	r, _, _, err := a.resolve("", &model.Request{Body: &model.Body{Content: body}})
	if err != nil {
		return err
	}
	return c.stream.Send(r.Body.Content)
}

func (a *App) GRPCCloseSend(callID string) error {
	c, err := a.grpcCall(callID)
	if err != nil {
		return err
	}
	return c.stream.CloseSend()
}

func (a *App) GRPCCancel(callID string) {
	if c, err := a.grpcCall(callID); err == nil {
		c.stream.Cancel()
	}
}
