package opnsense

import (
	"slices"

	"github.com/izakdev/talaia/internal/integrations"
)

// State devuelve el estado de OPNsense para el panel y los avisos.
func (d Data) State() integrations.State {
	has := func(state string) bool {
		return slices.ContainsFunc(d.Gateways, func(g Gateway) bool { return g.State == state })
	}
	switch {
	case len(d.Gateways) == 0:
		return integrations.State{Tone: integrations.ToneMuted, Key: "opnsense.noGateways"}
	case has("offline"):
		return integrations.State{Tone: integrations.ToneBad, Key: "opnsense.offline"}
	case has("degraded"):
		return integrations.State{Tone: integrations.ToneWarn, Key: "opnsense.degraded"}
	case d.WireGuard != nil && slices.ContainsFunc(d.WireGuard.Tunnels, func(t WireGuardTunnel) bool { return !t.Up }):
		return integrations.State{Tone: integrations.ToneWarn, Key: "opnsense.wireguardDown"}
	// Un gateway sin monitorizar siempre figura como correcto: no demuestra que haya conexión.
	case !has("online"):
		return integrations.State{Tone: integrations.ToneMuted, Key: "opnsense.unmonitored"}
	}
	return integrations.State{Tone: integrations.ToneGood, Key: "opnsense.online"}
}
