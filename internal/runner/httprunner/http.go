// Package httprunner executes a (variable-resolved) model.Request over HTTP.
package httprunner

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"apiclient/internal/model"
)

const (
	DefaultTimeout      = 60 * time.Second
	DefaultMaxBodyBytes = 50 << 20
	UserAgent           = "apiclient/0.1"
)

type Options struct {
	Jar          http.CookieJar
	MaxBodyBytes int64
}

type Response struct {
	Status         int        `json:"status"`
	StatusText     string     `json:"statusText"`
	Proto          string     `json:"proto"`
	Headers        []model.KV `json:"headers"`
	ContentType    string     `json:"contentType"`
	Body           string     `json:"body"`
	BodyBase64     bool       `json:"bodyBase64"`
	Size           int64      `json:"size"`
	Truncated      bool       `json:"truncated"`
	DurationMs     float64    `json:"durationMs"`
	URL            string     `json:"url"`
	Method         string     `json:"method"`
	RequestHeaders []model.KV `json:"requestHeaders"`
}

var (
	secureTransport   = newTransport(false)
	insecureTransport = newTransport(true)
)

func newTransport(insecure bool) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec // user opt-in
	return t
}

// Build turns a resolved request into an *http.Request.
func Build(ctx context.Context, r *model.Request) (*http.Request, error) {
	raw := strings.TrimSpace(r.URL)
	if strings.Contains(raw, "{{") {
		return nil, fmt.Errorf("URL has unresolved variables: %s", raw)
	}
	if raw == "" {
		return nil, errors.New("URL is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid URL %q: missing host", raw)
	}

	extra := url.Values{}
	var extraKeys []string
	add := func(k, v string) {
		if _, ok := extra[k]; !ok {
			extraKeys = append(extraKeys, k)
		}
		extra.Add(k, v)
	}
	for _, p := range r.Params {
		if !p.Disabled && p.Key != "" {
			add(p.Key, p.Value)
		}
	}
	if a := r.Auth; a != nil && a.Type == model.AuthAPIKey && a.In == "query" && a.Key != "" {
		add(a.Key, a.Value)
	}
	if len(extraKeys) > 0 {
		// Keep insertion order (url.Values.Encode sorts keys).
		var parts []string
		for _, k := range extraKeys {
			for _, v := range extra[k] {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
		q := strings.Join(parts, "&")
		if u.RawQuery != "" {
			u.RawQuery += "&" + q
		} else {
			u.RawQuery = q
		}
	}

	body, contentType, err := buildBody(r.Body)
	if err != nil {
		return nil, err
	}
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if a := r.Auth; a != nil {
		switch a.Type {
		case model.AuthBearer:
			req.Header.Set("Authorization", "Bearer "+a.Token)
		case model.AuthBasic:
			req.SetBasicAuth(a.Username, a.Password)
		case model.AuthAPIKey:
			if a.In != "query" && a.Key != "" {
				req.Header.Set(a.Key, a.Value)
			}
		}
	}
	for _, h := range r.Headers {
		if h.Disabled || h.Key == "" {
			continue
		}
		if strings.EqualFold(h.Key, "Host") {
			req.Host = h.Value
			continue
		}
		req.Header.Set(h.Key, h.Value)
	}
	return req, nil
}

func buildBody(b *model.Body) (io.Reader, string, error) {
	if b == nil {
		return nil, "", nil
	}
	switch b.Type {
	case "", model.BodyNone:
		return nil, "", nil
	case model.BodyJSON:
		return strings.NewReader(b.Content), "application/json", nil
	case model.BodyXML:
		return strings.NewReader(b.Content), "application/xml", nil
	case model.BodyText:
		return strings.NewReader(b.Content), "text/plain; charset=utf-8", nil
	case model.BodyForm:
		var parts []string
		for _, kv := range b.Form {
			if !kv.Disabled && kv.Key != "" {
				parts = append(parts, url.QueryEscape(kv.Key)+"="+url.QueryEscape(kv.Value))
			}
		}
		return strings.NewReader(strings.Join(parts, "&")), "application/x-www-form-urlencoded", nil
	case model.BodyMultipart:
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		for _, kv := range b.Form {
			if kv.Disabled || kv.Key == "" {
				continue
			}
			// A value of "@/path/to/file" uploads that file (curl convention).
			if strings.HasPrefix(kv.Value, "@") {
				p := kv.Value[1:]
				f, err := os.Open(p)
				if err != nil {
					return nil, "", fmt.Errorf("form file %q: %w", kv.Key, err)
				}
				w, err := mw.CreateFormFile(kv.Key, filepath.Base(p))
				if err == nil {
					_, err = io.Copy(w, f)
				}
				f.Close()
				if err != nil {
					return nil, "", err
				}
				continue
			}
			if err := mw.WriteField(kv.Key, kv.Value); err != nil {
				return nil, "", err
			}
		}
		if err := mw.Close(); err != nil {
			return nil, "", err
		}
		return &buf, mw.FormDataContentType(), nil
	default:
		return nil, "", fmt.Errorf("unknown body type %q", b.Type)
	}
}

// Execute sends the request and captures the full response.
func Execute(ctx context.Context, r *model.Request, opts Options) (*Response, error) {
	timeout := DefaultTimeout
	follow := true
	transport := secureTransport
	if s := r.Settings; s != nil {
		if s.TimeoutMs > 0 {
			timeout = time.Duration(s.TimeoutMs) * time.Millisecond
		}
		if s.FollowRedirects != nil {
			follow = *s.FollowRedirects
		}
		if s.InsecureSkipVerify {
			transport = insecureTransport
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := Build(ctx, r)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Transport: transport, Jar: opts.Jar}
	if !follow {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, wrapErr(ctx, err, timeout)
	}
	defer resp.Body.Close()

	limit := opts.MaxBodyBytes
	if limit <= 0 {
		limit = DefaultMaxBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, wrapErr(ctx, err, timeout)
	}
	size := int64(len(body))
	rest, _ := io.Copy(io.Discard, resp.Body)
	size += rest
	dur := time.Since(start)

	out := &Response{
		Status:         resp.StatusCode,
		StatusText:     resp.Status,
		Proto:          resp.Proto,
		Headers:        flatten(resp.Header),
		ContentType:    resp.Header.Get("Content-Type"),
		Size:           size,
		Truncated:      rest > 0,
		DurationMs:     float64(dur.Microseconds()) / 1000,
		URL:            resp.Request.URL.String(),
		Method:         req.Method,
		RequestHeaders: flatten(req.Header),
	}
	if utf8.Valid(body) {
		out.Body = string(body)
	} else {
		out.Body = base64.StdEncoding.EncodeToString(body)
		out.BodyBase64 = true
	}
	return out, nil
}

func wrapErr(ctx context.Context, err error, timeout time.Duration) error {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Errorf("request timed out after %s", timeout)
	case errors.Is(ctx.Err(), context.Canceled):
		return errors.New("request cancelled")
	}
	return err
}

func flatten(h http.Header) []model.KV {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []model.KV
	for _, k := range keys {
		for _, v := range h[k] {
			out = append(out, model.KV{Key: k, Value: v})
		}
	}
	return out
}
