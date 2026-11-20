package adguard

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
		st.C("dns parado", Data{Running: false, ProtectionEnabled: true}, s(bad, "adguard.dnsStopped")),
		st.C("protección activa", Data{Running: true, ProtectionEnabled: true}, s(good, "adguard.protectionOn")),
		st.C("protección desactivada", Data{Running: true}, s(warn, "adguard.protectionOff")),
	})
}
