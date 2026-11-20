package nut

import (
	"slices"

	"github.com/izakdev/talaia/internal/integrations"
)

// El primer estado de la lista da la etiqueta; el orden es el de gravedad.
var states = []struct {
	flag string
	tone integrations.Tone
	key  string
}{
	{"LB", integrations.ToneBad, "nut.state.lowBattery"},
	{"OB", integrations.ToneWarn, "nut.state.onBattery"},
	{"RB", integrations.ToneWarn, "nut.state.replaceBattery"},
	{"OVER", integrations.ToneWarn, "nut.state.overload"},
	{"OL", integrations.ToneGood, "nut.state.online"},
}

// State devuelve el estado del SAI para el panel y los avisos.
func (d Data) State() integrations.State {
	for _, s := range states {
		if slices.Contains(d.Status, s.flag) {
			return integrations.State{Tone: s.tone, Key: s.key}
		}
	}
	return integrations.State{Tone: integrations.ToneMuted, Key: "nut.state.unknown"}
}
