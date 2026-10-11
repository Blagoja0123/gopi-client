package vars

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"apiclient/internal/model"
)

func TestExpandSimple(t *testing.T) {
	r := New(map[string]string{"host": "example.com", "id": "42"})
	got, missing, err := r.Expand("https://{{host}}/users/{{ id }}")
	if err != nil || len(missing) != 0 {
		t.Fatalf("err=%v missing=%v", err, missing)
	}
	if got != "https://example.com/users/42" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandNested(t *testing.T) {
	r := New(map[string]string{"base": "http://{{host}}:{{port}}", "host": "localhost", "port": "8080", "path": "v1"})
	got, _, err := r.Expand("{{base}}/{{path}}")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://localhost:8080/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandCycle(t *testing.T) {
	r := New(map[string]string{"a": "{{b}}", "b": "{{a}}"})
	_, _, err := r.Expand("x {{a}}")
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
	// Self-reference too.
	if _, _, err := New(map[string]string{"a": "{{a}}"}).Expand("{{a}}"); err == nil {
		t.Fatal("expected self-cycle error")
	}
}

func TestUnknownLeftIntactAndReported(t *testing.T) {
	r := New(map[string]string{"a": "1"})
	got, missing, err := r.Expand("{{a}}-{{nope}}-{{nope}}-{{other}}")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1-{{nope}}-{{nope}}-{{other}}" {
		t.Fatalf("got %q", got)
	}
	if strings.Join(missing, ",") != "nope,other" {
		t.Fatalf("missing = %v (want deduped, ordered)", missing)
	}
}

func TestPrecedence(t *testing.T) {
	overrides := map[string]string{"a": "override"}
	env := map[string]string{"a": "env", "b": "env"}
	globals := map[string]string{"a": "global", "b": "global", "c": "global"}
	r := New(overrides, env, globals)
	got, _, _ := r.Expand("{{a}} {{b}} {{c}}")
	if got != "override env global" {
		t.Fatalf("got %q", got)
	}
	// Empty string is a real value, not "missing".
	got, missing, _ := New(map[string]string{"e": ""}, map[string]string{"e": "fallback"}).Expand("[{{e}}]")
	if got != "[]" || len(missing) != 0 {
		t.Fatalf("got %q missing %v", got, missing)
	}
}

func TestDynamicVars(t *testing.T) {
	r := New()
	r.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	got, missing, err := r.Expand("{{$uuid}}|{{$timestamp}}|{{$isoTimestamp}}|{{$randomInt}}")
	if err != nil || len(missing) != 0 {
		t.Fatalf("err=%v missing=%v", err, missing)
	}
	parts := strings.Split(got, "|")
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(parts[0]) {
		t.Errorf("uuid = %q", parts[0])
	}
	if parts[1] != "1767323045" {
		t.Errorf("timestamp = %q", parts[1])
	}
	if parts[2] != "2026-01-02T03:04:05Z" {
		t.Errorf("iso = %q", parts[2])
	}
	if n, err := strconv.Atoi(parts[3]); err != nil || n < 0 || n > 1000 {
		t.Errorf("randomInt = %q", parts[3])
	}
	// A user variable can shadow nothing in the $ namespace.
	got, _, _ = New(map[string]string{"$timestamp": "x"}).Expand("{{$timestamp}}")
	if got == "x" {
		t.Error("$ vars must not be overridable")
	}
}

func TestMask(t *testing.T) {
	got := Mask("Bearer abc123 and abc123 again; short ab", []string{"abc123", "", "ab"})
	if strings.Contains(got, "abc123") {
		t.Fatalf("secret visible: %q", got)
	}
	if !strings.Contains(got, "Bearer "+MaskString) {
		t.Fatalf("got %q", got)
	}
}

func TestApplyRequest(t *testing.T) {
	req := &model.Request{
		Kind:    model.KindHTTP,
		Method:  "POST",
		URL:     "{{base}}/items",
		Params:  []model.KV{{Key: "q", Value: "{{q}}"}},
		Headers: []model.KV{{Key: "X-{{h}}", Value: "{{hv}}"}},
		Auth:    &model.Auth{Type: model.AuthBearer, Token: "{{token}}"},
		Body:    &model.Body{Type: model.BodyJSON, Content: `{"name":"{{name}}"}`, Form: []model.KV{{Key: "f", Value: "{{name}}"}}},
	}
	r := New(map[string]string{"base": "http://x", "q": "search", "h": "Trace", "hv": "1", "token": "T", "name": "Ada"})
	out, missing, err := r.ApplyRequest(req)
	if err != nil || len(missing) != 0 {
		t.Fatalf("err=%v missing=%v", err, missing)
	}
	if out.URL != "http://x/items" || out.Params[0].Value != "search" || out.Headers[0].Key != "X-Trace" ||
		out.Headers[0].Value != "1" || out.Auth.Token != "T" || out.Body.Content != `{"name":"Ada"}` || out.Body.Form[0].Value != "Ada" {
		t.Fatalf("out = %+v auth=%+v body=%+v", out, out.Auth, out.Body)
	}
	if req.URL != "{{base}}/items" {
		t.Error("ApplyRequest mutated input")
	}
	_, missing, _ = New().ApplyRequest(req)
	if len(missing) != 6 { // base q h hv token name (deduped)
		t.Errorf("missing = %v", missing)
	}
}
