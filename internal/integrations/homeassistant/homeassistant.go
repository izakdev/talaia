// Package homeassistant lee el estado de Home Assistant y de las entidades elegidas.
package homeassistant

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/izakdev/talaia/internal/integrations"
)

// StateMissing indica que una entidad elegida ya no existe en Home Assistant.
const StateMissing = "missing"

// Entity es el dato público de una entidad elegida.
type Entity struct {
	ID          string `json:"entity_id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Unit        string `json:"unit,omitempty"`
	DeviceClass string `json:"device_class,omitempty"`
}

// Data es la información pública de un Home Assistant.
type Data struct {
	Version  string   `json:"version"`
	State    string   `json:"state"`
	SafeMode bool     `json:"safe_mode"`
	Recovery bool     `json:"recovery_mode"`
	Entities []Entity `json:"entities"`
}

type client struct {
	cfg  integrations.Config
	http *http.Client
}

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration {
	return &client{cfg: cfg, http: integrations.NewHTTPClient(cfg)}
}

type configResponse struct {
	Version  string `json:"version"`
	State    string `json:"state"`
	SafeMode bool   `json:"safe_mode"`
	Recovery bool   `json:"recovery_mode"`
}

type stateResponse struct {
	EntityID   string `json:"entity_id"`
	State      string `json:"state"`
	Attributes struct {
		FriendlyName string `json:"friendly_name"`
		Unit         string `json:"unit_of_measurement"`
		DeviceClass  string `json:"device_class"`
	} `json:"attributes"`
}

func (c *client) get(ctx context.Context, path string, dst any) error {
	return integrations.GetJSON(ctx, c.http, c.cfg.BaseURL+path, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+c.cfg.Secret)
	}, dst)
}

func (c *client) config(ctx context.Context) (configResponse, error) {
	var config configResponse
	if err := c.get(ctx, "/api/config", &config); err != nil {
		return config, err
	}
	// Un JSON válido de otro servicio no debe parecer un Home Assistant sano.
	if config.Version == "" {
		return config, &integrations.Error{Code: integrations.CodeBadResponse, Err: errors.New("la respuesta no es de Home Assistant")}
	}
	return config, nil
}

func toEntity(s stateResponse) Entity {
	name := s.Attributes.FriendlyName
	if name == "" {
		name = s.EntityID
	}
	return Entity{ID: s.EntityID, Name: name, State: s.State, Unit: s.Attributes.Unit, DeviceClass: s.Attributes.DeviceClass}
}

func sortByName(entities []Entity) {
	sort.SliceStable(entities, func(i, j int) bool {
		a, b := strings.ToLower(entities[i].Name), strings.ToLower(entities[j].Name)
		if a != b {
			return a < b
		}
		return entities[i].ID < entities[j].ID
	})
}

// Collect consulta el estado y solo publica las entidades elegidas.
func (c *client) Collect(ctx context.Context) (any, error) {
	config, err := c.config(ctx)
	if err != nil {
		return nil, err
	}
	data := Data{Version: config.Version, State: config.State, SafeMode: config.SafeMode, Recovery: config.Recovery, Entities: []Entity{}}
	if len(c.cfg.Entities) == 0 {
		return data, nil
	}
	var states []stateResponse
	if err := c.get(ctx, "/api/states", &states); err != nil {
		return nil, err
	}
	byID := make(map[string]stateResponse, len(states))
	for _, s := range states {
		byID[s.EntityID] = s
	}
	for _, id := range c.cfg.Entities {
		s, found := byID[id]
		if !found {
			data.Entities = append(data.Entities, Entity{ID: id, Name: id, State: StateMissing})
			continue
		}
		data.Entities = append(data.Entities, toEntity(s))
	}
	sortByName(data.Entities)
	return data, nil
}

// ListEntities devuelve todas las entidades, para elegirlas en la administración.
func (c *client) ListEntities(ctx context.Context) ([]integrations.EntityOption, error) {
	if _, err := c.config(ctx); err != nil {
		return nil, err
	}
	var states []stateResponse
	if err := c.get(ctx, "/api/states", &states); err != nil {
		return nil, err
	}
	entities := make([]Entity, len(states))
	for i, s := range states {
		entities[i] = toEntity(s)
	}
	sortByName(entities)
	options := make([]integrations.EntityOption, len(entities))
	for i, e := range entities {
		options[i] = integrations.EntityOption{ID: e.ID, Name: e.Name, State: e.State, Unit: e.Unit, DeviceClass: e.DeviceClass}
	}
	return options, nil
}
