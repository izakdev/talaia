package portainer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/izakdev/talaia/internal/integrations"
)

// Forma tomada de un Portainer 2.39 real, con datos inventados.
const endpointsBody = `[
	{"Id":3,"Name":"local","Type":1,"Status":1,"URL":"tcp://192.0.2.60:2375","PublicURL":"docker.interno","EdgeKey":"clave-edge",
	 "Snapshots":[{"Time":1793915962,"DockerVersion":"29.9.2","ContainerCount":8,"RunningContainerCount":7,"StoppedContainerCount":0,"HealthyContainerCount":1,"UnhealthyContainerCount":1,"StackCount":6}]},
	{"Id":5,"Name":"pi","Type":2,"Status":2,"URL":"tcp://192.0.2.61:9001","Snapshots":[]},
	{"Id":6,"Name":"borde","Type":4,"Status":1,"Heartbeat":false,"Snapshots":[]},
	{"Id":7,"Name":"borde-vivo","Type":4,"Status":2,"Heartbeat":true,"Snapshots":[]},
	{"Id":8,"Name":"kubernetes","Type":5,"Status":1,"Snapshots":[]}
]`

func server(t *testing.T, status, endpoints string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("método no permitido: %s", r.Method)
		}
		switch r.URL.Path {
		case pathStatus:
			w.Write([]byte(status))
		case pathEndpoints:
			if r.Header.Get("X-API-Key") != "token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Write([]byte(endpoints))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func collect(t *testing.T, ts *httptest.Server, secret string) (any, error) {
	t.Helper()
	return New(integrations.Config{BaseURL: ts.URL, Secret: secret}).Collect(context.Background())
}

func TestCollectEnvironments(t *testing.T) {
	got, err := collect(t, server(t, `{"Version":"2.39.1"}`, endpointsBody), "token")
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Version != "2.39.1" || len(d.Environments) != 4 {
		t.Fatalf("datos incorrectos: %+v", d)
	}
	local := d.Environments[0]
	if !local.Online || local.Containers == nil || *local.Containers != (Containers{7, 8, 1, 6, 1793915962}) {
		t.Fatalf("entorno incorrecto: %+v %+v", local, local.Containers)
	}
	if pi := d.Environments[1]; pi.Online || pi.Containers != nil {
		t.Fatalf("entorno caído incorrecto: %+v", pi)
	}
}

func TestEdgeAgentUsesHeartbeat(t *testing.T) {
	got, err := collect(t, server(t, `{"Version":"2.39.1"}`, endpointsBody), "token")
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Environments[2].Online || !d.Environments[3].Online {
		t.Fatalf("estado de Edge incorrecto: %+v", d.Environments)
	}
}

func TestPortainerDataHidesInfrastructure(t *testing.T) {
	got, err := collect(t, server(t, `{"Version":"2.39.1","InstanceID":"id-secreto"}`, endpointsBody), "token")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"192.0.2", "docker.interno", "clave-edge", "id-secreto", "token"} {
		if strings.Contains(string(body), leak) {
			t.Errorf("los datos públicos incluyen %q: %s", leak, body)
		}
	}
}

func TestPortainerErrors(t *testing.T) {
	ts := server(t, `{"Version":"2.39.1"}`, endpointsBody)
	if _, err := collect(t, ts, "malo"); integrations.Code(err) != integrations.CodeUnauthorized {
		t.Fatalf("token: %v", err)
	}
	for name, ts := range map[string]*httptest.Server{
		"estado ajeno":    server(t, `{"ok":true}`, endpointsBody),
		"entornos ajenos": server(t, `{"Version":"2.39.1"}`, `{"ok":true}`),
	} {
		if _, err := collect(t, ts, "token"); integrations.Code(err) != integrations.CodeBadResponse {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestNoEnvironments(t *testing.T) {
	got, err := collect(t, server(t, `{"Version":"2.39.1"}`, `[]`), "token")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(got)
	if !strings.Contains(string(body), `"environments":[]`) {
		t.Fatalf("sin entornos debe ser una lista vacía: %s", body)
	}
}
