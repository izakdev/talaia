// Package portainer lee el estado de los entornos Docker de Portainer.
package portainer

import (
	"context"
	"errors"
	"net/http"

	"github.com/izakdev/talaia/internal/integrations"
)

const (
	pathStatus    = "/api/status"
	pathEndpoints = "/api/endpoints"

	typeDocker    = 1
	typeAgent     = 2
	typeEdgeAgent = 4
	statusUp      = 1
	headerAPIKey  = "X-API-Key"
)

// Containers es el último recuento que Portainer guarda de un entorno.
type Containers struct {
	Running    int   `json:"running"`
	Total      int   `json:"total"`
	Unhealthy  int   `json:"unhealthy"`
	Stacks     int   `json:"stacks"`
	SnapshotAt int64 `json:"snapshot_at"`
}

// Environment es la información pública de un entorno. No incluye su URL.
type Environment struct {
	Name       string      `json:"name"`
	Online     bool        `json:"online"`
	Containers *Containers `json:"containers,omitempty"`
}

// Data es la información pública de Portainer.
type Data struct {
	Version      string        `json:"version"`
	Environments []Environment `json:"environments"`
}

type client struct {
	cfg  integrations.Config
	http *http.Client
}

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration {
	return &client{cfg: cfg, http: integrations.NewHTTPClient(cfg)}
}

func (c *client) get(ctx context.Context, path string, dst any) error {
	return integrations.GetJSON(ctx, c.http, c.cfg.BaseURL+path, func(r *http.Request) {
		r.Header.Set(headerAPIKey, c.cfg.Secret)
	}, dst)
}

type statusResponse struct {
	Version string `json:"Version"`
}

type endpoint struct {
	Name      string `json:"Name"`
	Type      int    `json:"Type"`
	Status    int    `json:"Status"`
	Heartbeat bool   `json:"Heartbeat"`
	Snapshots []struct {
		Time       int64 `json:"Time"`
		Total      int   `json:"ContainerCount"`
		Running    int   `json:"RunningContainerCount"`
		Unhealthy  int   `json:"UnhealthyContainerCount"`
		StackCount int   `json:"StackCount"`
	} `json:"Snapshots"`
}

// Collect consulta los entornos. Los recuentos salen de las instantáneas de
// Portainer: un usuario no administrador no ve los contenedores uno a uno.
func (c *client) Collect(ctx context.Context) (any, error) {
	var status statusResponse
	if err := c.get(ctx, pathStatus, &status); err != nil {
		return nil, err
	}
	if status.Version == "" {
		return nil, &integrations.Error{Code: integrations.CodeBadResponse, Err: errors.New("la respuesta no es de Portainer")}
	}
	var endpoints []endpoint
	if err := c.get(ctx, pathEndpoints, &endpoints); err != nil {
		return nil, err
	}
	data := Data{Version: status.Version, Environments: []Environment{}}
	for _, e := range endpoints {
		if e.Type != typeDocker && e.Type != typeAgent && e.Type != typeEdgeAgent {
			continue
		}
		environment := Environment{Name: e.Name, Online: e.Status == statusUp}
		if e.Type == typeEdgeAgent {
			// Portainer no actualiza el estado de los agentes Edge: se deduce de su latido.
			environment.Online = e.Heartbeat
		}
		if len(e.Snapshots) > 0 {
			s := e.Snapshots[0]
			environment.Containers = &Containers{s.Running, s.Total, s.Unhealthy, s.StackCount, s.Time}
		}
		data.Environments = append(data.Environments, environment)
	}
	return data, nil
}
