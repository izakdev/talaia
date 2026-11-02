package proxmox

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

const token = "talaia@pve!dashboard=11111111-2222"

const clusterStatus = `{"data":[
{"type":"cluster","id":"cluster","name":"casa","nodes":3,"quorate":1,"version":4},
{"type":"node","id":"node/pve1","name":"pve1","online":1,"local":1,"nodeid":1,"ip":"192.0.2.1","level":""}]}`

const clusterResources = `{"data":[
{"id":"node/pve2","type":"node","node":"pve2","status":"online","cpu":0.5,"maxcpu":4,"mem":4000,"maxmem":8000,"disk":100,"maxdisk":1000},
{"id":"node/pve1","type":"node","node":"pve1","status":"online","cpu":0.1,"maxcpu":12,"mem":2000,"maxmem":8000,"disk":300,"maxdisk":1000},
{"id":"node/pve3","type":"node","node":"pve3","status":"offline","cpu":0,"maxcpu":8,"mem":0,"maxmem":0},
{"id":"storage/pve1/local","type":"storage","node":"pve1","status":"available","disk":5,"maxdisk":10},
{"id":"qemu/101","type":"qemu","node":"pve1","vmid":101,"name":"ha","status":"running","cpu":0.2,"maxcpu":2,"mem":1000,"maxmem":2000,"template":0},
{"id":"qemu/100","type":"qemu","node":"pve1","vmid":100,"name":"docker","status":"stopped","cpu":0,"maxcpu":4,"mem":0,"maxmem":4000,"template":0},
{"id":"qemu/900","type":"qemu","node":"pve1","vmid":900,"name":"plantilla","status":"stopped","template":1},
{"id":"lxc/200","type":"lxc","node":"pve2","vmid":200,"name":"mqtt","status":"running","cpu":0.01,"maxcpu":1,"mem":100,"maxmem":512},
{"id":"lxc/201","type":"lxc","node":"pve3","vmid":201,"name":"caida","status":"unknown"}
]}`

func server(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken="+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func routes() map[string]string {
	return map[string]string{
		"/api2/json/version":           `{"data":{"version":"9.0.3","release":"9.0","repoid":"abc"}}`,
		"/api2/json/cluster/status":    clusterStatus,
		"/api2/json/cluster/resources": clusterResources,
	}
}

func collect(t *testing.T, ts *httptest.Server, options map[string]bool) Data {
	t.Helper()
	got, err := New(integrations.Config{BaseURL: ts.URL, Secret: token, Options: options}).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got.(Data)
}

func TestClusterAggregates(t *testing.T) {
	d := collect(t, server(t, routes()), nil)
	if d.Version != "9.0.3" || d.ClusterName != "casa" || d.Quorate == nil || !*d.Quorate {
		t.Fatalf("clúster incorrecto: %+v", d)
	}
	if d.NodesOnline != 2 || d.NodesTotal != 3 {
		t.Fatalf("nodos %d/%d", d.NodesOnline, d.NodesTotal)
	}
	if d.VMs != (Count{1, 2}) || d.Containers != (Count{1, 2}) {
		t.Fatalf("invitados: VM %+v, LXC %+v", d.VMs, d.Containers)
	}
	// (0,5·4 + 0,1·12) / 16 = 0,2: se pondera por CPU y se ignora el nodo caído.
	if math.Abs(d.CPU-0.2) > 1e-9 || d.MaxCPU != 16 {
		t.Fatalf("CPU %v de %d", d.CPU, d.MaxCPU)
	}
	if d.MemUsed != 6000 || d.MemTotal != 16000 {
		t.Fatalf("memoria %d/%d", d.MemUsed, d.MemTotal)
	}
}

func TestNodesAndGuests(t *testing.T) {
	d := collect(t, server(t, routes()), map[string]bool{OptionShowGuests: true})
	if len(d.Nodes) != 3 || d.Nodes[0].Name != "pve1" || d.Nodes[2].Name != "pve3" {
		t.Fatalf("nodos: %+v", d.Nodes)
	}
	pve1 := d.Nodes[0]
	if !pve1.Online || pve1.DiskUsed != 300 || pve1.VMs != (Count{1, 2}) {
		t.Fatalf("pve1: %+v", pve1)
	}
	if len(pve1.Guests) != 2 || pve1.Guests[0].VMID != 100 || pve1.Guests[1].Name != "ha" {
		t.Fatalf("invitados sin ordenar o con plantillas: %+v", pve1.Guests)
	}
	pve3 := d.Nodes[2]
	if pve3.Online || pve3.Containers != (Count{0, 1}) || pve3.MemTotal != 0 {
		t.Fatalf("pve3: %+v", pve3)
	}
}

func TestGuestsAreOptional(t *testing.T) {
	raw, err := json.Marshal(collect(t, server(t, routes()), nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{`"guests"`, "docker", "mqtt", "192.0.2.1"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("incluye %q sin haberlo pedido: %s", leak, raw)
		}
	}
	if !strings.Contains(string(raw), `"vms":{"running":1,"total":2}`) {
		t.Fatalf("debe conservar los contadores: %s", raw)
	}
}

func TestStandaloneNode(t *testing.T) {
	r := routes()
	r["/api2/json/cluster/status"] = `{"data":[{"type":"node","id":"node/solo","name":"solo","online":1}]}`
	r["/api2/json/cluster/resources"] = `{"data":[{"type":"node","node":"solo","status":"online","cpu":0.25,"maxcpu":4,"mem":1,"maxmem":2}]}`
	d := collect(t, server(t, r), nil)
	if d.ClusterName != "" || d.Quorate != nil || d.NodesTotal != 1 {
		t.Fatalf("nodo independiente: %+v", d)
	}
}

func TestLostQuorum(t *testing.T) {
	r := routes()
	r["/api2/json/cluster/status"] = `{"data":[{"type":"cluster","name":"casa","quorate":0}]}`
	if d := collect(t, server(t, r), nil); d.Quorate == nil || *d.Quorate {
		t.Fatalf("quórum: %+v", d.Quorate)
	}
}

func TestErrors(t *testing.T) {
	noNodes := routes()
	noNodes["/api2/json/cluster/resources"] = `{"data":[]}`
	foreign := routes()
	foreign["/api2/json/version"] = `{"message":"hola"}`
	notJSON := routes()
	notJSON["/api2/json/cluster/status"] = `<html>`
	cases := []struct {
		name, secret string
		routes       map[string]string
		want         string
	}{
		{"token incorrecto", "otro!x=y", routes(), integrations.CodeUnauthorized},
		{"sin permisos", token, noNodes, integrations.CodeUnauthorized},
		{"otro servicio", token, foreign, integrations.CodeBadResponse},
		{"no es JSON", token, notJSON, integrations.CodeBadResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := server(t, tc.routes)
			_, err := New(integrations.Config{BaseURL: ts.URL, Secret: tc.secret}).Collect(context.Background())
			if got := integrations.Code(err); got != tc.want {
				t.Fatalf("código = %s (%v), quería %s", got, err, tc.want)
			}
		})
	}
}

func TestSelfSignedCertificate(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()
	_, err := New(integrations.Config{BaseURL: ts.URL, Secret: token}).Collect(context.Background())
	if got := integrations.Code(err); got != integrations.CodeTLS {
		t.Fatalf("código = %s (%v)", got, err)
	}
}

func TestHTTPSAgainstPlainHTTP(t *testing.T) {
	ts := server(t, routes())
	url := strings.Replace(ts.URL, "http://", "https://", 1)
	_, err := New(integrations.Config{BaseURL: url, Secret: token}).Collect(context.Background())
	if got := integrations.Code(err); got != integrations.CodeTLS {
		t.Fatalf("código = %s (%v)", got, err)
	}
}
