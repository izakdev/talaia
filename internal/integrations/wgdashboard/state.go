package wgdashboard

import (
	"slices"

	"github.com/izakdev/talaia/internal/integrations"
)

// State devuelve el estado de WGDashboard para el panel y los avisos.
func (d Data) State() integrations.State {
	switch {
	case len(d.Tunnels) == 0:
		return integrations.State{Tone: integrations.ToneMuted, Key: "wgdashboard.none"}
	case slices.ContainsFunc(d.Tunnels, func(t Tunnel) bool { return !t.Up }):
		return integrations.State{Tone: integrations.ToneWarn, Key: "wgdashboard.down"}
	}
	return integrations.State{Tone: integrations.ToneGood, Key: "wgdashboard.allUp"}
}
