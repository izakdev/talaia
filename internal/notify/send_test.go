package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/store"
)

type received struct {
	path    string
	headers http.Header
	body    string
}

func receiver(t *testing.T, status int) (*httptest.Server, chan received) {
	t.Helper()
	got := make(chan received, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- received{r.URL.Path, r.Header.Clone(), string(body)}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func sample() Message {
	return Compose("es", []Change{
		change("Portainer", "portainer", integrations.ToneBad, integrations.ToneGood, "portainer.down"),
		change("AdGuard", "adguard", integrations.ToneGood, integrations.ToneWarn, "adguard.protectionOn"),
	}, now)
}

func send(t *testing.T, ch store.Channel, secret string, m Message) error {
	t.Helper()
	s, err := NewSender(ch, secret)
	if err != nil {
		t.Fatal(err)
	}
	return s.Send(context.Background(), m)
}

func TestTelegramRequest(t *testing.T) {
	srv, got := receiver(t, 200)
	t.Setenv("TALAIA_TELEGRAM_API", srv.URL)
	ch := store.Channel{Type: "telegram", Config: map[string]string{"chat_id": "-1001"}}
	if err := send(t, ch, `{"token":"123:abc"}`, sample()); err != nil {
		t.Fatal(err)
	}
	r := <-got
	var body map[string]any
	if err := json.Unmarshal([]byte(r.body), &body); err != nil {
		t.Fatal(err)
	}
	if r.path != "/bot123:abc/sendMessage" || body["chat_id"] != "-1001" || body["text"] != sample().Text {
		t.Fatalf("petición = %+v", r)
	}
	if _, ok := body["parse_mode"]; ok || r.headers.Get("Content-Type") != "application/json" {
		t.Fatalf("Telegram debe recibir texto plano en JSON: %+v", r)
	}
}

type ntfyBody struct {
	Topic    string   `json:"topic"`
	Title    string   `json:"title"`
	Message  string   `json:"message"`
	Priority int      `json:"priority"`
	Tags     []string `json:"tags"`
}

func ntfyReceived(t *testing.T, got chan received) (received, ntfyBody) {
	t.Helper()
	r := <-got
	var b ntfyBody
	if err := json.Unmarshal([]byte(r.body), &b); err != nil {
		t.Fatalf("cuerpo %q: %v", r.body, err)
	}
	return r, b
}

func TestNtfyJSON(t *testing.T) {
	srv, got := receiver(t, 200)
	ch := store.Channel{Type: "ntfy", Config: map[string]string{"server": srv.URL + "/"}}
	if err := send(t, ch, `{"topic":"talaia-avisos","token":"tk_1"}`, sample()); err != nil {
		t.Fatal(err)
	}
	r, b := ntfyReceived(t, got)
	// ntfy pone delante el emoji de la etiqueta: el título va sin él y el cuerpo no lo repite.
	if r.path != "/" || b.Topic != "talaia-avisos" || b.Title != "Talaia · Necesita atención" || !strings.HasPrefix(b.Message, "Portainer: ") || strings.Contains(b.Message, "Necesita atención") {
		t.Fatalf("petición = %+v %+v", r, b)
	}
	if !strings.Contains(b.Message, "\n\n🟢 Talaia · Vuelve a estar en orden\nAdGuard: ") {
		t.Fatalf("el bloque de recuperados se pierde: %q", b.Message)
	}
	if b.Priority != 4 || len(b.Tags) != 1 || b.Tags[0] != "red_circle" || r.headers.Get("Authorization") != "Bearer tk_1" {
		t.Fatalf("prioridad, etiquetas o token: %+v %v", b, r.headers)
	}

	// Sin token y solo con recuperaciones.
	recovery := Compose("es", []Change{change("AdGuard", "adguard", integrations.ToneGood, integrations.ToneWarn, "adguard.protectionOn")}, now)
	if err := send(t, ch, `{"topic":"talaia-avisos"}`, recovery); err != nil {
		t.Fatal(err)
	}
	if r, b = ntfyReceived(t, got); b.Priority != 2 || b.Tags[0] != "green_circle" || r.headers.Get("Authorization") != "" {
		t.Fatalf("recuperación: %+v %v", b, r.headers)
	}
	warn := Compose("es", []Change{change("VPN", "wgdashboard", integrations.ToneWarn, integrations.ToneGood, "wgdashboard.down")}, now)
	_ = send(t, ch, `{"topic":"t"}`, warn)
	if _, b = ntfyReceived(t, got); b.Priority != 3 || b.Tags[0] != "orange_circle" {
		t.Fatalf("aviso: %+v", b)
	}
	_ = send(t, ch, `{"topic":"t"}`, ComposeTest("es", now))
	if _, b = ntfyReceived(t, got); b.Tags[0] != "bell" || b.Priority != 3 || b.Title != "Talaia · Mensaje de prueba" || b.Message != "Si lo ves, este canal funciona." {
		t.Fatalf("prueba: %+v", b)
	}
}

func TestNtfyDefaultServer(t *testing.T) {
	s, err := NewSender(store.Channel{Type: "ntfy", Config: map[string]string{}}, `{"topic":"t"}`)
	if err != nil || s.(*ntfy).server != "https://ntfy.sh" {
		t.Fatalf("servidor por defecto: %+v %v", s, err)
	}
}

func TestWebhookBody(t *testing.T) {
	srv, got := receiver(t, 204)
	ch := store.Channel{Type: "webhook"}
	if err := send(t, ch, `{"url":"`+srv.URL+`/hook?clave=1"}`, sample()); err != nil {
		t.Fatal(err)
	}
	r := <-got
	var body struct {
		Event    string `json:"event"`
		Language string `json:"language"`
		Text     string `json:"text"`
		SentAt   string `json:"sent_at"`
		Services []map[string]string
	}
	if err := json.Unmarshal([]byte(r.body), &body); err != nil {
		t.Fatal(err)
	}
	if r.path != "/hook" || body.Event != "change" || body.Language != "es" || body.Text != sample().Text || body.SentAt != "2026-11-19T10:00:00Z" {
		t.Fatalf("cuerpo = %s", r.body)
	}
	want := map[string]string{"name": "Portainer", "type": "portainer", "tone": "bad", "previous": "good", "label": "Entorno caído"}
	if len(body.Services) != 2 || len(body.Services[0]) != len(want) {
		t.Fatalf("servicios = %+v", body.Services)
	}
	for k, v := range want {
		if body.Services[0][k] != v {
			t.Fatalf("servicio[%s] = %q", k, body.Services[0][k])
		}
	}
	// El webhook de prueba lleva una lista vacía, no null.
	_ = send(t, ch, `{"url":"`+srv.URL+`/hook"}`, ComposeTest("es", now))
	if r = <-got; !strings.Contains(r.body, `"services":[]`) || !strings.Contains(r.body, `"event":"test"`) {
		t.Fatalf("prueba = %s", r.body)
	}
}

func TestSendErrorCodes(t *testing.T) {
	for status, code := range map[int]string{401: integrations.CodeUnauthorized, 403: integrations.CodeUnauthorized, 400: CodeRejected, 404: CodeRejected, 429: CodeRejected, 500: integrations.CodeBadResponse, 302: integrations.CodeBadResponse} {
		srv, _ := receiver(t, status)
		err := send(t, store.Channel{Type: "webhook"}, `{"url":"`+srv.URL+`"}`, sample())
		if integrations.Code(err) != code {
			t.Errorf("HTTP %d: código %q, quiero %q (%v)", status, integrations.Code(err), code, err)
		}
	}
	srv, _ := receiver(t, 200)
	srv.Close()
	if err := send(t, store.Channel{Type: "webhook"}, `{"url":"`+srv.URL+`"}`, sample()); integrations.Code(err) != integrations.CodeUnreachable {
		t.Errorf("servidor cerrado: %v", err)
	}
}

func TestSendErrorsHideURL(t *testing.T) {
	srv, _ := receiver(t, 200)
	srv.Close()
	secretURL := srv.URL + "/hook/token-secreto"
	err := send(t, store.Channel{Type: "webhook"}, `{"url":"`+secretURL+`"}`, sample())
	if err == nil || strings.Contains(err.Error(), "token-secreto") || strings.Contains(err.Error(), srv.URL) {
		t.Fatalf("el error deja ver la dirección: %v", err)
	}
	t.Setenv("TALAIA_TELEGRAM_API", srv.URL)
	err = send(t, store.Channel{Type: "telegram", Config: map[string]string{"chat_id": "1"}}, `{"token":"9:bot-secreto"}`, sample())
	if err == nil || strings.Contains(err.Error(), "bot-secreto") {
		t.Fatalf("el error deja ver el token: %v", err)
	}
}

func TestRetryable(t *testing.T) {
	cases := map[string]bool{
		integrations.CodeUnreachable: true, integrations.CodeTimeout: true, integrations.CodeBadResponse: true,
		integrations.CodeUnauthorized: false, CodeRejected: false, integrations.CodeTLS: false,
		CodeUnconfirmed: false,
	}
	for code, want := range cases {
		if got := Retryable(&integrations.Error{Code: code, Err: errors.New("x")}); got != want {
			t.Errorf("%s: %v", code, got)
		}
	}
	srv, _ := receiver(t, 429)
	err := send(t, store.Channel{Type: "webhook"}, `{"url":"`+srv.URL+`"}`, sample())
	if !Retryable(err) {
		t.Errorf("429 se reintenta: %v", err)
	}
}

func TestNewSenderRejectsBadSecret(t *testing.T) {
	for _, c := range []struct{ typ, secret string }{{"telegram", `{}`}, {"ntfy", `{"token":"x"}`}, {"webhook", `no-json`}, {"correo", `{}`}} {
		if _, err := NewSender(store.Channel{Type: c.typ, Config: map[string]string{}}, c.secret); err == nil {
			t.Errorf("%s con %s debería fallar", c.typ, c.secret)
		}
	}
}

func TestSendTimeoutIsLonger(t *testing.T) {
	if sendTimeout < 30*time.Second {
		t.Fatalf("plazo de envío = %v, quiero al menos 30 s", sendTimeout)
	}
	cfg := map[string]string{"chat_id": "1"}
	for typ, secret := range map[string]string{"telegram": `{"token":"9:x"}`, "ntfy": `{"topic":"t"}`, "webhook": `{"url":"http://x"}`} {
		s, err := NewSender(store.Channel{Type: typ, Config: cfg}, secret)
		if err != nil {
			t.Fatal(err)
		}
		var got time.Duration
		switch c := s.(type) {
		case *telegram:
			got = c.client.Timeout
		case *ntfy:
			got = c.client.Timeout
		case *webhook:
			got = c.client.Timeout
		}
		if got != sendTimeout {
			t.Errorf("%s: plazo %v, quiero %v", typ, got, sendTimeout)
		}
	}
}

func TestSlowReplyIsUnconfirmed(t *testing.T) {
	short(t)
	got := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		got <- struct{}{}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	err := send(t, store.Channel{Type: "webhook"}, `{"url":"`+srv.URL+`"}`, sample())
	<-got
	if integrations.Code(err) != CodeUnconfirmed || Retryable(err) {
		t.Fatalf("respuesta tardía tras enviar: código %q, reintentable %v (%v)", integrations.Code(err), Retryable(err), err)
	}
}

func TestTimeoutBeforeSendingIsRetried(t *testing.T) {
	short(t)
	// Acepta la conexión pero nunca completa el saludo TLS: la petición no sale.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()
	err = send(t, store.Channel{Type: "webhook"}, `{"url":"https://`+ln.Addr().String()+`/hook"}`, sample())
	if integrations.Code(err) != integrations.CodeTimeout || !Retryable(err) {
		t.Fatalf("plazo agotado antes de enviar: código %q, reintentable %v (%v)", integrations.Code(err), Retryable(err), err)
	}
}

// short acorta el plazo de envío durante una prueba.
func short(t *testing.T) {
	t.Helper()
	old := sendTimeout
	sendTimeout = 300 * time.Millisecond
	t.Cleanup(func() { sendTimeout = old })
}
