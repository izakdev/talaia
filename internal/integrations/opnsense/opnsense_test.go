package opnsense

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
)

// Formas tomadas de un OPNsense 26.7 real, con datos inventados.
var responses = map[string]string{
	pathActivity: `{"headers":["last pid:  8875;  load averages:    0.07,    0.14,    0.11  up 2+01:00:05    15:43:52",
		"409 threads:   5 running, 362 sleeping, 42 waiting",
		"CPU:  3.0% user,  0.0% nice,  1.0% system,  0.0% interrupt, 96.0% idle",
		"Mem: 163M Active, 732M Inact, 2043M Wired, 56K Buf, 4993M Free"],
		"details":[{"PID":"11","USERNAME":"root","COMMAND":"proceso-secreto"}]}`,
	pathGateways: `{"items":[
		{"name":"WAN_V6","address":"~","status":"none","loss":"~","delay":"~","stddev":"~","monitor":"~","status_translated":"Online"},
		{"name":"WAN_V4","address":"192.0.2.1","status":"none","loss":"0.0 %","delay":"1.5 ms","stddev":"0.1 ms","monitor":"192.0.2.2","status_translated":"Online"}],"status":"ok"}`,
	pathInformation: `{"name":"cortafuegos.interno","versions":["OPNsense 26.7.1_1-amd64","FreeBSD 15.1-RELEASE-p1","OpenSSL 3.5.7"],"updates":"Click to view pending updates."}`,
	pathResources:   `{"memory":{"total":"8000000000","total_frmt":"8141","used":2000000000,"used_frmt":"2271","arc":"1","arc_frmt":"1","arc_txt":"ARC"}}`,
	pathDisk: `{"devices":[{"device":"zroot/tmp","type":"zfs","blocks":"21G","used":"3.0M","available":"21G","used_pct":0,"mountpoint":"/tmp"},
		{"device":"zroot/ROOT/default","type":"zfs","blocks":"22G","used":"1.5G","available":"21G","used_pct":7,"mountpoint":"/"}]}`,
	pathTraffic: `{"interfaces":{"lan":{"device":"vtnet1","bytes received":"1","bytes transmitted":"2","name":"LAN"},
		"wan":{"device":"pppoe1","bytes received":"1000","bytes transmitted":"500","name":"WAN"}},"time":1}`,
}

func server(t *testing.T, override map[string]string, forbidden ...string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "clave" || pass != "secreto" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("método no permitido: %s %s", r.Method, r.URL.Path)
		}
		for _, path := range forbidden {
			if r.URL.Path == path {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"status":403,"message":"Forbidden"}`))
				return
			}
		}
		body, ok := override[r.URL.Path]
		if !ok {
			body, ok = responses[r.URL.Path]
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func config(ts *httptest.Server) integrations.Config {
	return integrations.Config{BaseURL: ts.URL, Username: "clave", Secret: "secreto"}
}

func TestCollect(t *testing.T) {
	got, err := New(config(server(t, nil))).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Version != "26.7.1_1" || math.Abs(d.CPUPercent-4) > 1e-9 || d.UptimeSeconds != 2*86400+3600+5 {
		t.Fatalf("sistema incorrecto: %+v", d)
	}
	if d.MemoryUsed != 2000000000 || d.MemoryTotal != 8000000000 {
		t.Fatalf("memoria incorrecta: %+v", d)
	}
	if d.DiskUsed != 1610612736 || d.DiskTotal != 22*1024*1024*1024 {
		t.Fatalf("disco incorrecto: %+v", d)
	}
	if d.WAN == nil || d.WAN.ReceivedBytes != 1000 || d.WAN.SentBytes != 500 {
		t.Fatalf("WAN incorrecta: %+v", d.WAN)
	}
	if notes := d.Notes(); len(notes) != 0 {
		t.Fatalf("avisos: %v", notes)
	}
}

func TestGatewayStates(t *testing.T) {
	cases := []struct {
		status, delay, loss, want string
	}{
		{"none", "1.5 ms", "0.0 %", GatewayOnline},
		{"none", "~", "~", GatewayUnmonitored},
		{"loss", "1.5 ms", "20.0 %", GatewayDegraded},
		{"delay", "900 ms", "0.0 %", GatewayDegraded},
		{"down", "~", "100.0 %", GatewayOffline},
		{"force_down", "~", "~", GatewayOffline},
	}
	for _, c := range cases {
		if got := newGateway("g", c.status, c.delay, c.loss).State; got != c.want {
			t.Errorf("%s/%s/%s: %s, quería %s", c.status, c.delay, c.loss, got, c.want)
		}
	}
	gateway := newGateway("g", "none", "1.5 ms", "0.0 %")
	if gateway.DelayMs == nil || *gateway.DelayMs != 1.5 || gateway.LossPct == nil || *gateway.LossPct != 0 {
		t.Fatalf("medidas incorrectas: %+v", gateway)
	}
}

func TestWANSpeedNeedsTwoReadings(t *testing.T) {
	received, sent := int64(1000), int64(500)
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathTraffic {
			json.NewEncoder(w).Encode(map[string]any{"interfaces": map[string]any{"wan": map[string]string{
				"bytes received": itoa(received), "bytes transmitted": itoa(sent)}}})
			return
		}
		w.Write([]byte(responses[r.URL.Path]))
	}))
	defer ts.Close()

	now := time.Unix(1000, 0)
	c := New(config(ts)).(*client)
	c.now = func() time.Time { return now }
	read := func() *Traffic {
		t.Helper()
		got, err := c.Collect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return got.(Data).WAN
	}

	if first := read(); first.ReceiveBitsPerS != nil || first.TransmitBitsPerS != nil {
		t.Fatalf("la primera lectura no puede tener velocidad: %+v", first)
	}
	now, received, sent = now.Add(10*time.Second), 2000, 1500
	second := read()
	if second.ReceiveBitsPerS == nil || *second.ReceiveBitsPerS != 800 || *second.TransmitBitsPerS != 800 {
		t.Fatalf("velocidad incorrecta: %+v", second)
	}
	now, received, sent = now.Add(10*time.Second), 10, 10
	if reset := read(); reset.ReceiveBitsPerS != nil {
		t.Fatalf("tras reiniciar los contadores no hay velocidad: %+v", reset)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestMissingPrivilegesLimitData(t *testing.T) {
	got, err := New(config(server(t, nil, pathInformation, pathResources, pathDisk, pathTraffic))).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Version != "" || d.MemoryTotal != 0 || d.DiskTotal != 0 || d.WAN != nil {
		t.Fatalf("no debería haber datos opcionales: %+v", d)
	}
	if d.CPUPercent == 0 || len(d.Gateways) != 2 {
		t.Fatalf("faltan los datos imprescindibles: %+v", d)
	}
	if notes := d.Notes(); len(notes) != 1 || notes[0] != NoteLimitedPrivileges {
		t.Fatalf("avisos: %v", notes)
	}
}

func TestPublicDataHidesInfrastructure(t *testing.T) {
	got, err := New(config(server(t, nil))).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"192.0.2", "cortafuegos.interno", "proceso-secreto", "pppoe1", "vtnet1", "zroot", "secreto", "clave"} {
		if strings.Contains(string(body), leak) {
			t.Errorf("los datos públicos incluyen %q: %s", leak, body)
		}
	}
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	ts := server(t, nil)
	wrong := config(ts)
	wrong.Secret = "malo"
	if _, err := New(wrong).Collect(ctx); integrations.Code(err) != integrations.CodeUnauthorized {
		t.Fatalf("credenciales: %v", err)
	}
	for name, path := range map[string]string{"actividad": pathActivity, "gateways": pathGateways} {
		_, err := New(config(server(t, nil, path))).Collect(ctx)
		if integrations.Code(err) != integrations.CodeUnauthorized {
			t.Errorf("%s sin privilegios: %v", name, err)
		}
	}
	for name, override := range map[string]map[string]string{
		"actividad ajena": {pathActivity: `{"ok":true}`},
		"gateways ajenos": {pathGateways: `{"ok":true}`},
	} {
		_, err := New(config(server(t, override))).Collect(ctx)
		if integrations.Code(err) != integrations.CodeBadResponse {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Un fallo que no es de permisos no se oculta como dato ausente.
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathDisk {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(responses[r.URL.Path]))
	}))
	defer ts.Close()
	if _, err := New(config(ts)).Collect(ctx); integrations.Code(err) != integrations.CodeBadResponse {
		t.Fatalf("error del servicio: %v", err)
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{"96K": 98304, "1.5G": 1610612736, "3.0M": 3145728, "512B": 512, "0": 0, "": 0, "raro": 0}
	for text, want := range cases {
		if got := parseSize(text); got != want {
			t.Errorf("%q: %d, quería %d", text, got, want)
		}
	}
}
