// Package adguard lee el estado de AdGuard Home.
package adguard

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/izakdev/talaia/internal/integrations"
)

const topLimit = 5

// DomainCount es un dominio con su número de consultas.
type DomainCount struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}

// Data es la información pública de un AdGuard Home.
type Data struct {
	Version           string        `json:"version"`
	Running           bool          `json:"running"`
	ProtectionEnabled bool          `json:"protection_enabled"`
	Queries           int64         `json:"queries"`
	Blocked           int64         `json:"blocked"`
	BlockedPercent    float64       `json:"blocked_percent"`
	AvgProcessingMs   float64       `json:"avg_processing_ms"`
	PeriodUnit        string        `json:"period_unit"`
	PeriodLength      int           `json:"period_length"`
	TopQueried        []DomainCount `json:"top_queried"`
	TopBlocked        []DomainCount `json:"top_blocked"`
}

type client struct {
	cfg  integrations.Config
	http *http.Client
}

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration {
	return &client{cfg: cfg, http: integrations.NewHTTPClient(cfg)}
}

type statusResponse struct {
	Version           string `json:"version"`
	Running           bool   `json:"running"`
	ProtectionEnabled *bool  `json:"protection_enabled"`
}

type statsResponse struct {
	TimeUnits             string             `json:"time_units"`
	NumDNSQueries         *int64             `json:"num_dns_queries"`
	NumBlockedFiltering   int64              `json:"num_blocked_filtering"`
	NumReplacedSafeBrowse int64              `json:"num_replaced_safebrowsing"`
	NumReplacedParental   int64              `json:"num_replaced_parental"`
	AvgProcessingTime     float64            `json:"avg_processing_time"`
	TopQueriedDomains     []map[string]int64 `json:"top_queried_domains"`
	TopBlockedDomains     []map[string]int64 `json:"top_blocked_domains"`
	DNSQueries            []int64            `json:"dns_queries"`
}

func (c *client) get(ctx context.Context, path string, dst any) error {
	return integrations.GetJSON(ctx, c.http, c.cfg.BaseURL+path, func(r *http.Request) {
		r.SetBasicAuth(c.cfg.Username, c.cfg.Secret)
	}, dst)
}

// Collect consulta el estado y las estadísticas.
func (c *client) Collect(ctx context.Context) (any, error) {
	var status statusResponse
	if err := c.get(ctx, "/control/status", &status); err != nil {
		return nil, err
	}
	var stats statsResponse
	if err := c.get(ctx, "/control/stats", &stats); err != nil {
		return nil, err
	}
	// Un JSON válido de otro servicio no debe parecer un AdGuard sano.
	if status.ProtectionEnabled == nil || stats.NumDNSQueries == nil {
		return nil, &integrations.Error{Code: integrations.CodeBadResponse, Err: errors.New("la respuesta no es de AdGuard Home")}
	}

	blocked := stats.NumBlockedFiltering + stats.NumReplacedSafeBrowse + stats.NumReplacedParental
	var percent float64
	if *stats.NumDNSQueries > 0 {
		percent = float64(blocked) / float64(*stats.NumDNSQueries) * 100
	}
	return Data{
		Version:           status.Version,
		Running:           status.Running,
		ProtectionEnabled: *status.ProtectionEnabled,
		Queries:           *stats.NumDNSQueries,
		Blocked:           blocked,
		BlockedPercent:    percent,
		AvgProcessingMs:   stats.AvgProcessingTime * 1000,
		PeriodUnit:        stats.TimeUnits,
		PeriodLength:      len(stats.DNSQueries),
		TopQueried:        top(stats.TopQueriedDomains),
		TopBlocked:        top(stats.TopBlockedDomains),
	}, nil
}

func top(entries []map[string]int64) []DomainCount {
	out := []DomainCount{}
	for _, entry := range entries {
		for domain, count := range entry {
			out = append(out, DomainCount{domain, count})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Domain < out[j].Domain
	})
	if len(out) > topLimit {
		out = out[:topLimit]
	}
	return out
}
