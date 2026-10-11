package openapi

import (
	"flag"
	"os"
	"strings"
	"testing"

	"apiclient/internal/model"
	"apiclient/internal/store"
	"gopkg.in/yaml.v3"
)

var update = flag.Bool("update", false, "rewrite golden files")

func load(t *testing.T, name string) *Result {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func golden(t *testing.T, name string, v any) {
	t.Helper()
	got, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := "testdata/" + name
	if *update {
		os.WriteFile(path, got, 0o644)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(got) != string(want) {
		t.Errorf("%s mismatch (run with -update to accept)\n--- got\n%s", name, got)
	}
}

func find(t *testing.T, res *Result, name string) Imported {
	t.Helper()
	for _, r := range res.Requests {
		if r.Request.Name == name {
			return r
		}
	}
	t.Fatalf("request %q not imported", name)
	return Imported{}
}

func TestPetstoreV3(t *testing.T) {
	res := load(t, "petstore.yaml")
	golden(t, "petstore.golden.yaml", res)

	if res.Title != "Swagger Petstore" {
		t.Errorf("title = %q", res.Title)
	}
	if res.BaseURL != "https://api.petstore.example.com/v1" {
		t.Errorf("baseUrl = %q (server variables should use defaults)", res.BaseURL)
	}
	if res.Auth == nil || res.Auth.Type != model.AuthBearer || res.Auth.Token != "{{token}}" {
		t.Errorf("default auth = %+v", res.Auth)
	}
	if len(res.Requests) != 6 {
		t.Fatalf("got %d requests", len(res.Requests))
	}

	show := find(t, res, "showPetById")
	if show.Folder != "pets" || show.Request.URL != "{{baseUrl}}/pets/{{petId}}" || show.Request.Method != "GET" {
		t.Errorf("show = %+v / %+v", show, show.Request)
	}
	if h := show.Request.Headers; len(h) != 1 || h[0].Key != "X-Request-ID" || !h[0].Disabled {
		t.Errorf("optional header param = %+v", h)
	}

	list := find(t, res, "List all pets")
	p := map[string]model.KV{}
	for _, kv := range list.Request.Params {
		p[kv.Key] = kv
	}
	if !p["limit"].Disabled || p["status"].Disabled || p["status"].Value != "available" {
		t.Errorf("params = %+v", list.Request.Params)
	}

	create := find(t, res, "Create a pet")
	if create.Request.Body == nil || create.Request.Body.Type != model.BodyJSON ||
		!strings.Contains(create.Request.Body.Content, `"name": "doggie"`) ||
		!strings.Contains(create.Request.Body.Content, `"status": "available"`) ||
		!strings.Contains(create.Request.Body.Content, `"email": "user@example.com"`) {
		t.Errorf("body = %+v", create.Request.Body)
	}
	if create.Request.Docs != "Adds a pet to the store." {
		t.Errorf("docs = %q", create.Request.Docs)
	}

	del := find(t, res, "Delete a pet")
	if a := del.Request.Auth; a == nil || a.Type != model.AuthAPIKey || a.Key != "X-API-Key" || a.Value != "{{apiKey}}" {
		t.Errorf("per-op security = %+v", a)
	}
	inv := find(t, res, "Inventory")
	if inv.Folder != "" || inv.Request.Auth == nil || inv.Request.Auth.Type != model.AuthNone {
		t.Errorf("inventory = %+v auth=%+v", inv, inv.Request.Auth)
	}
	login := find(t, res, "Log in")
	if login.Folder != "auth" || login.Request.Body.Type != model.BodyForm || len(login.Request.Body.Form) != 2 {
		t.Errorf("login body = %+v", login.Request.Body)
	}
}

func TestSwagger2(t *testing.T) {
	res := load(t, "swagger2.json")
	golden(t, "swagger2.golden.yaml", res)
	if res.BaseURL != "https://legacy.example.com/api" {
		t.Errorf("baseUrl = %q", res.BaseURL)
	}
	r := find(t, res, "Update order")
	if r.Request.Method != "PUT" || r.Request.URL != "{{baseUrl}}/orders/{{id}}" || r.Folder != "orders" {
		t.Errorf("req = %+v", r.Request)
	}
	if r.Request.Body == nil || !strings.Contains(r.Request.Body.Content, `"qty": 2`) {
		t.Errorf("body = %+v", r.Request.Body)
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "not: [valid", `{"foo": "bar"}`} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestWriteToWorkspace(t *testing.T) {
	ws, err := store.Init(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	ws.Meta.Variables = []model.Variable{{Key: "baseUrl", Value: "keep-me"}}
	res := load(t, "petstore.yaml")
	folder, err := Write(ws, res)
	if err != nil {
		t.Fatal(err)
	}
	if folder != "swagger-petstore" {
		t.Errorf("folder = %q", folder)
	}
	tree, _ := ws.Tree()
	if len(tree.Children) != 1 || len(tree.Children[0].Children) != 3 { // auth/, pets/, Inventory
		t.Fatalf("tree = %+v", tree.Children[0])
	}
	f, _ := ws.LoadFolder(folder)
	if f.Auth == nil || f.Auth.Type != model.AuthBearer {
		t.Errorf("folder auth = %+v", f.Auth)
	}
	// Existing baseUrl isn't clobbered; the import uses a namespaced var.
	if ws.Meta.Variables[0].Value != "keep-me" {
		t.Errorf("globals = %+v", ws.Meta.Variables)
	}
	req, err := ws.LoadRequest(folder + "/pets/showpetbyid.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(req.URL, "{{swaggerPetstoreBaseUrl}}/pets/") {
		t.Errorf("url = %q", req.URL)
	}
	found := false
	for _, v := range ws.Meta.Variables {
		if v.Key == "swaggerPetstoreBaseUrl" && v.Value == "https://api.petstore.example.com/v1" {
			found = true
		}
	}
	if !found {
		t.Errorf("globals = %+v", ws.Meta.Variables)
	}
	// Second import goes to a new folder rather than overwriting.
	folder2, _ := Write(ws, res)
	if folder2 == folder {
		t.Error("reimport overwrote existing folder")
	}
}

// Environments override workspace variables, so a name defined in any
// environment can't be reused for the imported base URL.
func TestWriteAvoidsEnvironmentShadowing(t *testing.T) {
	ws, err := store.Init(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.SaveEnvironment(&model.Environment{Name: "dev", Variables: []model.Variable{{Key: "baseUrl", Value: "http://other"}}}); err != nil {
		t.Fatal(err)
	}
	folder, err := Write(ws, load(t, "petstore.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	req, _ := ws.LoadRequest(folder + "/inventory.yaml")
	if !strings.HasPrefix(req.URL, "{{swaggerPetstoreBaseUrl}}") {
		t.Errorf("url = %q; baseUrl is shadowed by env dev", req.URL)
	}
}
