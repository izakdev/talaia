package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/izakdev/talaia/internal/collector"
	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/secrets"
	"github.com/izakdev/talaia/internal/store"
)

const password = "una-contraseña-larga"

// fakeIntegration acepta solo la credencial "correcta".
type fakeIntegration struct{ cfg integrations.Config }

func (f fakeIntegration) Collect(context.Context) (any, error) {
	if f.cfg.Secret != "correcta" {
		return nil, &integrations.Error{Code: integrations.CodeUnauthorized, Err: errors.New("credencial")}
	}
	return map[string]int{"queries": 7}, nil
}

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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := integrations.Registry{"adguard": func(cfg integrations.Config) integrations.Integration { return fakeIntegration{cfg} }}
	manager := collector.New(st, registry, logger)
	ctx, cancel := context.WithCancel(context.Background())
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	srv, err := New(st, manager, registry, logger, "test", web)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(srv.Handler())
	ts.Config.BaseContext = func(net.Listener) context.Context { return ctx }
	ts.Start()
	t.Cleanup(func() { cancel(); ts.Close(); manager.Wait(); st.Close() })
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

func adguardConn(secret string) map[string]any {
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
	body := c.want(201, "POST", "/api/admin/connections", adguardConn(secret))
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
			m := adguardConn("s")
			mutate(m)
			c.want(400, "POST", "/api/admin/connections", m)
		})
	}
	unknown := adguardConn("s")
	unknown["options"] = map[string]bool{"otra": true}
	c.want(400, "POST", "/api/admin/connections", unknown)
	m := adguardConn("sin-formato")
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

func createCorrect(t *testing.T, c *client) connectionOut {
	t.Helper()
	body := c.want(201, "POST", "/api/admin/connections", adguardConn("correcta"))
	var out connectionOut
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func waitDashboard(t *testing.T, c *client, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if body := c.want(200, "GET", "/api/dashboard", nil); strings.Contains(body, want) {
			return body
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("el dashboard no contiene %q", want)
	return ""
}

func TestDashboardIsPublicAndHidesConfiguration(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	createCorrect(t, c)
	anonymous := newClient(t, c.base)
	body := waitDashboard(t, anonymous, `"status":"ok"`)
	if !strings.Contains(body, `"queries":7`) {
		t.Fatalf("sin datos: %s", body)
	}
	for _, leak := range []string{"192.0.2.1", "admin", "correcta", "base_url", "username", "secret"} {
		if strings.Contains(body, leak) {
			t.Fatalf("el dashboard público filtra %q: %s", leak, body)
		}
	}
}

func TestConnectionOptionsRoundTrip(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	m := adguardConn("correcta")
	m["options"] = map[string]bool{"show_top_domains": true}
	if got := c.want(201, "POST", "/api/admin/connections", m); !strings.Contains(got, `"show_top_domains":true`) {
		t.Fatal(got)
	}
}

func TestMultipleConnectionsOfSameService(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	for _, name := range []string{"AdGuard A", "AdGuard B"} {
		m := adguardConn("correcta")
		m["name"] = name
		c.want(201, "POST", "/api/admin/connections", m)
	}
	body := waitDashboard(t, c, "AdGuard B")
	if strings.Index(body, "AdGuard A") < 0 || strings.Index(body, "AdGuard A") > strings.Index(body, "AdGuard B") {
		t.Fatalf("faltan conexiones o no están ordenadas: %s", body)
	}
}

func TestDashboardFollowsConnectionChanges(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	out := createCorrect(t, c)
	waitDashboard(t, c, `"status":"ok"`)
	path := "/api/admin/connections/" + out.ID
	update := map[string]any{"name": "AdGuard", "base_url": out.BaseURL, "username": "admin", "secret": "mala", "interval_seconds": 30}
	c.want(200, "PUT", path, update)
	if body := waitDashboard(t, c, `"error":"unauthorized"`); !strings.Contains(body, `"queries":7`) {
		t.Fatalf("debe conservar el último dato: %s", body)
	}
	c.want(204, "DELETE", path, nil)
	if body := c.want(200, "GET", "/api/dashboard", nil); !strings.Contains(body, `"connections":[]`) {
		t.Fatalf("sigue la conexión eliminada: %s", body)
	}
}

func TestDashboardEvents(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", c.base+"/api/dashboard/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %s", ct)
	}
	lines := bufio.NewScanner(resp.Body)
	next := func(want string) {
		t.Helper()
		for lines.Scan() {
			if strings.Contains(lines.Text(), want) {
				return
			}
		}
		t.Fatalf("evento %q no recibido: %v", want, lines.Err())
	}
	next(`"connections":[]`)
	createCorrect(t, c)
	next(`"status":"ok"`)
}

func TestConnectionTest(t *testing.T) {
	c, _ := newServer(t)
	body := func(secret any) map[string]any {
		m := adguardConn("")
		delete(m, "secret")
		if secret != nil {
			m["secret"] = secret
		}
		return m
	}
	anonymous := newClient(t, c.base)
	c.want(200, "POST", "/api/setup", creds())
	anonymous.want(401, "POST", "/api/admin/connections/test", body("correcta"))

	if got := c.want(200, "POST", "/api/admin/connections/test", body("correcta")); !strings.Contains(got, `"ok":true`) {
		t.Fatal(got)
	}
	if got := c.want(200, "POST", "/api/admin/connections/test", body("mala")); !strings.Contains(got, `"error":"unauthorized"`) {
		t.Fatal(got)
	}
	c.want(400, "POST", "/api/admin/connections/test", body(nil))
	if got := c.want(200, "GET", "/api/admin/connections", nil); got != "[]\n" {
		t.Fatalf("una prueba no debe guardar nada: %s", got)
	}

	out := createCorrect(t, c)
	stored := body(nil)
	stored["id"] = out.ID
	stored["base_url"] = out.BaseURL
	if got := c.want(200, "POST", "/api/admin/connections/test", stored); !strings.Contains(got, `"ok":true`) {
		t.Fatalf("debe usar la credencial guardada: %s", got)
	}
	stored["base_url"] = "http://198.51.100.9"
	if got := c.want(400, "POST", "/api/admin/connections/test", stored); !strings.Contains(got, "secret_required_for_url_change") {
		t.Fatalf("no debe enviar la credencial guardada a otra URL: %s", got)
	}

	proxmox := map[string]any{"name": "P", "type": "proxmox", "base_url": "http://192.0.2.2", "secret": "a!b=c", "interval_seconds": 30}
	if got := c.want(400, "POST", "/api/admin/connections/test", proxmox); !strings.Contains(got, "not_supported") {
		t.Fatal(got)
	}
}
