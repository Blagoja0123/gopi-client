package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"sync"

	"apiclient/internal/model"
	"apiclient/internal/runner/wsrunner"
	"apiclient/internal/scratch"
	"apiclient/internal/store"
	"apiclient/internal/vars"
	"apiclient/internal/webhook"
	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound backend. Every exported method is callable from
// the frontend (see frontend/wailsjs/go/main/App.d.ts).
type App struct {
	ctx       context.Context
	configDir string

	mu        sync.Mutex
	ws        *store.Workspace
	activeEnv string
	settings  settings
	scratch   *scratch.Store
	jars      map[string]http.CookieJar
	runs      map[string]context.CancelFunc
	wsConns   map[string]*wsrunner.Session
	grpcCalls map[string]*grpcCall
	watcher   *fsnotify.Watcher
	receiver  *webhook.Receiver
}

// settings is per-user app state (not part of any workspace).
type settings struct {
	Recent        []string          `json:"recent"`
	LastWorkspace string            `json:"lastWorkspace"`
	ActiveEnv     map[string]string `json:"activeEnv"`
}

func NewApp() *App {
	return &App{
		jars:      map[string]http.CookieJar{},
		runs:      map[string]context.CancelFunc{},
		wsConns:   map[string]*wsrunner.Session{},
		grpcCalls: map[string]*grpcCall{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	a.configDir = filepath.Join(base, "apiclient")
	a.loadSettings()
	if s, err := scratch.New(filepath.Join(a.configDir, "scratch")); err == nil {
		a.scratch = s
	}
	a.receiver = webhook.NewReceiver(500, func(c webhook.Capture) {
		runtime.EventsEmit(a.ctx, "webhook:capture", c)
	})
	if a.settings.LastWorkspace != "" {
		if _, err := a.OpenWorkspace(a.settings.LastWorkspace); err != nil {
			a.settings.LastWorkspace = ""
		}
	}
}

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	for _, c := range a.runs {
		c()
	}
	for _, s := range a.wsConns {
		s.Close()
	}
	for _, g := range a.grpcCalls {
		g.stream.Cancel()
	}
	if a.watcher != nil {
		a.watcher.Close()
	}
	a.mu.Unlock()
	if a.receiver != nil {
		a.receiver.Stop()
	}
}

func (a *App) loadSettings() {
	b, err := os.ReadFile(filepath.Join(a.configDir, "settings.json"))
	if err == nil {
		json.Unmarshal(b, &a.settings)
	}
	if a.settings.ActiveEnv == nil {
		a.settings.ActiveEnv = map[string]string{}
	}
}

// saveSettings must be called with a.mu held.
func (a *App) saveSettings() {
	b, _ := json.MarshalIndent(a.settings, "", "  ")
	os.MkdirAll(a.configDir, 0o755)
	os.WriteFile(filepath.Join(a.configDir, "settings.json"), b, 0o600)
}

func (a *App) workspace() (*store.Workspace, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ws == nil {
		return nil, errors.New("no workspace open")
	}
	return a.ws, nil
}

// resolve applies folder defaults (when rel points into the workspace) and
// variables from the active environment and workspace globals. It returns
// the resolved request, unresolved variable names and secret values (for
// masking anything shown back to the user).
func (a *App) resolve(rel string, req *model.Request) (*model.Request, []string, []string, error) {
	a.mu.Lock()
	ws, envName := a.ws, a.activeEnv
	a.mu.Unlock()
	eff := req
	var layers []map[string]string
	var secrets []string
	if ws != nil {
		if rel != "" {
			e, err := ws.Effective(rel, req)
			if err != nil {
				return nil, nil, nil, err
			}
			eff = e
		}
		if envName != "" {
			env, err := ws.LoadEnvironment(envName)
			if err != nil {
				return nil, nil, nil, err
			}
			layers = append(layers, env.Values())
			secrets = env.SecretValues()
		}
		layers = append(layers, (&model.Environment{Variables: ws.Meta.Variables}).Values())
	}
	out, missing, err := vars.New(layers...).ApplyRequest(eff)
	return out, missing, secrets, err
}

func (a *App) cookieJar() http.CookieJar {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := "scratch"
	if a.ws != nil {
		key = a.ws.Root + "|" + a.activeEnv
	}
	if j, ok := a.jars[key]; ok {
		return j
	}
	j, _ := cookiejar.New(nil)
	a.jars[key] = j
	return j
}

// ClearCookies drops cookies for the current workspace/environment.
func (a *App) ClearCookies() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.jars = map[string]http.CookieJar{}
}
