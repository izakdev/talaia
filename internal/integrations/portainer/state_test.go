package portainer

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
		st.C("sin entornos", Data{}, s(muted, "portainer.none")),
		st.C("entorno caído", Data{Environments: []Environment{{Online: true}, {Online: false, Containers: &Containers{Unhealthy: 1}}}}, s(bad, "portainer.down")),
		st.C("contenedor con problemas", Data{Environments: []Environment{{Online: true, Containers: &Containers{Unhealthy: 2}}}}, s(warn, "portainer.problems")),
		st.C("todo en línea", Data{Environments: []Environment{{Online: true, Containers: &Containers{Running: 3}}, {Online: true}}}, s(good, "portainer.allOnline")),
	})
}
