// Package grpcrunner invokes gRPC methods dynamically (no generated code),
// using descriptors from server reflection or .proto files.
package grpcrunner

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"apiclient/internal/model"
	codepb "google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	EventHeaders = "headers"
	EventMessage = "message"
	EventSent    = "sent"
	EventEnd     = "end"
)

// Event is one item of a call's timeline. Seq orders events within a call,
// since UI event delivery may reorder them.
type Event struct {
	Seq        int64      `json:"seq"`
	Type       string     `json:"type"`
	Data       string     `json:"data,omitempty"`
	Code       string     `json:"code,omitempty"`
	Metadata   []model.KV `json:"metadata,omitempty"`
	DurationMs float64    `json:"durationMs,omitempty"`
	Time       time.Time  `json:"time"`
}

// target turns the request URL into a dial address and whether to use
// plaintext. Schemes: grpc:// and http:// are plaintext; grpcs:// and
// https:// are TLS; no scheme follows the request's Plaintext flag.
func target(raw string, plaintext bool) (string, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || !strings.Contains(raw, "://") {
		return raw, plaintext
	}
	host := u.Host
	switch u.Scheme {
	case "grpc", "http":
		if u.Port() == "" && u.Scheme == "http" {
			host += ":80"
		}
		return host, true
	case "grpcs", "https":
		if u.Port() == "" && u.Scheme == "https" {
			host += ":443"
		}
		return host, false
	}
	return raw, plaintext
}

// Dial opens a client connection for the (resolved) request.
func Dial(r *model.Request, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	if strings.Contains(r.URL, "{{") {
		return nil, fmt.Errorf("address has unresolved variables: %s", r.URL)
	}
	plain := r.GRPC != nil && r.GRPC.Plaintext
	addr, plain := target(r.URL, plain)
	if addr == "" {
		return nil, errors.New("address is empty")
	}
	var creds credentials.TransportCredentials
	if plain {
		creds = insecure.NewCredentials()
	} else {
		skip := r.Settings != nil && r.Settings.InsecureSkipVerify
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: skip}) //nolint:gosec // user opt-in
	}
	opts := append([]grpc.DialOption{grpc.WithTransportCredentials(creds)}, extra...)
	return grpc.NewClient(addr, opts...)
}

// Metadata builds outgoing metadata from enabled headers and auth.
func Metadata(r *model.Request) metadata.MD {
	md := metadata.MD{}
	if a := r.Auth; a != nil {
		switch a.Type {
		case model.AuthBearer:
			md.Set("authorization", "Bearer "+a.Token)
		case model.AuthBasic:
			md.Set("authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(a.Username+":"+a.Password)))
		case model.AuthAPIKey:
			if a.Key != "" {
				md.Set(strings.ToLower(a.Key), a.Value)
			}
		}
	}
	for _, h := range r.Headers {
		if !h.Disabled && h.Key != "" {
			md.Append(strings.ToLower(h.Key), h.Value)
		}
	}
	return md
}

// Stream is a running call of any shape (unary, client, server or bidi).
type Stream struct {
	cs      grpc.ClientStream
	method  protoreflect.MethodDescriptor
	cancel  context.CancelFunc
	onEvent func(Event)
	types   *dynamicpb.Types
	sendMu  sync.Mutex
	done    chan struct{}
	start   time.Time
	seq     atomic.Int64
}

func (s *Stream) emit(e Event) {
	e.Seq = s.seq.Add(1)
	s.onEvent(e)
}

// Run resolves the request's method from src, opens the call, sends the
// request body as the first message, and closes the send side unless the
// method is client-streaming. Responses arrive via onEvent.
func Run(ctx context.Context, conn grpc.ClientConnInterface, src Source, r *model.Request, onEvent func(Event)) (*Stream, error) {
	if r.GRPC == nil || r.GRPC.Service == "" || r.GRPC.Method == "" {
		return nil, errors.New("choose a service and method")
	}
	sd, err := src.FindService(ctx, r.GRPC.Service)
	if err != nil {
		return nil, err
	}
	md := sd.Methods().ByName(protoreflect.Name(r.GRPC.Method))
	if md == nil {
		return nil, fmt.Errorf("method %s not found on %s", r.GRPC.Method, r.GRPC.Service)
	}
	body := "{}"
	if r.Body != nil && strings.TrimSpace(r.Body.Content) != "" {
		body = r.Body.Content
	}
	types := typesFor(md)
	first, err := decode(md.Input(), types, body)
	if err != nil {
		return nil, err
	}
	s, err := Start(ctx, conn, md, Metadata(r), onEvent)
	if err != nil {
		return nil, err
	}
	if err := s.sendMsg(first, body); err != nil {
		s.Cancel()
		return nil, err
	}
	if !md.IsStreamingClient() {
		s.CloseSend()
	}
	return s, nil
}

// Start opens a call without sending anything.
func Start(ctx context.Context, conn grpc.ClientConnInterface, md protoreflect.MethodDescriptor, mdata metadata.MD, onEvent func(Event)) (*Stream, error) {
	ctx, cancel := context.WithCancel(metadata.NewOutgoingContext(ctx, mdata))
	full := fmt.Sprintf("/%s/%s", md.Parent().FullName(), md.Name())
	cs, err := conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    string(md.Name()),
		ClientStreams: md.IsStreamingClient(),
		ServerStreams: md.IsStreamingServer(),
	}, full)
	if err != nil {
		cancel()
		return nil, err
	}
	s := &Stream{cs: cs, method: md, cancel: cancel, onEvent: onEvent, types: typesFor(md), done: make(chan struct{}), start: time.Now()}
	go s.recvLoop()
	return s, nil
}

func (s *Stream) recvLoop() {
	defer close(s.done)
	defer s.cancel()
	if h, err := s.cs.Header(); err == nil && len(h) > 0 {
		s.emit(Event{Type: EventHeaders, Metadata: mdToKV(h), Time: time.Now()})
	}
	for {
		m := dynamicpb.NewMessage(s.method.Output())
		err := s.cs.RecvMsg(m)
		if err != nil {
			end := Event{Type: EventEnd, Code: "OK", Time: time.Now(), Metadata: mdToKV(s.cs.Trailer()),
				DurationMs: float64(time.Since(s.start).Microseconds()) / 1000}
			if !errors.Is(err, io.EOF) {
				st := status.Convert(err)
				end.Code = codepb.Code(st.Code()).String()
				end.Data = st.Message()
			}
			s.emit(end)
			return
		}
		s.emit(Event{Type: EventMessage, Data: encode(m, s.types), Time: time.Now()})
	}
}

// Send parses a JSON message and sends it on the stream.
func (s *Stream) Send(body string) error {
	m, err := decode(s.method.Input(), s.types, body)
	if err != nil {
		return err
	}
	return s.sendMsg(m, body)
}

func (s *Stream) sendMsg(m *dynamicpb.Message, raw string) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if err := s.cs.SendMsg(m); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("stream is closed")
		}
		return err
	}
	s.emit(Event{Type: EventSent, Data: raw, Time: time.Now()})
	return nil
}

func (s *Stream) CloseSend() error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.cs.CloseSend()
}

func (s *Stream) Cancel()               { s.cancel() }
func (s *Stream) Done() <-chan struct{} { return s.done }

func decode(md protoreflect.MessageDescriptor, types *dynamicpb.Types, body string) (*dynamicpb.Message, error) {
	m := dynamicpb.NewMessage(md)
	if err := (protojson.UnmarshalOptions{Resolver: types}).Unmarshal([]byte(body), m); err != nil {
		return nil, fmt.Errorf("invalid %s JSON: %w", md.FullName(), err)
	}
	return m, nil
}

// encode renders a message as stable, indented JSON (protojson randomizes
// whitespace, so normalize it).
func encode(m *dynamicpb.Message, types *dynamicpb.Types) string {
	b, err := protojson.MarshalOptions{Resolver: types, EmitUnpopulated: true}.Marshal(m)
	if err != nil {
		return fmt.Sprintf("<unable to encode response: %v>", err)
	}
	var compact, out bytes.Buffer
	if json.Compact(&compact, b) != nil || json.Indent(&out, compact.Bytes(), "", "  ") != nil {
		return string(b)
	}
	return out.String()
}

// typesFor builds a type resolver from the method's file and its imports so
// google.protobuf.Any payloads can be (un)marshalled.
func typesFor(md protoreflect.MethodDescriptor) *dynamicpb.Types {
	files := new(protoregistry.Files)
	var add func(fd protoreflect.FileDescriptor)
	add = func(fd protoreflect.FileDescriptor) {
		if _, err := files.FindFileByPath(fd.Path()); err == nil {
			return
		}
		imports := fd.Imports()
		for i := 0; i < imports.Len(); i++ {
			add(imports.Get(i).FileDescriptor)
		}
		// Ignore conflicts; a partial registry still resolves most types.
		_ = files.RegisterFile(fd)
	}
	add(md.ParentFile())
	return dynamicpb.NewTypes(files)
}

func mdToKV(md metadata.MD) []model.KV {
	keys := make([]string, 0, len(md))
	for k := range md {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []model.KV
	for _, k := range keys {
		for _, v := range md[k] {
			out = append(out, model.KV{Key: k, Value: v})
		}
	}
	return out
}
