package printer

import (
	"slices"

	"github.com/izakdev/talaia/internal/integrations"
)

// rules agrupa los motivos de printer-state-reasons por etiqueta, en orden de
// gravedad. Con blocking, el motivo como error impide imprimir (rojo); como
// aviso, o si no bloquea, es ámbar (DEC-58).
var rules = []struct {
	reasons  []string
	key      string
	blocking bool
}{
	{[]string{"media-jam"}, "printer.state.jam", true},
	{[]string{"door-open", "cover-open", "interlock-open"}, "printer.state.doorOpen", true},
	{[]string{"media-empty", "media-needed"}, "printer.state.mediaEmpty", true},
	{[]string{"toner-empty", "marker-supply-empty", "developer-empty"}, "printer.state.tonerEmpty", true},
	{[]string{"marker-waste-full"}, "printer.state.wasteFull", true},
	{[]string{"input-tray-missing", "output-tray-missing"}, "printer.state.trayMissing", true},
	{[]string{"output-area-full"}, "printer.state.outputFull", true},
	{[]string{"opc-life-over"}, "printer.state.drumEnd", true},
	{[]string{"toner-low", "marker-supply-low", "developer-low"}, "printer.state.tonerLow", false},
	{[]string{"marker-waste-almost-full"}, "printer.state.wasteAlmostFull", false},
	{[]string{"opc-near-eol"}, "printer.state.drumNearEnd", false},
	{[]string{"media-low"}, "printer.state.mediaLow", false},
	{[]string{"output-area-almost-full"}, "printer.state.outputAlmostFull", false},
	{[]string{"fuser-over-temp"}, "printer.state.fuser", false},
}

var lowKeys = map[string]string{
	"#000000": "printer.state.lowBlack",
	"#00FFFF": "printer.state.lowCyan",
	"#FF00FF": "printer.state.lowMagenta",
	"#FFFF00": "printer.state.lowYellow",
}

func (d Data) has(reasons []string, severity string) bool {
	for _, r := range d.Reasons {
		if slices.Contains(reasons, r.Reason) && (severity == "" || r.Severity == severity) {
			return true
		}
	}
	return false
}

// State devuelve el estado de la impresora para el panel y los avisos.
func (d Data) State() integrations.State {
	for _, r := range rules {
		if r.blocking && d.has(r.reasons, SeverityError) {
			return integrations.State{Tone: integrations.ToneBad, Key: r.key}
		}
	}
	if d.Status == StateStopped {
		return integrations.State{Tone: integrations.ToneBad, Key: "printer.state.stopped"}
	}
	for _, r := range rules {
		if d.has(r.reasons, "") {
			return integrations.State{Tone: integrations.ToneWarn, Key: r.key}
		}
	}
	// El consumible más gastado de los que están en su nivel bajo o por debajo.
	var low *Marker
	for i, m := range d.Markers {
		if m.Level != nil && *m.Level <= m.Low && (low == nil || *m.Level < *low.Level) {
			low = &d.Markers[i]
		}
	}
	if low != nil {
		// Los cuatro colores de proceso tienen su etiqueta traducida; el resto, su nombre.
		if key, ok := lowKeys[low.Color]; ok {
			return integrations.State{Tone: integrations.ToneWarn, Key: key, Params: map[string]any{"level": *low.Level}}
		}
		return integrations.State{Tone: integrations.ToneWarn, Key: "printer.state.supplyLow", Params: map[string]any{"name": low.Name, "level": *low.Level}}
	}
	if len(d.Reasons) > 0 {
		return integrations.State{Tone: integrations.ToneWarn, Key: "printer.state.warning"}
	}
	if d.Status == StateProcessing {
		return integrations.State{Tone: integrations.ToneGood, Key: "printer.state.printing"}
	}
	return integrations.State{Tone: integrations.ToneGood, Key: "printer.state.idle"}
}
