package opnsense

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

func gw(states ...string) []Gateway {
	out := []Gateway{}
	for _, state := range states {
		out = append(out, Gateway{State: state})
	}
	return out
}

func TestState(t *testing.T) {
	down := &WireGuard{Tunnels: []WireGuardTunnel{{Up: true}, {Up: false}}}
	st.Run(t, []st.Case{
		st.C("sin gateways", Data{}, s(muted, "opnsense.noGateways")),
		st.C("gateway caído", Data{Gateways: gw("online", "offline", "degraded")}, s(bad, "opnsense.offline")),
		st.C("gateway degradado", Data{Gateways: gw("online", "degraded"), WireGuard: down}, s(warn, "opnsense.degraded")),
		st.C("túnel wireguard parado", Data{Gateways: gw("online"), WireGuard: down}, s(warn, "opnsense.wireguardDown")),
		st.C("sin monitorizar", Data{Gateways: gw("none")}, s(muted, "opnsense.unmonitored")),
		st.C("en línea", Data{Gateways: gw("online", "none"), WireGuard: &WireGuard{Tunnels: []WireGuardTunnel{{Up: true}}}}, s(good, "opnsense.online")),
	})
}
