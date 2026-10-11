package grpcrunner

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"apiclient/internal/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type testServer struct {
	conn   *grpc.ClientConn
	health *health.Server
	mu     sync.Mutex
	md     metadata.MD
}

func (ts *testServer) lastMD() metadata.MD {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.md
}

// startServer runs an in-process server with the real health + reflection
// services. If echo is non-nil, unknown services are handled as a dynamic
// echo of that message type (used for the .proto file tests).
func startServer(t *testing.T, echo protoreflect.MessageDescriptor) *testServer {
	t.Helper()
	ts := &testServer{}
	lis := bufconn.Listen(1 << 20)
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			md, _ := metadata.FromIncomingContext(ctx)
			ts.mu.Lock()
			ts.md = md
			ts.mu.Unlock()
			return h(ctx, req)
		}),
	}
	if echo != nil {
		opts = append(opts, grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
			for {
				m := dynamicpb.NewMessage(echo)
				if err := stream.RecvMsg(m); err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
				if err := stream.SendMsg(m); err != nil {
					return err
				}
			}
		}))
	}
	s := grpc.NewServer(opts...)
	ts.health = health.NewServer()
	healthpb.RegisterHealthServer(s, ts.health)
	reflection.Register(s)
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	ts.conn = conn
	return ts
}

type events struct {
	ch  chan Event
	all []Event
}

func newEvents() *events      { return &events{ch: make(chan Event, 64)} }
func (e *events) on(ev Event) { e.ch <- ev }
func (e *events) wait(t *testing.T, typ string) Event {
	t.Helper()
	to := time.After(3 * time.Second)
	for {
		select {
		case ev := <-e.ch:
			e.all = append(e.all, ev)
			if ev.Type == typ {
				return ev
			}
		case <-to:
			t.Fatalf("timeout waiting for %q; got %+v", typ, e.all)
		}
	}
}

func healthReq(method, body string) *model.Request {
	return &model.Request{
		Kind: model.KindGRPC, URL: "bufnet",
		Headers: []model.KV{{Key: "x-test", Value: "abc"}, {Key: "x-off", Value: "1", Disabled: true}},
		Auth:    &model.Auth{Type: model.AuthBearer, Token: "tok"},
		Body:    &model.Body{Type: model.BodyJSON, Content: body},
		GRPC:    &model.GRPC{Service: "grpc.health.v1.Health", Method: method, Plaintext: true},
	}
}

func TestReflectionListAndDescribe(t *testing.T) {
	ts := startServer(t, nil)
	src := NewReflectionSource(context.Background(), ts.conn)
	defer src.Close()
	services, err := Describe(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var health *ServiceInfo
	for i := range services {
		if services[i].Name == "grpc.health.v1.Health" {
			health = &services[i]
		}
		if strings.HasPrefix(services[i].Name, "grpc.reflection.") {
			t.Errorf("reflection service should be hidden: %s", services[i].Name)
		}
	}
	if health == nil {
		t.Fatalf("health service not listed: %+v", services)
	}
	methods := map[string]MethodInfo{}
	for _, m := range health.Methods {
		methods[m.Name] = m
	}
	if m := methods["Check"]; m.ClientStreaming || m.ServerStreaming || !strings.Contains(m.Template, `"service"`) {
		t.Errorf("Check = %+v", m)
	}
	if m := methods["Watch"]; !m.ServerStreaming || m.ClientStreaming {
		t.Errorf("Watch = %+v", m)
	}
}

func TestUnaryCheck(t *testing.T) {
	ts := startServer(t, nil)
	ts.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	src := NewReflectionSource(context.Background(), ts.conn)
	defer src.Close()
	ev := newEvents()
	if _, err := Run(context.Background(), ts.conn, src, healthReq("Check", `{"service":""}`), ev.on); err != nil {
		t.Fatal(err)
	}
	msg := ev.wait(t, EventMessage)
	var got map[string]any
	if err := json.Unmarshal([]byte(msg.Data), &got); err != nil || got["status"] != "SERVING" {
		t.Fatalf("message = %s (%v)", msg.Data, err)
	}
	end := ev.wait(t, EventEnd)
	if end.Code != "OK" {
		t.Errorf("end = %+v", end)
	}
	// sent, (headers), message, end — numbered in emission order so the UI
	// can restore order if delivery reorders them.
	if end.Seq != int64(len(ev.all)) || msg.Seq >= end.Seq {
		t.Errorf("seq: message=%d end=%d events=%d", msg.Seq, end.Seq, len(ev.all))
	}
	md := ts.lastMD()
	if md.Get("x-test")[0] != "abc" || len(md.Get("x-off")) != 0 || md.Get("authorization")[0] != "Bearer tok" {
		t.Errorf("metadata = %v", md)
	}
}

func TestUnaryErrorStatus(t *testing.T) {
	ts := startServer(t, nil)
	src := NewReflectionSource(context.Background(), ts.conn)
	defer src.Close()
	ev := newEvents()
	Run(context.Background(), ts.conn, src, healthReq("Check", `{"service":"nope"}`), ev.on)
	end := ev.wait(t, EventEnd)
	if end.Code != "NOT_FOUND" {
		t.Errorf("end = %+v", end)
	}
}

func TestInvalidInput(t *testing.T) {
	ts := startServer(t, nil)
	src := NewReflectionSource(context.Background(), ts.conn)
	defer src.Close()
	if _, err := Run(context.Background(), ts.conn, src, healthReq("Check", `{"nope":1}`), func(Event) {}); err == nil {
		t.Error("expected error for unknown field")
	}
	if _, err := Run(context.Background(), ts.conn, src, healthReq("Missing", `{}`), func(Event) {}); err == nil {
		t.Error("expected error for unknown method")
	}
}

func TestServerStreamWatch(t *testing.T) {
	ts := startServer(t, nil)
	ts.health.SetServingStatus("svc", healthpb.HealthCheckResponse_SERVING)
	src := NewReflectionSource(context.Background(), ts.conn)
	defer src.Close()
	ev := newEvents()
	st, err := Run(context.Background(), ts.conn, src, healthReq("Watch", `{"service":"svc"}`), ev.on)
	if err != nil {
		t.Fatal(err)
	}
	if m := ev.wait(t, EventMessage); !strings.Contains(m.Data, `"SERVING"`) {
		t.Fatalf("first = %s", m.Data)
	}
	ts.health.SetServingStatus("svc", healthpb.HealthCheckResponse_NOT_SERVING)
	if m := ev.wait(t, EventMessage); !strings.Contains(m.Data, "NOT_SERVING") {
		t.Fatalf("second = %s", m.Data)
	}
	st.Cancel()
	if end := ev.wait(t, EventEnd); end.Code != "CANCELLED" {
		t.Errorf("end = %+v", end)
	}
}

func TestProtoFileSourceAndBidi(t *testing.T) {
	src, err := NewFileSource([]string{"testdata"}, []string{"echo.proto"})
	if err != nil {
		t.Fatal(err)
	}
	services, err := Describe(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0].Name != "echo.v1.Echo" || len(services[0].Methods) != 2 {
		t.Fatalf("services = %+v", services)
	}
	chat := services[0].Methods[0]
	if chat.Name != "Chat" || !chat.ClientStreaming || !chat.ServerStreaming {
		t.Fatalf("chat = %+v", chat)
	}
	for _, field := range []string{`"text"`, `"count"`, `"tags"`, `"inner"`, `"flag"`} {
		if !strings.Contains(chat.Template, field) {
			t.Errorf("template missing %s:\n%s", field, chat.Template)
		}
	}

	sd, _ := src.FindService(context.Background(), "echo.v1.Echo")
	ts := startServer(t, sd.Methods().ByName("Chat").Input())
	ev := newEvents()
	req := &model.Request{Kind: model.KindGRPC, URL: "bufnet", Body: &model.Body{Type: model.BodyJSON, Content: `{"text":"one"}`},
		GRPC: &model.GRPC{Service: "echo.v1.Echo", Method: "Chat", Plaintext: true}}
	st, err := Run(context.Background(), ts.conn, src, req, ev.on)
	if err != nil {
		t.Fatal(err)
	}
	// Bidi: the initial body is sent, the stream stays open for more.
	if m := ev.wait(t, EventMessage); !strings.Contains(m.Data, `"one"`) {
		t.Fatalf("echo 1 = %s", m.Data)
	}
	if err := st.Send(`{"text":"two","at":"2026-01-01T00:00:00Z","count":3}`); err != nil {
		t.Fatal(err)
	}
	m := ev.wait(t, EventMessage)
	if !strings.Contains(m.Data, `"two"`) || !strings.Contains(m.Data, "2026-01-01T00:00:00Z") {
		t.Fatalf("echo 2 = %s", m.Data)
	}
	st.CloseSend()
	if end := ev.wait(t, EventEnd); end.Code != "OK" {
		t.Errorf("end = %+v", end)
	}
	select {
	case <-st.Done():
	case <-time.After(time.Second):
		t.Fatal("stream not done")
	}
}

func TestFileSourceErrors(t *testing.T) {
	if _, err := NewFileSource([]string{"testdata"}, []string{"missing.proto"}); err == nil {
		t.Error("expected error for missing proto")
	}
	if _, err := NewFileSource(nil, nil); err == nil {
		t.Error("expected error with no files")
	}
}

func TestTarget(t *testing.T) {
	cases := []struct {
		url, addr string
		plain     bool
		cfg       bool
	}{
		{"localhost:50051", "localhost:50051", false, false},
		{"localhost:50051", "localhost:50051", true, true},
		{"grpc://h:1", "h:1", true, false},
		{"grpcs://h:1", "h:1", false, false},
		{"https://h", "h:443", false, false},
		{"http://h", "h:80", true, false},
	}
	for _, c := range cases {
		addr, plain := target(c.url, c.cfg)
		if addr != c.addr || plain != c.plain {
			t.Errorf("target(%q,%v) = %q,%v want %q,%v", c.url, c.cfg, addr, plain, c.addr, c.plain)
		}
	}
}
