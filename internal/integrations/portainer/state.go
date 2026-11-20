package portainer

import (
	"slices"

	"github.com/izakdev/talaia/internal/integrations"
)

// State devuelve el estado de Portainer para el panel y los avisos.
func (d Data) State() integrations.State {
	switch {
	case len(d.Environments) == 0:
		return integrations.State{Tone: integrations.ToneMuted, Key: "portainer.none"}
	case slices.ContainsFunc(d.Environments, func(e Environment) bool { return !e.Online }):
		return integrations.State{Tone: integrations.ToneBad, Key: "portainer.down"}
	case slices.ContainsFunc(d.Environments, func(e Environment) bool { return e.Containers != nil && e.Containers.Unhealthy > 0 }):
		return integrations.State{Tone: integrations.ToneWarn, Key: "portainer.problems"}
	}
	return integrations.State{Tone: integrations.ToneGood, Key: "portainer.allOnline"}
}
