package icons

import (
	"os"
	"regexp"
	"testing"
	"testing/fstest"
)

var testFS = fstest.MapFS{"icons/index.json": {Data: []byte(`[
	{"slug":"adguard","title":"AdGuard","hex":"68BC71"},
	{"slug":"pihole","title":"Pi-hole","hex":"96060C"},
	{"slug":"proxmox","title":"Proxmox","hex":"E57000"}
]`)}}

func TestResolveDefault(t *testing.T) {
	got := Load(testFS).Resolve("proxmox", "")
	if got == nil || *got != (Icon{Slug: "proxmox", Title: "Proxmox", Hex: "E57000"}) {
		t.Fatalf("icono por defecto: %+v", got)
	}
}

func TestResolveNone(t *testing.T) {
	if got := Load(testFS).Resolve("proxmox", None); got != nil {
		t.Fatalf("sin icono: %+v", got)
	}
}

func TestResolveCustom(t *testing.T) {
	if got := Load(testFS).Resolve("adguard", "pihole"); got == nil || got.Slug != "pihole" {
		t.Fatalf("icono elegido: %+v", got)
	}
}

func TestResolveUnknownStoredSlug(t *testing.T) {
	if got := Load(testFS).Resolve("adguard", "desaparecido"); got != nil {
		t.Fatalf("un icono que ya no existe: %+v", got)
	}
}

func TestHas(t *testing.T) {
	set := Load(testFS)
	if !set.Has("pihole") || set.Has("no-existe") || set.Has(None) {
		t.Fatal("Has no distingue los iconos del índice")
	}
}

func TestLoadMissingIndex(t *testing.T) {
	set := Load(fstest.MapFS{})
	if set.Has("adguard") || set.Resolve("adguard", "") != nil {
		t.Fatal("sin índice no hay iconos")
	}
}

func TestLoadBrokenIndex(t *testing.T) {
	if Load(fstest.MapFS{"icons/index.json": {Data: []byte("{")}}).Has("adguard") {
		t.Fatal("un índice roto no da iconos")
	}
}

// El catálogo de la web y Defaults deben dar el mismo icono por defecto: si no,
// elegir en el formulario el icono «del tipo» guardaría uno y el panel pintaría otro.
func TestDefaultsMatchCatalog(t *testing.T) {
	source, err := os.ReadFile("../../web/src/catalog.ts")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, m := range regexp.MustCompile(`type: "(\w+)", iconSlug: "([\w-]+)"`).FindAllStringSubmatch(string(source), -1) {
		found[m[1]] = m[2]
	}
	if len(found) != len(Defaults) {
		t.Fatalf("el catálogo tiene %d iconos por defecto y Defaults %d", len(found), len(Defaults))
	}
	for typ, slug := range Defaults {
		if found[typ] != slug {
			t.Errorf("%s: catálogo %q, Defaults %q", typ, found[typ], slug)
		}
	}
}
