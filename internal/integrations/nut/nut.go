// Package nut lee el estado de un SAI desde un servidor NUT (upsd).
//
// Habla el protocolo de red de NUT (texto sobre TCP, puerto 3493) y solo envía
// órdenes de consulta: VER, LIST UPS, LIST VAR y LOGOUT. NUT no exige usuario
// para leer; upsd.users solo hace falta para enviar órdenes al SAI, que Talaia
// nunca envía. Referencia: https://networkupstools.org/docs/developer-guide.chunked/net-protocol.html
package nut

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
)

// DefaultPort es el puerto de upsd si la dirección no indica otro.
const DefaultPort = 3493

// Códigos de error propios, además de los comunes de integrations.
const (
	// CodeUPSNotFound: el SAI indicado no existe, o no se indicó y el servidor
	// tiene ninguno o varios.
	CodeUPSNotFound = "ups_not_found"
	// CodeUPSStale: upsd responde pero no recibe datos del driver del SAI.
	CodeUPSStale = "ups_stale"
)

// Límites de la respuesta, para que un servidor ajeno no bloquee la consulta.
const (
	maxLineBytes   = 4096
	maxLines       = 2000
	defaultTimeout = 10 * time.Second
	// maxText recorta los textos publicados (modelo y versión).
	maxText = 128
)

// upsName es el formato de los nombres de SAI de ups.conf. Impide además
// colar otra orden en la consulta con un salto de línea.
var upsName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

var upsdVersion = regexp.MustCompile(`upsd (\S+)`)

// knownFlags son las banderas de ups.status que se publican; el resto se descarta.
var knownFlags = []string{"OL", "OB", "LB", "HB", "RB", "CHRG", "DISCHRG", "BYPASS", "CAL", "OFF", "OVER", "TRIM", "BOOST", "FSD"}

// Data es la información pública del SAI. No incluye su nombre en upsd, su
// descripción, el número de serie, el driver ni la dirección del servidor.
type Data struct {
	Version        string   `json:"version"`
	Model          string   `json:"model"`
	Status         []string `json:"status"`
	ChargePercent  *float64 `json:"charge_percent"`
	RuntimeSeconds *int     `json:"runtime_seconds"`
}

type client struct{ cfg integrations.Config }

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration { return &client{cfg} }

// Target devuelve el host, el puerto y el SAI (vacío si no se indica) de una
// dirección nut://host[:puerto][/sai].
func Target(base string) (string, int, string, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "nut" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", 0, "", errors.New("la dirección debe tener la forma nut://host[:puerto][/sai]")
	}
	port := DefaultPort
	if p := u.Port(); p != "" {
		port, err = strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return "", 0, "", errors.New("puerto no válido")
		}
	}
	ups := strings.TrimPrefix(u.Path, "/")
	if ups != "" && !upsName.MatchString(ups) {
		return "", 0, "", errors.New("nombre de SAI no válido")
	}
	return u.Hostname(), port, ups, nil
}

func fail(code, format string, args ...any) error {
	return &integrations.Error{Code: code, Err: fmt.Errorf(format, args...)}
}

// protocolError traduce un ERR de upsd a un código público.
func protocolError(line string) error {
	switch strings.TrimSpace(strings.TrimPrefix(line, "ERR")) {
	case "ACCESS-DENIED":
		return fail(integrations.CodeUnauthorized, "upsd: %s", line)
	case "UNKNOWN-UPS":
		return fail(CodeUPSNotFound, "upsd: %s", line)
	case "DATA-STALE", "DRIVER-NOT-CONNECTED":
		return fail(CodeUPSStale, "upsd: %s", line)
	}
	return fail(integrations.CodeBadResponse, "upsd: %s", line)
}

// parseLine separa una línea en palabras; las que van entre comillas admiten
// \" y \\ como escapes.
func parseLine(line string) ([]string, error) {
	var words []string
	for i := 0; i < len(line); {
		switch {
		case line[i] == ' ':
			i++
		case line[i] == '"':
			var b strings.Builder
			i++
			closed := false
			for i < len(line) && !closed {
				switch c := line[i]; {
				case c == '\\' && i+1 < len(line):
					b.WriteByte(line[i+1])
					i += 2
				case c == '"':
					closed = true
					i++
				default:
					b.WriteByte(c)
					i++
				}
			}
			if !closed {
				return nil, errors.New("comillas sin cerrar")
			}
			words = append(words, b.String())
		default:
			end := strings.IndexByte(line[i:], ' ')
			if end < 0 {
				end = len(line) - i
			}
			words = append(words, line[i:i+end])
			i += end
		}
	}
	return words, nil
}

// clip recorta un texto a maxText caracteres sin partir ninguno.
func clip(s string) string {
	if r := []rune(s); len(r) > maxText {
		return string(r[:maxText])
	}
	return s
}

type session struct {
	conn  net.Conn
	lines *bufio.Scanner
}

func (s *session) send(cmd string) error {
	_, err := fmt.Fprintf(s.conn, "%s\n", cmd)
	return err
}

func (s *session) readLine() (string, error) {
	if !s.lines.Scan() {
		if err := s.lines.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				return "", fail(integrations.CodeBadResponse, "línea demasiado larga")
			}
			return "", err
		}
		return "", fail(integrations.CodeBadResponse, "upsd ha cerrado la conexión")
	}
	return strings.TrimSuffix(s.lines.Text(), "\r"), nil
}

// list envía LIST <what> y devuelve las líneas entre BEGIN y END, ya separadas.
func (s *session) list(what string) ([][]string, error) {
	if err := s.send("LIST " + what); err != nil {
		return nil, err
	}
	first, err := s.readLine()
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(first, "ERR") {
		return nil, protocolError(first)
	}
	if first != "BEGIN LIST "+what {
		return nil, fail(integrations.CodeBadResponse, "respuesta inesperada a LIST")
	}
	var out [][]string
	for {
		line, err := s.readLine()
		if err != nil {
			return nil, err
		}
		if line == "END LIST "+what {
			return out, nil
		}
		if len(out) >= maxLines {
			return nil, fail(integrations.CodeBadResponse, "lista demasiado larga")
		}
		words, err := parseLine(line)
		if err != nil {
			return nil, fail(integrations.CodeBadResponse, "línea no válida: %v", err)
		}
		out = append(out, words)
	}
}

// Collect lee la versión de upsd y las variables del SAI.
func (c *client) Collect(ctx context.Context) (any, error) {
	host, port, ups, err := Target(c.cfg.BaseURL)
	if err != nil {
		return nil, fail(integrations.CodeInternal, "%v", err)
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > defaultTimeout {
		deadline = time.Now().Add(defaultTimeout)
	}
	conn.SetDeadline(deadline)
	s := &session{conn: conn, lines: bufio.NewScanner(conn)}
	s.lines.Buffer(make([]byte, 0, 512), maxLineBytes)

	data, err := read(s, ups)
	if err == nil {
		// Despedida cortés; si falla, los datos ya están leídos.
		if s.send("LOGOUT") == nil {
			s.readLine()
		}
	}
	return data, err
}

func read(s *session, ups string) (Data, error) {
	var data Data
	if err := s.send("VER"); err != nil {
		return data, err
	}
	ver, err := s.readLine()
	if err != nil {
		return data, err
	}
	// VER es opcional en el protocolo: si upsd no la conoce, sin versión.
	if m := upsdVersion.FindStringSubmatch(ver); m != nil {
		data.Version = clip(m[1])
	}

	if ups == "" {
		list, err := s.list("UPS")
		if err != nil {
			return data, err
		}
		if len(list) != 1 || len(list[0]) < 2 || list[0][0] != "UPS" {
			return data, fail(CodeUPSNotFound, "el servidor tiene %d SAI; hay que indicar uno", len(list))
		}
		ups = list[0][1]
		if !upsName.MatchString(ups) {
			return data, fail(integrations.CodeBadResponse, "nombre de SAI no válido")
		}
	}

	vars, err := s.list("VAR " + ups)
	if err != nil {
		return data, err
	}
	values := make(map[string]string, len(vars))
	for _, v := range vars {
		if len(v) != 4 || v[0] != "VAR" || v[1] != ups {
			return data, fail(integrations.CodeBadResponse, "variable con forma inesperada")
		}
		values[v[2]] = v[3]
	}
	data.Model = values["ups.model"]
	if data.Model == "" {
		data.Model = values["device.model"]
	}
	data.Model = clip(data.Model)
	data.Status = []string{}
	for _, flag := range strings.Fields(values["ups.status"]) {
		if slices.Contains(knownFlags, flag) {
			data.Status = append(data.Status, flag)
		}
	}
	if v, err := strconv.ParseFloat(values["battery.charge"], 64); err == nil && v >= 0 && v <= 100 {
		data.ChargePercent = &v
	}
	if v, err := strconv.ParseFloat(values["battery.runtime"], 64); err == nil && v >= 0 {
		seconds := int(v)
		data.RuntimeSeconds = &seconds
	}
	return data, nil
}
