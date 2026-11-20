package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// hookServer recibe avisos y guarda los cuerpos.
type hookServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []string
}

func newHook(t *testing.T, status int) *hookServer {
	t.Helper()
	h := &hookServer{}
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		h.mu.Lock()
		h.bodies = append(h.bodies, r.URL.Path+" "+string(b))
		h.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(h.Close)
	return h
}

func (h *hookServer) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.bodies)
}

func loggedIn(t *testing.T) *client {
	t.Helper()
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	return c
}

func TestChannelsNeverExposeSecrets(t *testing.T) {
	c := loggedIn(t)
	hook := newHook(t, 200)
	tg := newHook(t, 200)
	t.Setenv("TALAIA_TELEGRAM_API", tg.URL)
	secrets := []string{"123456:token-del-bot", "topico-secreto", "tk_acceso", hook.URL + "/ruta-secreta"}
	var ids []string
	for _, in := range []map[string]any{
		{"name": "Móvil", "type": "telegram", "enabled": true, "chat_id": "-1001", "token": secrets[0], "language": "en"},
		{"name": "ntfy", "type": "ntfy", "enabled": true, "server": hook.URL, "topic": secrets[1], "token": secrets[2]},
		{"name": "Hook", "type": "webhook", "enabled": true, "url": secrets[3]},
	} {
		body := c.want(201, "POST", "/api/admin/notifications/channels", in)
		var out map[string]any
		_ = json.Unmarshal([]byte(body), &out)
		ids = append(ids, out["id"].(string))
		if out["name"] != in["name"] || out["type"] != in["type"] || out["enabled"] != true {
			t.Fatalf("alta = %s", body)
		}
	}
	responses := []string{c.want(200, "GET", "/api/admin/notifications/channels", nil)}
	for _, id := range ids {
		responses = append(responses,
			c.want(200, "PUT", "/api/admin/notifications/channels/"+id, map[string]any{"name": "Editado", "enabled": true, "chat_id": "-1001", "server": hook.URL}),
			c.want(200, "POST", "/api/admin/notifications/channels/"+id+"/test", nil))
	}
	responses = append(responses,
		c.want(200, "GET", "/api/admin/notifications/log", nil),
		c.want(200, "GET", "/api/admin/settings", nil))
	for _, r := range responses {
		for _, secret := range secrets {
			if strings.Contains(r, secret) || strings.Contains(r, "ruta-secreta") {
				t.Fatalf("la respuesta deja ver un secreto: %s", r)
			}
		}
	}
	if !strings.Contains(responses[0], `"chat_id":"-1001"`) {
		t.Fatalf("lista = %s", responses[0])
	}
	// Las pruebas han llegado con el secreto guardado, aunque la edición los dejara en blanco.
	if tg.count() != 1 || hook.count() != 2 {
		t.Fatalf("telegram = %d, hook = %d", tg.count(), hook.count())
	}
	if !strings.Contains(c.want(200, "GET", "/api/admin/settings", nil), `"notify_language":"en"`) {
		t.Fatal("el primer canal no fija el idioma de los avisos")
	}
	c.want(204, "DELETE", "/api/admin/notifications/channels/"+ids[0], nil)
	c.want(404, "DELETE", "/api/admin/notifications/channels/"+ids[0], nil)
}

func TestChannelValidation(t *testing.T) {
	c := loggedIn(t)
	for _, in := range []map[string]any{
		{"name": "", "type": "webhook", "url": "http://x"},
		{"name": strings.Repeat("x", 61), "type": "webhook", "url": "http://x"},
		{"name": "a", "type": "correo", "url": "http://x"},
		{"name": "a", "type": "webhook", "url": "ftp://x"},
		{"name": "a", "type": "webhook"},
		{"name": "a", "type": "telegram", "chat_id": "1", "token": "sin-dos-puntos"},
		{"name": "a", "type": "telegram", "chat_id": "nombre", "token": "1:abc"},
		{"name": "a", "type": "ntfy", "server": "http://x", "topic": "con espacios"},
		{"name": "a", "type": "ntfy", "server": "ntfy.sh", "topic": "t"},
	} {
		if got, body := c.do("POST", "/api/admin/notifications/channels", in); got != 400 || !strings.Contains(body, "invalid_channel") {
			t.Errorf("%v: %d %s", in, got, body)
		}
	}
	// ntfy sin servidor usa ntfy.sh; un chat público de Telegram es válido.
	c.want(201, "POST", "/api/admin/notifications/channels", map[string]any{"name": "a", "type": "ntfy", "topic": "talaia_1"})
	c.want(201, "POST", "/api/admin/notifications/channels", map[string]any{"name": "b", "type": "telegram", "chat_id": "@canal_avisos", "token": "1:abc"})
}

func TestTestChannelUsesStoredSecret(t *testing.T) {
	c := loggedIn(t)
	hook := newHook(t, 200)
	body := c.want(201, "POST", "/api/admin/notifications/channels", map[string]any{"name": "Hook", "type": "webhook", "enabled": true, "url": hook.URL + "/guardada"})
	var out map[string]any
	_ = json.Unmarshal([]byte(body), &out)
	// Una prueba sin guardar, con el id y la URL en blanco, usa la guardada.
	res := c.want(200, "POST", "/api/admin/notifications/test", map[string]any{"id": out["id"], "name": "Hook", "type": "webhook", "url": ""})
	if !strings.Contains(res, `"result":"ok"`) || hook.count() != 1 || !strings.HasPrefix(hook.bodies[0], "/guardada ") {
		t.Fatalf("prueba = %s, recibidos = %v", res, hook.bodies)
	}
	// Una prueba sin guardar y sin id no necesita ningún canal.
	failing := newHook(t, 401)
	res = c.want(200, "POST", "/api/admin/notifications/test", map[string]any{"name": "Nuevo", "type": "webhook", "url": failing.URL})
	if !strings.Contains(res, `"result":"error"`) || !strings.Contains(res, `"code":"unauthorized"`) {
		t.Fatalf("prueba = %s", res)
	}
	c.want(400, "POST", "/api/admin/notifications/test", map[string]any{"name": "Nuevo", "type": "webhook"})
	log := c.want(200, "GET", "/api/admin/notifications/log", nil)
	if strings.Count(log, `"event":"test"`) != 2 {
		t.Fatalf("registro = %s", log)
	}
}

func TestNotifySettingsValidation(t *testing.T) {
	c := loggedIn(t)
	ok := map[string]any{"theme": "auto", "accent": "amber", "notify_language": "en", "notify_delay_seconds": 300}
	c.want(200, "PUT", "/api/admin/settings", ok)
	for _, bad := range []map[string]any{
		{"theme": "auto", "accent": "amber", "notify_language": "fr", "notify_delay_seconds": 300},
		{"theme": "auto", "accent": "amber", "notify_language": "es", "notify_delay_seconds": 45},
	} {
		c.want(400, "PUT", "/api/admin/settings", bad)
	}
}

func TestConnectionNotifyRoundTrip(t *testing.T) {
	c := loggedIn(t)
	body := c.want(201, "POST", "/api/admin/connections", adguardConn("clave"))
	var out map[string]any
	_ = json.Unmarshal([]byte(body), &out)
	if out["notify"] != true {
		t.Fatalf("por defecto avisa: %s", body)
	}
	in := adguardConn("")
	in["notify"] = false
	body = c.want(200, "PUT", "/api/admin/connections/"+out["id"].(string), in)
	if !strings.Contains(body, `"notify":false`) {
		t.Fatalf("edición = %s", body)
	}
}

func TestNotifyRoutesRequireAuth(t *testing.T) {
	c, _ := newServer(t)
	for _, r := range [][2]string{
		{"GET", "/api/admin/notifications/channels"}, {"POST", "/api/admin/notifications/channels"},
		{"PUT", "/api/admin/notifications/channels/x"}, {"DELETE", "/api/admin/notifications/channels/x"},
		{"POST", "/api/admin/notifications/channels/x/test"}, {"POST", "/api/admin/notifications/test"},
		{"GET", "/api/admin/notifications/log"},
	} {
		if got, _ := c.do(r[0], r[1], map[string]any{}); got != 401 {
			t.Errorf("%s %s = %d", r[0], r[1], got)
		}
	}
}

// Cambiar el servidor de ntfy obliga a escribir otra vez el tópico y el token,
// para no enviar los guardados a otro servidor.
func TestNtfyServerChangeRequiresSecrets(t *testing.T) {
	c := loggedIn(t)
	original := newHook(t, 200)
	other := newHook(t, 200)
	body := c.want(201, "POST", "/api/admin/notifications/channels", map[string]any{"name": "ntfy", "type": "ntfy", "enabled": true, "server": original.URL, "topic": "topico_guardado", "token": "tk_guardado"})
	var out map[string]any
	_ = json.Unmarshal([]byte(body), &out)
	id := out["id"].(string)
	moved := map[string]any{"name": "ntfy", "enabled": true, "server": other.URL}
	if got, resp := c.do("PUT", "/api/admin/notifications/channels/"+id, moved); got != 400 || !strings.Contains(resp, "secret_required_for_url_change") {
		t.Fatalf("editar = %d %s", got, resp)
	}
	test := map[string]any{"id": id, "name": "ntfy", "type": "ntfy", "server": other.URL}
	if got, resp := c.do("POST", "/api/admin/notifications/test", test); got != 400 || !strings.Contains(resp, "secret_required_for_url_change") {
		t.Fatalf("probar = %d %s", got, resp)
	}
	// Con el tópico, pero sin el token guardado, tampoco.
	moved["topic"] = "otro_topico"
	if got, _ := c.do("PUT", "/api/admin/notifications/channels/"+id, moved); got != 400 {
		t.Fatalf("sin token = %d", got)
	}
	if other.count() != 0 {
		t.Fatal("los secretos guardados han llegado al servidor nuevo")
	}
	moved["token"] = "tk_nuevo"
	c.want(200, "PUT", "/api/admin/notifications/channels/"+id, moved)
	// El mismo servidor, escrito con barra final, no cuenta como cambio.
	c.want(200, "PUT", "/api/admin/notifications/channels/"+id, map[string]any{"name": "ntfy", "enabled": true, "server": other.URL + "/"})
}
