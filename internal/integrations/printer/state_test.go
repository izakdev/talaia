package printer

import (
	"testing"

	"github.com/izakdev/talaia/internal/integrations"
	st "github.com/izakdev/talaia/internal/integrations/statetest"
)

func s(tone integrations.Tone, key string) integrations.State {
	return integrations.State{Tone: tone, Key: key}
}

const (
	good = integrations.ToneGood
	warn = integrations.ToneWarn
	bad  = integrations.ToneBad
)

func level(n int) *int { return &n }

func toners(levels ...int) []Marker {
	out := make([]Marker, len(levels))
	for i, l := range levels {
		out[i] = Marker{Name: []string{"BK", "C", "M", "Y"}[i], Color: []string{"#000000", "#00FFFF", "#FF00FF", "#FFFF00"}[i], Type: "toner", Level: level(l), Low: 10}
		if l < 0 {
			out[i].Level = nil
		}
	}
	return out
}

func with(state string, reasons ...Reason) Data {
	return Data{Status: state, Reasons: reasons, Markers: toners(90, 90, 90, 90)}
}

func e(r string) Reason { return Reason{r, SeverityError} }
func w(r string) Reason { return Reason{r, SeverityWarning} }

func TestState(t *testing.T) {
	low := Data{Status: StateIdle, Reasons: []Reason{}, Markers: toners(80, 9, 5, -1)}
	st.Run(t, []st.Case{
		st.C("atasco", with(StateStopped, e("media-jam"), e("cover-open")), s(bad, "printer.state.jam")),
		st.C("tapa abierta", with(StateStopped, e("cover-open")), s(bad, "printer.state.doorOpen")),
		st.C("puerta abierta", with(StateStopped, e("door-open")), s(bad, "printer.state.doorOpen")),
		st.C("sin papel", with(StateStopped, e("media-empty")), s(bad, "printer.state.mediaEmpty")),
		st.C("sin tóner", with(StateStopped, e("toner-empty")), s(bad, "printer.state.tonerEmpty")),
		st.C("caja residual llena", with(StateStopped, e("marker-waste-full")), s(bad, "printer.state.wasteFull")),
		st.C("falta la bandeja", with(StateStopped, e("input-tray-missing")), s(bad, "printer.state.trayMissing")),
		st.C("salida llena", with(StateStopped, e("output-area-full")), s(bad, "printer.state.outputFull")),
		st.C("tambor agotado", with(StateStopped, e("opc-life-over")), s(bad, "printer.state.drumEnd")),
		st.C("detenida sin motivo", with(StateStopped), s(bad, "printer.state.stopped")),
		st.C("detenida en pausa", with(StateStopped, e("paused")), s(bad, "printer.state.stopped")),
		st.C("bandeja vacía con otra", with(StateIdle, w("media-empty")), s(warn, "printer.state.mediaEmpty")),
		st.C("tóner bajo", with(StateIdle, w("toner-low")), s(warn, "printer.state.tonerLow")),
		st.C("tóner bajo sin sufijo", with(StateIdle, e("toner-low")), s(warn, "printer.state.tonerLow")),
		st.C("caja residual casi llena", with(StateIdle, w("marker-waste-almost-full")), s(warn, "printer.state.wasteAlmostFull")),
		st.C("tambor casi agotado", with(StateIdle, w("opc-near-eol")), s(warn, "printer.state.drumNearEnd")),
		st.C("poco papel", with(StateIdle, w("media-low")), s(warn, "printer.state.mediaLow")),
		st.C("salida casi llena", with(StateIdle, w("output-area-almost-full")), s(warn, "printer.state.outputAlmostFull")),
		st.C("fusor", with(StateProcessing, w("fuser-over-temp")), s(warn, "printer.state.fuser")),
		st.C("magenta bajo", low, integrations.State{Tone: warn, Key: "printer.state.lowMagenta", Params: map[string]any{"level": 5}}),
		st.C("negro bajo", Data{Status: StateIdle, Markers: toners(3, 50, 50, 50)}, integrations.State{Tone: warn, Key: "printer.state.lowBlack", Params: map[string]any{"level": 3}}),
		st.C("cian bajo", Data{Status: StateIdle, Markers: toners(50, 10, 50, 50)}, integrations.State{Tone: warn, Key: "printer.state.lowCyan", Params: map[string]any{"level": 10}}),
		st.C("amarillo bajo", Data{Status: StateIdle, Markers: toners(50, 50, 50, 0)}, integrations.State{Tone: warn, Key: "printer.state.lowYellow", Params: map[string]any{"level": 0}}),
		st.C("otro consumible bajo", Data{Status: StateIdle, Markers: []Marker{{Name: "Light Cyan", Color: "#80FFFF", Level: level(4), Low: 10}}}, integrations.State{Tone: warn, Key: "printer.state.supplyLow", Params: map[string]any{"name": "Light Cyan", "level": 4}}),
		st.C("aviso desconocido", with(StateIdle, w("other")), s(warn, "printer.state.warning")),
		st.C("imprimiendo", with(StateProcessing), s(good, "printer.state.printing")),
		st.C("lista", with(StateIdle), s(good, "printer.state.idle")),
		st.C("lista con niveles desconocidos", Data{Status: StateIdle, Markers: toners(-3, -3, -3, -3)}, s(good, "printer.state.idle")),
	})
}
