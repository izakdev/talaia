package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
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

func (f fakeIntegration) ListEntities(context.Context) ([]integrations.EntityOption, error) {
	if f.cfg.Secret != "correcta" {
		return nil, &integrations.Error{Code: integrations.CodeUnauthorized, Err: errors.New("credencial")}
	}
	return []integrations.EntityOption{{ID: "light.salon", Name: "Salón", State: "on"}}, nil
}

func (f fakeIntegration) Collect(context.Context) (any, error) {
	if f.cfg.Secret != "correcta" {
		return nil, &integrations.Error{Code: integrations.CodeUnauthorized, Err: errors.New("credencial")}
	}
	return notedData{Queries: 7}, nil
}

// notedData simula datos con avisos que no impiden mostrar el servicio.
type notedData struct {
	Queries int `json:"queries"`
}

func (notedData) Notes() []string { return []string{"aviso"} }

// plainIntegration no ofrece entidades que elegir.
type plainIntegration struct{ cfg integrations.Config }

func (p plainIntegration) Collect(ctx context.Context) (any, error) {
	return fakeIntegration(p).Collect(ctx)
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
	fake := func(cfg integrations.Config) integrations.Integration { return fakeIntegration{cfg} }
	plain := func(cfg integrations.Config) integrations.Integration { return plainIntegration{cfg} }
	registry := integrations.Registry{"adguard": plain, "homeassistant": fake}
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
	for _, token := range []string{"sin-formato", "root@pam!sin-valor", "sin-reino!nombre=valor", "root@pam!a=con espacio"} {
		m := adguardConn(token)
		m["type"] = "proxmox"
		if got := c.want(400, "POST", "/api/admin/connections", m); !strings.Contains(got, "invalid_proxmox_token") {
			t.Fatalf("%s: %s", token, got)
		}
	}
	valid := adguardConn("talaia@pve!dashboard=1234-abcd")
	valid["type"] = "proxmox"
	valid["options"] = map[string]bool{"show_guests": true}
	c.want(201, "POST", "/api/admin/connections", valid)
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
	if !strings.Contains(body, `"link":"http://192.0.2.1:3000"`) {
		t.Fatalf("falta el enlace al servicio: %s", body)
	}
	for _, leak := range []string{"admin", "correcta", "base_url", "link_url", "username", "secret"} {
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

	proxmox := map[string]any{"name": "P", "type": "proxmox", "base_url": "http://192.0.2.2", "secret": "root@pam!talaia=valor", "interval_seconds": 30}
	if got := c.want(400, "POST", "/api/admin/connections/test", proxmox); !strings.Contains(got, "not_supported") {
		t.Fatal(got)
	}
}

func homeAssistantConn(secret string) map[string]any {
	return map[string]any{"name": "Casa", "type": "homeassistant", "base_url": "http://192.0.2.3:8123", "secret": secret, "interval_seconds": 30}
}

func TestListEntitiesEndpoint(t *testing.T) {
	c, _ := newServer(t)
	path := "/api/admin/connections/entities"
	newClient(t, c.base).want(401, "POST", path, homeAssistantConn("correcta"))
	c.want(200, "POST", "/api/setup", creds())

	if got := c.want(200, "POST", path, homeAssistantConn("correcta")); !strings.Contains(got, "light.salon") {
		t.Fatal(got)
	}
	if got := c.want(200, "POST", path, homeAssistantConn("mala")); !strings.Contains(got, `"error":"unauthorized"`) {
		t.Fatal(got)
	}
	if got := c.want(400, "POST", path, adguardConn("correcta")); !strings.Contains(got, "not_supported") {
		t.Fatal(got)
	}

	var out connectionOut
	if err := json.Unmarshal([]byte(c.want(201, "POST", "/api/admin/connections", homeAssistantConn("correcta"))), &out); err != nil {
		t.Fatal(err)
	}
	stored := homeAssistantConn("correcta")
	delete(stored, "secret")
	stored["id"] = out.ID
	if got := c.want(200, "POST", path, stored); !strings.Contains(got, "light.salon") {
		t.Fatalf("debe usar la credencial guardada: %s", got)
	}
	stored["base_url"] = "http://198.51.100.9"
	c.want(400, "POST", path, stored)
}

func TestEntitySelectionValidation(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	create := "/api/admin/connections"
	many := make([]string, maxEntities+1)
	for i := range many {
		many[i] = fmt.Sprintf("sensor.s%d", i)
	}
	cases := map[string][]string{
		"formato":     {"no valida"},
		"sin dominio": {"salon"},
		"mayúsculas":  {"Light.Salon"},
		"repetida":    {"light.a", "light.a"},
		"demasiadas":  many,
	}
	for name, entities := range cases {
		t.Run(name, func(t *testing.T) {
			m := homeAssistantConn("correcta")
			m["entities"] = entities
			c.want(400, "POST", create, m)
		})
	}
	wrongType := adguardConn("correcta")
	wrongType["entities"] = []string{"light.salon"}
	c.want(400, "POST", create, wrongType)

	valid := homeAssistantConn("correcta")
	valid["entities"] = []string{"light.salon", "sensor.casa_potencia"}
	if got := c.want(201, "POST", create, valid); !strings.Contains(got, `"entities":["light.salon","sensor.casa_potencia"]`) {
		t.Fatal(got)
	}
}

func TestConnectionLink(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	for _, bad := range []string{"javascript:alert(1)", "data:text/html,x", "ftp://host", "http://usuario:clave@host", "sin-esquema"} {
		m := adguardConn("correcta")
		m["link_url"] = bad
		if got := c.want(400, "POST", "/api/admin/connections", m); !strings.Contains(got, "invalid_link") {
			t.Fatalf("%s: %s", bad, got)
		}
	}
	m := adguardConn("correcta")
	m["link_url"] = "https://adguard.example/panel?vista=1"
	if got := c.want(201, "POST", "/api/admin/connections", m); !strings.Contains(got, `"link_url":"https://adguard.example/panel?vista=1"`) {
		t.Fatal(got)
	}
	body := waitDashboard(t, newClient(t, c.base), `"status":"ok"`)
	if !strings.Contains(body, `"link":"https://adguard.example/panel?vista=1"`) || strings.Contains(body, "192.0.2.1") {
		t.Fatalf("con enlace propio no debe mostrarse la URL de conexión: %s", body)
	}
}

func TestConnectionTestReportsNotes(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	if got := c.want(200, "POST", "/api/admin/connections/test", homeAssistantConn("correcta")); !strings.Contains(got, `"notes":["aviso"]`) {
		t.Fatal(got)
	}
}

func panelOf(t *testing.T, c *client) []store.Page {
	t.Helper()
	var out struct {
		Pages []store.Page `json:"pages"`
	}
	if err := json.Unmarshal([]byte(c.want(200, "GET", "/api/admin/panel", nil)), &out); err != nil {
		t.Fatal(err)
	}
	return out.Pages
}

func TestPanelSaveAndValidation(t *testing.T) {
	c, _ := newServer(t)
	newClient(t, c.base).want(401, "GET", "/api/admin/panel", nil)
	c.want(200, "POST", "/api/setup", creds())
	conn := createCorrect(t, c)
	second := createCorrect(t, c)
	pages := panelOf(t, c)
	if len(pages) != 1 || pages[0].Name != "Inicio" || len(pages[0].Widgets) != 2 {
		t.Fatalf("panel inicial: %+v", pages)
	}

	good := func() []store.Page {
		return []store.Page{{ID: "a", Name: "Una", Widgets: []store.Widget{
			{ID: "w1", ConnectionID: conn.ID, X: 0, Y: 0, W: 6, H: 4},
			{ID: "w2", ConnectionID: second.ID, X: 6, Y: 0, W: 6, H: 4},
		}}}
	}
	cases := map[string]struct {
		mutate func([]store.Page) []store.Page
		code   string
	}{
		"sin páginas":    {func([]store.Page) []store.Page { return nil }, "invalid_pages"},
		"nombre vacío":   {func(p []store.Page) []store.Page { p[0].Name = " "; return p }, "invalid_page_name"},
		"id no válido":   {func(p []store.Page) []store.Page { p[0].ID = "a b"; return p }, "invalid_pages"},
		"conexión ajena": {func(p []store.Page) []store.Page { p[0].Widgets[0].ConnectionID = "otra"; return p }, "invalid_widget"},
		"se sale":        {func(p []store.Page) []store.Page { p[0].Widgets[1].X = 8; return p }, "invalid_widget"},
		"muy estrecho":   {func(p []store.Page) []store.Page { p[0].Widgets[0].W = 1; return p }, "invalid_widget"},
		"solapados":      {func(p []store.Page) []store.Page { p[0].Widgets[1].X = 5; p[0].Widgets[1].W = 6; return p }, "widgets_overlap"},
		"repetida":       {func(p []store.Page) []store.Page { p[0].Widgets[1].ConnectionID = conn.ID; return p }, "invalid_widget"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := c.want(400, "PUT", "/api/admin/panel", map[string]any{"pages": tc.mutate(good())})
			if !strings.Contains(got, tc.code) {
				t.Fatal(got)
			}
		})
	}
	c.want(200, "PUT", "/api/admin/panel", map[string]any{"pages": good()})
	if pages = panelOf(t, c); pages[0].Name != "Una" || pages[0].Widgets[1].X != 6 {
		t.Fatalf("panel guardado: %+v", pages)
	}
}

func TestUnplacedConnectionIsHidden(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	conn := createCorrect(t, c)
	waitDashboard(t, c, `"status":"ok"`)
	empty := []store.Page{{ID: "a", Name: "Vacía", Widgets: []store.Widget{}}}
	c.want(200, "PUT", "/api/admin/panel", map[string]any{"pages": empty})
	if got := c.want(200, "GET", "/api/dashboard", nil); strings.Contains(got, conn.ID) || !strings.Contains(got, `"connections":[]`) {
		t.Fatalf("una conexión fuera del panel no debe publicarse: %s", got)
	}
	if got := c.want(200, "GET", "/api/admin/status", nil); !strings.Contains(got, conn.ID) {
		t.Fatalf("la administración debe ver todas las conexiones: %s", got)
	}
	newClient(t, c.base).want(401, "GET", "/api/admin/status", nil)
}

func TestDashboardCanRequireLogin(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	anonymous := newClient(t, c.base)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", c.base+"/api/dashboard/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	lines := bufio.NewScanner(resp.Body)
	if !lines.Scan() {
		t.Fatal("sin evento inicial")
	}

	settings := map[string]any{"theme": "dark", "accent": "teal", "require_login": true}
	c.want(200, "PUT", "/api/admin/settings", settings)

	// El flujo anónimo que ya estaba abierto debe cerrarse.
	for lines.Scan() {
	}
	if ctx.Err() != nil {
		t.Fatal("el flujo anónimo sigue abierto")
	}
	anonymous.want(401, "GET", "/api/dashboard", nil)
	anonymous.want(401, "GET", "/api/dashboard/events", nil)
	c.want(200, "GET", "/api/dashboard", nil)
	state := anonymous.want(200, "GET", "/api/state", nil)
	for _, want := range []string{`"require_login":true`, `"theme":"dark"`, `"accent":"teal"`} {
		if !strings.Contains(state, want) {
			t.Fatalf("falta %s: %s", want, state)
		}
	}
}

func TestSettingsValidation(t *testing.T) {
	c, _ := newServer(t)
	newClient(t, c.base).want(401, "PUT", "/api/admin/settings", map[string]any{"theme": "dark", "accent": "teal"})
	c.want(200, "POST", "/api/setup", creds())
	for _, bad := range []map[string]any{
		{"theme": "otro", "accent": "teal"},
		{"theme": "dark", "accent": "url(x)"},
	} {
		c.want(400, "PUT", "/api/admin/settings", bad)
	}
}

func TestChangePassword(t *testing.T) {
	c, _ := newServer(t)
	path := "/api/admin/password"
	newClient(t, c.base).want(401, "POST", path, map[string]string{"current": password, "new": "otra-contraseña-larga"})
	c.want(200, "POST", "/api/setup", creds())
	other := newClient(t, c.base)
	other.want(200, "POST", "/api/auth/login", creds())

	c.want(400, "POST", path, map[string]string{"current": "incorrecta-larga", "new": "otra-contraseña-larga"})
	c.want(400, "POST", path, map[string]string{"current": password, "new": "corta"})
	c.want(204, "POST", path, map[string]string{"current": password, "new": "otra-contraseña-larga"})

	c.want(200, "GET", "/api/admin/connections", nil)
	other.want(401, "GET", "/api/admin/connections", nil)
	fresh := newClient(t, c.base)
	fresh.want(401, "POST", "/api/auth/login", creds())
	fresh.want(200, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": "otra-contraseña-larga"})
}

func newServerWithSetupToken(t *testing.T, token string) (*client, string) {
	t.Helper()
	box, err := secrets.New(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := collector.New(st, integrations.Registry{}, logger)
	ctx, cancel := context.WithCancel(context.Background())
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	srv, err := New(st, manager, integrations.Registry{}, logger, "test", fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "setup.token")
	if err := os.WriteFile(file, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv.SetupTokenFile = file
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { cancel(); ts.Close(); manager.Wait(); st.Close() })
	return newClient(t, ts.URL), file
}

func TestSetupRequiresTokenWhenConfigured(t *testing.T) {
	c, file := newServerWithSetupToken(t, "codigo-secreto")
	if got := c.want(200, "GET", "/api/state", nil); !strings.Contains(got, `"setup_token_required":true`) || strings.Contains(got, "codigo-secreto") {
		t.Fatal(got)
	}
	c.want(403, "POST", "/api/setup", creds())
	bad := creds()
	bad["token"] = "incorrecto"
	c.want(403, "POST", "/api/setup", bad)
	if got := c.want(200, "GET", "/api/state", nil); !strings.Contains(got, `"setup_required":true`) {
		t.Fatalf("un intento fallido no debe crear el administrador: %s", got)
	}
	good := creds()
	good["token"] = "codigo-secreto"
	c.want(200, "POST", "/api/setup", good)
	if _, err := os.Stat(file); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("el código debe eliminarse tras usarlo: %v", err)
	}
	if got := c.want(200, "GET", "/api/state", nil); !strings.Contains(got, `"setup_token_required":false`) {
		t.Fatal(got)
	}
}

func TestSetupTokenAttemptsAreLimited(t *testing.T) {
	c, _ := newServerWithSetupToken(t, "codigo-secreto")
	bad := creds()
	bad["token"] = "incorrecto"
	for i := 0; i < maxFailures; i++ {
		c.want(403, "POST", "/api/setup", bad)
	}
	good := creds()
	good["token"] = "codigo-secreto"
	c.want(429, "POST", "/api/setup", good)
}

func TestEmptySetupTokenNeverMatches(t *testing.T) {
	c, _ := newServerWithSetupToken(t, "")
	c.want(403, "POST", "/api/setup", creds())
}

func TestPastedTokenIsTrimmed(t *testing.T) {
	c, st := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	if got := c.want(200, "POST", "/api/admin/connections/entities", homeAssistantConn(" correcta\n")); !strings.Contains(got, "light.salon") {
		t.Fatal(got)
	}
	var out connectionOut
	if err := json.Unmarshal([]byte(c.want(201, "POST", "/api/admin/connections", homeAssistantConn("correcta\n"))), &out); err != nil {
		t.Fatal(err)
	}
	if got, err := st.ConnectionSecret(t.Context(), out.ID); err != nil || got != "correcta" {
		t.Fatalf("secreto = %q, err = %v", got, err)
	}
	c.want(400, "POST", "/api/admin/connections", homeAssistantConn("dos\nlineas"))
}
