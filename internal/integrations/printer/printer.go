// Package printer lee el estado y los consumibles de una impresora por IPP.
//
// Solo envía la operación de consulta Get-Printer-Attributes (RFC 8011,
// apartado 4.2.5), sin credenciales: las impresoras la responden a cualquiera
// en la red local. Los consumibles salen de los atributos marker-* de CUPS
// (PWG 5100.13 los recoge como extensión), que publican las impresoras AirPrint
// y Mopria. Referencias: https://www.rfc-editor.org/rfc/rfc8011 y
// https://ftp.pwg.org/pub/pwg/candidates/cs-ippjobprinterext3v10-20120727-5100.13.pdf
package printer

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/izakdev/talaia/internal/integrations"
)

// DefaultPort es el puerto de IPP si la dirección no indica otro.
const DefaultPort = 631

// DefaultPath es la ruta de IPP Everywhere y AirPrint.
const DefaultPath = "/ipp/print"

// Límites de la respuesta, para que un servicio ajeno no bloquee la consulta.
const (
	maxResponse = 1 << 20
	maxText     = 128
	maxMarkers  = 16
	maxReasons  = 16
)

// Estados de printer-state (RFC 8011, 5.4.11).
const (
	StateIdle       = "idle"
	StateProcessing = "processing"
	StateStopped    = "stopped"
)

// Severidades de un motivo de printer-state-reasons (RFC 8011, 5.4.12).
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// requested son los atributos que se piden; el resto (nombre, ubicación,
// UUID, direcciones…) ni se pide ni se lee.
var requested = []string{
	"printer-state", "printer-state-reasons", "printer-is-accepting-jobs",
	"printer-make-and-model", "printer-firmware-string-version",
	"marker-names", "marker-colors", "marker-types", "marker-levels", "marker-low-levels",
}

// knownReasons son los motivos que se publican con su nombre; cualquier otro
// sale como «other». Vienen de RFC 8011 y PWG 5100.9.
var knownReasons = []string{
	"media-jam", "media-empty", "media-needed", "media-low",
	"door-open", "cover-open", "interlock-open",
	"toner-empty", "toner-low", "marker-supply-empty", "marker-supply-low",
	"marker-waste-full", "marker-waste-almost-full", "developer-empty", "developer-low",
	"opc-life-over", "opc-near-eol", "fuser-over-temp",
	"input-tray-missing", "output-tray-missing", "output-area-full", "output-area-almost-full",
	"paused", "shutdown", "spool-area-full", "stopped-partly", "timed-out", "other",
}

// knownTypes son los tipos de marker-types que se publican; el resto, «other».
var knownTypes = []string{"toner", "ink", "ink-cartridge", "toner-cartridge", "opc", "developer", "waste-toner", "waste-ink", "fuser", "transfer-unit", "staples", "other"}

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Reason es un motivo activo de printer-state-reasons, sin los «-report».
type Reason struct {
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

// Marker es un consumible. Level es el porcentaje, o nil si la impresora no
// lo sabe (marker-levels -2 o -3).
type Marker struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Type  string `json:"type"`
	Level *int   `json:"level"`
	Low   int    `json:"low"`
}

// Data es la información pública de la impresora. No incluye su nombre, su
// ubicación, su UUID, sus direcciones ni el mensaje libre de su pantalla.
type Data struct {
	Model         string   `json:"model"`
	Firmware      string   `json:"firmware"`
	Status        string   `json:"status"`
	AcceptingJobs bool     `json:"accepting_jobs"`
	Reasons       []Reason `json:"reasons"`
	Markers       []Marker `json:"markers"`
}

type client struct {
	cfg  integrations.Config
	http *http.Client
}

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration {
	return &client{cfg, integrations.NewHTTPClient(cfg)}
}

// Target devuelve la URL HTTP de la petición y la printer-uri de una dirección
// ipp://host[:puerto][/ruta] o ipps://host[:puerto][/ruta].
func Target(base string) (string, string, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "ipp" && u.Scheme != "ipps") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", errors.New("la dirección debe tener la forma ipp://host[:puerto][/ruta]")
	}
	port := DefaultPort
	if p := u.Port(); p != "" {
		port, err = strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return "", "", errors.New("puerto no válido")
		}
	}
	path := u.EscapedPath()
	if path == "" || path == "/" {
		path = DefaultPath
	}
	host := u.Hostname()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	scheme := "http"
	if u.Scheme == "ipps" {
		scheme = "https"
	}
	authority := host + ":" + strconv.Itoa(port)
	return scheme + "://" + authority + path, u.Scheme + "://" + authority + path, nil
}

func fail(code, format string, args ...any) error {
	return &integrations.Error{Code: code, Err: fmt.Errorf(format, args...)}
}

// clip recorta un texto a maxText caracteres sin partir ninguno.
func clip(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxText {
		return string(r[:maxText])
	}
	return s
}

// Etiquetas de valor de IPP (RFC 8010, 3.5).
const (
	tagOperation = 0x01
	tagEnd       = 0x03
	tagInteger   = 0x21
	tagBoolean   = 0x22
	tagEnum      = 0x23
	tagBegColl   = 0x34
	tagEndColl   = 0x37
	tagTextLang  = 0x35
	tagNameLang  = 0x36
	tagKeyword   = 0x44
	tagURI       = 0x45
	tagCharset   = 0x47
	tagLanguage  = 0x48
)

func writeAttr(b *bytes.Buffer, tag byte, name, value string) {
	b.WriteByte(tag)
	binary.Write(b, binary.BigEndian, uint16(len(name)))
	b.WriteString(name)
	binary.Write(b, binary.BigEndian, uint16(len(value)))
	b.WriteString(value)
}

// request codifica Get-Printer-Attributes (operación 0x000B) en IPP 2.0.
func request(printerURI string) []byte {
	var b bytes.Buffer
	b.Write([]byte{2, 0, 0x00, 0x0B, 0, 0, 0, 1, tagOperation})
	writeAttr(&b, tagCharset, "attributes-charset", "utf-8")
	writeAttr(&b, tagLanguage, "attributes-natural-language", "en")
	writeAttr(&b, tagURI, "printer-uri", printerURI)
	for i, name := range requested {
		attr := "requested-attributes"
		if i > 0 {
			attr = "" // valores adicionales del mismo atributo
		}
		writeAttr(&b, tagKeyword, attr, name)
	}
	b.WriteByte(tagEnd)
	return b.Bytes()
}

// value es un valor de un atributo: su etiqueta y sus bytes.
type value struct {
	tag byte
	raw []byte
}

func (v value) text() string {
	raw := v.raw
	if (v.tag == tagTextLang || v.tag == tagNameLang) && len(raw) >= 4 {
		n := int(binary.BigEndian.Uint16(raw))
		if 4+n <= len(raw) {
			raw = raw[4+n:]
		}
	}
	return string(raw)
}

func (v value) integer() (int, bool) {
	if (v.tag != tagInteger && v.tag != tagEnum) || len(v.raw) != 4 {
		return 0, false
	}
	return int(int32(binary.BigEndian.Uint32(v.raw))), true
}

// parse lee una respuesta IPP y devuelve el status-code y los atributos de
// primer nivel (los miembros de las colecciones se saltan).
func parse(data []byte) (int, map[string][]value, error) {
	if len(data) < 9 {
		return 0, nil, errors.New("respuesta demasiado corta")
	}
	if data[0] < 1 || data[0] > 2 {
		return 0, nil, fmt.Errorf("versión de IPP %d", data[0])
	}
	status := int(binary.BigEndian.Uint16(data[2:4]))
	attrs := map[string][]value{}
	last, depth := "", 0
	for i := 8; i < len(data); {
		tag := data[i]
		i++
		if tag == tagEnd {
			return status, attrs, nil
		}
		if tag < 0x10 {
			last = ""
			continue
		}
		if i+2 > len(data) {
			break
		}
		nl := int(binary.BigEndian.Uint16(data[i:]))
		i += 2
		if i+nl+2 > len(data) {
			break
		}
		name := string(data[i : i+nl])
		i += nl
		vl := int(binary.BigEndian.Uint16(data[i:]))
		i += 2
		if i+vl > len(data) {
			break
		}
		raw := data[i : i+vl]
		i += vl
		switch {
		case tag == tagBegColl:
			depth++
			continue
		case tag == tagEndColl:
			depth--
			continue
		case depth > 0:
			continue
		}
		if name != "" {
			last = name
		}
		if last == "" {
			return 0, nil, errors.New("valor sin atributo")
		}
		attrs[last] = append(attrs[last], value{tag, raw})
	}
	return 0, nil, errors.New("respuesta cortada")
}

// Collect consulta los atributos de la impresora.
func (c *client) Collect(ctx context.Context) (any, error) {
	endpoint, printerURI, err := Target(c.cfg.BaseURL)
	if err != nil {
		return nil, fail(integrations.CodeInternal, "%v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(request(printerURI)))
	if err != nil {
		return nil, fail(integrations.CodeInternal, "%v", err)
	}
	req.Header.Set("Content-Type", "application/ipp")
	req.Header.Set("Accept", "application/ipp")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return nil, fail(integrations.CodeUnauthorized, "HTTP %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fail(integrations.CodeBadResponse, "HTTP %d", resp.StatusCode)
	case !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/ipp"):
		return nil, fail(integrations.CodeBadResponse, "no es una respuesta IPP")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponse {
		return nil, fail(integrations.CodeBadResponse, "respuesta demasiado grande")
	}
	status, attrs, err := parse(body)
	if err != nil {
		return nil, fail(integrations.CodeBadResponse, "%v", err)
	}
	switch {
	case status == 0x0401 || status == 0x0402 || status == 0x0403:
		return nil, fail(integrations.CodeUnauthorized, "IPP 0x%04x", status)
	case status > 0x00FF:
		return nil, fail(integrations.CodeBadResponse, "IPP 0x%04x", status)
	}
	return decode(attrs)
}

func decode(attrs map[string][]value) (Data, error) {
	data := Data{Reasons: []Reason{}, Markers: []Marker{}}
	state, ok := first(attrs, "printer-state").integer()
	if !ok {
		return data, fail(integrations.CodeBadResponse, "sin printer-state")
	}
	switch state {
	case 3:
		data.Status = StateIdle
	case 4:
		data.Status = StateProcessing
	case 5:
		data.Status = StateStopped
	default:
		return data, fail(integrations.CodeBadResponse, "printer-state %d", state)
	}
	if v := first(attrs, "printer-is-accepting-jobs"); v.tag == tagBoolean && len(v.raw) == 1 {
		data.AcceptingJobs = v.raw[0] == 1
	}
	data.Model = clip(first(attrs, "printer-make-and-model").text())
	data.Firmware = clip(first(attrs, "printer-firmware-string-version").text())

	for _, v := range attrs["printer-state-reasons"] {
		if r, ok := parseReason(v.text()); ok && len(data.Reasons) < maxReasons && !slices.Contains(data.Reasons, r) {
			data.Reasons = append(data.Reasons, r)
		}
	}

	names, colors, types := attrs["marker-names"], attrs["marker-colors"], attrs["marker-types"]
	levels, lows := attrs["marker-levels"], attrs["marker-low-levels"]
	for i := 0; i < len(names) && i < maxMarkers; i++ {
		m := Marker{Name: clip(names[i].text()), Type: "other", Low: 10}
		if i < len(colors) && hexColor.MatchString(colors[i].text()) {
			m.Color = strings.ToUpper(colors[i].text())
		}
		if i < len(types) && slices.Contains(knownTypes, types[i].text()) {
			m.Type = types[i].text()
		}
		if i < len(levels) {
			if l, ok := levels[i].integer(); ok && l >= 0 && l <= 100 {
				m.Level = &l
			}
		}
		if i < len(lows) {
			if l, ok := lows[i].integer(); ok && l >= 0 && l <= 100 {
				m.Low = l
			}
		}
		data.Markers = append(data.Markers, m)
	}
	return data, nil
}

func first(attrs map[string][]value, name string) value {
	if v := attrs[name]; len(v) > 0 {
		return v[0]
	}
	return value{}
}

// parseReason separa el motivo de su sufijo. «none», los «-report» y el fusor
// calentándose (normal al salir del reposo) no se publican; sin sufijo, el
// motivo es un error (RFC 8011, 5.4.12).
func parseReason(s string) (Reason, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "none" || strings.HasSuffix(s, "-report") {
		return Reason{}, false
	}
	r := Reason{Reason: s, Severity: SeverityError}
	if base, ok := strings.CutSuffix(s, "-warning"); ok {
		r = Reason{Reason: base, Severity: SeverityWarning}
	} else if base, ok := strings.CutSuffix(s, "-error"); ok {
		r.Reason = base
	}
	if r.Reason == "fuser-under-temp" {
		return Reason{}, false
	}
	if !slices.Contains(knownReasons, r.Reason) {
		r.Reason = "other"
	}
	return r, true
}
