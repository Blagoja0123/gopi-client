// Package curl converts between cURL command lines and requests.
package curl

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"apiclient/internal/model"
)

// tokenize splits a shell command line, honouring single/double quotes,
// backslash escapes and line continuations.
func tokenize(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inTok := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && (s[i+1] == '\n' || s[i+1] == '\r'):
			i++
			if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if inTok {
				out = append(out, cur.String())
				cur.Reset()
				inTok = false
			}
		case c == '\'':
			inTok = true
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return nil, errors.New("unterminated single quote")
			}
			cur.WriteString(s[i+1 : i+1+j])
			i += j + 1
		case c == '"':
			inTok = true
			i++
			for ; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("\"\\$`\n", s[i+1]) >= 0 {
					i++
				}
				cur.WriteByte(s[i])
			}
			if i >= len(s) {
				return nil, errors.New("unterminated double quote")
			}
		case c == '\\' && i+1 < len(s):
			inTok = true
			i++
			cur.WriteByte(s[i])
		default:
			inTok = true
			cur.WriteByte(c)
		}
	}
	if inTok {
		out = append(out, cur.String())
	}
	return out, nil
}

// Parse converts a cURL command into a request.
func Parse(cmd string) (*model.Request, error) {
	toks, err := tokenize(strings.TrimSpace(cmd))
	if err != nil {
		return nil, err
	}
	if len(toks) == 0 || toks[0] != "curl" {
		return nil, errors.New("command must start with curl")
	}
	r := &model.Request{Kind: model.KindHTTP}
	var data []string
	var form []model.KV
	var rawURL, contentType string
	get := false
	next := func(i *int, flag string) (string, error) {
		*i++
		if *i >= len(toks) {
			return "", fmt.Errorf("%s needs a value", flag)
		}
		return toks[*i], nil
	}
	for i := 1; i < len(toks); i++ {
		t := toks[i]
		flag, val, hasEq := t, "", false
		if strings.HasPrefix(t, "--") {
			flag, val, hasEq = strings.Cut(t, "=")
		}
		arg := func() (string, error) {
			if hasEq {
				return val, nil
			}
			return next(&i, flag)
		}
		var v string
		switch flag {
		case "-X", "--request":
			if v, err = arg(); err != nil {
				return nil, err
			}
			r.Method = strings.ToUpper(v)
		case "-H", "--header":
			if v, err = arg(); err != nil {
				return nil, err
			}
			k, hv, _ := strings.Cut(v, ":")
			k, hv = strings.TrimSpace(k), strings.TrimSpace(hv)
			if strings.EqualFold(k, "content-type") {
				contentType = strings.ToLower(hv)
			}
			r.Headers = append(r.Headers, model.KV{Key: k, Value: hv})
		case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode":
			if v, err = arg(); err != nil {
				return nil, err
			}
			data = append(data, v)
		case "-F", "--form":
			if v, err = arg(); err != nil {
				return nil, err
			}
			k, fv, _ := strings.Cut(v, "=")
			form = append(form, model.KV{Key: k, Value: fv})
		case "-u", "--user":
			if v, err = arg(); err != nil {
				return nil, err
			}
			u, p, _ := strings.Cut(v, ":")
			r.Auth = &model.Auth{Type: model.AuthBasic, Username: u, Password: p}
		case "-A", "--user-agent":
			if v, err = arg(); err != nil {
				return nil, err
			}
			r.Headers = append(r.Headers, model.KV{Key: "User-Agent", Value: v})
		case "-b", "--cookie":
			if v, err = arg(); err != nil {
				return nil, err
			}
			r.Headers = append(r.Headers, model.KV{Key: "Cookie", Value: v})
		case "--url":
			if rawURL, err = arg(); err != nil {
				return nil, err
			}
		case "-k", "--insecure":
			if r.Settings == nil {
				r.Settings = &model.Settings{}
			}
			r.Settings.InsecureSkipVerify = true
		case "-G", "--get":
			get = true
		case "-L", "--location", "--compressed", "-s", "--silent", "-S", "--show-error", "-v", "--verbose", "-i", "--include":
		case "--no-location":
			if r.Settings == nil {
				r.Settings = &model.Settings{}
			}
			f := false
			r.Settings.FollowRedirects = &f
		default:
			if strings.HasPrefix(t, "-") {
				continue // unknown flag: ignore
			}
			if rawURL == "" {
				rawURL = t
			}
		}
	}
	if rawURL == "" {
		return nil, errors.New("no URL found")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.RawQuery != "" {
		for _, pair := range strings.Split(u.RawQuery, "&") {
			k, v, _ := strings.Cut(pair, "=")
			ku, _ := url.QueryUnescape(k)
			vu, _ := url.QueryUnescape(v)
			r.Params = append(r.Params, model.KV{Key: ku, Value: vu})
		}
		u.RawQuery = ""
	}
	r.URL = u.String()

	switch {
	case get && len(data) > 0:
		for _, d := range data {
			k, v, _ := strings.Cut(d, "=")
			r.Params = append(r.Params, model.KV{Key: k, Value: v})
		}
	case len(form) > 0:
		r.Body = &model.Body{Type: model.BodyMultipart, Form: form}
	case len(data) > 0:
		joined := strings.Join(data, "&")
		trim := strings.TrimSpace(joined)
		switch {
		case strings.Contains(contentType, "json") || (contentType == "" && (strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "["))):
			r.Body = &model.Body{Type: model.BodyJSON, Content: joined}
		case contentType == "" || strings.Contains(contentType, "x-www-form-urlencoded"):
			b := &model.Body{Type: model.BodyForm}
			for _, d := range data {
				for _, pair := range strings.Split(d, "&") {
					k, v, _ := strings.Cut(pair, "=")
					ku, _ := url.QueryUnescape(k)
					vu, _ := url.QueryUnescape(v)
					b.Form = append(b.Form, model.KV{Key: ku, Value: vu})
				}
			}
			r.Body = b
		case strings.Contains(contentType, "xml"):
			r.Body = &model.Body{Type: model.BodyXML, Content: joined}
		default:
			r.Body = &model.Body{Type: model.BodyText, Content: joined}
		}
	}
	if r.Method == "" {
		r.Method = "GET"
		if r.Body != nil {
			r.Method = "POST"
		}
	}
	r.Name = r.Method + " " + u.Path
	return r, nil
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Export renders a (resolved) request as a cURL command.
func Export(r *model.Request) string {
	var parts []string
	parts = append(parts, "curl")
	method := strings.ToUpper(r.Method)
	if method == "" {
		method = "GET"
	}
	parts = append(parts, "-X", method)

	u := r.URL
	var q []string
	for _, p := range r.Params {
		if !p.Disabled && p.Key != "" {
			q = append(q, url.QueryEscape(p.Key)+"="+url.QueryEscape(p.Value))
		}
	}
	if a := r.Auth; a != nil && a.Type == model.AuthAPIKey && a.In == "query" {
		q = append(q, url.QueryEscape(a.Key)+"="+url.QueryEscape(a.Value))
	}
	if len(q) > 0 {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + strings.Join(q, "&")
	}
	parts = append(parts, quote(u))

	header := func(k, v string) { parts = append(parts, "-H", quote(k+": "+v)) }
	if a := r.Auth; a != nil {
		switch a.Type {
		case model.AuthBearer:
			header("Authorization", "Bearer "+a.Token)
		case model.AuthBasic:
			parts = append(parts, "-u", quote(a.Username+":"+a.Password))
		case model.AuthAPIKey:
			if a.In != "query" {
				header(a.Key, a.Value)
			}
		}
	}
	hasCT := false
	for _, h := range r.Headers {
		if h.Disabled || h.Key == "" {
			continue
		}
		if strings.EqualFold(h.Key, "content-type") {
			hasCT = true
		}
		header(h.Key, h.Value)
	}
	if b := r.Body; b != nil {
		ct := map[string]string{model.BodyJSON: "application/json", model.BodyXML: "application/xml", model.BodyText: "text/plain"}[b.Type]
		switch b.Type {
		case model.BodyJSON, model.BodyXML, model.BodyText:
			if !hasCT {
				header("Content-Type", ct)
			}
			parts = append(parts, "--data-raw", quote(b.Content))
		case model.BodyForm:
			for _, f := range b.Form {
				if !f.Disabled {
					parts = append(parts, "--data-urlencode", quote(f.Key+"="+f.Value))
				}
			}
		case model.BodyMultipart:
			for _, f := range b.Form {
				if !f.Disabled {
					parts = append(parts, "-F", quote(f.Key+"="+f.Value))
				}
			}
		}
	}
	if s := r.Settings; s != nil {
		if s.InsecureSkipVerify {
			parts = append(parts, "-k")
		}
	}
	if s := r.Settings; s == nil || s.FollowRedirects == nil || *s.FollowRedirects {
		parts = append(parts, "-L")
	}
	return strings.Join(parts, " ")
}
