package nut

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
)

// Forma tomada de un upsd 2.8.1 real (NETVER 1.3), con datos inventados.
const varsBody = `BEGIN LIST VAR sai-simulado
VAR sai-simulado battery.charge "100"
VAR sai-simulado battery.charge.low "10"
VAR sai-simulado battery.runtime "2430"
VAR sai-simulado battery.type "PbAcid"
VAR sai-simulado device.mfr "Simulado"
VAR sai-simulado device.model "Simulado 1500"
VAR sai-simulado device.serial "SIM-SERIE-0001"
VAR sai-simulado driver.name "usbhid-ups"
VAR sai-simulado driver.parameter.port "auto"
VAR sai-simulado driver.parameter.serial "SIM-SERIE-0001"
VAR sai-simulado input.voltage "231.0"
VAR sai-simulado ups.load "10"
VAR sai-simulado ups.model "Simulado 1500"
VAR sai-simulado ups.serial "SIM-SERIE-0001"
VAR sai-simulado ups.status "OL CHRG"
END LIST VAR sai-simulado`

const versionLine = "Network UPS Tools upsd 2.8.1 - https://www.networkupstools.org/"

// fakeUPSD responde a cada orden con las líneas de answers; las que no están dan ERR UNKNOWN-COMMAND.
type fakeUPSD struct {
	addr     string
	mu       sync.Mutex
	received []string
}

func serve(t *testing.T, answers map[string]string) *fakeUPSD {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeUPSD{addr: ln.Addr().String()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				lines := bufio.NewScanner(conn)
				for lines.Scan() {
					cmd := lines.Text()
					f.mu.Lock()
					f.received = append(f.received, cmd)
					f.mu.Unlock()
					if cmd == "LOGOUT" {
						fmt.Fprint(conn, "OK Goodbye\n")
						return
					}
					answer, ok := answers[cmd]
					if !ok {
						answer = "ERR UNKNOWN-COMMAND"
					}
					if answer == "<silencio>" {
						continue
					}
					fmt.Fprint(conn, answer+"\n")
				}
			}()
		}
	}()
	return f
}

func (f *fakeUPSD) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.received...)
}

func standard() map[string]string {
	return map[string]string{
		"VER":                      versionLine,
		"LIST UPS":                 "BEGIN LIST UPS\nUPS sai-simulado \"SAI del rack de pruebas\"\nEND LIST UPS",
		"LIST VAR sai-simulado":    varsBody,
		"LIST VAR sai-desconocido": "ERR UNKNOWN-UPS",
		"LIST VAR sai-sin-driver":  "ERR DRIVER-NOT-CONNECTED",
		"LIST VAR sai-sin-datos":   "ERR DATA-STALE",
		"LIST VAR sai-restringido": "ERR ACCESS-DENIED",
	}
}

func collect(t *testing.T, addr, path string) (any, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return New(integrations.Config{BaseURL: "nut://" + addr + path}).Collect(ctx)
}

func TestTarget(t *testing.T) {
	ok := []struct {
		in   string
		host string
		port int
		ups  string
	}{
		{"nut://sai.simulado", "sai.simulado", 3493, ""},
		{"nut://sai.simulado/", "sai.simulado", 3493, ""},
		{"nut://192.0.2.10:3494/sai-1", "192.0.2.10", 3494, "sai-1"},
		{"nut://[2001:db8::1]/ups_rack.2", "2001:db8::1", 3493, "ups_rack.2"},
	}
	for _, c := range ok {
		host, port, ups, err := Target(c.in)
		if err != nil || host != c.host || port != c.port || ups != c.ups {
			t.Errorf("Target(%q) = %q, %d, %q, %v", c.in, host, port, ups, err)
		}
	}
	for _, bad := range []string{
		"http://sai.simulado", "nut://", "nut://usuario@sai.simulado", "nut://sai.simulado:0",
		"nut://sai.simulado:99999", "nut://sai.simulado/a/b", "nut://sai.simulado/sai%0ALOGOUT",
		"nut://sai.simulado/sai%20uno", "nut://sai.simulado?x=1", "nut://sai.simulado#x",
		"nut://sai.simulado/" + strings.Repeat("a", 65),
	} {
		if _, _, _, err := Target(bad); err == nil {
			t.Errorf("Target(%q) debería fallar", bad)
		}
	}
}

func TestParseLine(t *testing.T) {
	got, err := parseLine(`VAR sai ups.model "Modelo \"raro\" \\ 2"`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"VAR", "sai", "ups.model", `Modelo "raro" \ 2`}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("parseLine = %q", got)
	}
	if _, err := parseLine(`VAR sai x "sin cerrar`); err == nil {
		t.Fatal("una comilla sin cerrar debería fallar")
	}
}

func TestCollect(t *testing.T) {
	f := serve(t, standard())
	got, err := collect(t, f.addr, "/sai-simulado")
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Version != "2.8.1" || d.Model != "Simulado 1500" || strings.Join(d.Status, " ") != "OL CHRG" {
		t.Fatalf("datos incorrectos: %+v", d)
	}
	if d.ChargePercent == nil || *d.ChargePercent != 100 || d.RuntimeSeconds == nil || *d.RuntimeSeconds != 2430 {
		t.Fatalf("batería incorrecta: %+v", d)
	}
}

func TestCollectPicksOnlyUPS(t *testing.T) {
	f := serve(t, standard())
	got, err := collect(t, f.addr, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.(Data).Model != "Simulado 1500" {
		t.Fatalf("no ha leído el único SAI: %+v", got)
	}
}

func TestCollectMinimal(t *testing.T) {
	answers := standard()
	answers["VER"] = "ERR UNKNOWN-COMMAND"
	answers["LIST VAR sai-simulado"] = "BEGIN LIST VAR sai-simulado\nVAR sai-simulado device.model \"Otro" + strings.Repeat("ñ", 200) + "\"\nVAR sai-simulado ups.status \"OB DISCHRG XYZ\"\nEND LIST VAR sai-simulado"
	got, err := collect(t, serve(t, answers).addr, "/sai-simulado")
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Version != "" || d.Model != "Otro"+strings.Repeat("ñ", 124) || d.ChargePercent != nil || d.RuntimeSeconds != nil {
		t.Fatalf("datos incorrectos: %+v", d)
	}
	// Las banderas desconocidas no se publican.
	if strings.Join(d.Status, " ") != "OB DISCHRG" {
		t.Fatalf("banderas incorrectas: %q", d.Status)
	}
}

func TestNUTDataHidesPrivate(t *testing.T) {
	f := serve(t, standard())
	got, err := collect(t, f.addr, "")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(got)
	host, port, _ := net.SplitHostPort(f.addr)
	for _, private := range []string{"sai-simulado", "SAI del rack", "SIM-SERIE", "usbhid", "auto", host, port} {
		if strings.Contains(string(body), private) {
			t.Errorf("los datos públicos incluyen %q: %s", private, body)
		}
	}
}

func TestNUTSendsOnlyReadCommands(t *testing.T) {
	f := serve(t, standard())
	if _, err := collect(t, f.addr, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	for _, cmd := range f.commands() {
		if !strings.HasPrefix(cmd, "VER") && !strings.HasPrefix(cmd, "LIST ") && cmd != "LOGOUT" {
			t.Errorf("orden no permitida: %q", cmd)
		}
	}
}

func TestNUTErrors(t *testing.T) {
	several := standard()
	several["LIST UPS"] = "BEGIN LIST UPS\nUPS uno \"\"\nUPS dos \"\"\nEND LIST UPS"
	none := standard()
	none["LIST UPS"] = "BEGIN LIST UPS\nEND LIST UPS"
	foreign := standard()
	foreign["LIST VAR sai-simulado"] = "HTTP/1.1 400 Bad Request"
	endless := standard()
	endless["LIST VAR sai-simulado"] = "BEGIN LIST VAR sai-simulado\n" + strings.Repeat("VAR sai-simulado x \"1\"\n", maxLines+1)
	silent := standard()
	silent["LIST VAR sai-simulado"] = "<silencio>"
	cases := []struct {
		name    string
		answers map[string]string
		path    string
		code    string
	}{
		{"sai desconocido", standard(), "/sai-desconocido", CodeUPSNotFound},
		{"varios sai", several, "", CodeUPSNotFound},
		{"ningún sai", none, "", CodeUPSNotFound},
		{"acceso denegado", standard(), "/sai-restringido", integrations.CodeUnauthorized},
		{"sin driver", standard(), "/sai-sin-driver", CodeUPSStale},
		{"datos caducados", standard(), "/sai-sin-datos", CodeUPSStale},
		{"respuesta ajena", foreign, "/sai-simulado", integrations.CodeBadResponse},
		{"lista sin fin", endless, "/sai-simulado", integrations.CodeBadResponse},
		{"sin respuesta", silent, "/sai-simulado", integrations.CodeTimeout},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := collect(t, serve(t, c.answers).addr, c.path)
			if got := integrations.Code(err); got != c.code {
				t.Fatalf("código %q, se esperaba %q (%v)", got, c.code, err)
			}
		})
	}
	t.Run("sin conexión", func(t *testing.T) {
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		addr := ln.Addr().String()
		ln.Close()
		_, err := collect(t, addr, "")
		if got := integrations.Code(err); got != integrations.CodeUnreachable {
			t.Fatalf("código %q (%v)", got, err)
		}
		var e *integrations.Error
		if errors.As(err, &e) && strings.Contains(e.Code, addr) {
			t.Fatal("el código no debe llevar la dirección")
		}
	})
}
