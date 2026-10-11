// Package scratch persists one-off requests that don't belong to any
// workspace. They live in the user's config dir, not in a repo.
package scratch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"apiclient/internal/model"
	"apiclient/internal/store"
	"gopkg.in/yaml.v3"
)

type Tab struct {
	ID      string         `json:"id"`
	Request *model.Request `json:"request"`
}

type Store struct {
	Dir string
}

var (
	validID = regexp.MustCompile(`^[a-z0-9-]+$`)
	counter atomic.Int64
)

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

func (s *Store) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", fmt.Errorf("invalid scratch id %q", id)
	}
	return filepath.Join(s.Dir, id+".yaml"), nil
}

// newID sorts lexically in creation order.
func newID() string {
	return fmt.Sprintf("%016x-%s", time.Now().UnixNano(), strconv.FormatInt(counter.Add(1), 36))
}

func (s *Store) Create(kind model.Kind) (Tab, error) {
	t := Tab{ID: newID(), Request: store.NewRequest("Untitled", kind)}
	return t, s.Save(t)
}

func (s *Store) Save(t Tab) error {
	p, err := s.path(t.ID)
	if err != nil {
		return err
	}
	if t.Request == nil {
		return errors.New("tab has no request")
	}
	b, err := yaml.Marshal(t.Request)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

func (s *Store) List() ([]Tab, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if id, ok := strings.CutSuffix(e.Name(), ".yaml"); ok && validID.MatchString(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	tabs := make([]Tab, 0, len(ids))
	for _, id := range ids {
		b, err := os.ReadFile(filepath.Join(s.Dir, id+".yaml"))
		if err != nil {
			return nil, err
		}
		var r model.Request
		if err := yaml.Unmarshal(b, &r); err != nil {
			continue // skip corrupt scratch files rather than failing startup
		}
		tabs = append(tabs, Tab{ID: id, Request: &r})
	}
	return tabs, nil
}

func (s *Store) Get(id string) (Tab, error) {
	p, err := s.path(id)
	if err != nil {
		return Tab{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Tab{}, err
	}
	var r model.Request
	if err := yaml.Unmarshal(b, &r); err != nil {
		return Tab{}, err
	}
	return Tab{ID: id, Request: &r}, nil
}

func (s *Store) Delete(id string) error {
	p, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Promote moves a scratch request into a workspace folder and returns its
// new path.
func (s *Store) Promote(ws *store.Workspace, id, folder string) (string, error) {
	t, err := s.Get(id)
	if err != nil {
		return "", err
	}
	rel, err := ws.AddRequest(folder, t.Request)
	if err != nil {
		return "", err
	}
	return rel, s.Delete(id)
}
