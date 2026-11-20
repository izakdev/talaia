package homeassistant

import (
	"testing"

	"github.com/izakdev/talaia/internal/integrations"
	st "github.com/izakdev/talaia/internal/integrations/statetest"
)

func s(tone integrations.Tone, key string) integrations.State {
	return integrations.State{Tone: tone, Key: key}
}

const (
	good  = integrations.ToneGood
	warn  = integrations.ToneWarn
	bad   = integrations.ToneBad
	muted = integrations.ToneMuted
)

func TestState(t *testing.T) {
	ents := []Entity{{State: "on"}, {State: "unavailable"}, {State: "unknown"}, {State: "21.5"}}
	st.Run(t, []st.Case{
		st.C("arrancando", Data{RunState: "NOT_RUNNING", Recovery: true}, s(warn, "homeassistant.starting")),
		st.C("modo recuperación", Data{RunState: "RUNNING", Recovery: true, SafeMode: true}, s(bad, "homeassistant.recovery")),
		st.C("modo seguro", Data{RunState: "RUNNING", SafeMode: true, Entities: ents}, s(warn, "homeassistant.safeMode")),
		st.C("entidades sin datos", Data{RunState: "RUNNING", Entities: ents}, integrations.State{Tone: warn, Key: "homeassistant.unavailable", Params: map[string]any{"n": 2}}),
		st.C("en marcha", Data{RunState: "RUNNING", Entities: []Entity{{State: "on"}}}, s(good, "homeassistant.running")),
		st.C("una entidad que falta", Data{RunState: "RUNNING", Entities: []Entity{{State: "missing"}}}, integrations.State{Tone: warn, Key: "homeassistant.unavailable", Params: map[string]any{"n": 1}}),
	})
}
