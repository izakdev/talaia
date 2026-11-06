package wgdashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/izakdev/talaia/internal/integrations"
)

// Forma tomada de un WGDashboard 4.3.3 real, con datos inventados.
const configurationsBody = `{"status":true,"message":null,"data":[
	{"Status":true,"Name":"wg0","PrivateKey":"clave-privada","PublicKey":"clave-publica","Address":"10.9.0.1/24",
	 "ListenPort":"51820","PreUp":"","PostUp":"iptables -A FORWARD -i wg0 -j ACCEPT","SaveConfig":true,"Table":"",
	 "DataUsage":{"Total":1.5,"Sent":1.25,"Receive":0.25},"ConnectedPeers":1,"TotalPeers":3,"Protocol":"wg",
	 "Info":{"Description":"oficina","OverridePeerSettings":{"DNS":"192.0.2.53","PeerRemoteEndpoint":"vpn.interno"}}},
	{"Status":false,"Name":"awg1","PrivateKey":"otra-clave","Address":"10.10.0.1/24","ListenPort":"51821",
	 "DataUsage":{"Total":0,"Sent":0,"Receive":0},"ConnectedPeers":0,"TotalPeers":0,"Protocol":"awg","Info":{}}
]}`

func server(t *testing.T, version, configurations string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("método no permitido: %s", r.Method)
		}
		switch r.URL.Path {
		case pathVersion:
			if r.Header.Get(headerAPIKey) != "" {
				t.Errorf("la versión no necesita la clave y WGDashboard la registraría")
			}
			w.Write([]byte(version))
		case pathConfigurations:
			if r.Header.Get(headerAPIKey) != "clave" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"status":false,"message":"API Key does not exist","data":null}`))
				return
			}
			w.Write([]byte(configurations))
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

const versionBody = `{"status":true,"message":null,"data":"v4.3.3"}`

func TestCollectTunnels(t *testing.T) {
	got, err := collect(t, server(t, versionBody, configurationsBody), "clave")
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Version != "v4.3.3" || len(d.Tunnels) != 2 {
		t.Fatalf("datos incorrectos: %+v", d)
	}
	want := Tunnel{Name: "wg0", Protocol: "wg", Up: true, ConnectedPeers: 1, TotalPeers: 3, ReceivedBytes: 1 << 28, SentBytes: 5 << 28}
	if d.Tunnels[0] != want {
		t.Fatalf("túnel incorrecto:\n got %+v\nwant %+v", d.Tunnels[0], want)
	}
	if down := d.Tunnels[1]; down.Up || down.Protocol != "awg" || down.TotalPeers != 0 {
		t.Fatalf("túnel parado incorrecto: %+v", down)
	}
}

func TestWGDashboardDataHidesInfrastructure(t *testing.T) {
	got, err := collect(t, server(t, versionBody, configurationsBody), "clave")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"clave", "10.9.0", "51820", "iptables", "oficina", "192.0.2", "vpn.interno"} {
		if strings.Contains(string(body), leak) {
			t.Errorf("los datos públicos incluyen %q: %s", leak, body)
		}
	}
}

func TestWGDashboardErrors(t *testing.T) {
	ts := server(t, versionBody, configurationsBody)
	if _, err := collect(t, ts, "mala"); integrations.Code(err) != integrations.CodeUnauthorized {
		t.Fatalf("clave: %v", err)
	}
	for name, ts := range map[string]*httptest.Server{
		"versión ajena":            server(t, `{"Version":"2.39.1"}`, configurationsBody),
		"túneles ajenos":           server(t, versionBody, `[]`),
		"respuesta sin éxito":      server(t, versionBody, `{"status":false,"message":"error","data":null}`),
		"túneles que no son lista": server(t, versionBody, `{"status":true,"data":{"Name":"wg0"}}`),
	} {
		if _, err := collect(t, ts, "clave"); integrations.Code(err) != integrations.CodeBadResponse {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestNoTunnels(t *testing.T) {
	got, err := collect(t, server(t, versionBody, `{"status":true,"message":null,"data":[]}`), "clave")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(got)
	if !strings.Contains(string(body), `"tunnels":[]`) {
		t.Fatalf("sin túneles debe ser una lista vacía: %s", body)
	}
}
