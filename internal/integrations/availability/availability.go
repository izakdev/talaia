// Package availability comprueba si un equipo de red responde, por TCP o HTTP.
//
// Sirve para equipos sin API ni SNMP, como los switches TP-Link Easy Smart. No
// usa credenciales ni envía datos: por TCP abre la conexión y la cierra, y por
// HTTP hace un GET, no sigue redirecciones y solo lee el código de respuesta.
// No hay ICMP: en un LXC sin privilegios exige cambiar net.ipv4.ping_group_range.
package availability

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
)

// Tipos de prueba.
const (
	CheckTCP  = "tcp"
	CheckHTTP = "http"
)

const timeout = 10 * time.Second

// Data es la información pública del equipo. No incluye su dirección, su
// puerto ni su URL.
type Data struct {
	Check     string `json:"check"`
	LatencyMS int64  `json:"latency_ms"`
	// HTTPStatus es el código de respuesta; solo en las pruebas HTTP.
	HTTPStatus int `json:"http_status,omitempty"`
}

type client struct{ cfg integrations.Config }

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration { return &client{cfg} }

// Target valida la dirección y devuelve el tipo de prueba y su destino:
// host:puerto para tcp://host:puerto, o la URL para http(s)://.
func Target(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", "", errors.New("la dirección debe tener la forma tcp://host:puerto o http(s)://host")
	}
	switch u.Scheme {
	case "tcp":
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 || u.Path != "" {
			return "", "", errors.New("la dirección TCP debe tener la forma tcp://host:puerto")
		}
		return CheckTCP, net.JoinHostPort(u.Hostname(), u.Port()), nil
	case "http", "https":
		return CheckHTTP, raw, nil
	}
	return "", "", errors.New("la dirección debe empezar por tcp://, http:// o https://")
}

// latency redondea hacia arriba, para que un equipo que responde nunca dé 0 ms.
func latency(start time.Time) int64 {
	return max(1, (time.Since(start) + time.Millisecond - 1).Milliseconds())
}

// Collect hace la prueba. Un equipo que no responde es un error, con el código
// común (timeout, unreachable o tls); uno que responde por HTTP con un código
// que no es 2xx ni 3xx devuelve sus datos, y State lo marca en ámbar.
func (c *client) Collect(ctx context.Context) (any, error) {
	check, target, err := Target(c.cfg.BaseURL)
	if err != nil {
		return nil, &integrations.Error{Code: integrations.CodeInternal, Err: err}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	if check == CheckTCP {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", target)
		if err != nil {
			return nil, err
		}
		ms := latency(start)
		conn.Close()
		return Data{Check: CheckTCP, LatencyMS: ms}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, &integrations.Error{Code: integrations.CodeInternal, Err: err}
	}
	resp, err := integrations.NewHTTPClient(c.cfg).Do(req)
	if err != nil {
		return nil, err
	}
	ms := latency(start)
	// El cuerpo no se lee: basta el código.
	resp.Body.Close()
	return Data{Check: CheckHTTP, LatencyMS: ms, HTTPStatus: resp.StatusCode}, nil
}
