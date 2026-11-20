// Package icons resuelve el icono de cada conexión con el índice que genera la compilación web.
package icons

import (
	"encoding/json"
	"io/fs"
)

// None es el valor guardado de una conexión sin icono.
const None = "none"

// Icon es lo que el panel necesita para pintar un icono: un dato visual, sin nada del servicio.
type Icon struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Hex   string `json:"hex"`
}

// Defaults es el icono de cada tipo de conexión, el mismo que el catálogo.
var Defaults = map[string]string{
	"adguard":       "adguard",
	"availability":  "talaia-availability",
	"homeassistant": "homeassistant",
	"nut":           "talaia-nut",
	"opnsense":      "opnsense",
	"portainer":     "portainer",
	"printer":       "talaia-printer",
	"proxmox":       "proxmox",
	"qnap":          "qnap",
	"wgdashboard":   "wireguard",
}

// Set es el índice de iconos disponibles.
type Set struct{ bySlug map[string]Icon }

// Load lee icons/index.json; sin índice (la web sin compilar) no hay iconos.
func Load(fsys fs.FS) Set {
	set := Set{bySlug: map[string]Icon{}}
	data, err := fs.ReadFile(fsys, "icons/index.json")
	if err != nil {
		return set
	}
	var list []Icon
	if json.Unmarshal(data, &list) != nil {
		return set
	}
	for _, icon := range list {
		set.bySlug[icon.Slug] = icon
	}
	return set
}

// Has indica si el icono está en el índice.
func (s Set) Has(slug string) bool {
	_, ok := s.bySlug[slug]
	return ok
}

// Resolve da el icono que se pinta: el del tipo si no se eligió otro, nil sin icono
// o si el elegido ya no existe.
func (s Set) Resolve(connType, stored string) *Icon {
	slug := stored
	if slug == "" {
		slug = Defaults[connType]
	}
	icon, ok := s.bySlug[slug]
	if !ok {
		return nil
	}
	return &icon
}
