package proxmox

import "github.com/izakdev/talaia/internal/integrations"

// State devuelve el estado de Proxmox VE para el panel y los avisos.
func (d Data) State() integrations.State {
	switch {
	case d.Quorate != nil && !*d.Quorate:
		return integrations.State{Tone: integrations.ToneBad, Key: "proxmox.noQuorum"}
	case d.NodesOnline < d.NodesTotal:
		return integrations.State{Tone: integrations.ToneWarn, Key: "proxmox.nodesDown", Params: map[string]any{"online": d.NodesOnline, "total": d.NodesTotal}}
	case d.ClusterName != "":
		return integrations.State{Tone: integrations.ToneGood, Key: "proxmox.clusterOk"}
	}
	return integrations.State{Tone: integrations.ToneGood, Key: "proxmox.nodeOk"}
}
