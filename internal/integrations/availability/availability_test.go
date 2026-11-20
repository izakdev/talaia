package availability

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
)

func collect(t *testing.T, cfg integrations.Config) (Data, error) {
	t.Helper()
	got, err := New(cfg).Collect(context.Background())
	if err != nil {
		return Data{}, err
	}
	return got.(Data), nil
}

func TestCollectTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	d, err := collect(t, integrations.Config{BaseURL: "tcp://" + ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	if d.Check != "tcp" || d.LatencyMS < 1 || d.HTTPStatus != 0 {
		t.Fatalf("datos = %+v", d)
	}
}

func TestCollectHTTP(t *testing.T) {
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.URL.Path == "/redirige" {
			http.Redirect(w, r, "http://otro.simulado/", http.StatusMovedPermanently)
			return
		}
		w.Write([]byte("<html>login</html>"))
	}))
	defer srv.Close()
	for path, status := range map[string]int{"/": 200, "/redirige": 301} {
		d, err := collect(t, integrations.Config{BaseURL: srv.URL + path})
		if err != nil {
			t.Fatal(err)
		}
		if d.Check != "http" || d.HTTPStatus != status || d.LatencyMS < 1 {
			t.Fatalf("%s: datos = %+v", path, d)
		}
		if d.State().Tone != integrations.ToneGood {
			t.Fatalf("%s: estado = %+v", path, d.State())
		}
	}
	// Solo GET, y la redirección no se sigue.
	if len(methods) != 2 || methods[0] != http.MethodGet || methods[1] != http.MethodGet {
		t.Fatalf("peticiones = %v", methods)
	}
}

func TestCollectHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	d, err := collect(t, integrations.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if d.HTTPStatus != 503 || d.State().Tone != integrations.ToneWarn {
		t.Fatalf("datos = %+v, estado = %+v", d, d.State())
	}
}

func TestAvailabilityErrors(t *testing.T) {
	// Puerto cerrado.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := ln.Addr().String()
	ln.Close()
	for _, base := range []string{"tcp://" + closed, "http://" + closed} {
		if _, err := collect(t, integrations.Config{BaseURL: base}); integrations.Code(err) != integrations.CodeUnreachable {
			t.Fatalf("%s: código %q (%v)", base, integrations.Code(err), err)
		}
	}

	// Un servidor que no responde agota el plazo.
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer slow.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := New(integrations.Config{BaseURL: slow.URL}).Collect(ctx); integrations.Code(err) != integrations.CodeTimeout {
		t.Fatalf("código %q (%v)", integrations.Code(err), err)
	}

	// Certificado autofirmado: falla salvo que no se verifique.
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer tlsSrv.Close()
	if _, err := collect(t, integrations.Config{BaseURL: tlsSrv.URL}); integrations.Code(err) != integrations.CodeTLS {
		t.Fatalf("código %q (%v)", integrations.Code(err), err)
	}
	if d, err := collect(t, integrations.Config{BaseURL: tlsSrv.URL, TLSSkipVerify: true}); err != nil || d.HTTPStatus != 200 {
		t.Fatalf("datos = %+v, error %v", d, err)
	}

	// Dirección no válida.
	if _, err := collect(t, integrations.Config{BaseURL: "tcp://equipo.simulado"}); integrations.Code(err) != integrations.CodeInternal {
		t.Fatalf("código %q (%v)", integrations.Code(err), err)
	}
}

func TestAvailabilityDataHidesTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	d, err := collect(t, integrations.Config{BaseURL: srv.URL + "/ruta-privada"})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(d)
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	for _, leak := range []string{"127.0.0.1", port, "ruta-privada", "http://"} {
		if strings.Contains(string(out), leak) {
			t.Fatalf("%s aparece en %s", leak, out)
		}
	}
}

func TestTarget(t *testing.T) {
	ok := map[string][2]string{
		"tcp://192.0.2.10:80":                {"tcp", "192.0.2.10:80"},
		"tcp://switch.simulado:443":          {"tcp", "switch.simulado:443"},
		"tcp://[2001:db8::1]:22":             {"tcp", "[2001:db8::1]:22"},
		"http://192.0.2.10":                  {"http", "http://192.0.2.10"},
		"https://switch.simulado:8443/login": {"http", "https://switch.simulado:8443/login"},
	}
	for raw, want := range ok {
		check, addr, err := Target(raw)
		if err != nil || check != want[0] || addr != want[1] {
			t.Errorf("%s: %q %q %v", raw, check, addr, err)
		}
	}
	for _, raw := range []string{
		"tcp://192.0.2.10", "tcp://192.0.2.10:0", "tcp://192.0.2.10:70000", "tcp://192.0.2.10:80/ruta",
		"tcp://usuario@192.0.2.10:80", "tcp://192.0.2.10:80?a=1", "tcp://:80", "ftp://192.0.2.10",
		"http://", "http://usuario:clave@192.0.2.10", "http://192.0.2.10/?a=1", "http://192.0.2.10/#a", "192.0.2.10:80",
	} {
		if _, _, err := Target(raw); err == nil {
			t.Errorf("%s: se acepta", raw)
		}
	}
}
