package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	supervisorTimeout   = 10 * time.Second
	supervisorReadLimit = 1 << 20
	bytesPerGB          = 1000 * 1000 * 1000
)

// System son los recursos que informa Supervisor. Solo existe en instalaciones con
// Supervisor y con un token de administrador.
type System struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryUsed    int64   `json:"memory_used"`
	MemoryLimit   int64   `json:"memory_limit"`
	DiskUsed      int64   `json:"disk_used"`
	DiskTotal     int64   `json:"disk_total"`
	BootedAt      int64   `json:"booted_at,omitempty"`
}

type wsMessage struct {
	ID      int             `json:"id"`
	Type    string          `json:"type"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result"`
}

type coreStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   int64   `json:"memory_usage"`
	MemoryLimit   int64   `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
}

type hostInfo struct {
	DiskTotal     float64 `json:"disk_total"`
	DiskUsed      float64 `json:"disk_used"`
	BootTimestamp int64   `json:"boot_timestamp"`
}

// La API REST no permite estas rutas, por eso se usa el WebSocket de Home Assistant.
func (c *client) system(ctx context.Context) (*System, error) {
	ctx, cancel := context.WithTimeout(ctx, supervisorTimeout)
	defer cancel()

	httpClient := *c.http
	httpClient.Timeout = 0
	conn, _, err := websocket.Dial(ctx, c.cfg.BaseURL+"/api/websocket", &websocket.DialOptions{HTTPClient: &httpClient})
	if err != nil {
		return nil, err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(supervisorReadLimit)

	var message wsMessage
	if err := wsjson.Read(ctx, conn, &message); err != nil {
		return nil, err
	}
	if message.Type != "auth_required" {
		return nil, fmt.Errorf("mensaje inicial inesperado: %s", message.Type)
	}
	if err := wsjson.Write(ctx, conn, map[string]string{"type": "auth", "access_token": c.cfg.Secret}); err != nil {
		return nil, err
	}
	if err := wsjson.Read(ctx, conn, &message); err != nil {
		return nil, err
	}
	if message.Type != "auth_ok" {
		return nil, errors.New("autenticación rechazada")
	}

	const coreID, hostID = 1, 2
	for id, endpoint := range map[int]string{coreID: "/core/stats", hostID: "/host/info"} {
		command := map[string]any{"id": id, "type": "supervisor/api", "endpoint": endpoint, "method": "get"}
		if err := wsjson.Write(ctx, conn, command); err != nil {
			return nil, err
		}
	}

	var core coreStats
	var host hostInfo
	for pending := 2; pending > 0; {
		var result wsMessage
		if err := wsjson.Read(ctx, conn, &result); err != nil {
			return nil, err
		}
		if result.Type != "result" {
			continue
		}
		if !result.Success {
			return nil, errors.New("Supervisor no disponible")
		}
		var dst any
		switch result.ID {
		case coreID:
			dst = &core
		case hostID:
			dst = &host
		default:
			continue
		}
		if err := json.Unmarshal(result.Result, dst); err != nil {
			return nil, err
		}
		pending--
	}
	conn.Close(websocket.StatusNormalClosure, "")

	system := &System{
		CPUPercent: core.CPUPercent, MemoryPercent: core.MemoryPercent,
		MemoryUsed: core.MemoryUsage, MemoryLimit: core.MemoryLimit,
		DiskUsed: int64(host.DiskUsed * bytesPerGB), DiskTotal: int64(host.DiskTotal * bytesPerGB),
	}
	if host.BootTimestamp > 0 {
		system.BootedAt = host.BootTimestamp / 1_000_000
	}
	return system, nil
}
