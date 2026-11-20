package nut

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
	st.Run(t, []st.Case{
		st.C("batería baja", Data{Status: []string{"OB", "LB"}}, s(bad, "nut.state.lowBattery")),
		st.C("con batería", Data{Status: []string{"OB", "DISCHRG"}}, s(warn, "nut.state.onBattery")),
		st.C("cambiar batería", Data{Status: []string{"OL", "RB"}}, s(warn, "nut.state.replaceBattery")),
		st.C("sobrecarga", Data{Status: []string{"OL", "OVER"}}, s(warn, "nut.state.overload")),
		st.C("con red", Data{Status: []string{"OL", "CHRG"}}, s(good, "nut.state.online")),
		st.C("desconocido", Data{Status: []string{"BYPASS"}}, s(muted, "nut.state.unknown")),
	})
}
