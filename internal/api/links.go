package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/izakdev/talaia/internal/icons"
	"github.com/izakdev/talaia/internal/store"
)

const (
	maxLinkGroups   = 20
	maxLinks        = 100
	maxLinkGroupLen = 40
	maxLinkNameLen  = 60
	maxLinkURLLen   = 2048
	maxLinksBody    = 512 << 10
)

// validLink admite solo http o https con host y sin credenciales: el enlace se muestra en el panel.
func validLink(raw string) bool {
	link, err := url.Parse(raw)
	return err == nil && (link.Scheme == "http" || link.Scheme == "https") && link.Host != "" && link.User == nil && len(raw) <= maxLinkURLLen
}

type linksBody struct {
	Groups []store.LinkGroup `json:"groups"`
}

func (s *Server) getLinks(w http.ResponseWriter, r *http.Request) {
	groups, err := s.store.LinkGroups(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, linksBody{groups})
}

func (s *Server) saveLinks(w http.ResponseWriter, r *http.Request) {
	var in linksBody
	if !decode(w, r, &in) {
		return
	}
	saved, err := s.store.LinkGroups(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	// Un icono ya guardado se admite aunque una versión nueva de simple-icons lo haya retirado.
	savedIcons := map[string]string{}
	for _, g := range saved {
		for _, l := range g.Links {
			savedIcons[l.ID] = l.Icon
		}
	}
	if code := validateLinks(in.Groups, s.icons, savedIcons); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	if err := s.store.SaveLinkGroups(r.Context(), in.Groups); err != nil {
		s.internal(w, err)
		return
	}
	s.collector.Notify()
	writeJSON(w, http.StatusOK, linksBody{in.Groups})
}

// validateLinks comprueba y recorta los grupos; devuelve el código del primer error.
func validateLinks(groups []store.LinkGroup, index icons.Set, savedIcons map[string]string) string {
	if groups == nil || len(groups) > maxLinkGroups {
		return "invalid_links"
	}
	ids := map[string]bool{}
	total := 0
	for i := range groups {
		g := &groups[i]
		if !panelID.MatchString(g.ID) || ids["g:"+g.ID] {
			return "invalid_links"
		}
		ids["g:"+g.ID] = true
		g.Name = strings.TrimSpace(g.Name)
		if g.Name == "" || len([]rune(g.Name)) > maxLinkGroupLen {
			return "invalid_group_name"
		}
		if g.Links == nil {
			g.Links = []store.Link{}
		}
		total += len(g.Links)
		if total > maxLinks {
			return "invalid_links"
		}
		for j := range g.Links {
			l := &g.Links[j]
			if !panelID.MatchString(l.ID) || ids["l:"+l.ID] {
				return "invalid_links"
			}
			ids["l:"+l.ID] = true
			l.Name, l.URL, l.Icon = strings.TrimSpace(l.Name), strings.TrimSpace(l.URL), strings.TrimSpace(l.Icon)
			if l.Name == "" || len([]rune(l.Name)) > maxLinkNameLen {
				return "invalid_link_name"
			}
			if !validLink(l.URL) {
				return "invalid_link"
			}
			if l.Icon != "" && !index.Has(l.Icon) && savedIcons[l.ID] != l.Icon {
				return "invalid_icon"
			}
		}
	}
	return ""
}

type publicLink struct {
	Name string      `json:"name"`
	URL  string      `json:"url"`
	Icon *icons.Icon `json:"icon"`
}

type publicLinkGroup struct {
	Name  string       `json:"name"`
	Links []publicLink `json:"links"`
}

// publicLinks da los grupos para el panel: sin ID ni grupos vacíos, con el icono resuelto.
func (s *Server) publicLinks(groups []store.LinkGroup) []publicLinkGroup {
	out := []publicLinkGroup{}
	for _, g := range groups {
		if len(g.Links) == 0 {
			continue
		}
		pg := publicLinkGroup{Name: g.Name}
		for _, l := range g.Links {
			pg.Links = append(pg.Links, publicLink{l.Name, l.URL, s.icons.Lookup(l.Icon)})
		}
		out = append(out, pg)
	}
	return out
}
