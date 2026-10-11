// Package store reads and writes a workspace: a plain folder (usually a git
// repo) with one YAML file per request.
package store

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"apiclient/internal/model"
)

const (
	MetaFile     = "apiclient.yaml"
	RequestsDir  = "requests"
	EnvDir       = "environments"
	LocalDir     = ".apiclient"
	FolderFile   = "_folder.yaml"
	secretSuffix = ".secret.yaml"
)

var ignoreLines = []string{"*.secret.yaml", ".apiclient/"}

type Workspace struct {
	Root string
	Meta model.WorkspaceMeta
}

// Init creates (or upgrades) a workspace in root. It is safe to call on an
// existing folder or git repo.
func Init(root, name string) (*Workspace, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, d := range []string{RequestsDir, EnvDir} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return nil, err
		}
	}
	if err := ensureGitignore(filepath.Join(root, ".gitignore")); err != nil {
		return nil, err
	}
	ws := &Workspace{Root: root}
	if _, err := os.Stat(filepath.Join(root, MetaFile)); err == nil {
		if err := readYAML(filepath.Join(root, MetaFile), &ws.Meta); err != nil {
			return nil, err
		}
		return ws, nil
	}
	ws.Meta = model.WorkspaceMeta{Name: name, Version: 1}
	return ws, ws.SaveMeta()
}

func ensureGitignore(p string) error {
	b, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	content := string(b)
	have := map[string]bool{}
	for _, l := range strings.Split(content, "\n") {
		have[strings.TrimSpace(l)] = true
	}
	var add []string
	for _, l := range ignoreLines {
		if !have[l] {
			add = append(add, l)
		}
	}
	if len(add) == 0 {
		return nil
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += strings.Join(add, "\n") + "\n"
	return writeFileAtomic(p, []byte(content))
}

// Open loads an existing workspace.
func Open(root string) (*Workspace, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	ws := &Workspace{Root: root}
	if err := readYAML(filepath.Join(root, MetaFile), &ws.Meta); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s is not a workspace (no %s)", root, MetaFile)
		}
		return nil, err
	}
	return ws, nil
}

func (w *Workspace) SaveMeta() error {
	return writeYAML(filepath.Join(w.Root, MetaFile), &w.Meta)
}

// reqPath converts a slash-separated path relative to requests/ into an
// absolute path, rejecting anything that escapes the requests folder.
func (w *Workspace) reqPath(rel string) (string, error) {
	return safeJoin(filepath.Join(w.Root, RequestsDir), rel)
}

func safeJoin(base, rel string) (string, error) {
	if rel == "" {
		return base, nil
	}
	if strings.HasPrefix(rel, "/") || strings.Contains(rel, "\\") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid path %q", rel)
	}
	clean := path.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid path %q", rel)
	}
	return filepath.Join(base, filepath.FromSlash(clean)), nil
}

func (w *Workspace) LoadRequest(rel string) (*model.Request, error) {
	p, err := w.reqPath(rel)
	if err != nil {
		return nil, err
	}
	var r model.Request
	if err := readYAML(p, &r); err != nil {
		return nil, err
	}
	if r.Kind == "" {
		r.Kind = model.KindHTTP
	}
	return &r, nil
}

func (w *Workspace) SaveRequest(rel string, r *model.Request) error {
	if !strings.HasSuffix(rel, ".yaml") {
		return fmt.Errorf("request path %q must end in .yaml", rel)
	}
	p, err := w.reqPath(rel)
	if err != nil {
		return err
	}
	return writeYAML(p, r)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(name string) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if s == "" {
		s = "request"
	}
	return s
}

// uniquePath returns folder/slug.yaml, adding -2, -3... if taken.
func (w *Workspace) uniquePath(folder, slug, ext string) (string, error) {
	for i := 1; ; i++ {
		name := slug
		if i > 1 {
			name += "-" + strconv.Itoa(i)
		}
		rel := path.Join(folder, name+ext)
		p, err := w.reqPath(rel)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			return rel, nil
		}
	}
}

// NewRequest returns a sensible blank request of the given kind.
func NewRequest(name string, kind model.Kind) *model.Request {
	r := &model.Request{Name: name, Kind: kind}
	switch kind {
	case model.KindGRPC:
		r.Body = &model.Body{Type: model.BodyJSON, Content: "{}"}
		r.GRPC = &model.GRPC{}
	case model.KindWS:
		r.WS = &model.WS{}
	default:
		r.Kind = model.KindHTTP
		r.Method = "GET"
	}
	return r
}

// CreateRequest writes a new blank request into folder and returns its path.
func (w *Workspace) CreateRequest(folder, name string, kind model.Kind) (string, error) {
	return w.AddRequest(folder, NewRequest(name, kind))
}

// AddRequest saves req under folder using a unique slug of its name.
func (w *Workspace) AddRequest(folder string, req *model.Request) (string, error) {
	rel, err := w.uniquePath(folder, Slug(req.Name), ".yaml")
	if err != nil {
		return "", err
	}
	return rel, w.SaveRequest(rel, req)
}

func (w *Workspace) CreateFolder(parent, name string) (string, error) {
	rel, err := w.uniquePath(parent, Slug(name), "")
	if err != nil {
		return "", err
	}
	p, _ := w.reqPath(rel)
	return rel, os.MkdirAll(p, 0o755)
}

// Delete removes a request file or a whole folder.
func (w *Workspace) Delete(rel string) error {
	if rel == "" {
		return errors.New("refusing to delete the requests root")
	}
	p, err := w.reqPath(rel)
	if err != nil {
		return err
	}
	return os.RemoveAll(p)
}

// Move renames a request or folder; it never overwrites.
func (w *Workspace) Move(from, to string) error {
	src, err := w.reqPath(from)
	if err != nil {
		return err
	}
	dst, err := w.reqPath(to)
	if err != nil || from == "" || to == "" {
		return fmt.Errorf("invalid move %q -> %q", from, to)
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", to)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// Tree lists requests/ for the sidebar: folders first, then requests by name.
// A file that fails to parse is still listed (by file name) so the user can
// find and fix it.
func (w *Workspace) Tree() (*model.Node, error) {
	root := &model.Node{Name: w.Meta.Name, IsFolder: true}
	if err := w.walk(root, ""); err != nil {
		return nil, err
	}
	return root, nil
}

func (w *Workspace) walk(n *model.Node, rel string) error {
	dir, _ := w.reqPath(rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		childRel := path.Join(rel, name)
		if e.IsDir() {
			c := &model.Node{Name: name, Path: childRel, IsFolder: true}
			if err := w.walk(c, childRel); err != nil {
				return err
			}
			n.Children = append(n.Children, c)
			continue
		}
		if !strings.HasSuffix(name, ".yaml") || strings.HasPrefix(name, "_") {
			continue
		}
		c := &model.Node{Name: name, Path: childRel}
		if r, err := w.LoadRequest(childRel); err == nil {
			if r.Name != "" {
				c.Name = r.Name
			}
			c.Kind, c.Method = r.Kind, r.Method
		}
		n.Children = append(n.Children, c)
	}
	sort.SliceStable(n.Children, func(i, j int) bool {
		a, b := n.Children[i], n.Children[j]
		if a.IsFolder != b.IsFolder {
			return a.IsFolder
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return nil
}

func (w *Workspace) LoadFolder(rel string) (*model.Folder, error) {
	p, err := w.reqPath(path.Join(rel, FolderFile))
	if err != nil {
		return nil, err
	}
	f := &model.Folder{}
	if err := readYAML(p, f); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return f, nil
}

func (w *Workspace) SaveFolder(rel string, f *model.Folder) error {
	p, err := w.reqPath(path.Join(rel, FolderFile))
	if err != nil {
		return err
	}
	return writeYAML(p, f)
}

// Effective returns a copy of req with folder defaults (headers, auth)
// applied from the requests root down to the request's own folder.
func (w *Workspace) Effective(rel string, req *model.Request) (*model.Request, error) {
	out := req.Clone()
	var headers []model.KV
	var auth *model.Auth
	dirs := []string{""}
	if d := path.Dir(rel); d != "." && d != "" {
		parts := strings.Split(d, "/")
		for i := range parts {
			dirs = append(dirs, strings.Join(parts[:i+1], "/"))
		}
	}
	for _, d := range dirs {
		f, err := w.LoadFolder(d)
		if err != nil {
			return nil, err
		}
		headers = mergeKV(headers, f.Headers)
		if f.Auth != nil && f.Auth.Type != "" && f.Auth.Type != model.AuthInherit {
			a := *f.Auth
			auth = &a
		}
	}
	out.Headers = mergeKV(headers, req.Headers)
	if out.Auth == nil || out.Auth.Type == "" || out.Auth.Type == model.AuthInherit {
		out.Auth = auth
	}
	return out, nil
}

// mergeKV appends over onto base; keys in over replace base keys
// (case-insensitive) in place.
func mergeKV(base, over []model.KV) []model.KV {
	out := append([]model.KV(nil), base...)
	for _, kv := range over {
		replaced := false
		for i := range out {
			if strings.EqualFold(out[i].Key, kv.Key) {
				out[i] = kv
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, kv)
		}
	}
	return out
}
