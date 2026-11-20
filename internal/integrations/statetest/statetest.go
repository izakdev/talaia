// Package statetest ayuda a probar el estado de las integraciones.
package statetest

import (
	"reflect"
	"testing"

	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/web"
)

// Case es un caso de prueba de State.
type Case struct {
	Name string
	Data integrations.Stater
	Want integrations.State
}

// Run comprueba cada caso y que su clave existe en los textos de ambos idiomas.
func Run(t *testing.T, cases []Case) {
	t.Helper()
	labels := web.StateLabels()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			got := c.Data.State()
			if !reflect.DeepEqual(got, c.Want) {
				t.Fatalf("State() = %+v, quiero %+v", got, c.Want)
			}
			if labels["es"][got.Key] == "" || labels["en"][got.Key] == "" {
				t.Fatalf("la clave %q no está en states.json", got.Key)
			}
		})
	}
}

// C crea un caso.
func C(name string, data integrations.Stater, want integrations.State) Case {
	return Case{Name: name, Data: data, Want: want}
}
