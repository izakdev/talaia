package homeassistant

import "github.com/izakdev/talaia/internal/integrations"

// State devuelve el estado de Home Assistant para el panel y los avisos.
func (d Data) State() integrations.State {
	switch {
	case d.RunState != "RUNNING":
		return integrations.State{Tone: integrations.ToneWarn, Key: "homeassistant.starting"}
	case d.Recovery:
		return integrations.State{Tone: integrations.ToneBad, Key: "homeassistant.recovery"}
	case d.SafeMode:
		return integrations.State{Tone: integrations.ToneWarn, Key: "homeassistant.safeMode"}
	}
	missing := 0
	for _, e := range d.Entities {
		if e.State == "unavailable" || e.State == "unknown" || e.State == StateMissing {
			missing++
		}
	}
	if missing > 0 {
		return integrations.State{Tone: integrations.ToneWarn, Key: "homeassistant.unavailable", Params: map[string]any{"n": missing}}
	}
	return integrations.State{Tone: integrations.ToneGood, Key: "homeassistant.running"}
}
