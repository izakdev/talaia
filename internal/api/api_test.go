package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/izakdev/talaia/internal/secrets"
	"github.com/izakdev/talaia/internal/store"
)

const password = "una-contraseña-larga"

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newServer(t *testing.T) (*client, *store.Store) {
	t.Helper()
	box, err := secrets.New(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	web := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	srv, err := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", web)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close(); st.Close() })
	return newClient(t, ts.URL), st
}

func newClient(t *testing.T, base string) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t, base, &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any) (int, string) {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (c *client) want(status int, method, path string, body any) string {
	c.t.Helper()
	got, resp := c.do(method, path, body)
	if got != status {
		c.t.Fatalf("%s %s = %d, quería %d: %s", method, path, got, status, resp)
	}
	return resp
}

func creds() map[string]string { return map[string]string{"username": "admin", "password": password} }

func adguard(secret string) map[string]any {
	return map[string]any{"name": "AdGuard", "type": "adguard", "base_url": "http://192.0.2.1:3000/", "username": "admin", "secret": secret, "interval_seconds": 30}
}

func TestSetupOnlyOnceAndLogin(t *testing.T) {
	c, _ := newServer(t)
	if !strings.Contains(c.want(200, "GET", "/api/state", nil), `"setup_required":true`) {
		t.Fatal("debe pedir configuración inicial")
	}
	c.want(400, "POST", "/api/setup", map[string]string{"username": "admin", "password": "corta"})
	c.want(200, "POST", "/api/setup", creds())
	c.want(409, "POST", "/api/setup", map[string]string{"username": "otro", "password": password})
	if !strings.Contains(c.want(200, "GET", "/api/state", nil), `"user":"admin"`) {
		t.Fatal("el asistente debe iniciar sesión")
	}
	c.want(204, "POST", "/api/auth/logout", nil)
	c.want(401, "GET", "/api/admin/connections", nil)
	c.want(401, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": "incorrecta-larga"})
	c.want(200, "POST", "/api/auth/login", creds())
	c.want(200, "GET", "/api/admin/connections", nil)
}

func TestLogoutRevokesSessionOnServer(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	var saved []*http.Cookie
	req, _ := http.NewRequest("GET", c.base+"/api/state", nil)
	saved = c.http.Jar.Cookies(req.URL)
	c.want(204, "POST", "/api/auth/logout", nil)
	stolen := newClient(t, c.base)
	stolen.http.Jar.SetCookies(req.URL, saved)
	stolen.want(401, "GET", "/api/admin/connections", nil)
}

func TestLoginLimiter(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	bad := map[string]string{"username": "admin", "password": "incorrecta-larga"}
	for i := 0; i < maxFailures; i++ {
		c.want(401, "POST", "/api/auth/login", bad)
	}
	c.want(429, "POST", "/api/auth/login", creds())
}

func TestConnectionsNeverExposeSecret(t *testing.T) {
	c, st := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	const secret = "valor-secreto-123"
	body := c.want(201, "POST", "/api/admin/connections", adguard(secret))
	var out connectionOut
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	if out.BaseURL != "http://192.0.2.1:3000" {
		t.Fatalf("URL = %s", out.BaseURL)
	}
	for _, resp := range []string{body, c.want(200, "GET", "/api/admin/connections", nil), c.want(200, "GET", "/api/admin/connections/"+out.ID, nil)} {
		if strings.Contains(resp, secret) || strings.Contains(resp, `"secret"`) {
			t.Fatalf("respuesta con secreto: %s", resp)
		}
	}

	path := "/api/admin/connections/" + out.ID
	same := map[string]any{"name": "Renombrada", "base_url": out.BaseURL, "username": "admin", "interval_seconds": 60}
	c.want(200, "PUT", path, same)
	if got, err := st.ConnectionSecret(t.Context(), out.ID); err != nil || got != secret {
		t.Fatalf("secreto = %q, %v", got, err)
	}

	moved := map[string]any{"name": "Renombrada", "base_url": "http://198.51.100.9", "username": "admin", "interval_seconds": 60}
	c.want(400, "PUT", path, moved)
	moved["secret"] = "nuevo"
	c.want(200, "PUT", path, moved)

	c.want(204, "DELETE", path, nil)
	c.want(404, "GET", path, nil)
}

func TestConnectionValidation(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	cases := map[string]func(map[string]any){
		"tipo":      func(m map[string]any) { m["type"] = "otro" },
		"esquema":   func(m map[string]any) { m["base_url"] = "file:///etc/passwd" },
		"userinfo":  func(m map[string]any) { m["base_url"] = "http://a:b@host" },
		"usuario":   func(m map[string]any) { m["username"] = "" },
		"intervalo": func(m map[string]any) { m["interval_seconds"] = 1 },
		"secreto":   func(m map[string]any) { delete(m, "secret") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := adguard("s")
			mutate(m)
			c.want(400, "POST", "/api/admin/connections", m)
		})
	}
	m := adguard("sin-formato")
	m["type"] = "proxmox"
	c.want(400, "POST", "/api/admin/connections", m)
}

func TestWritesRequireJSONAndSameOrigin(t *testing.T) {
	c, _ := newServer(t)
	req, _ := http.NewRequest("POST", c.base+"/api/setup", strings.NewReader("username=a"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("formulario = %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", c.base+"/api/setup", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	resp, err = c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("otro origen = %d", resp.StatusCode)
	}
}

func TestWebFallback(t *testing.T) {
	c, _ := newServer(t)
	if body := c.want(200, "GET", "/admin/connections", nil); !strings.Contains(body, "app") {
		t.Fatalf("SPA = %s", body)
	}
	c.want(404, "GET", "/assets/no-existe.js", nil)
	c.want(404, "GET", "/api/no-existe", nil)
}
