package proxmox

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
	no, yes := false, true
	st.Run(t, []st.Case{
		st.C("sin quórum", Data{Quorate: &no, NodesOnline: 1, NodesTotal: 3}, s(bad, "proxmox.noQuorum")),
		st.C("nodos caídos", Data{Quorate: &yes, ClusterName: "c", NodesOnline: 1, NodesTotal: 2}, integrations.State{Tone: warn, Key: "proxmox.nodesDown", Params: map[string]any{"online": 1, "total": 2}}),
		st.C("clúster bien", Data{Quorate: &yes, ClusterName: "c", NodesOnline: 2, NodesTotal: 2}, s(good, "proxmox.clusterOk")),
		st.C("nodo suelto bien", Data{NodesOnline: 1, NodesTotal: 1}, s(good, "proxmox.nodeOk")),
	})
}
