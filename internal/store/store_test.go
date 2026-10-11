package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"apiclient/internal/model"
)

// copyFixture copies testdata/workspace into a temp dir so tests can mutate it.
func copyFixture(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := "testdata/workspace"
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInitCreatesLayout(t *testing.T) {
	dir := t.TempDir()
	// Pre-existing .gitignore must be preserved and appended to.
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules\n"), 0o644)

	ws, err := Init(dir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Meta.Name != "demo" || ws.Meta.Version != 1 {
		t.Fatalf("meta = %+v", ws.Meta)
	}
	for _, p := range []string{"apiclient.yaml", "environments", "requests"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	gi := readFile(t, filepath.Join(dir, ".gitignore"))
	for _, want := range []string{"node_modules", "*.secret.yaml", ".apiclient/"} {
		if !strings.Contains(gi, want) {
			t.Errorf(".gitignore missing %q:\n%s", want, gi)
		}
	}

	// Init is idempotent: no duplicate ignore lines.
	if _, err := Init(dir, "demo"); err != nil {
		t.Fatal(err)
	}
	gi = readFile(t, filepath.Join(dir, ".gitignore"))
	if c := strings.Count(gi, "*.secret.yaml"); c != 1 {
		t.Errorf("*.secret.yaml appears %d times", c)
	}
}

func TestOpenRequiresMeta(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("expected error opening a folder without apiclient.yaml")
	}
}

// Loading and re-saving every file must not change a single byte; otherwise
// simply opening a request would create git noise.
func TestRoundTripIsByteStable(t *testing.T) {
	dir := copyFixture(t)
	ws, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := []string{"users/get-user.yaml", "users/create-user.yaml", "chat.yaml", "greeter.yaml"}
	for _, rel := range files {
		before := readFile(t, filepath.Join(dir, "requests", rel))
		req, err := ws.LoadRequest(rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if err := ws.SaveRequest(rel, req); err != nil {
			t.Fatal(err)
		}
		if after := readFile(t, filepath.Join(dir, "requests", rel)); after != before {
			t.Errorf("%s changed after round trip\n--- before\n%s\n--- after\n%s", rel, before, after)
		}
	}

	before := readFile(t, filepath.Join(dir, "apiclient.yaml"))
	if err := ws.SaveMeta(); err != nil {
		t.Fatal(err)
	}
	if after := readFile(t, filepath.Join(dir, "apiclient.yaml")); after != before {
		t.Errorf("apiclient.yaml changed\n%s\nvs\n%s", before, after)
	}

	envBefore := readFile(t, filepath.Join(dir, "environments", "dev.yaml"))
	env, err := ws.LoadEnvironment("dev")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.SaveEnvironment(env); err != nil {
		t.Fatal(err)
	}
	if after := readFile(t, filepath.Join(dir, "environments", "dev.yaml")); after != envBefore {
		t.Errorf("dev.yaml changed\n%s\nvs\n%s", envBefore, after)
	}
}

func TestSaveIsDeterministic(t *testing.T) {
	ws, err := Init(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	req := &model.Request{Name: "A", Kind: model.KindHTTP, Method: "GET", URL: "http://x",
		Headers: []model.KV{{Key: "b", Value: "2"}, {Key: "a", Value: "1"}}}
	ws.SaveRequest("a.yaml", req)
	first := readFile(t, filepath.Join(ws.Root, "requests", "a.yaml"))
	for i := 0; i < 5; i++ {
		ws.SaveRequest("a.yaml", req)
		if got := readFile(t, filepath.Join(ws.Root, "requests", "a.yaml")); got != first {
			t.Fatalf("output differs on save %d", i)
		}
	}
	// Header order is preserved as given, not sorted.
	if strings.Index(first, "key: b") > strings.Index(first, "key: a") {
		t.Errorf("header order not preserved:\n%s", first)
	}
}

func TestTree(t *testing.T) {
	ws, err := Open(copyFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	root, err := ws.Tree()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range root.Children {
		names = append(names, c.Name)
	}
	// Folders first, then requests sorted by display name.
	want := []string{"users", "Chat", "Health check"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("root children = %v, want %v", names, want)
	}
	users := root.Children[0]
	if !users.IsFolder || users.Path != "users" || len(users.Children) != 2 {
		t.Fatalf("users node = %+v", users)
	}
	// _folder.yaml is not listed as a request.
	for _, c := range users.Children {
		if strings.HasPrefix(filepath.Base(c.Path), "_") {
			t.Errorf("folder settings listed as request: %s", c.Path)
		}
	}
	get := users.Children[1]
	if get.Path != "users/get-user.yaml" || get.Method != "GET" || get.Kind != model.KindHTTP {
		t.Errorf("get-user node = %+v", get)
	}
}

func TestCreateRequestSlugAndCollision(t *testing.T) {
	ws, _ := Init(t.TempDir(), "x")
	rel, err := ws.CreateRequest("users", "Get User!", model.KindHTTP)
	if err != nil {
		t.Fatal(err)
	}
	if rel != "users/get-user.yaml" {
		t.Fatalf("rel = %q", rel)
	}
	rel2, err := ws.CreateRequest("users", "Get User", model.KindHTTP)
	if err != nil {
		t.Fatal(err)
	}
	if rel2 != "users/get-user-2.yaml" {
		t.Fatalf("rel2 = %q", rel2)
	}
	req, err := ws.LoadRequest(rel)
	if err != nil || req.Name != "Get User!" || req.Method != "GET" {
		t.Fatalf("req = %+v err = %v", req, err)
	}
}

func TestMoveAndDelete(t *testing.T) {
	dir := copyFixture(t)
	ws, _ := Open(dir)
	if err := ws.Move("users/get-user.yaml", "admin/get-user.yaml"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "requests/users/get-user.yaml")); !os.IsNotExist(err) {
		t.Error("old file still exists")
	}
	if _, err := ws.LoadRequest("admin/get-user.yaml"); err != nil {
		t.Error(err)
	}
	if err := ws.Move("chat.yaml", "greeter.yaml"); err == nil {
		t.Error("move onto existing file should fail")
	}
	if err := ws.Delete("users"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "requests/users")); !os.IsNotExist(err) {
		t.Error("folder not deleted")
	}
}

func TestMalformedYAMLNamesFile(t *testing.T) {
	ws, _ := Init(t.TempDir(), "x")
	os.WriteFile(filepath.Join(ws.Root, "requests", "bad.yaml"), []byte("name: [unclosed"), 0o644)
	_, err := ws.LoadRequest("bad.yaml")
	if err == nil || !strings.Contains(err.Error(), "bad.yaml") {
		t.Fatalf("err = %v", err)
	}
	// A broken file must not break the whole tree.
	root, err := ws.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 1 || root.Children[0].Name != "bad.yaml" {
		t.Errorf("tree = %+v", root.Children)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	ws, _ := Init(t.TempDir(), "x")
	for _, p := range []string{"../evil.yaml", "a/../../evil.yaml", "/etc/passwd"} {
		if err := ws.SaveRequest(p, &model.Request{Name: "x"}); err == nil {
			t.Errorf("SaveRequest(%q) should fail", p)
		}
		if _, err := ws.LoadRequest(p); err == nil {
			t.Errorf("LoadRequest(%q) should fail", p)
		}
	}
	if _, err := ws.LoadEnvironment("../dev"); err == nil {
		t.Error("env traversal should fail")
	}
}

func TestEnvironmentSecretsSplit(t *testing.T) {
	dir := copyFixture(t)
	ws, _ := Open(dir)
	env := &model.Environment{Name: "dev", Variables: []model.Variable{
		{Key: "baseUrl", Value: "http://localhost:8080"},
		{Key: "token", Value: "s3cr3t", Secret: true},
	}}
	if err := ws.SaveEnvironment(env); err != nil {
		t.Fatal(err)
	}
	committed := readFile(t, filepath.Join(dir, "environments", "dev.yaml"))
	if strings.Contains(committed, "s3cr3t") {
		t.Fatalf("secret leaked into committed file:\n%s", committed)
	}
	secret := readFile(t, filepath.Join(dir, "environments", "dev.secret.yaml"))
	if !strings.Contains(secret, "s3cr3t") {
		t.Fatalf("secret file missing value:\n%s", secret)
	}
	got, err := ws.LoadEnvironment("dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Values()["token"] != "s3cr3t" {
		t.Errorf("merged env = %+v", got.Variables)
	}
	names, _ := ws.ListEnvironments()
	if len(names) != 1 || names[0] != "dev" {
		t.Errorf("envs = %v (secret file must not be listed)", names)
	}
	if err := ws.DeleteEnvironment("dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "environments", "dev.secret.yaml")); !os.IsNotExist(err) {
		t.Error("secret file not removed with env")
	}
}

func TestEffectiveInheritsFolderDefaults(t *testing.T) {
	ws, _ := Open(copyFixture(t))
	req, _ := ws.LoadRequest("users/get-user.yaml")
	eff, err := ws.Effective("users/get-user.yaml", req)
	if err != nil {
		t.Fatal(err)
	}
	if eff.Auth == nil || eff.Auth.Type != model.AuthBearer || eff.Auth.Token != "{{token}}" {
		t.Errorf("auth = %+v", eff.Auth)
	}
	h := map[string]string{}
	for _, kv := range eff.Headers {
		h[kv.Key] = kv.Value
	}
	if h["X-Team"] != "core" || h["Accept"] != "application/json" {
		t.Errorf("headers = %+v", eff.Headers)
	}
	// The original request is untouched.
	if req.Auth.Type != model.AuthInherit || len(req.Headers) != 1 {
		t.Error("Effective mutated its input")
	}

	// A request-level header overrides the folder's (case-insensitive).
	req.Headers = append(req.Headers, model.KV{Key: "x-team", Value: "mine"})
	eff, _ = ws.Effective("users/get-user.yaml", req)
	n := 0
	for _, kv := range eff.Headers {
		if strings.EqualFold(kv.Key, "x-team") {
			n++
			if kv.Value != "mine" {
				t.Errorf("override lost: %v", kv)
			}
		}
	}
	if n != 1 {
		t.Errorf("x-team appears %d times", n)
	}
}
