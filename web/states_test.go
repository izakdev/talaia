package web

import "testing"

func TestStateLabelsBothLanguages(t *testing.T) {
	labels := StateLabels()
	es, en := labels["es"], labels["en"]
	if len(es) == 0 || len(es) != len(en) {
		t.Fatalf("es=%d en=%d", len(es), len(en))
	}
	for key, text := range es {
		if text == "" || en[key] == "" {
			t.Errorf("falta %q en algún idioma", key)
		}
	}
	if es["status.noResponse"] != "Con fallos" {
		t.Errorf("status.noResponse = %q", es["status.noResponse"])
	}
}
