package adguard

import "github.com/izakdev/talaia/internal/integrations"

// State devuelve el estado de AdGuard Home para el panel y los avisos.
func (d Data) State() integrations.State {
	switch {
	case !d.Running:
		return integrations.State{Tone: integrations.ToneBad, Key: "adguard.dnsStopped"}
	case d.ProtectionEnabled:
		return integrations.State{Tone: integrations.ToneGood, Key: "adguard.protectionOn"}
	}
	return integrations.State{Tone: integrations.ToneWarn, Key: "adguard.protectionOff"}
}
