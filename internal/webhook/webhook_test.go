package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReceiverCapturesRequests(t *testing.T) {
	var mu sync.Mutex
	var notified []Capture
	r := NewReceiver(10, func(c Capture) {
		mu.Lock()
		notified = append(notified, c)
		mu.Unlock()
	})
	addr, err := r.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	if running, a := r.Status(); !running || a != addr {
		t.Fatalf("status = %v %q", running, a)
	}

	req, _ := http.NewRequest("POST", "http://"+addr+"/hooks/github?x=1", strings.NewReader(`{"action":"opened"}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}

	caps := r.Captures()
	if len(caps) != 1 {
		t.Fatalf("captures = %d", len(caps))
	}
	c := caps[0]
	if c.Method != "POST" || c.Path != "/hooks/github" || c.Query != "x=1" || c.Body != `{"action":"opened"}` || c.ID == "" {
		t.Errorf("capture = %+v", c)
	}
	if c.Header("x-github-event") != "pull_request" {
		t.Errorf("headers = %v", c.Headers)
	}
	mu.Lock()
	if len(notified) != 1 || notified[0].ID != c.ID {
		t.Errorf("notified = %v", notified)
	}
	mu.Unlock()
	if got, ok := r.Get(c.ID); !ok || got.Body != c.Body {
		t.Error("Get by id failed")
	}
}

func TestCustomResponse(t *testing.T) {
	r := NewReceiver(10, nil)
	r.SetResponse(Response{Status: 202, ContentType: "application/json", Body: `{"ok":1}`})
	addr, err := r.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 202 || string(b) != `{"ok":1}` || resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("got %d %q %q", resp.StatusCode, b, resp.Header.Get("Content-Type"))
	}
}

func TestRingBufferEvictsOldest(t *testing.T) {
	r := NewReceiver(3, nil)
	addr, _ := r.Start("127.0.0.1:0")
	defer r.Stop()
	for i := 0; i < 5; i++ {
		resp, err := http.Post("http://"+addr+"/", "text/plain", strings.NewReader(string(rune('a'+i))))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	caps := r.Captures()
	var bodies []string
	for _, c := range caps {
		bodies = append(bodies, c.Body)
	}
	if strings.Join(bodies, "") != "cde" {
		t.Errorf("bodies = %v", bodies)
	}
	r.Clear()
	if len(r.Captures()) != 0 {
		t.Error("clear failed")
	}
}

func TestStopAndRestart(t *testing.T) {
	r := NewReceiver(3, nil)
	addr, _ := r.Start("127.0.0.1:0")
	if _, err := r.Start("127.0.0.1:0"); err == nil {
		t.Error("double start should fail")
	}
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	if running, _ := r.Status(); running {
		t.Error("still running")
	}
	c := http.Client{Timeout: 200 * time.Millisecond}
	if _, err := c.Get("http://" + addr); err == nil {
		t.Error("server still accepting after stop")
	}
	if _, err := r.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	r.Stop()
}

func TestReplay(t *testing.T) {
	var gotMethod, gotPath, gotBody, gotEvent, gotCT string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.String()
		gotEvent, gotCT = r.Header.Get("X-GitHub-Event"), r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(204)
	}))
	defer target.Close()

	c := Capture{Method: "POST", Path: "/hooks/github", Query: "x=1", Body: `{"a":1}`,
		Headers: []Header{{"Content-Type", "application/json"}, {"X-GitHub-Event", "push"}, {"Content-Length", "7"}, {"Host", "old"}}}
	resp, err := Replay(context.Background(), c, target.URL+"/receiver")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 204 {
		t.Errorf("status = %d", resp.Status)
	}
	if gotMethod != "POST" || gotPath != "/receiver/hooks/github?x=1" || gotBody != `{"a":1}` || gotEvent != "push" || gotCT != "application/json" {
		t.Errorf("target saw %s %s %q event=%q ct=%q", gotMethod, gotPath, gotBody, gotEvent, gotCT)
	}
}

func TestGitHubSignatureKnownVector(t *testing.T) {
	// From GitHub's "Validating webhook deliveries" documentation.
	secret, payload := "It's a Secret to Everybody", "Hello, World!"
	want := "sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"
	if got := GitHubSignature(secret, []byte(payload)); got != want {
		t.Fatalf("got %s", got)
	}
	if !VerifyGitHub(secret, []byte(payload), want) {
		t.Error("verify failed for valid signature")
	}
	if VerifyGitHub(secret, []byte(payload+"!"), want) || VerifyGitHub("wrong", []byte(payload), want) || VerifyGitHub(secret, []byte(payload), "garbage") {
		t.Error("verify accepted an invalid signature")
	}
}

func TestSignAlgorithms(t *testing.T) {
	cases := []struct{ algo, want string }{
		{"sha256", "757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"},
		{"sha1", "01dc10d0c83e72ed246219cdd91669667fe2ca59"},
	}
	for _, c := range cases {
		got, err := Sign(c.algo, "It's a Secret to Everybody", []byte("Hello, World!"))
		if err != nil || got != c.want {
			t.Errorf("%s: got %s err %v", c.algo, got, err)
		}
	}
	if _, err := Sign("md5", "k", nil); err == nil {
		t.Error("unsupported algo should error")
	}
}

func TestStripeSignature(t *testing.T) {
	secret, payload := "whsec_test", []byte(`{"id":"evt_1"}`)
	ts := time.Unix(1700000000, 0)
	header := StripeSignature(secret, payload, ts)
	if !strings.HasPrefix(header, "t=1700000000,v1=") || len(header) != len("t=1700000000,v1=")+64 {
		t.Fatalf("header = %s", header)
	}
	if err := VerifyStripe(secret, payload, header, 5*time.Minute, ts.Add(time.Minute)); err != nil {
		t.Errorf("valid: %v", err)
	}
	if err := VerifyStripe(secret, payload, header, 5*time.Minute, ts.Add(10*time.Minute)); err == nil {
		t.Error("expired timestamp accepted")
	}
	if err := VerifyStripe(secret, []byte("tampered"), header, 5*time.Minute, ts); err == nil {
		t.Error("tampered payload accepted")
	}
	// Multiple v1 entries (secret rotation): any match passes.
	multi := "t=1700000000,v1=deadbeef," + header[len("t=1700000000,"):]
	if err := VerifyStripe(secret, payload, multi, 5*time.Minute, ts); err != nil {
		t.Errorf("rotation: %v", err)
	}
}
