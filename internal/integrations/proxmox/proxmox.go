// Package proxmox lee el estado de un clúster o nodo de Proxmox VE.
package proxmox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/izakdev/talaia/internal/integrations"
)

// OptionShowGuests incluye el detalle de cada VM y LXC en los datos públicos.
const OptionShowGuests = "show_guests"

// Count indica cuántos elementos están en marcha sobre el total.
type Count struct {
	Running int `json:"running"`
	Total   int `json:"total"`
}

// Guest es una VM o un LXC.
type Guest struct {
	VMID     int     `json:"vmid"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Status   string  `json:"status"`
	CPU      float64 `json:"cpu"`
	MaxCPU   int     `json:"max_cpu"`
	MemUsed  int64   `json:"mem_used"`
	MemTotal int64   `json:"mem_total"`
}

// Node es un nodo con sus consumos.
type Node struct {
	Name       string  `json:"name"`
	Online     bool    `json:"online"`
	CPU        float64 `json:"cpu"`
	MaxCPU     int     `json:"max_cpu"`
	MemUsed    int64   `json:"mem_used"`
	MemTotal   int64   `json:"mem_total"`
	DiskUsed   int64   `json:"disk_used"`
	DiskTotal  int64   `json:"disk_total"`
	VMs        Count   `json:"vms"`
	Containers Count   `json:"containers"`
	Guests     []Guest `json:"guests,omitzero"`
}

// Data es la información pública de un clúster o nodo.
type Data struct {
	Version     string  `json:"version"`
	ClusterName string  `json:"cluster_name,omitempty"`
	Quorate     *bool   `json:"quorate,omitempty"`
	NodesOnline int     `json:"nodes_online"`
	NodesTotal  int     `json:"nodes_total"`
	VMs         Count   `json:"vms"`
	Containers  Count   `json:"containers"`
	CPU         float64 `json:"cpu"`
	MaxCPU      int     `json:"max_cpu"`
	MemUsed     int64   `json:"mem_used"`
	MemTotal    int64   `json:"mem_total"`
	Nodes       []Node  `json:"nodes"`
}

type client struct {
	cfg  integrations.Config
	http *http.Client
}

// New crea la integración.
func New(cfg integrations.Config) integrations.Integration {
	return &client{cfg: cfg, http: integrations.NewHTTPClient(cfg)}
}

// flexBool acepta los valores booleanos de Proxmox, que suelen llegar como 0 o 1.
type flexBool bool

func (b *flexBool) UnmarshalJSON(raw []byte) error {
	switch string(bytes.Trim(raw, `"`)) {
	case "1", "true":
		*b = true
	case "0", "false", "null", "":
		*b = false
	default:
		return fmt.Errorf("valor booleano no válido: %s", raw)
	}
	return nil
}

type versionResponse struct {
	Version string `json:"version"`
}

type statusEntry struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Quorate flexBool `json:"quorate"`
}

type resource struct {
	Type     string   `json:"type"`
	Node     string   `json:"node"`
	Name     string   `json:"name"`
	VMID     int      `json:"vmid"`
	Status   string   `json:"status"`
	CPU      float64  `json:"cpu"`
	MaxCPU   float64  `json:"maxcpu"`
	Mem      float64  `json:"mem"`
	MaxMem   float64  `json:"maxmem"`
	Disk     float64  `json:"disk"`
	MaxDisk  float64  `json:"maxdisk"`
	Template flexBool `json:"template"`
}

func get[T any](ctx context.Context, c *client, path string) (T, error) {
	var envelope struct {
		Data *T `json:"data"`
	}
	var zero T
	err := integrations.GetJSON(ctx, c.http, c.cfg.BaseURL+"/api2/json"+path, func(r *http.Request) {
		r.Header.Set("Authorization", "PVEAPIToken="+c.cfg.Secret)
	}, &envelope)
	if err != nil {
		return zero, err
	}
	// Un JSON válido de otro servicio no debe parecer un Proxmox sano.
	if envelope.Data == nil {
		return zero, &integrations.Error{Code: integrations.CodeBadResponse, Err: errors.New("la respuesta no es de Proxmox VE")}
	}
	return *envelope.Data, nil
}

// Collect consulta la versión, el estado del clúster y los recursos.
func (c *client) Collect(ctx context.Context) (any, error) {
	version, err := get[versionResponse](ctx, c, "/version")
	if err != nil {
		return nil, err
	}
	status, err := get[[]statusEntry](ctx, c, "/cluster/status")
	if err != nil {
		return nil, err
	}
	resources, err := get[[]resource](ctx, c, "/cluster/resources")
	if err != nil {
		return nil, err
	}
	data := build(resources, c.cfg.Options[OptionShowGuests])
	// Proxmox filtra los recursos según los permisos: sin nodos, el token no ve nada.
	if data.NodesTotal == 0 {
		return nil, &integrations.Error{Code: integrations.CodeUnauthorized, Err: errors.New("el token no tiene permiso para ver los nodos")}
	}
	data.Version = version.Version
	for _, entry := range status {
		if entry.Type == "cluster" {
			quorate := bool(entry.Quorate)
			data.ClusterName, data.Quorate = entry.Name, &quorate
		}
	}
	return data, nil
}

func build(resources []resource, showGuests bool) Data {
	nodes := map[string]*Node{}
	for _, r := range resources {
		if r.Type != "node" {
			continue
		}
		name := r.Node
		if name == "" {
			name = r.Name
		}
		node := Node{Name: name, Online: r.Status == "online"}
		if node.Online {
			node.CPU, node.MaxCPU = r.CPU, int(r.MaxCPU)
			node.MemUsed, node.MemTotal = int64(r.Mem), int64(r.MaxMem)
			node.DiskUsed, node.DiskTotal = int64(r.Disk), int64(r.MaxDisk)
		}
		if showGuests {
			node.Guests = []Guest{}
		}
		nodes[name] = &node
	}

	var data Data
	for _, r := range resources {
		if (r.Type != "qemu" && r.Type != "lxc") || bool(r.Template) {
			continue
		}
		global, perNode := &data.VMs, (*Count)(nil)
		node := nodes[r.Node]
		if r.Type == "lxc" {
			global = &data.Containers
		}
		if node != nil {
			perNode = &node.VMs
			if r.Type == "lxc" {
				perNode = &node.Containers
			}
			perNode.Total++
		}
		global.Total++
		if r.Status == "running" {
			global.Running++
			if perNode != nil {
				perNode.Running++
			}
		}
		if node != nil && showGuests {
			node.Guests = append(node.Guests, Guest{
				VMID: r.VMID, Name: r.Name, Type: r.Type, Status: r.Status, CPU: r.CPU,
				MaxCPU: int(r.MaxCPU), MemUsed: int64(r.Mem), MemTotal: int64(r.MaxMem),
			})
		}
	}

	data.Nodes = make([]Node, 0, len(nodes))
	var weightedCPU float64
	for _, node := range nodes {
		slices.SortFunc(node.Guests, func(a, b Guest) int { return a.VMID - b.VMID })
		data.Nodes = append(data.Nodes, *node)
		data.NodesTotal++
		if node.Online {
			data.NodesOnline++
			weightedCPU += node.CPU * float64(node.MaxCPU)
			data.MaxCPU += node.MaxCPU
			data.MemUsed += node.MemUsed
			data.MemTotal += node.MemTotal
		}
	}
	slices.SortFunc(data.Nodes, func(a, b Node) int { return strings.Compare(a.Name, b.Name) })
	if data.MaxCPU > 0 {
		data.CPU = weightedCPU / float64(data.MaxCPU)
	}
	return data
}
