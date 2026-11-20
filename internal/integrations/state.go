package integrations

// Tone es el color de un servicio en el panel.
type Tone string

// Colores de un servicio.
const (
	ToneGood  Tone = "good"
	ToneWarn  Tone = "warn"
	ToneBad   Tone = "bad"
	ToneMuted Tone = "muted"
)

// State es el estado de un servicio: su color y la clave del texto que lo describe.
// Las claves están en web/src/i18n/states.json.
type State struct {
	Tone   Tone           `json:"tone"`
	Key    string         `json:"key"`
	Params map[string]any `json:"params,omitempty"`
}

// Stater lo implementan los datos de cada integración. Es la única fuente del
// color de un servicio, para el panel y para los avisos.
type Stater interface {
	State() State
}

// Severity ordena los colores: 0 sin problema, 1 aviso y 2 fallo.
func Severity(t Tone) int {
	switch t {
	case ToneWarn:
		return 1
	case ToneBad:
		return 2
	}
	return 0
}
