package adguard

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/izakdev/talaia/internal/integrations"
)

const statsBody = `{"time_units":"hours","num_dns_queries":200,"num_blocked_filtering":30,"num_replaced_safebrowsing":5,
"num_replaced_safesearch":9,"num_replaced_parental":5,"avg_processing_time":0.0125,
"top_queried_domains":[{"a.example":10},{"b.example":20},{"c.example":1},{"d.example":2},{"e.example":3},{"f.example":4}],
"top_blocked_domains":[{"ads.example":30}],"dns_queries":[1,2,3]}`

func server(t *testing.T, status, stats string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "admin" || pass != "clave" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/control/status":
			w.Write([]byte(status))
		case "/control/stats":
			w.Write([]byte(stats))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestCollect(t *testing.T) {
	ts := server(t, `{"version":"v0.107.1","running":true,"protection_enabled":false}`, statsBody)
	got, err := New(integrations.Config{
		BaseURL: ts.URL, Username: "admin", Secret: "clave",
		Options: map[string]bool{OptionShowTopDomains: true},
	}).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Version != "v0.107.1" || !d.Running || d.ProtectionEnabled {
		t.Fatalf("estado incorrecto: %+v", d)
	}
	if d.Queries != 200 || d.Blocked != 40 || math.Abs(d.BlockedPercent-20) > 1e-9 {
		t.Fatalf("contadores incorrectos: %+v", d)
	}
	if math.Abs(d.AvgProcessingMs-12.5) > 1e-9 || d.PeriodUnit != "hours" || d.PeriodLength != 3 {
		t.Fatalf("métricas incorrectas: %+v", d)
	}
	if len(d.TopQueried) != topLimit || d.TopQueried[0].Domain != "b.example" || d.TopQueried[4].Domain != "d.example" {
		t.Fatalf("top incorrecto: %+v", d.TopQueried)
	}
}

func TestTopDomainsAreOptional(t *testing.T) {
	ts := server(t, `{"version":"v","running":true,"protection_enabled":true}`, statsBody)
	got, err := New(integrations.Config{BaseURL: ts.URL, Username: "admin", Secret: "clave"}).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "example") || strings.Contains(string(raw), "top_") {
		t.Fatalf("incluye dominios sin haberlos pedido: %s", raw)
	}

	empty := server(t, `{"version":"v","running":true,"protection_enabled":true}`, `{"num_dns_queries":0}`)
	got, err = New(integrations.Config{
		BaseURL: empty.URL, Username: "admin", Secret: "clave",
		Options: map[string]bool{OptionShowTopDomains: true},
	}).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(got)
	if !strings.Contains(string(raw), `"top_queried":[]`) {
		t.Fatalf("activado y sin datos debe enviar una lista vacía: %s", raw)
	}
}

func TestErrors(t *testing.T) {
	ok := `{"version":"v","running":true,"protection_enabled":true}`
	cases := []struct {
		name, password, status, stats, want string
	}{
		{"credenciales", "mala", ok, statsBody, integrations.CodeUnauthorized},
		{"no es JSON", "clave", "<html>", statsBody, integrations.CodeBadResponse},
		{"otro servicio", "clave", `{"message":"hola"}`, statsBody, integrations.CodeBadResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := server(t, tc.status, tc.stats)
			_, err := New(integrations.Config{BaseURL: ts.URL, Username: "admin", Secret: tc.password}).Collect(context.Background())
			if got := integrations.Code(err); got != tc.want {
				t.Fatalf("código = %s (%v), quería %s", got, err, tc.want)
			}
		})
	}
}

func TestConnectionFailures(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	url := ts.URL
	ts.Close()
	_, err := New(integrations.Config{BaseURL: url, Username: "a", Secret: "b"}).Collect(context.Background())
	if got := integrations.Code(err); got != integrations.CodeUnreachable {
		t.Fatalf("código = %s (%v)", got, err)
	}

	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/control/status" {
			w.Write([]byte(`{"version":"v","running":true,"protection_enabled":true}`))
			return
		}
		w.Write([]byte(statsBody))
	}))
	defer tlsServer.Close()
	_, err = New(integrations.Config{BaseURL: tlsServer.URL, Username: "a", Secret: "b"}).Collect(context.Background())
	if got := integrations.Code(err); got != integrations.CodeTLS {
		t.Fatalf("código = %s (%v)", got, err)
	}
	if _, err = New(integrations.Config{BaseURL: tlsServer.URL, Username: "a", Secret: "b", TLSSkipVerify: true}).Collect(context.Background()); err != nil {
		t.Fatalf("con verificación desactivada: %v", err)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	var leaked bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked = true }))
	defer target.Close()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer ts.Close()
	_, err := New(integrations.Config{BaseURL: ts.URL, Username: "a", Secret: "b"}).Collect(context.Background())
	if got := integrations.Code(err); got != integrations.CodeBadResponse || leaked {
		t.Fatalf("código = %s, filtrado = %v", got, leaked)
	}
}
