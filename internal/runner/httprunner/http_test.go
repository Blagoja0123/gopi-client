package httprunner

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"apiclient/internal/model"
)

type captured struct {
	method, path, rawQuery, body, contentType string
	header                                    http.Header
	form                                      map[string][]string
	files                                     map[string]string
}

func recorder(t *testing.T) (*httptest.Server, *captured) {
	t.Helper()
	c := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.method, c.path, c.rawQuery, c.header = r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone()
		c.contentType = r.Header.Get("Content-Type")
		if strings.HasPrefix(c.contentType, "multipart/") {
			r.ParseMultipartForm(1 << 20)
			c.form = r.MultipartForm.Value
			c.files = map[string]string{}
			for k, fhs := range r.MultipartForm.File {
				f, _ := fhs[0].Open()
				b, _ := io.ReadAll(f)
				c.files[k] = fhs[0].Filename + ":" + string(b)
			}
		} else {
			b, _ := io.ReadAll(r.Body)
			c.body = string(b)
		}
		w.Header().Set("X-Reply", "yes")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func TestMethodQueryHeadersJSON(t *testing.T) {
	srv, c := recorder(t)
	resp, err := Execute(context.Background(), &model.Request{
		Kind:   model.KindHTTP,
		Method: "put",
		URL:    srv.URL + "/items/1?existing=1",
		Params: []model.KV{{Key: "a", Value: "x y"}, {Key: "skip", Value: "1", Disabled: true}},
		Headers: []model.KV{
			{Key: "X-Custom", Value: "v"},
			{Key: "X-Off", Value: "v", Disabled: true},
		},
		Body: &model.Body{Type: model.BodyJSON, Content: `{"n":1}`},
	}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if c.method != "PUT" || c.path != "/items/1" {
		t.Errorf("method/path = %s %s", c.method, c.path)
	}
	if c.rawQuery != "existing=1&a=x+y" {
		t.Errorf("query = %q", c.rawQuery)
	}
	if c.header.Get("X-Custom") != "v" || c.header.Get("X-Off") != "" {
		t.Errorf("headers = %v", c.header)
	}
	if c.contentType != "application/json" || c.body != `{"n":1}` {
		t.Errorf("ct=%q body=%q", c.contentType, c.body)
	}
	if c.header.Get("User-Agent") == "" {
		t.Error("missing default User-Agent")
	}

	if resp.Status != 201 || resp.StatusText != "201 Created" {
		t.Errorf("status = %d %q", resp.Status, resp.StatusText)
	}
	if resp.Body != `{"ok":true}` || resp.BodyBase64 || resp.Size != 11 {
		t.Errorf("body=%q b64=%v size=%d", resp.Body, resp.BodyBase64, resp.Size)
	}
	if resp.ContentType != "application/json" {
		t.Errorf("content type = %q", resp.ContentType)
	}
	found := false
	for _, h := range resp.Headers {
		if h.Key == "X-Reply" && h.Value == "yes" {
			found = true
		}
	}
	if !found {
		t.Errorf("response headers = %v", resp.Headers)
	}
	if resp.DurationMs <= 0 {
		t.Errorf("duration = %v", resp.DurationMs)
	}
}

func TestExplicitContentTypeWins(t *testing.T) {
	srv, c := recorder(t)
	Execute(context.Background(), &model.Request{Method: "POST", URL: srv.URL,
		Headers: []model.KV{{Key: "content-type", Value: "application/vnd.api+json"}},
		Body:    &model.Body{Type: model.BodyJSON, Content: "{}"}}, Options{})
	if c.contentType != "application/vnd.api+json" {
		t.Errorf("ct = %q", c.contentType)
	}
}

func TestFormBody(t *testing.T) {
	srv, c := recorder(t)
	_, err := Execute(context.Background(), &model.Request{Method: "POST", URL: srv.URL,
		Body: &model.Body{Type: model.BodyForm, Form: []model.KV{{Key: "a", Value: "1"}, {Key: "b", Value: "x&y"}, {Key: "c", Value: "off", Disabled: true}}}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if c.contentType != "application/x-www-form-urlencoded" || c.body != "a=1&b=x%26y" {
		t.Errorf("ct=%q body=%q", c.contentType, c.body)
	}
}

func TestMultipartBodyWithFile(t *testing.T) {
	srv, c := recorder(t)
	fp := filepath.Join(t.TempDir(), "hello.txt")
	os.WriteFile(fp, []byte("file-content"), 0o644)
	_, err := Execute(context.Background(), &model.Request{Method: "POST", URL: srv.URL,
		Body: &model.Body{Type: model.BodyMultipart, Form: []model.KV{{Key: "field", Value: "v"}, {Key: "upload", Value: "@" + fp}}}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.contentType, "multipart/form-data; boundary=") {
		t.Fatalf("ct = %q", c.contentType)
	}
	if c.form["field"][0] != "v" || c.files["upload"] != "hello.txt:file-content" {
		t.Errorf("form=%v files=%v", c.form, c.files)
	}
}

func TestAuth(t *testing.T) {
	cases := []struct {
		name  string
		auth  *model.Auth
		check func(c *captured) bool
	}{
		{"bearer", &model.Auth{Type: model.AuthBearer, Token: "tok"}, func(c *captured) bool { return c.header.Get("Authorization") == "Bearer tok" }},
		{"basic", &model.Auth{Type: model.AuthBasic, Username: "u", Password: "p"}, func(c *captured) bool {
			return c.header.Get("Authorization") == "Basic "+base64.StdEncoding.EncodeToString([]byte("u:p"))
		}},
		{"apikey header", &model.Auth{Type: model.AuthAPIKey, Key: "X-Api-Key", Value: "k"}, func(c *captured) bool { return c.header.Get("X-Api-Key") == "k" }},
		{"apikey query", &model.Auth{Type: model.AuthAPIKey, Key: "api_key", Value: "k", In: "query"}, func(c *captured) bool { return c.rawQuery == "api_key=k" }},
		{"none", &model.Auth{Type: model.AuthNone}, func(c *captured) bool { return c.header.Get("Authorization") == "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, c := recorder(t)
			if _, err := Execute(context.Background(), &model.Request{Method: "GET", URL: srv.URL, Auth: tc.auth}, Options{}); err != nil {
				t.Fatal(err)
			}
			if !tc.check(c) {
				t.Errorf("headers=%v query=%q", c.header, c.rawQuery)
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	start := time.Now()
	_, err := Execute(context.Background(), &model.Request{Method: "GET", URL: srv.URL, Settings: &model.Settings{TimeoutMs: 50}}, Options{})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Error("timeout not honoured")
	}
}

func TestCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	if _, err := Execute(ctx, &model.Request{Method: "GET", URL: srv.URL}, Options{}); err == nil || !strings.Contains(err.Error(), "cancel") {
		t.Fatalf("err = %v", err)
	}
}

func TestRedirects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/end", http.StatusFound) })
	mux.HandleFunc("/end", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "done") })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := Execute(context.Background(), &model.Request{Method: "GET", URL: srv.URL + "/start"}, Options{})
	if err != nil || resp.Status != 200 || resp.Body != "done" || !strings.HasSuffix(resp.URL, "/end") {
		t.Fatalf("follow: resp=%+v err=%v", resp, err)
	}
	no := false
	resp, err = Execute(context.Background(), &model.Request{Method: "GET", URL: srv.URL + "/start", Settings: &model.Settings{FollowRedirects: &no}}, Options{})
	if err != nil || resp.Status != 302 {
		t.Fatalf("no-follow: resp=%+v err=%v", resp, err)
	}
}

func TestBinaryResponseIsBase64(t *testing.T) {
	data := []byte{0xff, 0x00, 0xfe, 0x01}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(data)
	}))
	defer srv.Close()
	resp, err := Execute(context.Background(), &model.Request{Method: "GET", URL: srv.URL}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.BodyBase64 || resp.Body != base64.StdEncoding.EncodeToString(data) {
		t.Errorf("resp = %+v", resp)
	}
}

func TestBodyLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.Repeat("a", 100))
	}))
	defer srv.Close()
	resp, err := Execute(context.Background(), &model.Request{Method: "GET", URL: srv.URL}, Options{MaxBodyBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != 10 || !resp.Truncated || resp.Size != 100 {
		t.Errorf("len=%d truncated=%v size=%d", len(resp.Body), resp.Truncated, resp.Size)
	}
}

func TestInvalidURL(t *testing.T) {
	if _, err := Execute(context.Background(), &model.Request{Method: "GET", URL: "{{baseUrl}}/x"}, Options{}); err == nil {
		t.Fatal("expected error for unresolved URL")
	}
}
