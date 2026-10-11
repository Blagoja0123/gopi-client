package main

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"apiclient/internal/model"
	"apiclient/internal/scratch"
	"apiclient/internal/store"
	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type WorkspaceInfo struct {
	Root string `json:"root"`
	Name string `json:"name"`
}

type AppState struct {
	Workspace    *WorkspaceInfo `json:"workspace"`
	Environments []string       `json:"environments"`
	ActiveEnv    string         `json:"activeEnv"`
	Recent       []string       `json:"recent"`
}

func (a *App) GetState() AppState {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := AppState{Recent: append([]string(nil), a.settings.Recent...), ActiveEnv: a.activeEnv}
	if a.ws != nil {
		st.Workspace = &WorkspaceInfo{Root: a.ws.Root, Name: a.ws.Meta.Name}
		st.Environments, _ = a.ws.ListEnvironments()
	}
	return st
}

func (a *App) PickDirectory(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title, CanCreateDirectories: true})
}

func (a *App) PickFile(title string, pattern string) (string, error) {
	opts := runtime.OpenDialogOptions{Title: title}
	if pattern != "" {
		opts.Filters = []runtime.FileFilter{{DisplayName: pattern, Pattern: pattern}}
	}
	return runtime.OpenFileDialog(a.ctx, opts)
}

func (a *App) OpenWorkspace(root string) (AppState, error) {
	ws, err := store.Open(root)
	if err != nil {
		return AppState{}, err
	}
	a.setWorkspace(ws)
	return a.GetState(), nil
}

// CreateWorkspace initialises root (which may be an existing git repo).
func (a *App) CreateWorkspace(root, name string) (AppState, error) {
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(root)
	}
	ws, err := store.Init(root, name)
	if err != nil {
		return AppState{}, err
	}
	a.setWorkspace(ws)
	return a.GetState(), nil
}

func (a *App) CloseWorkspace() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.watcher != nil {
		a.watcher.Close()
		a.watcher = nil
	}
	a.ws, a.activeEnv = nil, ""
	a.settings.LastWorkspace = ""
	a.saveSettings()
}

func (a *App) setWorkspace(ws *store.Workspace) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.watcher != nil {
		a.watcher.Close()
		a.watcher = nil
	}
	a.ws = ws
	a.activeEnv = a.settings.ActiveEnv[ws.Root]
	if envs, _ := ws.ListEnvironments(); !slices.Contains(envs, a.activeEnv) {
		a.activeEnv = ""
	}
	a.settings.LastWorkspace = ws.Root
	recent := []string{ws.Root}
	for _, r := range a.settings.Recent {
		if r != ws.Root && len(recent) < 10 {
			recent = append(recent, r)
		}
	}
	a.settings.Recent = recent
	a.saveSettings()
	a.watch(ws)
}

// watch emits "workspace:changed" (debounced) when files change on disk,
// e.g. after a git pull or checkout. Must be called with a.mu held.
func (a *App) watch(ws *store.Workspace) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	a.watcher = w
	addTree := func(dir string) {
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				w.Add(p)
			}
			return nil
		})
	}
	w.Add(ws.Root)
	addTree(filepath.Join(ws.Root, store.RequestsDir))
	w.Add(filepath.Join(ws.Root, store.EnvDir))
	go func() {
		var timer *time.Timer
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Has(fsnotify.Create) {
					addTree(ev.Name)
				}
				if strings.Contains(filepath.Base(ev.Name), ".tmp-") {
					continue
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(300*time.Millisecond, func() {
					a.mu.Lock()
					if a.ws == ws {
						if fresh, err := store.Open(ws.Root); err == nil {
							ws.Meta = fresh.Meta
						}
					}
					a.mu.Unlock()
					runtime.EventsEmit(a.ctx, "workspace:changed")
				})
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

func (a *App) GetTree() (*model.Node, error) {
	ws, err := a.workspace()
	if err != nil {
		return nil, err
	}
	return ws.Tree()
}

func (a *App) LoadRequest(rel string) (*model.Request, error) {
	ws, err := a.workspace()
	if err != nil {
		return nil, err
	}
	return ws.LoadRequest(rel)
}

func (a *App) SaveRequest(rel string, req *model.Request) error {
	ws, err := a.workspace()
	if err != nil {
		return err
	}
	return ws.SaveRequest(rel, req)
}

func (a *App) CreateRequest(folder, name, kind string) (string, error) {
	ws, err := a.workspace()
	if err != nil {
		return "", err
	}
	return ws.CreateRequest(folder, name, model.Kind(kind))
}

func (a *App) CreateFolder(parent, name string) (string, error) {
	ws, err := a.workspace()
	if err != nil {
		return "", err
	}
	return ws.CreateFolder(parent, name)
}

func (a *App) DeletePath(rel string) error {
	ws, err := a.workspace()
	if err != nil {
		return err
	}
	return ws.Delete(rel)
}

func (a *App) MovePath(from, to string) error {
	ws, err := a.workspace()
	if err != nil {
		return err
	}
	return ws.Move(from, to)
}

func (a *App) LoadFolder(rel string) (*model.Folder, error) {
	ws, err := a.workspace()
	if err != nil {
		return nil, err
	}
	return ws.LoadFolder(rel)
}

func (a *App) SaveFolder(rel string, f *model.Folder) error {
	ws, err := a.workspace()
	if err != nil {
		return err
	}
	return ws.SaveFolder(rel, f)
}

// ---- environments ----

func (a *App) GetGlobals() ([]model.Variable, error) {
	ws, err := a.workspace()
	if err != nil {
		return nil, err
	}
	return ws.Meta.Variables, nil
}

func (a *App) SaveGlobals(vars []model.Variable) error {
	ws, err := a.workspace()
	if err != nil {
		return err
	}
	for _, v := range vars {
		if v.Secret {
			return errInvalid("workspace variables are committed to git; put secrets in an environment instead")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	ws.Meta.Variables = vars
	return ws.SaveMeta()
}

func (a *App) GetEnvironment(name string) (*model.Environment, error) {
	ws, err := a.workspace()
	if err != nil {
		return nil, err
	}
	return ws.LoadEnvironment(name)
}

func (a *App) SaveEnvironment(env *model.Environment) error {
	ws, err := a.workspace()
	if err != nil {
		return err
	}
	return ws.SaveEnvironment(env)
}

func (a *App) DeleteEnvironment(name string) error {
	ws, err := a.workspace()
	if err != nil {
		return err
	}
	if err := ws.DeleteEnvironment(name); err != nil {
		return err
	}
	a.mu.Lock()
	if a.activeEnv == name {
		a.activeEnv = ""
	}
	a.mu.Unlock()
	return nil
}

func (a *App) SetActiveEnvironment(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.activeEnv = name
	if a.ws != nil {
		a.settings.ActiveEnv[a.ws.Root] = name
		a.saveSettings()
	}
}

// ---- scratch ----

func (a *App) ListScratch() ([]scratch.Tab, error) {
	if a.scratch == nil {
		return nil, errInvalid("scratch storage unavailable")
	}
	return a.scratch.List()
}

func (a *App) CreateScratch(kind string) (scratch.Tab, error) {
	if a.scratch == nil {
		return scratch.Tab{}, errInvalid("scratch storage unavailable")
	}
	return a.scratch.Create(model.Kind(kind))
}

func (a *App) SaveScratch(tab scratch.Tab) error {
	if a.scratch == nil {
		return errInvalid("scratch storage unavailable")
	}
	return a.scratch.Save(tab)
}

func (a *App) DeleteScratch(id string) error {
	if a.scratch == nil {
		return nil
	}
	return a.scratch.Delete(id)
}

func (a *App) PromoteScratch(id, folder string) (string, error) {
	ws, err := a.workspace()
	if err != nil {
		return "", err
	}
	return a.scratch.Promote(ws, id, folder)
}

type errInvalid string

func (e errInvalid) Error() string { return string(e) }
