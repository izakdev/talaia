package icons

import (
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
