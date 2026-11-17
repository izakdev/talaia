// Package qnap lee el estado de un NAS QNAP (QTS o QuTS hero) por SNMPv3.
//
// Solo hace GET y walk con autenticación y cifrado (authPriv). Los OID son los
// del MIB que publica el propio NAS (NAS-MIB, enterprises 24681, y QTS-MIB,
// enterprises 55062).
package qnap

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/izakdev/talaia/internal/integrations"
)

// OptionPrivacyAES cifra con AES en lugar de DES. QTS solo ofrece DES (CBC-DES).
const OptionPrivacyAES = "privacy_aes"

// DefaultPort es el puerto SNMP si la dirección no indica otro.
const DefaultPort = 161

// SecretSeparator separa en el secreto guardado la clave de autenticación de la
// de cifrado. Ninguna clave puede contener saltos de línea.
const SecretSeparator = "\n"

// Valores sueltos: QNAP solo los da con GET, no al recorrer el árbol.
const (
	oidCPU          = ".1.3.6.1.4.1.24681.1.3.1.0"   // systemCPU-UsageEX, %
	oidMemTotal     = ".1.3.6.1.4.1.24681.1.3.2.0"   // systemTotalMemEX, bytes
	oidMemFree      = ".1.3.6.1.4.1.24681.1.3.3.0"   // systemFreeMemEX, bytes
	oidCPUTemp      = ".1.3.6.1.4.1.24681.1.3.5.0"   // cpu-TemperatureEX, °C
	oidSysTemp      = ".1.3.6.1.4.1.24681.1.3.6.0"   // systemTemperatureEX, °C
	oidFirmware     = ".1.3.6.1.4.1.55062.1.12.6.0"  // firmwareVersion (QTS-MIB, opcional)
	oidMemAvailable = ".1.3.6.1.4.1.55062.1.12.15.0" // systemAvailableMem (QTS-MIB, opcional)
)

// Columnas de las tablas de NAS-MIB (storageSystemEx). Solo se recorren las
// públicas: nunca el modelo, el número de serie ni los nombres.
const (
	components     = ".1.3.6.1.4.1.24681.1.4.1.1.1.1"
	space          = ".1.3.6.1.4.1.24681.1.4.1.1.1.2"
	colDiskID      = components + ".5.2.1.2" // diskID: la bahía
	colDiskEnclose = components + ".5.2.1.3" // diskEnclosureID: 0 es el propio NAS
	colDiskSmart   = components + ".5.2.1.5" // diskSmartInfo: 0 bien, 1 aviso, 2 anómalo, -1 error
	colDiskTemp    = components + ".5.2.1.6" // diskTemperture, °C
	colFanStatus   = components + ".2.2.1.4" // systemFanStatus: 0 bien, -1 fallo
	colRAIDStatus  = space + ".1.2.1.5"      // raidStatus, texto
	colPoolStatus  = space + ".2.2.1.5"      // poolStatus: 0 listo, -1 aviso, -2 no listo, -3 error
	colVolTotal    = space + ".3.2.1.3"      // volumeCapacity, KiB (el MIB dice bytes)
	colVolFree     = space + ".3.2.1.4"      // volumeFreeSize, KiB
	colVolStatus   = space + ".3.2.1.5"      // volumeStatus, texto
)

// Umbrales de temperatura: SNMP no da los de QTS, así que se usan sus valores
// por defecto. Los discos usan el umbral de error de un HDD como aviso, para no
// marcar en rojo un SSD que QTS considera normal.
const (
	CPUTempWarnC    = 75
	CPUTempErrC     = 80
	SystemTempWarnC = 60
	SystemTempErrC  = 70
	DiskTempWarnC   = 60
	DiskTempErrC    = 70
)

// Salud pública de un disco.
const (
	HealthOK      = "ok"
	HealthWarning = "warning"
	HealthError   = "error"
	// HealthUnknown es un disco de una caja USB que no pasa el SMART al NAS.
	HealthUnknown = "unknown"
)

// Estado público del almacenamiento y de los volúmenes.
const (
	StatusReady    = "ready"
	StatusSyncing  = "syncing"
	StatusWarning  = "warning"
	StatusDegraded = "degraded"
	StatusError    = "error"
)

// Temperature es una temperatura con los umbrales que se aplican.
type Temperature struct {
	C     int `json:"c"`
	WarnC int `json:"warn_c"`
	ErrC  int `json:"err_c"`
}

// Disk es el estado público de un disco. Number es la bahía para los del NAS y
// un número correlativo para los de cajas USB. No incluye modelo ni número de serie.
type Disk struct {
	Number   int    `json:"number"`
	External bool   `json:"external"`
	Health   string `json:"health"`
	TempC    *int   `json:"temp_c"`
}

// Volume es el estado público de un volumen, sin su nombre.
type Volume struct {
	Number     int    `json:"number"`
	Status     string `json:"status"`
	TotalBytes int64  `json:"total_bytes"`
	FreeBytes  int64  `json:"free_bytes"`
}

// Data es la información pública del NAS. No incluye el nombre del equipo, el
// modelo, los números de serie ni las direcciones.
type Data struct {
	Version          string       `json:"version"`
	CPUPercent       float64      `json:"cpu_percent"`
	MemoryTotalBytes int64        `json:"memory_total_bytes"`
	MemoryUsedBytes  int64        `json:"memory_used_bytes"`
	CPUTemperature   *Temperature `json:"cpu_temperature,omitempty"`
	SystemTemp       *Temperature `json:"system_temperature,omitempty"`
	FanFailed        bool         `json:"fan_failed"`
	Storage          string       `json:"storage"`
	DiskTempWarnC    int          `json:"disk_temp_warn_c"`
	DiskTempErrC     int          `json:"disk_temp_err_c"`
	Disks            []Disk       `json:"disks"`
	Volumes          []Volume     `json:"volumes"`
}

// reader es lo que la integración necesita de SNMP. Se sustituye en las pruebas.
type reader interface {
	get(oids []string) (map[string]gosnmp.SnmpPDU, error)
	walk(column string) ([]gosnmp.SnmpPDU, error)
}

type client struct {
	cfg  integrations.Config
	dial func(ctx context.Context) (reader, func(), error)
}

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration {
	c := &client{cfg: cfg}
	c.dial = c.connect
	return c
}

// Target devuelve el host y el puerto de una dirección snmp://host[:puerto].
func Target(base string) (string, uint16, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "snmp" || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", 0, errors.New("la dirección debe tener la forma snmp://host[:puerto]")
	}
	port := DefaultPort
	if p := u.Port(); p != "" {
		port, err = strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return "", 0, errors.New("puerto no válido")
		}
	}
	return u.Hostname(), uint16(port), nil
}

type snmpReader struct{ g *gosnmp.GoSNMP }

func (r snmpReader) get(oids []string) (map[string]gosnmp.SnmpPDU, error) {
	packet, err := r.g.Get(oids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]gosnmp.SnmpPDU, len(packet.Variables))
	for _, v := range packet.Variables {
		out[v.Name] = v
	}
	return out, nil
}

func (r snmpReader) walk(column string) ([]gosnmp.SnmpPDU, error) {
	return r.g.BulkWalkAll(column)
}

func (c *client) connect(ctx context.Context) (reader, func(), error) {
	host, port, err := Target(c.cfg.BaseURL)
	if err != nil {
		return nil, nil, &integrations.Error{Code: integrations.CodeInternal, Err: err}
	}
	auth, priv, _ := strings.Cut(c.cfg.Secret, SecretSeparator)
	privacy := gosnmp.DES
	if c.cfg.Options[OptionPrivacyAES] {
		privacy = gosnmp.AES
	}
	g := &gosnmp.GoSNMP{
		Target:         host,
		Port:           port,
		Transport:      "udp",
		Version:        gosnmp.Version3,
		Timeout:        3 * time.Second,
		Retries:        1,
		MaxRepetitions: 20,
		Context:        ctx,
		SecurityModel:  gosnmp.UserSecurityModel,
		MsgFlags:       gosnmp.AuthPriv,
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			UserName:                 c.cfg.Username,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: auth,
			PrivacyProtocol:          privacy,
			PrivacyPassphrase:        priv,
		},
	}
	if err := g.ConnectIPv4(); err != nil {
		if err := g.Connect(); err != nil {
			return nil, nil, err
		}
	}
	return snmpReader{g}, func() { g.Conn.Close() }, nil
}

// classify traduce los errores de SNMP a los códigos públicos.
func classify(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gosnmp.ErrWrongDigest), errors.Is(err, gosnmp.ErrUnknownUsername),
		errors.Is(err, gosnmp.ErrDecryption), errors.Is(err, gosnmp.ErrUnknownSecurityLevel),
		strings.Contains(err.Error(), "not authentic"):
		return &integrations.Error{Code: integrations.CodeUnauthorized, Err: err}
	case errors.Is(err, gosnmp.ErrMaxRetriesExceeded), errors.Is(err, context.DeadlineExceeded):
		return &integrations.Error{Code: integrations.CodeTimeout, Err: err}
	}
	var e *integrations.Error
	if errors.As(err, &e) {
		return err
	}
	if integrations.Code(err) == integrations.CodeInternal {
		return &integrations.Error{Code: integrations.CodeBadResponse, Err: err}
	}
	return err
}

func badResponse(format string, args ...any) error {
	return &integrations.Error{Code: integrations.CodeBadResponse, Err: fmt.Errorf(format, args...)}
}

// Collect lee el sistema, los discos, los ventiladores y el almacenamiento.
func (c *client) Collect(ctx context.Context) (any, error) {
	r, closeConn, err := c.dial(ctx)
	if err != nil {
		return nil, classify(err)
	}
	defer closeConn()
	data, err := read(r)
	return data, classify(err)
}

func read(r reader) (Data, error) {
	values, err := r.get([]string{oidCPU, oidMemTotal, oidMemFree, oidCPUTemp, oidSysTemp, oidFirmware, oidMemAvailable})
	if err != nil {
		return Data{}, err
	}
	cpu, ok1 := number(values[oidCPU])
	total, ok2 := number(values[oidMemTotal])
	free, ok3 := number(values[oidMemFree])
	if !ok1 || !ok2 || !ok3 || total <= 0 {
		return Data{}, badResponse("el NAS no da CPU ni memoria por SNMP: no es un QNAP o no tiene NAS-MIB")
	}
	// La memoria disponible (QTS-MIB) descuenta la caché; la libre de NAS-MIB no.
	used := total - free
	if available, ok := number(values[oidMemAvailable]); ok && available > 0 && available <= total {
		used = total - available
	}
	data := Data{
		Version:          text(values[oidFirmware]),
		CPUPercent:       float64(cpu),
		MemoryTotalBytes: total,
		MemoryUsedBytes:  used,
		CPUTemperature:   temperature(values[oidCPUTemp], CPUTempWarnC, CPUTempErrC),
		SystemTemp:       temperature(values[oidSysTemp], SystemTempWarnC, SystemTempErrC),
		Storage:          StatusReady,
		DiskTempWarnC:    DiskTempWarnC,
		DiskTempErrC:     DiskTempErrC,
		Disks:            []Disk{},
		Volumes:          []Volume{},
	}

	cols, err := walkAll(r, colDiskID, colDiskEnclose, colDiskSmart, colDiskTemp, colFanStatus,
		colRAIDStatus, colPoolStatus, colVolTotal, colVolFree, colVolStatus)
	if err != nil {
		return Data{}, err
	}

	external := 0
	for _, idx := range indexes(cols[colDiskID]) {
		id, _ := number(cols[colDiskID][idx])
		enclosure, _ := number(cols[colDiskEnclose][idx])
		disk := Disk{Number: int(id), Health: diskHealth(cols[colDiskSmart][idx])}
		if enclosure != 0 {
			external++
			disk.Number, disk.External = external, true
		}
		if t, ok := number(cols[colDiskTemp][idx]); ok && t > 0 {
			v := int(t)
			disk.TempC = &v
		}
		// Una caja USB sin sensor no pasa el SMART al NAS: QNAP la da como
		// «Abnormal» (con -1 o 2), aunque QTS la muestre lista y sin datos de salud.
		if disk.External && disk.TempC == nil {
			disk.Health = HealthUnknown
		}
		data.Disks = append(data.Disks, disk)
	}
	for _, pdu := range cols[colFanStatus] {
		if status, ok := number(pdu); ok && status == -1 {
			data.FanFailed = true
		}
	}
	for _, pdu := range cols[colRAIDStatus] {
		data.Storage = worst(data.Storage, raidStatus(text(pdu)))
	}
	for _, pdu := range cols[colPoolStatus] {
		status, _ := number(pdu)
		data.Storage = worst(data.Storage, poolStatus(status))
	}
	for i, idx := range indexes(cols[colVolStatus]) {
		totalKiB, _ := number(cols[colVolTotal][idx])
		freeKiB, _ := number(cols[colVolFree][idx])
		data.Volumes = append(data.Volumes, Volume{
			Number:     i + 1,
			Status:     volumeStatus(text(cols[colVolStatus][idx])),
			TotalBytes: totalKiB * 1024,
			FreeBytes:  freeKiB * 1024,
		})
	}
	return data, nil
}

// walkAll recorre cada columna y la indexa por el sufijo del OID (la fila).
func walkAll(r reader, columns ...string) (map[string]map[string]gosnmp.SnmpPDU, error) {
	out := make(map[string]map[string]gosnmp.SnmpPDU, len(columns))
	for _, col := range columns {
		pdus, err := r.walk(col)
		if err != nil {
			return nil, err
		}
		rows := make(map[string]gosnmp.SnmpPDU, len(pdus))
		for _, pdu := range pdus {
			if idx, ok := strings.CutPrefix(pdu.Name, col+"."); ok {
				rows[idx] = pdu
			}
		}
		out[col] = rows
	}
	return out, nil
}

// indexes ordena las filas por su índice numérico.
func indexes(rows map[string]gosnmp.SnmpPDU) []string {
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, _ := strconv.Atoi(keys[i])
		b, _ := strconv.Atoi(keys[j])
		return a < b
	})
	return keys
}

// number lee un entero de un valor SNMP; falso si no existe o no es un número.
func number(pdu gosnmp.SnmpPDU) (int64, bool) {
	switch pdu.Type {
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Counter64, gosnmp.Gauge32, gosnmp.Uinteger32, gosnmp.TimeTicks:
		n := gosnmp.ToBigInt(pdu.Value)
		if n == nil || !n.IsInt64() {
			return 0, false
		}
		return n.Int64(), true
	case gosnmp.OctetString:
		// Algunos firmwares dan números como texto.
		if b, ok := pdu.Value.([]byte); ok {
			n, ok := new(big.Int).SetString(strings.TrimSpace(string(b)), 10)
			if ok && n.IsInt64() {
				return n.Int64(), true
			}
		}
	}
	return 0, false
}

func text(pdu gosnmp.SnmpPDU) string {
	if b, ok := pdu.Value.([]byte); ok && pdu.Type == gosnmp.OctetString {
		return strings.TrimSpace(string(b))
	}
	return ""
}

// temperature descarta 0 y negativos: QNAP los da cuando no hay sensor.
func temperature(pdu gosnmp.SnmpPDU, warn, crit int) *Temperature {
	c, ok := number(pdu)
	if !ok || c <= 0 {
		return nil
	}
	return &Temperature{C: int(c), WarnC: warn, ErrC: crit}
}

// diskHealth traduce diskSmartInfo. Un valor desconocido es un aviso.
func diskHealth(pdu gosnmp.SnmpPDU) string {
	v, ok := number(pdu)
	switch {
	case ok && v == 0:
		return HealthOK
	case ok && (v == 2 || v == -1):
		return HealthError
	default:
		return HealthWarning
	}
}

// poolStatus traduce poolStatus. Los positivos son estados de trabajo de QuTS
// hero o de QTS (Qtier, comprobación…) que no indican un fallo.
func poolStatus(v int64) string {
	switch {
	case v == -1:
		return StatusWarning
	case v < -1:
		return StatusError
	default:
		return StatusReady
	}
}

func contains(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// raidStatus traduce el texto del estado de un grupo RAID.
func raidStatus(s string) string {
	s = strings.ToLower(s)
	switch {
	case s == "" || s == "ready" || s == "good" || s == "normal":
		return StatusReady
	case contains(s, "degrad"):
		return StatusDegraded
	case contains(s, "rebuild", "resync", "sync", "recover", "resilver", "migrat", "expand", "reshap", "scrub"):
		return StatusSyncing
	case contains(s, "fail", "error", "not", "offline", "inactive", "crash"):
		return StatusError
	default:
		return StatusWarning
	}
}

// volumeStatus traduce el texto del estado de un volumen.
func volumeStatus(s string) string {
	s = strings.ToLower(s)
	switch {
	case s == "ready" || s == "good" || s == "normal":
		return StatusReady
	case contains(s, "degrad"):
		return StatusDegraded
	case contains(s, "fail", "error", "not", "unmount", "inactive", "read only", "read-only", "readonly"):
		return StatusError
	case contains(s, "rebuild", "sync", "check", "expand", "migrat", "creat", "initial"):
		return StatusSyncing
	default:
		return StatusWarning
	}
}

var severity = map[string]int{StatusReady: 0, StatusSyncing: 1, StatusWarning: 2, StatusDegraded: 3, StatusError: 4}

func worst(a, b string) string {
	if severity[b] > severity[a] {
		return b
	}
	return a
}
