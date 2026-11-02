package homeassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/izakdev/talaia/internal/integrations"
)

const token = "token-de-larga-duracion"

const configBody = `{"version":"2026.2.3","state":"RUNNING","safe_mode":false,"location_name":"Casa privada","latitude":40.1,"longitude":-3.2}`

const statesBody = `[
{"entity_id":"sensor.casa_potencia","state":"412.5","attributes":{"friendly_name":"Potencia de casa","unit_of_measurement":"W","device_class":"power","state_class":"measurement"},"last_changed":"2026-11-02T10:00:00+00:00"},
{"entity_id":"sensor.casa_energia","state":"1204.3","attributes":{"friendly_name":"Energía de casa","unit_of_measurement":"kWh","device_class":"energy"}},
{"entity_id":"light.salon","state":"on","attributes":{"friendly_name":"Luz del salón","supported_features":44}},
{"entity_id":"person.privada","state":"home","attributes":{"friendly_name":"Persona privada","latitude":40.1}},
{"entity_id":"sensor.sin_nombre","state":"unavailable","attributes":{}}
]`

func server(t *testing.T, config, states string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/config":
			w.Write([]byte(config))
		case "/api/states":
			w.Write([]byte(states))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// wsServer añade al servidor de pruebas un WebSocket de Home Assistant.
func wsHandler(t *testing.T, success bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		wsjson.Write(ctx, conn, map[string]string{"type": "auth_required"})
		var auth map[string]string
		if wsjson.Read(ctx, conn, &auth) != nil {
			return
		}
		if auth["access_token"] != token {
			wsjson.Write(ctx, conn, map[string]string{"type": "auth_invalid"})
			return
		}
		wsjson.Write(ctx, conn, map[string]string{"type": "auth_ok"})
		for range 2 {
			var command struct {
				ID       int    `json:"id"`
				Type     string `json:"type"`
				Endpoint string `json:"endpoint"`
			}
			if wsjson.Read(ctx, conn, &command) != nil {
				return
			}
			var result any
			switch command.Endpoint {
			case "/core/stats":
				result = map[string]any{"cpu_percent": 12.5, "memory_usage": 2000, "memory_limit": 8000, "memory_percent": 25.0}
			case "/host/info":
				result = map[string]any{"hostname": "nombre-privado", "disk_total": 32.5, "disk_used": 10.0, "boot_timestamp": 1793000000000000}
			}
			if !success {
				wsjson.Write(ctx, conn, map[string]any{"id": command.ID, "type": "result", "success": false, "error": map[string]string{"code": "unknown_command"}})
				continue
			}
			wsjson.Write(ctx, conn, map[string]any{"id": command.ID, "type": "result", "success": true, "result": result})
		}
		conn.Close(websocket.StatusNormalClosure, "")
	}
}

func serverWithSupervisor(t *testing.T, success bool) *httptest.Server {
	t.Helper()
	rest := server(t, configBody, statesBody)
	ws := wsHandler(t, success)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/websocket" {
			ws(w, r)
			return
		}
		rest.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func collect(t *testing.T, ts *httptest.Server, entities ...string) Data {
	t.Helper()
	got, err := New(integrations.Config{BaseURL: ts.URL, Secret: token, Entities: entities}).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got.(Data)
}

func TestStatusWithoutEntitiesSkipsStates(t *testing.T) {
	// Sin selección no se pide /api/states: con el cuerpo no válido fallaría.
	d := collect(t, server(t, configBody, "<html>"))
	if d.Version != "2026.2.3" || d.State != "RUNNING" || d.SafeMode || len(d.Entities) != 0 {
		t.Fatalf("estado: %+v", d)
	}
}

func TestOnlySelectedEntitiesArePublished(t *testing.T) {
	d := collect(t, server(t, configBody, statesBody), "sensor.casa_potencia", "light.salon", "sensor.sin_nombre")
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"Persona privada", "person.", "Casa privada", "latitude", "40.1", "casa_energia", "supported_features"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("publica %q: %s", leak, raw)
		}
	}
	if len(d.Entities) != 3 || d.Entities[0].Name != "Luz del salón" {
		t.Fatalf("entidades: %+v", d.Entities)
	}
	power := d.Entities[1]
	if power.State != "412.5" || power.Unit != "W" || power.DeviceClass != "power" {
		t.Fatalf("potencia: %+v", power)
	}
	if unnamed := d.Entities[2]; unnamed.Name != "sensor.sin_nombre" || unnamed.State != "unavailable" {
		t.Fatalf("sin nombre: %+v", unnamed)
	}
}

func TestMissingEntityIsReported(t *testing.T) {
	d := collect(t, server(t, configBody, statesBody), "sensor.borrada")
	if len(d.Entities) != 1 || d.Entities[0].State != StateMissing || d.Entities[0].Name != "sensor.borrada" {
		t.Fatalf("entidades: %+v", d.Entities)
	}
}

func TestStartingAndDegradedModes(t *testing.T) {
	d := collect(t, server(t, `{"version":"1","state":"NOT_RUNNING","safe_mode":true,"recovery_mode":true}`, statesBody))
	if d.State != "NOT_RUNNING" || !d.SafeMode || !d.Recovery {
		t.Fatalf("estado: %+v", d)
	}
}

func TestSystemFromSupervisor(t *testing.T) {
	d := collect(t, serverWithSupervisor(t, true))
	s := d.System
	if s == nil || s.CPUPercent != 12.5 || s.MemoryPercent != 25 || s.MemoryUsed != 2000 || s.MemoryLimit != 8000 {
		t.Fatalf("sistema: %+v", s)
	}
	if s.DiskUsed != 10_000_000_000 || s.DiskTotal != 32_500_000_000 || s.BootedAt != 1793000000 {
		t.Fatalf("disco o arranque: %+v", s)
	}
	if notes := d.Notes(); len(notes) != 0 {
		t.Fatalf("avisos: %v", notes)
	}
	raw, _ := json.Marshal(d)
	if strings.Contains(string(raw), "nombre-privado") {
		t.Fatalf("publica el nombre del equipo: %s", raw)
	}
}

func TestSystemIsOptional(t *testing.T) {
	// Sin Supervisor o sin permisos, el servicio sigue funcionando y lo indica.
	for name, ts := range map[string]*httptest.Server{
		"comando desconocido": serverWithSupervisor(t, false),
		"sin websocket":       server(t, configBody, statesBody),
	} {
		t.Run(name, func(t *testing.T) {
			d := collect(t, ts)
			if d.System != nil || d.Version == "" {
				t.Fatalf("datos: %+v", d)
			}
			if notes := d.Notes(); len(notes) != 1 || notes[0] != NoteSystemUnavailable {
				t.Fatalf("avisos: %v", notes)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	states := `[
{"entity_id":"light.a","state":"on","attributes":{}},
{"entity_id":"light.b","state":"off","attributes":{}},
{"entity_id":"light.grupo","state":"on","attributes":{"entity_id":["light.a","light.b"]}},
{"entity_id":"light.caida","state":"unavailable","attributes":{}},
{"entity_id":"switch.a","state":"on","attributes":{}},
{"entity_id":"climate.a","state":"heat","attributes":{}},
{"entity_id":"climate.b","state":"off","attributes":{}},
{"entity_id":"cover.persiana","state":"open","attributes":{"device_class":"blind"}},
{"entity_id":"cover.garaje","state":"open","attributes":{"device_class":"garage"}},
{"entity_id":"automation.a","state":"on","attributes":{}},
{"entity_id":"update.a","state":"on","attributes":{}},
{"entity_id":"update.b","state":"off","attributes":{}},
{"entity_id":"person.a","state":"home","attributes":{}},
{"entity_id":"lock.a","state":"unlocked","attributes":{}},
{"entity_id":"alarm_control_panel.a","state":"disarmed","attributes":{}},
{"entity_id":"sensor.x","state":"unknown","attributes":{}}
]`
	config := `{"version":"1","state":"RUNNING","components":["light","sensor","sensor.template","api"]}`
	ts := server(t, config, states)
	got, err := New(integrations.Config{BaseURL: ts.URL, Secret: token, Options: map[string]bool{OptionShowSummary: true}}).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := got.(Data).Summary
	want := []DomainSummary{{"light", 1, 2}, {"switch", 1, 1}, {"climate", 1, 2}, {"cover", 1, 1}, {"automation", 1, 1}}
	if s == nil || !slices.Equal(s.Domains, want) {
		t.Fatalf("dominios: %+v", s)
	}
	if s.Entities != 16 || s.Unavailable != 1 || s.Updates != 1 || s.Integrations != 3 {
		t.Fatalf("totales: %+v", s)
	}
	raw, _ := json.Marshal(got)
	for _, leak := range []string{"person", "lock", "alarm", "home", "unlocked", "garage"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("publica %q: %s", leak, raw)
		}
	}
	if collect(t, server(t, config, "<html>")).Summary != nil {
		t.Fatal("sin la opción no debe haber resumen")
	}
}

func TestListEntities(t *testing.T) {
	lister, ok := New(integrations.Config{BaseURL: server(t, configBody, statesBody).URL, Secret: token}).(integrations.EntityLister)
	if !ok {
		t.Fatal("debe poder listar entidades")
	}
	got, err := lister.ListEntities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || got[0].Name != "Energía de casa" || got[0].DeviceClass != "energy" || got[0].Unit != "kWh" {
		t.Fatalf("lista: %+v", got)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name, secret, config, want string
	}{
		{"token incorrecto", "otro", configBody, integrations.CodeUnauthorized},
		{"otro servicio", token, `{"message":"hola"}`, integrations.CodeBadResponse},
		{"no es JSON", token, "<html>", integrations.CodeBadResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := server(t, tc.config, statesBody)
			_, err := New(integrations.Config{BaseURL: ts.URL, Secret: tc.secret}).Collect(context.Background())
			if got := integrations.Code(err); got != tc.want {
				t.Fatalf("código = %s (%v), quería %s", got, err, tc.want)
			}
		})
	}
}
