package api

import (
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/izakdev/talaia/internal/auth"
	"github.com/izakdev/talaia/internal/store"
)

const (
	gridColumns     = 12
	minWidgetWidth  = 2
	minWidgetHeight = 2
	maxWidgetHeight = 60
	maxGridY        = 2000
	maxPages        = 20
	maxPageName     = 40
)

var (
	panelID = regexp.MustCompile(`^[a-zA-Z0-9-]{1,64}$`)
	themes  = []string{"auto", "light", "dark"}
	accents = []string{"amber", "teal", "lime", "coral", "pink"}
)

func (s *Server) getPanel(w http.ResponseWriter, r *http.Request) {
	pages, err := s.store.Panel(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages})
}

func validatePanel(pages []store.Page, connections map[string]bool) string {
	if len(pages) == 0 || len(pages) > maxPages {
		return "invalid_pages"
	}
	pageIDs, widgetIDs := map[string]bool{}, map[string]bool{}
	for i := range pages {
		page := &pages[i]
		page.Name = strings.TrimSpace(page.Name)
		if !panelID.MatchString(page.ID) || pageIDs[page.ID] {
			return "invalid_pages"
		}
		pageIDs[page.ID] = true
		if page.Name == "" || len([]rune(page.Name)) > maxPageName {
			return "invalid_page_name"
		}
		placed := map[string]bool{}
		for j, widget := range page.Widgets {
			if !panelID.MatchString(widget.ID) || widgetIDs[widget.ID] || !connections[widget.ConnectionID] || placed[widget.ConnectionID] {
				return "invalid_widget"
			}
			widgetIDs[widget.ID] = true
			placed[widget.ConnectionID] = true
			if widget.X < 0 || widget.Y < 0 || widget.Y > maxGridY || widget.W < minWidgetWidth || widget.X+widget.W > gridColumns ||
				widget.H < minWidgetHeight || widget.H > maxWidgetHeight {
				return "invalid_widget"
			}
			for _, other := range page.Widgets[:j] {
				if widget.X < other.X+other.W && other.X < widget.X+widget.W && widget.Y < other.Y+other.H && other.Y < widget.Y+widget.H {
					return "widgets_overlap"
				}
			}
		}
		if page.Widgets == nil {
			page.Widgets = []store.Widget{}
		}
	}
	return ""
}

func (s *Server) savePanel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Pages []store.Page `json:"pages"`
	}
	if !decode(w, r, &in) {
		return
	}
	conns, err := s.store.ListConnections(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	known := make(map[string]bool, len(conns))
	for _, c := range conns {
		known[c.ID] = true
	}
	if code := validatePanel(in.Pages, known); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	if err := s.store.SavePanel(r.Context(), in.Pages); err != nil {
		s.internal(w, err)
		return
	}
	s.collector.Notify()
	writeJSON(w, http.StatusOK, map[string]any{"pages": in.Pages})
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.GetSettings(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var in store.Settings
	if !decode(w, r, &in) {
		return
	}
	if !slices.Contains(themes, in.Theme) || !slices.Contains(accents, in.Accent) {
		writeError(w, http.StatusBadRequest, "invalid_settings")
		return
	}
	if err := s.store.SaveSettings(r.Context(), in); err != nil {
		s.internal(w, err)
		return
	}
	// Cierra los flujos abiertos que ya no puedan ver el panel.
	s.collector.Notify()
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.failures.blocked(ip) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts")
		return
	}
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len([]rune(in.New)) < auth.MinPasswordLen || len(in.New) > 1024 {
		writeError(w, http.StatusBadRequest, "weak_password")
		return
	}
	if len(in.Current) > 1024 {
		writeError(w, http.StatusBadRequest, "wrong_password")
		return
	}
	user, ok := r.Context().Value(ctxKey{}).(store.User)
	if !ok {
		s.internal(w, errors.New("sesión sin usuario"))
		return
	}
	if !s.acquireHash(w, r) {
		return
	}
	valid, err := auth.VerifyPassword(user.PasswordHash, in.Current)
	var hash string
	if err == nil && valid {
		hash, err = auth.HashPassword(in.New)
	}
	<-s.hashSlots
	if err != nil {
		s.internal(w, err)
		return
	}
	if !valid {
		s.failures.record(ip)
		writeError(w, http.StatusBadRequest, "wrong_password")
		return
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		s.internal(w, err)
		return
	}
	if err := s.store.ChangePassword(r.Context(), user.ID, hash, auth.HashToken(cookie.Value)); err != nil {
		s.internal(w, err)
		return
	}
	// El flujo de otras sesiones revocadas se cierra si el panel exige inicio de sesión.
	s.collector.Notify()
	w.WriteHeader(http.StatusNoContent)
}
