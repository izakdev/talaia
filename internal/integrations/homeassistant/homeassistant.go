// Package homeassistant lee el estado de Home Assistant y de las entidades elegidas.
package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/izakdev/talaia/internal/integrations"
)

// OptionShowSummary incluye los agregados de luces, interruptores y otros dominios.
const OptionShowSummary = "show_summary"

// NoteSystemUnavailable indica que Supervisor no ofrece CPU, memoria ni disco.
const NoteSystemUnavailable = "system_unavailable"

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
	System   *System  `json:"system,omitempty"`
	Summary  *Summary `json:"summary,omitempty"`
}

// Notes devuelve avisos que no impiden mostrar el servicio.
func (d Data) Notes() []string {
	if d.System == nil {
		return []string{NoteSystemUnavailable}
	}
	return nil
}

// DomainSummary cuenta los elementos activos de un dominio.
type DomainSummary struct {
	Domain string `json:"domain"`
	Active int    `json:"active"`
	Total  int    `json:"total"`
}

// Summary son agregados sin nombres. No incluye personas, cerraduras ni alarmas,
// porque revelarían si hay alguien en casa o el estado de la seguridad.
type Summary struct {
	Domains      []DomainSummary `json:"domains"`
	Entities     int             `json:"entities"`
	Unavailable  int             `json:"unavailable"`
	Updates      int             `json:"updates"`
	Integrations int             `json:"integrations"`
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
	Version    string   `json:"version"`
	State      string   `json:"state"`
	SafeMode   bool     `json:"safe_mode"`
	Recovery   bool     `json:"recovery_mode"`
	Components []string `json:"components"`
}

type stateResponse struct {
	EntityID   string `json:"entity_id"`
	State      string `json:"state"`
	Attributes struct {
		FriendlyName string `json:"friendly_name"`
		Unit         string `json:"unit_of_measurement"`
		DeviceClass  string `json:"device_class"`
		// Los grupos listan aquí sus miembros.
		Members json.RawMessage `json:"entity_id"`
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
	// CPU, memoria y disco son opcionales: sin Supervisor, el resto sigue funcionando.
	if system, err := c.system(ctx); err == nil {
		data.System = system
	}
	showSummary := c.cfg.Options[OptionShowSummary]
	if len(c.cfg.Entities) == 0 && !showSummary {
		return data, nil
	}
	var states []stateResponse
	if err := c.get(ctx, "/api/states", &states); err != nil {
		return nil, err
	}
	if showSummary {
		data.Summary = summarize(states, config.Components)
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

// summaryDomains define qué dominios se agregan y qué estados cuentan como activos.
var summaryDomains = []struct {
	domain string
	active []string
}{
	{"light", []string{"on"}},
	{"switch", []string{"on"}},
	{"fan", []string{"on"}},
	{"climate", nil}, // Activo si no está apagado.
	{"cover", []string{"open", "opening", "closing"}},
	{"media_player", []string{"on", "playing", "paused", "buffering"}},
	{"automation", []string{"on"}},
}

func summarize(states []stateResponse, components []string) *Summary {
	counts := map[string]*DomainSummary{}
	summary := &Summary{Domains: []DomainSummary{}, Entities: len(states)}
	for _, s := range states {
		domain, _, _ := strings.Cut(s.EntityID, ".")
		noData := s.State == "unavailable" || s.State == "unknown"
		if s.State == "unavailable" {
			summary.Unavailable++
		}
		if domain == "update" && s.State == "on" {
			summary.Updates++
		}
		// Un grupo contaría dos veces a sus miembros.
		if noData || len(s.Attributes.Members) > 0 {
			continue
		}
		// Una puerta o un garaje abiertos no deben publicarse como una persiana más.
		if domain == "cover" && slices.Contains([]string{"door", "garage", "gate"}, s.Attributes.DeviceClass) {
			continue
		}
		for _, rule := range summaryDomains {
			if rule.domain != domain {
				continue
			}
			entry := counts[domain]
			if entry == nil {
				entry = &DomainSummary{Domain: domain}
				counts[domain] = entry
			}
			entry.Total++
			if (rule.active == nil && s.State != "off") || slices.Contains(rule.active, s.State) {
				entry.Active++
			}
		}
	}
	for _, rule := range summaryDomains {
		if entry := counts[rule.domain]; entry != nil {
			summary.Domains = append(summary.Domains, *entry)
		}
	}
	for _, component := range components {
		if !strings.Contains(component, ".") {
			summary.Integrations++
		}
	}
	return summary
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
