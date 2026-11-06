// Package wgdashboard lee el estado de los túneles de WGDashboard.
package wgdashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/izakdev/talaia/internal/integrations"
)

const (
	pathVersion        = "/api/getDashboardVersion"
	pathConfigurations = "/api/getWireguardConfigurations"
	headerAPIKey       = "wg-dashboard-apikey"
	gibibyte           = 1 << 30
)

// Tunnel es la información pública de un túnel. No incluye claves, direcciones,
// puertos ni nombres de peers.
type Tunnel struct {
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	Up             bool   `json:"up"`
	ConnectedPeers int    `json:"connected_peers"`
	TotalPeers     int    `json:"total_peers"`
	ReceivedBytes  int64  `json:"received_bytes"`
	SentBytes      int64  `json:"sent_bytes"`
}

// Data es la información pública de WGDashboard.
type Data struct {
	Version string   `json:"version"`
	Tunnels []Tunnel `json:"tunnels"`
}

type client struct {
	cfg  integrations.Config
	http *http.Client
}

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration {
	return &client{cfg: cfg, http: integrations.NewHTTPClient(cfg)}
}

// response es el sobre común de las respuestas de WGDashboard.
type response struct {
	Status bool            `json:"status"`
	Data   json.RawMessage `json:"data"`
}

func (c *client) get(ctx context.Context, path string, withKey bool, dst any) error {
	var r response
	err := integrations.GetJSON(ctx, c.http, c.cfg.BaseURL+path, func(req *http.Request) {
		if withKey {
			req.Header.Set(headerAPIKey, c.cfg.Secret)
		}
	}, &r)
	if err != nil {
		return err
	}
	if !r.Status {
		return &integrations.Error{Code: integrations.CodeBadResponse, Err: errors.New("WGDashboard no ha atendido la petición")}
	}
	if err := json.Unmarshal(r.Data, dst); err != nil {
		return &integrations.Error{Code: integrations.CodeBadResponse, Err: err}
	}
	return nil
}

// configuration solo lee los campos públicos: la respuesta trae también la
// clave privada del servidor, que nunca se decodifica.
type configuration struct {
	Name           string `json:"Name"`
	Protocol       string `json:"Protocol"`
	Status         bool   `json:"Status"`
	ConnectedPeers int    `json:"ConnectedPeers"`
	TotalPeers     int    `json:"TotalPeers"`
	DataUsage      struct {
		Sent    float64 `json:"Sent"`
		Receive float64 `json:"Receive"`
	} `json:"DataUsage"`
}

// Collect consulta la versión y los túneles. La versión no necesita la clave, y
// sin ella WGDashboard no la anota en su registro.
func (c *client) Collect(ctx context.Context) (any, error) {
	var version string
	if err := c.get(ctx, pathVersion, false, &version); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(version, "v") {
		return nil, &integrations.Error{Code: integrations.CodeBadResponse, Err: errors.New("la respuesta no es de WGDashboard")}
	}
	var configurations []configuration
	if err := c.get(ctx, pathConfigurations, true, &configurations); err != nil {
		return nil, err
	}
	data := Data{Version: version, Tunnels: []Tunnel{}}
	for _, conf := range configurations {
		data.Tunnels = append(data.Tunnels, Tunnel{
			Name:           conf.Name,
			Protocol:       conf.Protocol,
			Up:             conf.Status,
			ConnectedPeers: conf.ConnectedPeers,
			TotalPeers:     conf.TotalPeers,
			// WGDashboard da el tráfico acumulado en GiB, visto desde el servidor.
			ReceivedBytes: int64(conf.DataUsage.Receive * gibibyte),
			SentBytes:     int64(conf.DataUsage.Sent * gibibyte),
		})
	}
	return data, nil
}
