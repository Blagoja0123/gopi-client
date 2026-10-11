package curl

import (
	"strings"
	"testing"

	"apiclient/internal/model"
)

func TestParseChromeStyle(t *testing.T) {
	cmd := `curl 'https://api.example.com/v1/items?page=2&q=a%20b' \
  -H 'accept: application/json' \
  -H "Authorization: Bearer abc" \
  --data-raw '{"name":"it'\''s"}' \
  --compressed`
	r, err := Parse(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "POST" || r.URL != "https://api.example.com/v1/items" {
		t.Errorf("method/url = %s %s", r.Method, r.URL)
	}
	if len(r.Params) != 2 || r.Params[0] != (model.KV{Key: "page", Value: "2"}) || r.Params[1] != (model.KV{Key: "q", Value: "a b"}) {
		t.Errorf("params = %+v", r.Params)
	}
	if len(r.Headers) != 2 || r.Headers[1].Value != "Bearer abc" {
		t.Errorf("headers = %+v", r.Headers)
	}
	if r.Body == nil || r.Body.Type != model.BodyJSON || r.Body.Content != `{"name":"it's"}` {
		t.Errorf("body = %+v", r.Body)
	}
}

func TestParseFlags(t *testing.T) {
	r, err := Parse(`curl -X PUT -u user:pa:ss -k -L --url http://h/x -d a=1 -d b=2`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "PUT" || r.URL != "http://h/x" {
		t.Errorf("%s %s", r.Method, r.URL)
	}
	if r.Auth == nil || r.Auth.Type != model.AuthBasic || r.Auth.Username != "user" || r.Auth.Password != "pa:ss" {
		t.Errorf("auth = %+v", r.Auth)
	}
	if r.Settings == nil || !r.Settings.InsecureSkipVerify {
		t.Errorf("settings = %+v", r.Settings)
	}
	if r.Body == nil || r.Body.Type != model.BodyForm || len(r.Body.Form) != 2 || r.Body.Form[1] != (model.KV{Key: "b", Value: "2"}) {
		t.Errorf("body = %+v", r.Body)
	}

	r, _ = Parse(`curl http://h -F name=ada -F file=@/tmp/a.txt`)
	if r.Method != "POST" || r.Body.Type != model.BodyMultipart || r.Body.Form[1].Value != "@/tmp/a.txt" {
		t.Errorf("multipart = %+v", r.Body)
	}

	r, _ = Parse(`curl http://h -H 'Content-Type: text/plain' -d 'hello world'`)
	if r.Body.Type != model.BodyText || r.Body.Content != "hello world" {
		t.Errorf("text = %+v", r.Body)
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "wget http://x", "curl -H 'x: y'", `curl 'unterminated`} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestExportRoundTrip(t *testing.T) {
	no := false
	req := &model.Request{
		Kind: model.KindHTTP, Method: "PATCH", URL: "https://h/items/1",
		Params:   []model.KV{{Key: "a", Value: "x y"}, {Key: "off", Value: "1", Disabled: true}},
		Headers:  []model.KV{{Key: "X-Note", Value: "it's"}},
		Auth:     &model.Auth{Type: model.AuthBearer, Token: "tok"},
		Body:     &model.Body{Type: model.BodyJSON, Content: `{"a":"b'c"}`},
		Settings: &model.Settings{FollowRedirects: &no},
	}
	cmd := Export(req)
	if strings.Contains(cmd, "off=1") {
		t.Errorf("disabled param exported: %s", cmd)
	}
	back, err := Parse(cmd)
	if err != nil {
		t.Fatalf("%v\n%s", err, cmd)
	}
	if back.Method != "PATCH" || back.URL != req.URL || back.Body.Content != req.Body.Content || back.Params[0].Value != "x y" {
		t.Errorf("round trip mismatch:\n%s\n%+v", cmd, back)
	}
	h := map[string]string{}
	for _, kv := range back.Headers {
		h[strings.ToLower(kv.Key)] = kv.Value
	}
	if h["authorization"] != "Bearer tok" || h["x-note"] != "it's" || h["content-type"] != "application/json" {
		t.Errorf("headers = %+v", back.Headers)
	}
}
