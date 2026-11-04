// Package opnsense lee el estado de un cortafuegos OPNsense.
package opnsense

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
)

// NoteLimitedPrivileges avisa de que faltan datos por los privilegios del usuario.
const NoteLimitedPrivileges = "limited_privileges"

// Estados públicos de un gateway.
const (
	GatewayOnline      = "online"
	GatewayDegraded    = "degraded"
	GatewayOffline     = "offline"
	GatewayUnmonitored = "unmonitored"
)

const (
	pathActivity    = "/api/diagnostics/activity/get_activity"
	pathGateways    = "/api/routes/gateway/status"
	pathInformation = "/api/diagnostics/system/system_information"
	pathResources   = "/api/diagnostics/system/system_resources"
	pathDisk        = "/api/diagnostics/system/system_disk"
	pathTraffic     = "/api/diagnostics/traffic/interface"
)

// Gateway es el estado público de un gateway. No incluye sus direcciones.
type Gateway struct {
	Name    string   `json:"name"`
	State   string   `json:"state"`
	DelayMs *float64 `json:"delay_ms,omitempty"`
	LossPct *float64 `json:"loss_percent,omitempty"`
}

// Traffic es el tráfico de la WAN. Las velocidades necesitan dos lecturas.
type Traffic struct {
	ReceivedBytes    int64    `json:"received_bytes"`
	SentBytes        int64    `json:"sent_bytes"`
	ReceiveBitsPerS  *float64 `json:"receive_bits_per_second,omitempty"`
	TransmitBitsPerS *float64 `json:"transmit_bits_per_second,omitempty"`
}

// Data es la información pública de un OPNsense. No incluye el nombre del
// equipo, las direcciones ni los procesos.
type Data struct {
	Version       string    `json:"version"`
	CPUPercent    float64   `json:"cpu_percent"`
	UptimeSeconds int64     `json:"uptime_seconds"`
	MemoryUsed    int64     `json:"memory_used,omitempty"`
	MemoryTotal   int64     `json:"memory_total,omitempty"`
	DiskUsed      int64     `json:"disk_used,omitempty"`
	DiskTotal     int64     `json:"disk_total,omitempty"`
	Gateways      []Gateway `json:"gateways"`
	WAN           *Traffic  `json:"wan,omitempty"`
	limited       bool
}

// Notes devuelve avisos que no impiden mostrar el servicio.
func (d Data) Notes() []string {
	if d.limited {
		return []string{NoteLimitedPrivileges}
	}
	return nil
}

type sample struct {
	at       time.Time
	received int64
	sent     int64
}

type client struct {
	cfg  integrations.Config
	http *http.Client
	now  func() time.Time

	mu   sync.Mutex
	last *sample
}

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration {
	return &client{cfg: cfg, http: integrations.NewHTTPClient(cfg), now: time.Now}
}

func (c *client) get(ctx context.Context, path string, dst any) error {
	return integrations.GetJSON(ctx, c.http, c.cfg.BaseURL+path, func(r *http.Request) {
		r.SetBasicAuth(c.cfg.Username, c.cfg.Secret)
	}, dst)
}

type activityResponse struct {
	Headers []string `json:"headers"`
}

type gatewaysResponse struct {
	Items *[]struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Loss   string `json:"loss"`
		Delay  string `json:"delay"`
	} `json:"items"`
}

type informationResponse struct {
	Versions []string `json:"versions"`
}

type resourcesResponse struct {
	Memory struct {
		Total json.Number `json:"total"`
		Used  json.Number `json:"used"`
	} `json:"memory"`
}

type diskResponse struct {
	Devices []struct {
		Blocks     string `json:"blocks"`
		Used       string `json:"used"`
		Mountpoint string `json:"mountpoint"`
	} `json:"devices"`
}

type trafficResponse struct {
	Interfaces map[string]struct {
		Received json.Number `json:"bytes received"`
		Sent     json.Number `json:"bytes transmitted"`
	} `json:"interfaces"`
}

// Collect consulta el estado. La actividad y los gateways son imprescindibles;
// el resto se omite si los privilegios del usuario no lo permiten.
func (c *client) Collect(ctx context.Context) (any, error) {
	var activity activityResponse
	if err := c.get(ctx, pathActivity, &activity); err != nil {
		return nil, err
	}
	cpu, uptime, ok := parseActivity(activity.Headers)
	if !ok {
		return nil, foreignResponse()
	}
	var gateways gatewaysResponse
	if err := c.get(ctx, pathGateways, &gateways); err != nil {
		return nil, err
	}
	if gateways.Items == nil {
		return nil, foreignResponse()
	}

	data := Data{CPUPercent: cpu, UptimeSeconds: uptime, Gateways: make([]Gateway, 0, len(*gateways.Items))}
	for _, item := range *gateways.Items {
		data.Gateways = append(data.Gateways, newGateway(item.Name, item.Status, item.Delay, item.Loss))
	}

	var information informationResponse
	if err := c.optional(ctx, pathInformation, &information, &data); err != nil {
		return nil, err
	}
	data.Version = parseVersion(information.Versions)

	var resources resourcesResponse
	if err := c.optional(ctx, pathResources, &resources, &data); err != nil {
		return nil, err
	}
	data.MemoryUsed, _ = resources.Memory.Used.Int64()
	data.MemoryTotal, _ = resources.Memory.Total.Int64()

	var disk diskResponse
	if err := c.optional(ctx, pathDisk, &disk, &data); err != nil {
		return nil, err
	}
	for _, device := range disk.Devices {
		if device.Mountpoint == "/" {
			data.DiskUsed = parseSize(device.Used)
			data.DiskTotal = parseSize(device.Blocks)
		}
	}

	var traffic trafficResponse
	if err := c.optional(ctx, pathTraffic, &traffic, &data); err != nil {
		return nil, err
	}
	if wan, ok := traffic.Interfaces["wan"]; ok {
		received, errReceived := wan.Received.Int64()
		sent, errSent := wan.Sent.Int64()
		if errReceived == nil && errSent == nil {
			data.WAN = c.wanTraffic(received, sent)
		}
	}
	return data, nil
}

// optional hace una consulta prescindible. Solo la falta de permisos deja sin
// el dato; otro fallo se trata como un fallo del servicio.
func (c *client) optional(ctx context.Context, path string, dst any, data *Data) error {
	err := c.get(ctx, path, dst)
	var public *integrations.Error
	if errors.As(err, &public) && public.Code == integrations.CodeUnauthorized {
		data.limited = true
		return nil
	}
	return err
}

// wanTraffic calcula la velocidad frente a la lectura anterior. Si los contadores
// bajan, porque la interfaz se ha reiniciado, espera a la siguiente.
func (c *client) wanTraffic(received, sent int64) *Traffic {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	traffic := &Traffic{ReceivedBytes: received, SentBytes: sent}
	if last := c.last; last != nil && received >= last.received && sent >= last.sent {
		if elapsed := now.Sub(last.at).Seconds(); elapsed > 0 {
			in := float64(received-last.received) * 8 / elapsed
			out := float64(sent-last.sent) * 8 / elapsed
			traffic.ReceiveBitsPerS, traffic.TransmitBitsPerS = &in, &out
		}
	}
	c.last = &sample{now, received, sent}
	return traffic
}

func foreignResponse() error {
	return &integrations.Error{Code: integrations.CodeBadResponse, Err: errors.New("la respuesta no es de OPNsense")}
}

var (
	idlePattern   = regexp.MustCompile(`^CPU:.*?([0-9.]+)%\s+idle`)
	uptimePattern = regexp.MustCompile(`\bup\s+(\d+)\+(\d+):(\d+):(\d+)`)
)

// parseActivity interpreta las cabeceras de top: la CPU es lo que no está inactivo.
func parseActivity(headers []string) (cpu float64, uptime int64, ok bool) {
	for _, line := range headers {
		if m := uptimePattern.FindStringSubmatch(line); m != nil {
			parts := [4]int64{}
			for i := range parts {
				parts[i], _ = strconv.ParseInt(m[i+1], 10, 64)
			}
			uptime = parts[0]*86400 + parts[1]*3600 + parts[2]*60 + parts[3]
		}
		if m := idlePattern.FindStringSubmatch(line); m != nil {
			idle, err := strconv.ParseFloat(m[1], 64)
			if err == nil && idle >= 0 && idle <= 100 {
				cpu, ok = 100-idle, true
			}
		}
	}
	return cpu, uptime, ok
}

// newGateway traduce el estado. OPNsense devuelve «~» en las medidas de un
// gateway sin monitorizar y lo indica como «none», igual que uno correcto.
func newGateway(name, status, delay, loss string) Gateway {
	gateway := Gateway{Name: name, DelayMs: parseMeasure(delay), LossPct: parseMeasure(loss)}
	switch status {
	case "none":
		gateway.State = GatewayOnline
		if gateway.DelayMs == nil && gateway.LossPct == nil {
			gateway.State = GatewayUnmonitored
		}
	case "down", "force_down":
		gateway.State = GatewayOffline
	default:
		gateway.State = GatewayDegraded
	}
	return gateway
}

// parseMeasure lee valores como «1.2 ms» o «0.0 %».
func parseMeasure(text string) *float64 {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return nil
	}
	return &value
}

// parseVersion extrae el número de «OPNsense 26.7.1_1-amd64».
func parseVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	version, found := strings.CutPrefix(versions[0], "OPNsense ")
	if !found {
		return ""
	}
	if i := strings.LastIndex(version, "-"); i > 0 {
		version = version[:i]
	}
	return version
}

// parseSize lee los tamaños de df -h, como «1.6G» o «96K», y devuelve 0 si no puede.
func parseSize(text string) int64 {
	if text == "" {
		return 0
	}
	number, unit := text, text[len(text)-1]
	exponent := strings.IndexByte("BKMGTP", unit)
	if exponent >= 0 {
		number = text[:len(text)-1]
	} else {
		exponent = 0
	}
	value, err := strconv.ParseFloat(number, 64)
	if err != nil || value < 0 {
		return 0
	}
	for range exponent {
		value *= 1024
	}
	return int64(value)
}
