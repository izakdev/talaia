// Package integrations define la interfaz común de los servicios y su cliente HTTP.
package integrations

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Códigos de error públicos. No incluyen detalles de la infraestructura.
const (
	CodeUnauthorized = "unauthorized"
	CodeTimeout      = "timeout"
	CodeUnreachable  = "unreachable"
	CodeTLS          = "tls"
	CodeBadResponse  = "bad_response"
	CodeInternal     = "internal"
)

const (
	requestTimeout = 10 * time.Second
	maxResponse    = 8 << 20
)

// Config contiene lo necesario para hablar con un servicio.
type Config struct {
	BaseURL       string
	Username      string
	Secret        string
	TLSSkipVerify bool
}

// Integration lee el estado de un servicio. El resultado se envía al dashboard
// público, así que no debe incluir credenciales ni direcciones internas.
type Integration interface {
	Collect(ctx context.Context) (any, error)
}

// Registry asocia cada tipo de conexión con su integración.
type Registry map[string]func(Config) Integration

// Error describe un fallo con su código público.
type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Err) }
func (e *Error) Unwrap() error { return e.Err }

// Code devuelve el código público de un error.
func Code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	var netErr net.Error
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var header tls.RecordHeaderError
	var verification *tls.CertificateVerificationError
	switch {
	case errors.As(err, &unknownAuthority), errors.As(err, &hostname), errors.As(err, &invalid),
		errors.As(err, &header), errors.As(err, &verification):
		return CodeTLS
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return CodeTimeout
	case errors.As(err, &netErr):
		return CodeUnreachable
	}
	return CodeInternal
}

// NewHTTPClient crea un cliente sin proxy que no sigue redirecciones, para que
// las credenciales no se envíen a otro destino.
func NewHTTPClient(cfg Config) *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			TLSHandshakeTimeout: 5 * time.Second,
			IdleConnTimeout:     30 * time.Second,
			MaxIdleConnsPerHost: 2,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: cfg.TLSSkipVerify, MinVersion: tls.VersionTLS12},
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// GetJSON hace un GET autenticado por authorize y decodifica la respuesta en dst.
func GetJSON(ctx context.Context, client *http.Client, url string, authorize func(*http.Request), dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &Error{CodeInternal, err}
	}
	req.Header.Set("Accept", "application/json")
	authorize(req)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return &Error{CodeUnauthorized, fmt.Errorf("HTTP %d", resp.StatusCode)}
	case resp.StatusCode != http.StatusOK:
		return &Error{CodeBadResponse, fmt.Errorf("HTTP %d", resp.StatusCode)}
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponse)).Decode(dst); err != nil {
		return &Error{CodeBadResponse, err}
	}
	return nil
}
