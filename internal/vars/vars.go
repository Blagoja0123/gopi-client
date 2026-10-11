// Package vars implements {{variable}} interpolation with layered
// environments and secret masking.
package vars

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"time"

	"apiclient/internal/model"
	"github.com/google/uuid"
)

const MaskString = "••••••"

var varRe = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-$]+)\s*\}\}`)

// Resolver looks names up through layers; the first layer wins.
type Resolver struct {
	layers []map[string]string
	now    func() time.Time
}

// New builds a resolver. Pass layers from highest to lowest precedence, e.g.
// New(overrides, env, globals).
func New(layers ...map[string]string) *Resolver {
	return &Resolver{layers: layers, now: time.Now}
}

func (r *Resolver) Lookup(name string) (string, bool) {
	if strings.HasPrefix(name, "$") {
		return r.dynamic(name)
	}
	for _, l := range r.layers {
		if v, ok := l[name]; ok {
			return v, true
		}
	}
	return "", false
}

func (r *Resolver) dynamic(name string) (string, bool) {
	switch name {
	case "$uuid":
		return uuid.NewString(), true
	case "$timestamp":
		return strconv.FormatInt(r.now().Unix(), 10), true
	case "$isoTimestamp":
		return r.now().UTC().Format(time.RFC3339), true
	case "$randomInt":
		return strconv.Itoa(rand.IntN(1001)), true
	}
	return "", false
}

// Expand replaces every known {{name}}. Unknown names are left as-is and
// returned (deduplicated, in order of appearance). A reference cycle is an
// error.
func (r *Resolver) Expand(s string) (string, []string, error) {
	var missing []string
	seen := map[string]bool{}
	out, err := r.expand(s, nil, func(n string) {
		if !seen[n] {
			seen[n] = true
			missing = append(missing, n)
		}
	})
	return out, missing, err
}

func (r *Resolver) expand(s string, stack []string, onMissing func(string)) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	var firstErr error
	out := varRe.ReplaceAllStringFunc(s, func(m string) string {
		if firstErr != nil {
			return m
		}
		name := varRe.FindStringSubmatch(m)[1]
		for _, n := range stack {
			if n == name {
				firstErr = fmt.Errorf("variable cycle: %s -> %s", strings.Join(stack, " -> "), name)
				return m
			}
		}
		v, ok := r.Lookup(name)
		if !ok {
			onMissing(name)
			return m
		}
		v, err := r.expand(v, append(stack, name), onMissing)
		if err != nil {
			firstErr = err
			return m
		}
		return v
	})
	return out, firstErr
}

// Names returns the variable names referenced in s.
func Names(s string) []string {
	var out []string
	for _, m := range varRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// ApplyRequest returns a copy of req with every string field expanded.
func (r *Resolver) ApplyRequest(req *model.Request) (*model.Request, []string, error) {
	out := req.Clone()
	var missing []string
	seen := map[string]bool{}
	var firstErr error
	exp := func(p *string) {
		v, err := r.expand(*p, nil, func(n string) {
			if !seen[n] {
				seen[n] = true
				missing = append(missing, n)
			}
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
		*p = v
	}
	expKV := func(kvs []model.KV) {
		for i := range kvs {
			if kvs[i].Disabled {
				continue
			}
			exp(&kvs[i].Key)
			exp(&kvs[i].Value)
		}
	}
	exp(&out.URL)
	expKV(out.Params)
	expKV(out.Headers)
	if a := out.Auth; a != nil {
		for _, p := range []*string{&a.Token, &a.Username, &a.Password, &a.Key, &a.Value} {
			exp(p)
		}
	}
	if b := out.Body; b != nil {
		exp(&b.Content)
		expKV(b.Form)
	}
	if g := out.GRPC; g != nil {
		exp(&g.Service)
		exp(&g.Method)
	}
	return out, missing, firstErr
}

// Mask hides each secret value in s. Very short secrets (< 3 chars) are not
// masked, since replacing them would garble unrelated text.
func Mask(s string, secrets []string) string {
	for _, sec := range secrets {
		if len(sec) < 3 {
			continue
		}
		s = strings.ReplaceAll(s, sec, MaskString)
	}
	return s
}
