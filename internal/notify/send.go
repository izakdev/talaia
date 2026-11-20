package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/store"
)

// CodeRejected indica que el canal ha rechazado el aviso (otro 4xx).
const CodeRejected = "rejected"

// CodeUnconfirmed indica que el aviso se envió entero pero no llegó la respuesta:
// puede haber llegado, así que no se reintenta para no duplicarlo (DEC-53).
const CodeUnconfirmed = "unconfirmed"

// sendTimeout es el plazo de cada envío. Es mayor que el de las consultas al
// panel porque los avisos van en segundo plano y no hacen esperar a nadie.
var sendTimeout = 30 * time.Second

const (
	maxReply        = 64 << 10
	defaultNtfy     = "https://ntfy.sh"
	defaultTelegram = "https://api.telegram.org"
)

// errTooMany marca un 429: se rechaza, pero se puede reintentar.
var errTooMany = errors.New("demasiadas peticiones")

// Sender envía un aviso por un canal.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

type secretFields struct {
	Token string `json:"token"`
	Topic string `json:"topic"`
	URL   string `json:"url"`
}

// NewSender crea el emisor de un canal. secret es el JSON guardado:
// telegram {"token"}, ntfy {"topic", "token"} y webhook {"url"}.
func NewSender(ch store.Channel, secret string) (Sender, error) {
	var f secretFields
	if err := json.Unmarshal([]byte(secret), &f); err != nil {
		return nil, errors.New("secreto del canal ilegible")
	}
	switch ch.Type {
	case "telegram":
		if f.Token == "" {
			return nil, errors.New("falta el token del bot")
		}
		api := strings.TrimRight(os.Getenv("TALAIA_TELEGRAM_API"), "/")
		if api == "" {
			api = defaultTelegram
		}
		// Telegram siempre verifica TLS.
		return &telegram{client: newClient(false), url: api + "/bot" + f.Token + "/sendMessage", chatID: ch.Config["chat_id"]}, nil
	case "ntfy":
		if f.Topic == "" {
			return nil, errors.New("falta el tópico")
		}
		server := strings.TrimRight(ch.Config["server"], "/")
		if server == "" {
			server = defaultNtfy
		}
		return &ntfy{client: newClient(ch.TLSSkipVerify), server: server, topic: f.Topic, token: f.Token}, nil
	case "webhook":
		if f.URL == "" {
			return nil, errors.New("falta la URL")
		}
		return &webhook{client: newClient(ch.TLSSkipVerify), url: f.URL}, nil
	}
	return nil, fmt.Errorf("tipo de canal desconocido: %q", ch.Type)
}

func newClient(skipTLS bool) *http.Client {
	client := integrations.NewHTTPClient(integrations.Config{TLSSkipVerify: skipTLS})
	client.Timeout = sendTimeout
	return client
}

// Retryable indica si merece la pena repetir un envío fallido.
func Retryable(err error) bool {
	if errors.Is(err, errTooMany) {
		return true
	}
	switch integrations.Code(err) {
	case integrations.CodeUnreachable, integrations.CodeTimeout, integrations.CodeBadResponse:
		return true
	}
	return false
}

// post envía un cuerpo y traduce el resultado a un código público. El error
// nunca incluye la dirección, que puede llevar un token. Si falla después de
// enviar la petición entera, el código es CodeUnconfirmed.
func post(ctx context.Context, client *http.Client, url, contentType string, body []byte, header http.Header) error {
	var wrote atomic.Bool
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		// El transporte puede repetir la petición por otra conexión si la reutilizada
		// estaba cerrada: cada intento empieza sin haber enviado nada.
		GetConn:      func(string) { wrote.Store(false) },
		WroteRequest: func(info httptrace.WroteRequestInfo) { wrote.Store(info.Err == nil) },
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &integrations.Error{Code: integrations.CodeInternal, Err: errors.New("dirección no válida")}
	}
	for k, v := range header {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Talaia")
	res, err := client.Do(req)
	if err != nil {
		code := integrations.Code(err)
		if code == integrations.CodeInternal {
			code = integrations.CodeUnreachable
		}
		if wrote.Load() && code != integrations.CodeTLS {
			return &integrations.Error{Code: CodeUnconfirmed, Err: errors.New("enviado sin respuesta")}
		}
		return &integrations.Error{Code: code, Err: errors.New("no se ha podido enviar")}
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxReply))
	switch {
	case res.StatusCode >= 200 && res.StatusCode < 300:
		return nil
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return &integrations.Error{Code: integrations.CodeUnauthorized, Err: fmt.Errorf("HTTP %d", res.StatusCode)}
	case res.StatusCode == http.StatusTooManyRequests:
		return &integrations.Error{Code: CodeRejected, Err: errTooMany}
	case res.StatusCode >= 400 && res.StatusCode < 500:
		return &integrations.Error{Code: CodeRejected, Err: fmt.Errorf("HTTP %d", res.StatusCode)}
	}
	return &integrations.Error{Code: integrations.CodeBadResponse, Err: fmt.Errorf("HTTP %d", res.StatusCode)}
}

type telegram struct {
	client *http.Client
	url    string
	chatID string
}

func (t *telegram) Send(ctx context.Context, m Message) error {
	// Texto plano: sin parse_mode, un nombre con * o _ no rompe el mensaje.
	body, _ := json.Marshal(map[string]string{"chat_id": t.chatID, "text": m.Text})
	return post(ctx, t.client, t.url, "application/json", body, nil)
}

type ntfy struct {
	client *http.Client
	server string
	topic  string
	token  string
}

func ntfyLook(m Message) (priority int, tag string) {
	if m.Event == EventTest {
		return 3, "bell"
	}
	switch m.Worst {
	case integrations.ToneBad:
		return 4, "red_circle"
	case integrations.ToneWarn:
		return 3, "orange_circle"
	}
	return 2, "green_circle"
}

// ntfyContent separa el título del cuerpo. ntfy ya pone delante del título el
// emoji de la etiqueta, así que el título va sin el suyo y el cuerpo no lo repite.
func ntfyContent(m Message) (title, body string) {
	title = m.Title
	if _, rest, ok := strings.Cut(title, " "); ok {
		title = rest
	}
	body = strings.TrimLeft(strings.TrimPrefix(m.Text, m.Title), ". \n")
	if body == "" {
		body = title
	}
	return title, body
}

func (n *ntfy) Send(ctx context.Context, m Message) error {
	priority, tag := ntfyLook(m)
	title, text := ntfyContent(m)
	// Se publica en JSON para que el título admita tildes y emojis.
	body, _ := json.Marshal(map[string]any{
		"topic": n.topic, "title": title, "message": text, "priority": priority, "tags": []string{tag},
	})
	header := http.Header{}
	if n.token != "" {
		header.Set("Authorization", "Bearer "+n.token)
	}
	return post(ctx, n.client, n.server+"/", "application/json", body, header)
}

type webhook struct {
	client *http.Client
	url    string
}

type webhookService struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Tone     string `json:"tone"`
	Previous string `json:"previous"`
	Label    string `json:"label"`
}

func (w *webhook) Send(ctx context.Context, m Message) error {
	services := make([]webhookService, 0, len(m.Changes))
	for _, c := range m.Changes {
		services = append(services, webhookService{c.Name, c.Type, string(c.Tone), string(c.Previous), Label(m.Language, c.State)})
	}
	body, _ := json.Marshal(map[string]any{
		"event": m.Event, "language": m.Language, "text": m.Text, "services": services,
		"sent_at": m.SentAt.UTC().Format(time.RFC3339),
	})
	return post(ctx, w.client, w.url, "application/json", body, nil)
}
