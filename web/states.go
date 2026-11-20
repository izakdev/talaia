package web

import (
	_ "embed"
	"encoding/json"
)

// Textos de estado de los servicios, compartidos con el panel.
//
//go:embed src/i18n/states.json
var statesJSON []byte

var stateLabels = func() map[string]map[string]string {
	var labels map[string]map[string]string
	if err := json.Unmarshal(statesJSON, &labels); err != nil {
		panic(err)
	}
	return labels
}()

// StateLabels devuelve los textos de estado por idioma y clave. No se debe modificar.
func StateLabels() map[string]map[string]string { return stateLabels }
