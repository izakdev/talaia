package availability

import "github.com/izakdev/talaia/internal/integrations"

// State devuelve el estado del equipo para el panel y los avisos: verde si
// conecta por TCP o responde por HTTP con 2xx o 3xx, ámbar con otro código.
// Si no responde, Collect falla y el bloque sale en rojo.
func (d Data) State() integrations.State {
	if d.Check == CheckHTTP && (d.HTTPStatus < 200 || d.HTTPStatus >= 400) {
		return integrations.State{Tone: integrations.ToneWarn, Key: "availability.state.httpError", Params: map[string]any{"code": d.HTTPStatus}}
	}
	return integrations.State{Tone: integrations.ToneGood, Key: "availability.state.up"}
}
