// Package openapi converts OpenAPI 3.x / Swagger 2.0 documents into
// workspace requests.
package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"apiclient/internal/model"
	"apiclient/internal/store"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

const baseVar = "baseUrl"

type Imported struct {
	Folder  string         `yaml:"folder" json:"folder"` // tag name, "" for untagged
	Request *model.Request `yaml:"request" json:"request"`
}

type Result struct {
	Title    string      `yaml:"title" json:"title"`
	BaseURL  string      `yaml:"baseUrl" json:"baseUrl"`
	Auth     *model.Auth `yaml:"auth,omitempty" json:"auth,omitempty"` // document-level default security
	Requests []Imported  `yaml:"requests" json:"requests"`
}

var methodOrder = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE"}

// Parse accepts YAML or JSON, OpenAPI 3.x or Swagger 2.0.
func Parse(data []byte) (*Result, error) {
	var probe map[string]any
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("not valid YAML/JSON: %w", err)
	}
	var doc *openapi3.T
	switch {
	case probe["openapi"] != nil:
		loader := openapi3.NewLoader()
		d, err := loader.LoadFromData(data)
		if err != nil {
			return nil, fmt.Errorf("invalid OpenAPI document: %w", err)
		}
		doc = d
	case probe["swagger"] != nil:
		js, err := json.Marshal(probe)
		if err != nil {
			return nil, err
		}
		var d2 openapi2.T
		if err := json.Unmarshal(js, &d2); err != nil {
			return nil, fmt.Errorf("invalid Swagger document: %w", err)
		}
		d, err := openapi2conv.ToV3(&d2)
		if err != nil {
			return nil, fmt.Errorf("convert Swagger 2.0: %w", err)
		}
		if err := openapi3.NewLoader().ResolveRefsIn(d, nil); err != nil {
			return nil, err
		}
		doc = d
	default:
		return nil, errors.New("not an OpenAPI or Swagger document (missing 'openapi'/'swagger' field)")
	}
	return convert(doc), nil
}

func convert(doc *openapi3.T) *Result {
	res := &Result{Title: "API"}
	if doc.Info != nil && doc.Info.Title != "" {
		res.Title = doc.Info.Title
	}
	if len(doc.Servers) > 0 && doc.Servers[0] != nil {
		s := doc.Servers[0]
		u := s.URL
		for name, v := range s.Variables {
			if v != nil {
				u = strings.ReplaceAll(u, "{"+name+"}", v.Default)
			}
		}
		res.BaseURL = strings.TrimRight(u, "/")
	}
	schemes := map[string]*openapi3.SecurityScheme{}
	if doc.Components != nil {
		for name, ref := range doc.Components.SecuritySchemes {
			if ref != nil && ref.Value != nil {
				schemes[name] = ref.Value
			}
		}
	}
	res.Auth = authFor(doc.Security, schemes)

	if doc.Paths == nil {
		return res
	}
	paths := doc.Paths.Map()
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, p := range keys {
		item := paths[p]
		ops := item.Operations()
		for _, m := range methodOrder {
			op := ops[m]
			if op == nil {
				continue
			}
			res.Requests = append(res.Requests, convertOp(p, m, item, op, schemes))
		}
	}
	return res
}

var pathParam = regexp.MustCompile(`\{([^}]+)\}`)

func convertOp(p, method string, item *openapi3.PathItem, op *openapi3.Operation, schemes map[string]*openapi3.SecurityScheme) Imported {
	name := op.Summary
	if name == "" {
		name = op.OperationID
	}
	if name == "" {
		name = method + " " + p
	}
	req := &model.Request{
		Name:   name,
		Kind:   model.KindHTTP,
		Method: method,
		URL:    "{{" + baseVar + "}}" + pathParam.ReplaceAllString(p, "{{$1}}"),
		Docs:   strings.TrimSpace(op.Description),
	}
	// Path-level params apply unless the operation overrides them.
	params := map[string]*openapi3.Parameter{}
	var order []string
	for _, list := range []openapi3.Parameters{item.Parameters, op.Parameters} {
		for _, ref := range list {
			if ref == nil || ref.Value == nil {
				continue
			}
			key := ref.Value.In + ":" + ref.Value.Name
			if _, ok := params[key]; !ok {
				order = append(order, key)
			}
			params[key] = ref.Value
		}
	}
	for _, key := range order {
		prm := params[key]
		val := ""
		if prm.Example != nil {
			val = scalar(prm.Example)
		} else if prm.Schema != nil && prm.Schema.Value != nil {
			val = scalarFromSchema(prm.Schema.Value)
		}
		kv := model.KV{Key: prm.Name, Value: val, Disabled: !prm.Required}
		switch prm.In {
		case "query":
			req.Params = append(req.Params, kv)
		case "header":
			req.Headers = append(req.Headers, kv)
		}
	}
	if op.Security != nil {
		req.Auth = authFor(*op.Security, schemes)
		if req.Auth == nil {
			req.Auth = &model.Auth{Type: model.AuthNone}
		}
	}
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		req.Body = bodyFor(op.RequestBody.Value.Content)
	}
	folder := ""
	if len(op.Tags) > 0 {
		folder = op.Tags[0]
	}
	return Imported{Folder: folder, Request: req}
}

func authFor(reqs openapi3.SecurityRequirements, schemes map[string]*openapi3.SecurityScheme) *model.Auth {
	for _, r := range reqs {
		names := make([]string, 0, len(r))
		for n := range r {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			s := schemes[n]
			if s == nil {
				continue
			}
			switch {
			case s.Type == "http" && strings.EqualFold(s.Scheme, "bearer"), s.Type == "oauth2", s.Type == "openIdConnect":
				return &model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
			case s.Type == "http" && strings.EqualFold(s.Scheme, "basic"):
				return &model.Auth{Type: model.AuthBasic, Username: "{{username}}", Password: "{{password}}"}
			case s.Type == "apiKey" && (s.In == "header" || s.In == "query"):
				a := &model.Auth{Type: model.AuthAPIKey, Key: s.Name, Value: "{{apiKey}}"}
				if s.In == "query" {
					a.In = "query"
				}
				return a
			}
		}
	}
	return nil
}

func bodyFor(content openapi3.Content) *model.Body {
	types := make([]string, 0, len(content))
	for ct := range content {
		types = append(types, ct)
	}
	sort.Strings(types)
	pick := func(match func(string) bool) (*openapi3.MediaType, bool) {
		for _, ct := range types {
			if match(ct) {
				return content[ct], true
			}
		}
		return nil, false
	}
	if mt, ok := pick(func(ct string) bool { return strings.Contains(ct, "json") }); ok {
		return &model.Body{Type: model.BodyJSON, Content: exampleJSON(mt)}
	}
	for _, f := range []struct{ ct, typ string }{
		{"application/x-www-form-urlencoded", model.BodyForm},
		{"multipart/form-data", model.BodyMultipart},
	} {
		if mt, ok := pick(func(ct string) bool { return ct == f.ct }); ok {
			b := &model.Body{Type: f.typ}
			if mt.Schema != nil && mt.Schema.Value != nil {
				for _, name := range sortedKeys(mt.Schema.Value.Properties) {
					b.Form = append(b.Form, model.KV{Key: name, Value: scalarFromSchema(mt.Schema.Value.Properties[name].Value)})
				}
			}
			return b
		}
	}
	if _, ok := pick(func(ct string) bool { return strings.Contains(ct, "xml") }); ok {
		return &model.Body{Type: model.BodyXML}
	}
	if _, ok := pick(func(ct string) bool { return strings.HasPrefix(ct, "text/") }); ok {
		return &model.Body{Type: model.BodyText}
	}
	return nil
}

func exampleJSON(mt *openapi3.MediaType) string {
	var v any
	switch {
	case mt.Example != nil:
		v = mt.Example
	case len(mt.Examples) > 0:
		for _, k := range sortedKeys(mt.Examples) {
			if ex := mt.Examples[k]; ex != nil && ex.Value != nil {
				v = ex.Value.Value
				break
			}
		}
	case mt.Schema != nil:
		v = example(mt.Schema.Value, map[*openapi3.Schema]bool{}, 0)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// example builds a sample value; cycles and deep nesting are cut short.
func example(s *openapi3.Schema, seen map[*openapi3.Schema]bool, depth int) any {
	if s == nil {
		return nil
	}
	if s.Example != nil {
		return s.Example
	}
	if s.Default != nil {
		return s.Default
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	if seen[s] || depth > 6 {
		return nil
	}
	seen[s] = true
	defer delete(seen, s)
	if len(s.AllOf) > 0 {
		merged := map[string]any{}
		for _, ref := range s.AllOf {
			if m, ok := example(ref.Value, seen, depth+1).(map[string]any); ok {
				for k, v := range m {
					merged[k] = v
				}
			}
		}
		return merged
	}
	for _, alts := range []openapi3.SchemaRefs{s.OneOf, s.AnyOf} {
		if len(alts) > 0 {
			return example(alts[0].Value, seen, depth+1)
		}
	}
	switch {
	case s.Type.Is("object") || (s.Type.IsEmpty() && len(s.Properties) > 0):
		m := map[string]any{}
		for name, ref := range s.Properties {
			if v := example(ref.Value, seen, depth+1); v != nil {
				m[name] = v
			}
		}
		return m
	case s.Type.Is("array"):
		if s.Items == nil {
			return []any{}
		}
		if v := example(s.Items.Value, seen, depth+1); v != nil {
			return []any{v}
		}
		return []any{}
	case s.Type.Is("integer"), s.Type.Is("number"):
		return 0
	case s.Type.Is("boolean"):
		return false
	case s.Type.Is("string"):
		return stringExample(s.Format)
	}
	return nil
}

func stringExample(format string) string {
	switch format {
	case "date-time":
		return "1970-01-01T00:00:00Z"
	case "date":
		return "1970-01-01"
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "uri", "url":
		return "https://example.com"
	}
	return "string"
}

func scalarFromSchema(s *openapi3.Schema) string {
	if s == nil {
		return ""
	}
	switch {
	case s.Example != nil:
		return scalar(s.Example)
	case s.Default != nil:
		return scalar(s.Default)
	case len(s.Enum) > 0:
		return scalar(s.Enum[0])
	case s.Type.Is("integer"), s.Type.Is("number"):
		return "0"
	case s.Type.Is("boolean"):
		return "false"
	}
	return ""
}

func scalar(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Write saves the import into ws under a new folder named after the API and
// returns that folder. Document-level auth goes into the folder's
// _folder.yaml. The base URL is stored as a workspace variable; if
// "baseUrl" is already taken by a different value, a namespaced variable
// (e.g. petstoreBaseUrl) is used instead. The same applies when an
// environment defines baseUrl, since it would shadow the workspace value.
func Write(ws *store.Workspace, res *Result) (string, error) {
	folder, err := ws.CreateFolder("", res.Title)
	if err != nil {
		return "", err
	}
	varName := baseVar
	if res.BaseURL != "" {
		varName = chooseVar(ws, res)
	}
	if res.Auth != nil {
		if err := ws.SaveFolder(folder, &model.Folder{Auth: res.Auth}); err != nil {
			return "", err
		}
	}
	for _, imp := range res.Requests {
		req := imp.Request.Clone()
		if varName != baseVar {
			req.URL = strings.Replace(req.URL, "{{"+baseVar+"}}", "{{"+varName+"}}", 1)
		}
		if req.Auth == nil {
			req.Auth = &model.Auth{Type: model.AuthInherit}
		}
		dir := folder
		if imp.Folder != "" {
			dir = path.Join(folder, store.Slug(imp.Folder))
		}
		if _, err := ws.AddRequest(dir, req); err != nil {
			return "", err
		}
	}
	return folder, ws.SaveMeta()
}

// envDefines reports whether any environment sets name with a value other
// than want (environments override workspace variables).
func envDefines(ws *store.Workspace, name, want string) bool {
	envs, _ := ws.ListEnvironments()
	for _, e := range envs {
		env, err := ws.LoadEnvironment(e)
		if err != nil {
			continue
		}
		if v, ok := env.Values()[name]; ok && v != want {
			return true
		}
	}
	return false
}

func chooseVar(ws *store.Workspace, res *Result) string {
	candidates := []string{baseVar, camel(res.Title) + "BaseUrl"}
	for _, name := range candidates {
		if envDefines(ws, name, res.BaseURL) {
			continue
		}
		idx := -1
		for i, v := range ws.Meta.Variables {
			if v.Key == name {
				idx = i
			}
		}
		if idx == -1 {
			ws.Meta.Variables = append(ws.Meta.Variables, model.Variable{Key: name, Value: res.BaseURL})
			return name
		}
		if ws.Meta.Variables[idx].Value == res.BaseURL {
			return name
		}
	}
	// Both taken with other values: overwrite the namespaced one.
	name := candidates[1]
	for i, v := range ws.Meta.Variables {
		if v.Key == name {
			ws.Meta.Variables[i].Value = res.BaseURL
		}
	}
	return name
}

func camel(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	var b strings.Builder
	for i, w := range words {
		w = strings.ToLower(w)
		if i > 0 {
			w = strings.ToUpper(w[:1]) + w[1:]
		}
		b.WriteString(w)
	}
	if b.Len() == 0 {
		return "api"
	}
	return b.String()
}
