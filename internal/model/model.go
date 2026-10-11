// Package model defines the on-disk (git-trackable) schema shared by every
// part of the app. Field order in these structs is the key order written to
// YAML, so keep it stable to avoid noisy diffs.
package model

// Kind is the protocol a request speaks.
type Kind string

const (
	KindHTTP Kind = "http"
	KindGRPC Kind = "grpc"
	KindWS   Kind = "ws"
)

// KV is an ordered key/value pair (headers, params, form fields, metadata).
type KV struct {
	Key      string `yaml:"key" json:"key"`
	Value    string `yaml:"value" json:"value"`
	Disabled bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

// Auth types.
const (
	AuthNone    = "none"
	AuthInherit = "inherit"
	AuthBearer  = "bearer"
	AuthBasic   = "basic"
	AuthAPIKey  = "apikey"
)

type Auth struct {
	Type     string `yaml:"type" json:"type"`
	Token    string `yaml:"token,omitempty" json:"token,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	Key      string `yaml:"key,omitempty" json:"key,omitempty"`
	Value    string `yaml:"value,omitempty" json:"value,omitempty"`
	// In is where an API key goes: "header" (default) or "query".
	In string `yaml:"in,omitempty" json:"in,omitempty"`
}

// Body types.
const (
	BodyNone      = "none"
	BodyJSON      = "json"
	BodyText      = "text"
	BodyXML       = "xml"
	BodyForm      = "form"
	BodyMultipart = "multipart"
)

type Body struct {
	Type    string `yaml:"type" json:"type"`
	Content string `yaml:"content,omitempty" json:"content,omitempty"`
	Form    []KV   `yaml:"form,omitempty" json:"form,omitempty"`
}

type Settings struct {
	TimeoutMs          int   `yaml:"timeoutMs,omitempty" json:"timeoutMs,omitempty"`
	FollowRedirects    *bool `yaml:"followRedirects,omitempty" json:"followRedirects,omitempty"`
	InsecureSkipVerify bool  `yaml:"insecureSkipVerify,omitempty" json:"insecureSkipVerify,omitempty"`
}

// GRPC holds gRPC-specific options. The request message lives in Body.Content
// (JSON) and metadata in Headers.
type GRPC struct {
	Service     string   `yaml:"service,omitempty" json:"service,omitempty"`
	Method      string   `yaml:"method,omitempty" json:"method,omitempty"`
	ProtoFiles  []string `yaml:"protoFiles,omitempty" json:"protoFiles,omitempty"`
	ImportPaths []string `yaml:"importPaths,omitempty" json:"importPaths,omitempty"`
	Plaintext   bool     `yaml:"plaintext,omitempty" json:"plaintext,omitempty"`
}

// WSMessage is a saved message that can be sent with one click.
type WSMessage struct {
	Name    string `yaml:"name" json:"name"`
	Content string `yaml:"content" json:"content"`
}

type WS struct {
	Subprotocols []string    `yaml:"subprotocols,omitempty" json:"subprotocols,omitempty"`
	Messages     []WSMessage `yaml:"messages,omitempty" json:"messages,omitempty"`
}

// Request is one request file. HTTP fields are top level; protocol-specific
// extras live in GRPC / WS.
type Request struct {
	Name     string    `yaml:"name" json:"name"`
	Kind     Kind      `yaml:"kind" json:"kind"`
	Method   string    `yaml:"method,omitempty" json:"method,omitempty"`
	URL      string    `yaml:"url" json:"url"`
	Params   []KV      `yaml:"params,omitempty" json:"params,omitempty"`
	Headers  []KV      `yaml:"headers,omitempty" json:"headers,omitempty"`
	Auth     *Auth     `yaml:"auth,omitempty" json:"auth,omitempty"`
	Body     *Body     `yaml:"body,omitempty" json:"body,omitempty"`
	GRPC     *GRPC     `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	WS       *WS       `yaml:"ws,omitempty" json:"ws,omitempty"`
	Settings *Settings `yaml:"settings,omitempty" json:"settings,omitempty"`
	Docs     string    `yaml:"docs,omitempty" json:"docs,omitempty"`
}

// Folder holds defaults inherited by every request below it (_folder.yaml).
type Folder struct {
	Headers []KV  `yaml:"headers,omitempty" json:"headers,omitempty"`
	Auth    *Auth `yaml:"auth,omitempty" json:"auth,omitempty"`
}

// Variable is one environment entry. Secret values never live in the
// committed file; they're stored in <env>.secret.yaml.
type Variable struct {
	Key      string `yaml:"key" json:"key"`
	Value    string `yaml:"value,omitempty" json:"value"`
	Secret   bool   `yaml:"secret,omitempty" json:"secret,omitempty"`
	Disabled bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

type Environment struct {
	Name      string     `yaml:"-" json:"name"`
	Variables []Variable `yaml:"variables" json:"variables"`
}

// Values returns enabled variables as a map.
func (e *Environment) Values() map[string]string {
	m := map[string]string{}
	if e == nil {
		return m
	}
	for _, v := range e.Variables {
		if !v.Disabled && v.Key != "" {
			m[v.Key] = v.Value
		}
	}
	return m
}

// SecretValues returns the values of enabled secret variables.
func (e *Environment) SecretValues() []string {
	var out []string
	if e == nil {
		return out
	}
	for _, v := range e.Variables {
		if v.Secret && !v.Disabled && v.Value != "" {
			out = append(out, v.Value)
		}
	}
	return out
}

// WorkspaceMeta is apiclient.yaml at the workspace root.
type WorkspaceMeta struct {
	Name      string     `yaml:"name" json:"name"`
	Version   int        `yaml:"version" json:"version"`
	Variables []Variable `yaml:"variables,omitempty" json:"variables,omitempty"`
}

// Node is an entry in the request tree shown in the sidebar.
type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"` // slash-separated, relative to requests/
	IsFolder bool    `json:"isFolder"`
	Kind     Kind    `json:"kind,omitempty"`
	Method   string  `json:"method,omitempty"`
	Children []*Node `json:"children,omitempty"`
}

// Clone returns a deep copy so runners can resolve variables without
// mutating the caller's request.
func (r *Request) Clone() *Request {
	if r == nil {
		return nil
	}
	c := *r
	c.Params = cloneKV(r.Params)
	c.Headers = cloneKV(r.Headers)
	if r.Auth != nil {
		a := *r.Auth
		c.Auth = &a
	}
	if r.Body != nil {
		b := *r.Body
		b.Form = cloneKV(r.Body.Form)
		c.Body = &b
	}
	if r.GRPC != nil {
		g := *r.GRPC
		g.ProtoFiles = append([]string(nil), r.GRPC.ProtoFiles...)
		g.ImportPaths = append([]string(nil), r.GRPC.ImportPaths...)
		c.GRPC = &g
	}
	if r.WS != nil {
		w := *r.WS
		w.Subprotocols = append([]string(nil), r.WS.Subprotocols...)
		w.Messages = append([]WSMessage(nil), r.WS.Messages...)
		c.WS = &w
	}
	if r.Settings != nil {
		s := *r.Settings
		if r.Settings.FollowRedirects != nil {
			f := *r.Settings.FollowRedirects
			s.FollowRedirects = &f
		}
		c.Settings = &s
	}
	return &c
}

func cloneKV(in []KV) []KV {
	if in == nil {
		return nil
	}
	return append([]KV(nil), in...)
}
