package homeassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
