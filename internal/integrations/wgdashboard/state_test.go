package wgdashboard

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
		st.C("sin túneles", Data{}, s(muted, "wgdashboard.none")),
		st.C("túnel parado", Data{Tunnels: []Tunnel{{Up: true}, {Up: false}}}, s(warn, "wgdashboard.down")),
		st.C("todos activos", Data{Tunnels: []Tunnel{{Up: true}}}, s(good, "wgdashboard.allUp")),
	})
}
