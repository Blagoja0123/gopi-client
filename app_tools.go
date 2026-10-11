package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"apiclient/internal/importer/openapi"
	"apiclient/internal/runner/httprunner"
	"apiclient/internal/webhook"
)

// ---- OpenAPI import ----

func (a *App) importOpenAPI(data []byte) (string, error) {
	ws, err := a.workspace()
	if err != nil {
		return "", err
	}
	res, err := openapi.Parse(data)
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return openapi.Write(ws, res)
}

// ImportOpenAPIFile imports a local spec and returns the created folder.
func (a *App) ImportOpenAPIFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return a.importOpenAPI(b)
}

func (a *App) ImportOpenAPIURL(url string) (string, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(url), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch spec: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return "", err
	}
	return a.importOpenAPI(b)
}

// ---- Webhooks ----

type WebhookStatus struct {
	Running bool   `json:"running"`
	Addr    string `json:"addr"`
}

// WebhookStart listens on addr, e.g. "127.0.0.1:8090". Captures arrive as
// "webhook:capture" events.
func (a *App) WebhookStart(addr string) (WebhookStatus, error) {
	if _, err := a.receiver.Start(addr); err != nil {
		return WebhookStatus{}, err
	}
	return a.WebhookGetStatus(), nil
}

func (a *App) WebhookStop() error { return a.receiver.Stop() }

func (a *App) WebhookGetStatus() WebhookStatus {
	running, addr := a.receiver.Status()
	return WebhookStatus{Running: running, Addr: addr}
}

func (a *App) WebhookCaptures() []webhook.Capture { return a.receiver.Captures() }
func (a *App) WebhookClear()                      { a.receiver.Clear() }
func (a *App) WebhookSetResponse(r webhook.Response) {
	a.receiver.SetResponse(r)
}

func (a *App) WebhookReplay(id, target string) (*httprunner.Response, error) {
	c, ok := a.receiver.Get(id)
	if !ok {
		return nil, errInvalid("capture not found")
	}
	r, _, _, err := a.resolve("", webhook.ToRequest(c, target))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	return httprunner.Execute(ctx, r, httprunner.Options{})
}

// WebhookSaveCapture stores a capture as a regular request in the workspace
// so the payload can be re-sent later (and committed as a fixture).
func (a *App) WebhookSaveCapture(id, folder, baseURL string) (string, error) {
	ws, err := a.workspace()
	if err != nil {
		return "", err
	}
	c, ok := a.receiver.Get(id)
	if !ok {
		return "", errInvalid("capture not found")
	}
	if baseURL == "" {
		baseURL = "{{webhookTarget}}"
	}
	return ws.AddRequest(folder, webhook.ToRequest(c, baseURL))
}

type Signature struct {
	Header string `json:"header"`
	Value  string `json:"value"`
}

// SignPayload computes a webhook signature. preset: "github", "stripe", or
// "hmac-sha1" / "hmac-sha256" / "hmac-sha512" (hex digest).
func (a *App) SignPayload(preset, secret, payload string) (Signature, error) {
	switch preset {
	case "github":
		return Signature{"X-Hub-Signature-256", webhook.GitHubSignature(secret, []byte(payload))}, nil
	case "stripe":
		return Signature{"Stripe-Signature", webhook.StripeSignature(secret, []byte(payload), time.Now())}, nil
	}
	algo, ok := strings.CutPrefix(preset, "hmac-")
	if !ok {
		return Signature{}, fmt.Errorf("unknown preset %q", preset)
	}
	v, err := webhook.Sign(algo, secret, []byte(payload))
	return Signature{"X-Signature", v}, err
}

// VerifySignature checks a captured webhook against a secret.
func (a *App) VerifySignature(preset, secret, captureID string) (bool, error) {
	c, ok := a.receiver.Get(captureID)
	if !ok {
		return false, errInvalid("capture not found")
	}
	body := []byte(c.Body)
	switch preset {
	case "github":
		return webhook.VerifyGitHub(secret, body, c.Header("X-Hub-Signature-256")), nil
	case "stripe":
		err := webhook.VerifyStripe(secret, body, c.Header("Stripe-Signature"), 5*time.Minute, time.Now())
		return err == nil, nil
	}
	return false, fmt.Errorf("verify supports github and stripe presets")
}
