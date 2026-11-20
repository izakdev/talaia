package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
)

var now = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)

func change(name, typ string, tone, previous integrations.Tone, key string) Change {
	return Change{Name: name, Type: typ, Tone: tone, Previous: previous, State: integrations.State{Tone: tone, Key: key}}
}

func TestComposeGroupsProblemsAndRecoveries(t *testing.T) {
	m := Compose("es", []Change{
		change("AdGuard", "adguard", integrations.ToneGood, integrations.ToneWarn, "adguard.protectionOn"),
		change("VPN", "wgdashboard", integrations.ToneWarn, integrations.ToneGood, "wgdashboard.down"),
		change("Portainer", "portainer", integrations.ToneBad, integrations.ToneGood, "portainer.down"),
	}, now)
	want := "🔴 Talaia · Necesita atención\nPortainer: Entorno caído\nVPN: Túnel parado\n\n🟢 Talaia · Vuelve a estar en orden\nAdGuard: Protección activa"
	if m.Text != want {
		t.Fatalf("texto:\n%s\nquiero:\n%s", m.Text, want)
	}
	if m.Title != "🔴 Talaia · Necesita atención" || m.Worst != integrations.ToneBad || m.Event != "change" || m.Language != "es" || !m.SentAt.Equal(now) {
		t.Fatalf("mensaje = %+v", m)
	}
	if len(m.Changes) != 3 || m.Changes[0].Name != "Portainer" || m.Changes[2].Name != "AdGuard" {
		t.Fatalf("cambios en orden de gravedad: %+v", m.Changes)
	}
}

func TestComposeWarnOnly(t *testing.T) {
	m := Compose("es", []Change{change("VPN", "wgdashboard", integrations.ToneWarn, integrations.ToneGood, "wgdashboard.down")}, now)
	if m.Text != "🟠 Talaia · Necesita atención\nVPN: Túnel parado" || m.Worst != integrations.ToneWarn {
		t.Fatalf("mensaje = %+v", m)
	}
}

func TestComposeEnglish(t *testing.T) {
	m := Compose("en", []Change{change("VPN", "wgdashboard", integrations.ToneWarn, integrations.ToneGood, "wgdashboard.down")}, now)
	if m.Text != "🟠 Talaia · Needs attention\nVPN: Tunnel stopped" {
		t.Fatalf("texto = %q", m.Text)
	}
}

func TestComposeOnlyRecoveries(t *testing.T) {
	m := Compose("es", []Change{change("AdGuard", "adguard", integrations.ToneGood, integrations.ToneBad, "adguard.protectionOn")}, now)
	if m.Text != "🟢 Talaia · Vuelve a estar en orden\nAdGuard: Protección activa" || m.Worst != integrations.ToneGood {
		t.Fatalf("mensaje = %+v", m)
	}
}

func TestComposeTest(t *testing.T) {
	es, en := ComposeTest("es", now), ComposeTest("en", now)
	if es.Text != "🔔 Talaia · Mensaje de prueba. Si lo ves, este canal funciona." || es.Event != "test" || es.Title != es.Text {
		t.Fatalf("es = %+v", es)
	}
	if en.Text != "🔔 Talaia · Test message. If you can see it, this channel works." {
		t.Fatalf("en = %q", en.Text)
	}
}

func TestComposeSanitizesNames(t *testing.T) {
	long := strings.Repeat("x", 200)
	m := Compose("es", []Change{
		change("a\nb\r\tc", "adguard", integrations.ToneWarn, integrations.ToneGood, "adguard.protectionOff"),
		change(long, "adguard", integrations.ToneWarn, integrations.ToneGood, "adguard.protectionOff"),
	}, now)
	lines := strings.Split(m.Text, "\n")
	if len(lines) != 3 || lines[1] != "a b c: Protección desactivada" {
		t.Fatalf("líneas = %q", lines)
	}
	if name, _, _ := strings.Cut(lines[2], ":"); len([]rune(name)) != 80 {
		t.Fatalf("nombre de %d caracteres", len([]rune(name)))
	}
	if m.Changes[0].Name != "a b c" {
		t.Fatalf("el webhook también recibe el nombre limpio: %q", m.Changes[0].Name)
	}
}

func TestLabelParams(t *testing.T) {
	s := integrations.State{Tone: integrations.ToneWarn, Key: "proxmox.nodesDown", Params: map[string]any{"online": 1, "total": 2}}
	if got := Label("es", s); got != "1 de 2 nodos en línea" {
		t.Fatalf("Label = %q", got)
	}
	if got := Label("fr", integrations.State{Key: "adguard.protectionOn"}); got != "Protección activa" {
		t.Fatalf("idioma desconocido = %q", got)
	}
	if got := Label("es", integrations.State{Key: "no.existe"}); got != "no.existe" {
		t.Fatalf("clave desconocida = %q", got)
	}
}
