package scratch

import (
	"os"
	"path/filepath"
	"testing"

	"apiclient/internal/model"
	"apiclient/internal/store"
)

func TestCreateSaveListDelete(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "scratch"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Create(model.KindHTTP)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Create(model.KindWS)
	a.Request.URL = "https://httpbin.org/get"
	if err := s.Save(a); err != nil {
		t.Fatal(err)
	}
	tabs, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 2 || tabs[0].ID != a.ID || tabs[1].ID != b.ID {
		t.Fatalf("tabs = %+v (want creation order)", tabs)
	}
	if tabs[0].Request.URL != "https://httpbin.org/get" || tabs[1].Request.Kind != model.KindWS {
		t.Errorf("tabs = %+v / %+v", tabs[0].Request, tabs[1].Request)
	}
	if err := s.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	if tabs, _ = s.List(); len(tabs) != 1 {
		t.Errorf("after delete: %d", len(tabs))
	}
}

func TestRejectsBadIDs(t *testing.T) {
	s, _ := New(t.TempDir())
	for _, id := range []string{"", "../x", "a/b"} {
		if err := s.Save(Tab{ID: id, Request: &model.Request{}}); err == nil {
			t.Errorf("Save(%q) should fail", id)
		}
		if err := s.Delete(id); err == nil {
			t.Errorf("Delete(%q) should fail", id)
		}
	}
}

func TestPromoteToWorkspace(t *testing.T) {
	s, _ := New(t.TempDir())
	ws, _ := store.Init(t.TempDir(), "demo")
	tab, _ := s.Create(model.KindHTTP)
	tab.Request.Name = "Quick check"
	tab.Request.URL = "http://x"
	s.Save(tab)

	rel, err := s.Promote(ws, tab.ID, "users")
	if err != nil {
		t.Fatal(err)
	}
	if rel != "users/quick-check.yaml" {
		t.Errorf("rel = %q", rel)
	}
	got, err := ws.LoadRequest(rel)
	if err != nil || got.URL != "http://x" {
		t.Fatalf("got %+v err %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, tab.ID+".yaml")); !os.IsNotExist(err) {
		t.Error("scratch file not removed after promote")
	}
}
