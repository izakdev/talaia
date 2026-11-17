package qnap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/gosnmp/gosnmp"

	"github.com/izakdev/talaia/internal/integrations"
)

// nas simula el agente SNMP de un QuTS hero 5.2 real, con datos inventados. Las
// columnas de modelo, serie y nombre existen, como en el NAS, para comprobar que
// nunca se leen.
type nas struct {
	scalars map[string]gosnmp.SnmpPDU
	tables  map[string]gosnmp.SnmpPDU
	walked  []string
	err     error
}

func integer(v int) gosnmp.SnmpPDU    { return gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: v} }
func counter(v uint64) gosnmp.SnmpPDU { return gosnmp.SnmpPDU{Type: gosnmp.Counter64, Value: v} }
func octets(v string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte(v)}
}
func missing() gosnmp.SnmpPDU                                         { return gosnmp.SnmpPDU{Type: gosnmp.NoSuchObject} }
func set(m map[string]gosnmp.SnmpPDU, oid string, pdu gosnmp.SnmpPDU) { pdu.Name = oid; m[oid] = pdu }

func newNAS() *nas {
	n := &nas{scalars: map[string]gosnmp.SnmpPDU{}, tables: map[string]gosnmp.SnmpPDU{}}
	set(n.scalars, oidCPU, integer(12))
	set(n.scalars, oidMemTotal, counter(16_000_000_000))
	set(n.scalars, oidMemFree, counter(10_000_000_000))
	set(n.scalars, oidMemAvailable, counter(8_000_000_000))
	set(n.scalars, oidCPUTemp, integer(54))
	set(n.scalars, oidSysTemp, integer(32))
	set(n.scalars, oidFirmware, octets("5.2.9.1234"))
	set(n.scalars, ".1.3.6.1.4.1.55062.1.12.4.0", octets("nas-salon"))
	// Discos: bahía 1 (SSD), 3 y 4 en el NAS; uno en una caja USB.
	disks := []struct {
		id, enclosure, smart, temp int
	}{{1, 0, 0, 36}, {3, 0, 1, 31}, {4, 0, 2, 64}, {2, 33, -1, -1}, {3, 33, 2, -1}, {1, 32, 2, 41}}
	for i, d := range disks {
		row := fmt.Sprintf(".%d", i+1)
		set(n.tables, colDiskID+row, integer(d.id))
		set(n.tables, colDiskEnclose+row, integer(d.enclosure))
		set(n.tables, colDiskSmart+row, integer(d.smart))
		set(n.tables, colDiskTemp+row, integer(d.temp))
		set(n.tables, components+".5.2.1.8"+row, octets("MODELO-SIMULADO S3RIE"))
	}
	set(n.tables, colFanStatus+".1", integer(0))
	set(n.tables, colRAIDStatus+".1", octets("Ready"))
	set(n.tables, colRAIDStatus+".2", octets("Ready"))
	set(n.tables, colPoolStatus+".1", integer(0))
	set(n.tables, colVolTotal+".1", counter(10_000_000))
	set(n.tables, colVolFree+".1", counter(2_500_000))
	set(n.tables, colVolStatus+".1", octets("Ready"))
	set(n.tables, space+".3.2.1.8.1", octets("Fotos-familia"))
	set(n.tables, colVolTotal+".2", counter(4_000_000))
	set(n.tables, colVolFree+".2", counter(3_000_000))
	set(n.tables, colVolStatus+".2", octets("Ready"))
	return n
}

func (n *nas) get(oids []string) (map[string]gosnmp.SnmpPDU, error) {
	if n.err != nil {
		return nil, n.err
	}
	out := map[string]gosnmp.SnmpPDU{}
	for _, oid := range oids {
		pdu, ok := n.scalars[oid]
		if !ok {
			pdu = missing()
			pdu.Name = oid
		}
		out[oid] = pdu
	}
	return out, nil
}

func (n *nas) walk(column string) ([]gosnmp.SnmpPDU, error) {
	n.walked = append(n.walked, column)
	var out []gosnmp.SnmpPDU
	for oid, pdu := range n.tables {
		if strings.HasPrefix(oid, column+".") {
			out = append(out, pdu)
		}
	}
	return out, nil
}

func collect(t *testing.T, n *nas) (Data, error) {
	t.Helper()
	c := New(integrations.Config{BaseURL: "snmp://nas.simulado", Username: "talaia", Secret: "clave-auth\nclave-priv"}).(*client)
	c.dial = func(context.Context) (reader, func(), error) { return n, func() {}, nil }
	got, err := c.Collect(context.Background())
	if err != nil {
		return Data{}, err
	}
	return got.(Data), nil
}

func TestCollectQNAP(t *testing.T) {
	d, err := collect(t, newNAS())
	if err != nil {
		t.Fatal(err)
	}
	if d.Version != "5.2.9.1234" || d.CPUPercent != 12 || d.FanFailed || d.Storage != StatusReady {
		t.Fatalf("sistema incorrecto: %+v", d)
	}
	// Usada = total - disponible, no total - libre (que incluye la caché).
	if d.MemoryTotalBytes != 16_000_000_000 || d.MemoryUsedBytes != 8_000_000_000 {
		t.Fatalf("memoria incorrecta: %+v", d)
	}
	if *d.CPUTemperature != (Temperature{54, CPUTempWarnC, CPUTempErrC}) || *d.SystemTemp != (Temperature{32, SystemTempWarnC, SystemTempErrC}) {
		t.Fatalf("temperaturas incorrectas: %+v %+v", d.CPUTemperature, d.SystemTemp)
	}
	if len(d.Disks) != 6 {
		t.Fatalf("discos incorrectos: %+v", d.Disks)
	}
	want := []struct {
		number   int
		external bool
		health   string
		temp     int
	}{
		{1, false, HealthOK, 36}, {3, false, HealthWarning, 31}, {4, false, HealthError, 64},
		// Como en el NAS real: las cajas USB sin sensor dan «Abnormal» (-1 o 2) sin tener SMART.
		{1, true, HealthUnknown, 0}, {2, true, HealthUnknown, 0},
		// Una caja USB que sí da temperatura pasa el SMART, y se respeta.
		{3, true, HealthError, 41},
	}
	for i, w := range want {
		got := d.Disks[i]
		temp := 0
		if got.TempC != nil {
			temp = *got.TempC
		}
		if got.Number != w.number || got.External != w.external || got.Health != w.health || temp != w.temp {
			t.Fatalf("disco %d: got %+v, want %+v", i, got, w)
		}
	}
	if d.Disks[3].TempC != nil {
		t.Fatal("una temperatura negativa es que no hay sensor")
	}
	wantVolumes := []Volume{
		{Number: 1, Status: StatusReady, TotalBytes: 10_000_000 * 1024, FreeBytes: 2_500_000 * 1024},
		{Number: 2, Status: StatusReady, TotalBytes: 4_000_000 * 1024, FreeBytes: 3_000_000 * 1024},
	}
	if len(d.Volumes) != 2 || d.Volumes[0] != wantVolumes[0] || d.Volumes[1] != wantVolumes[1] {
		t.Fatalf("volúmenes:\n got %+v\nwant %+v", d.Volumes, wantVolumes)
	}
}

func TestQNAPWithoutQTSMIB(t *testing.T) {
	n := newNAS()
	delete(n.scalars, oidFirmware)
	delete(n.scalars, oidMemAvailable)
	d, err := collect(t, n)
	if err != nil {
		t.Fatal(err)
	}
	if d.Version != "" || d.MemoryUsedBytes != 6_000_000_000 {
		t.Fatalf("sin QTS-MIB se usa la memoria libre: %+v", d)
	}
}

func TestQNAPStorageStates(t *testing.T) {
	cases := []struct {
		name, raid string
		pool       int
		volume     string
		storage    string
		volStatus  string
	}{
		{"reconstruyendo", "Rebuilding", 0, "Ready", StatusSyncing, StatusReady},
		{"degradado", "Degraded", -1, "Ready", StatusDegraded, StatusReady},
		{"pool en aviso", "Ready", -1, "Ready", StatusWarning, StatusReady},
		{"pool en error", "Ready", -3, "Not ready", StatusError, StatusError},
		{"comprobando", "Ready", 4, "Checking", StatusReady, StatusSyncing},
		{"estado desconocido", "Raro", 0, "Raro", StatusWarning, StatusWarning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := newNAS()
			set(n.tables, colRAIDStatus+".1", octets(tc.raid))
			set(n.tables, colPoolStatus+".1", integer(tc.pool))
			set(n.tables, colVolStatus+".1", octets(tc.volume))
			d, err := collect(t, n)
			if err != nil {
				t.Fatal(err)
			}
			if d.Storage != tc.storage || d.Volumes[0].Status != tc.volStatus {
				t.Fatalf("almacenamiento %q y volumen %q, se esperaba %q y %q", d.Storage, d.Volumes[0].Status, tc.storage, tc.volStatus)
			}
		})
	}
}

func TestQNAPFanFailure(t *testing.T) {
	n := newNAS()
	set(n.tables, colFanStatus+".1", integer(-1))
	d, err := collect(t, n)
	if err != nil {
		t.Fatal(err)
	}
	if !d.FanFailed {
		t.Fatal("el ventilador averiado no se detecta")
	}
}

func TestQNAPDataHidesInfrastructure(t *testing.T) {
	n := newNAS()
	d, err := collect(t, n)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(d)
	for _, secret := range []string{"clave-auth", "clave-priv", "talaia", "nas.simulado", "nas-salon", "MODELO", "S3RIE", "Fotos-familia"} {
		if strings.Contains(string(out), secret) {
			t.Fatalf("los datos públicos contienen %q: %s", secret, out)
		}
	}
	for _, col := range n.walked {
		if strings.HasSuffix(col, ".8") {
			t.Fatalf("no se recorren las columnas de modelo ni de nombre: %s", col)
		}
	}
}

func TestQNAPErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{"clave de autenticación mala", gosnmp.ErrWrongDigest, integrations.CodeUnauthorized},
		{"usuario desconocido", fmt.Errorf("informe: %w", gosnmp.ErrUnknownUsername), integrations.CodeUnauthorized},
		{"clave de cifrado mala", gosnmp.ErrDecryption, integrations.CodeUnauthorized},
		{"firma incorrecta", errors.New("incoming packet is not authentic, discarding"), integrations.CodeUnauthorized},
		{"sin respuesta", fmt.Errorf("request timeout (after 1 retries): %w", gosnmp.ErrMaxRetriesExceeded), integrations.CodeTimeout},
		{"red inalcanzable", &net.OpError{Op: "dial", Err: errors.New("no route")}, integrations.CodeUnreachable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := newNAS()
			n.err = tc.err
			_, err := collect(t, n)
			if integrations.Code(err) != tc.code {
				t.Fatalf("código %q, se esperaba %q: %v", integrations.Code(err), tc.code, err)
			}
		})
	}
	t.Run("no es un QNAP", func(t *testing.T) {
		n := newNAS()
		n.scalars = map[string]gosnmp.SnmpPDU{}
		_, err := collect(t, n)
		if integrations.Code(err) != integrations.CodeBadResponse {
			t.Fatalf("código %q: %v", integrations.Code(err), err)
		}
	})
}

func TestTarget(t *testing.T) {
	for in, want := range map[string]string{"snmp://nas.simulado": "nas.simulado:161", "snmp://192.0.2.10:1161": "192.0.2.10:1161", "snmp://nas.simulado/": "nas.simulado:161"} {
		host, port, err := Target(in)
		if err != nil || fmt.Sprintf("%s:%d", host, port) != want {
			t.Fatalf("%s: %s:%d %v", in, host, port, err)
		}
	}
	for _, bad := range []string{"http://nas.simulado", "snmp://", "snmp://u:p@nas.simulado", "snmp://nas.simulado/ruta", "snmp://nas.simulado:0", "snmp://nas.simulado?x=1"} {
		if _, _, err := Target(bad); err == nil {
			t.Fatalf("%s debería rechazarse", bad)
		}
	}
}
