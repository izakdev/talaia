// Package api expone la API HTTP y sirve la web embebida.
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/izakdev/talaia/internal/auth"
	"github.com/izakdev/talaia/internal/collector"
	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/integrations/adguard"
	"github.com/izakdev/talaia/internal/integrations/homeassistant"
	"github.com/izakdev/talaia/internal/integrations/proxmox"
	"github.com/izakdev/talaia/internal/store"
)

const (
	cookieName      = "talaia_session"
	sessionLifetime = 30 * 24 * time.Hour
	maxBody         = 64 << 10
	maxFailures     = 10
	failureWindow   = 15 * time.Minute
	maxEntities     = 200
	sseHeartbeat    = 25 * time.Second
	sseWriteTimeout = 10 * time.Second
)

// Server contiene las dependencias de la API.
type Server struct {
	store   *store.Store
	log     *slog.Logger
	version string
	web     fs.FS
	// Limita los hashes simultáneos: cada Argon2id reserva 64 MiB.
	hashSlots chan struct{}
	dummyHash string
	failures  *failureLimiter
	collector *collector.Manager
	registry  integrations.Registry
	// SetupTokenFile, si existe ese archivo, protege el primer arranque con su contenido.
	SetupTokenFile string
}

// New crea el servidor. web puede estar vacío si el frontend no se ha compilado.
func New(st *store.Store, manager *collector.Manager, registry integrations.Registry, log *slog.Logger, version string, web fs.FS) (*Server, error) {
	dummy, err := auth.HashPassword("talaia-dummy-password")
	if err != nil {
		return nil, err
	}
	return &Server{
		store: st, log: log, version: version, web: web,
		hashSlots: make(chan struct{}, 2),
		dummyHash: dummy,
		failures:  &failureLimiter{entries: map[string]*failureEntry{}},
		collector: manager,
		registry:  registry,
	}, nil
}

// Handler devuelve el enrutador HTTP.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer, securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Route("/api", func(r chi.Router) {
		r.Use(sameOrigin, limitBody)
		r.Get("/state", s.state)
		r.Get("/dashboard", s.dashboard)
		r.Get("/dashboard/events", s.dashboardEvents)
		r.Post("/setup", s.setup)
		r.Post("/auth/login", s.login)
		r.Post("/auth/logout", s.logout)
		r.Route("/admin", func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/status", s.adminStatus)
			r.Get("/panel", s.getPanel)
			r.Put("/panel", s.savePanel)
			r.Get("/settings", s.getSettings)
			r.Put("/settings", s.saveSettings)
			r.Post("/password", s.changePassword)
			r.Get("/connections", s.listConnections)
			r.Post("/connections", s.createConnection)
			r.Post("/connections/test", s.testConnection)
			r.Post("/connections/entities", s.listEntities)
			r.Get("/connections/{id}", s.getConnection)
			r.Put("/connections/{id}", s.updateConnection)
			r.Delete("/connections/{id}", s.deleteConnection)
		})
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeError(w, http.StatusNotFound, "not_found") })
		r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		})
	})
	r.NotFound(s.serveWeb)
	return r
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// sameOrigin rechaza escrituras desde otro origen.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(u.Host, r.Host) {
					writeError(w, http.StatusForbidden, "cross_origin")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		next.ServeHTTP(w, r)
	})
}

type ctxKey struct{}

func (s *Server) currentUser(r *http.Request) (store.User, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return store.User{}, store.ErrNotFound
	}
	return s.store.UserBySession(r.Context(), auth.HashToken(c.Value))
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.currentUser(r)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			s.internal(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	needSetup, err := s.store.SetupRequired(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	settings, err := s.store.GetSettings(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	tokenRequired, err := s.setupTokenRequired(needSetup)
	if err != nil {
		s.internal(w, err)
		return
	}
	resp := map[string]any{
		"setup_required": needSetup, "setup_token_required": tokenRequired, "version": s.version, "user": nil,
		"theme": settings.Theme, "accent": settings.Accent, "require_login": settings.RequireLogin,
	}
	if u, err := s.currentUser(r); err == nil {
		resp["user"] = u.Username
	} else if !errors.Is(err, store.ErrNotFound) {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token,omitempty"`
}

func (s *Server) setupTokenRequired(needSetup bool) (bool, error) {
	if !needSetup || s.SetupTokenFile == "" {
		return false, nil
	}
	_, err := os.Stat(s.SetupTokenFile)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// validSetupToken compara el código en tiempo constante. Si no se puede leer o está
// vacío, nunca coincide.
func (s *Server) validSetupToken(token string) (bool, error) {
	raw, err := os.ReadFile(s.SetupTokenFile)
	if err != nil {
		return false, err
	}
	expected := strings.TrimSpace(string(raw))
	return expected != "" && subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1, nil
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || len(in.Username) > 64 {
		writeError(w, http.StatusBadRequest, "invalid_username")
		return
	}
	if len([]rune(in.Password)) < auth.MinPasswordLen || len(in.Password) > 1024 {
		writeError(w, http.StatusBadRequest, "weak_password")
		return
	}
	needSetup, err := s.store.SetupRequired(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	if !needSetup {
		writeError(w, http.StatusConflict, "already_setup")
		return
	}
	tokenRequired, err := s.setupTokenRequired(needSetup)
	if err != nil {
		s.internal(w, err)
		return
	}
	if tokenRequired {
		ip := clientIP(r)
		if s.failures.blocked(ip) {
			writeError(w, http.StatusTooManyRequests, "too_many_attempts")
			return
		}
		valid, err := s.validSetupToken(in.Token)
		if err != nil {
			s.internal(w, err)
			return
		}
		if !valid {
			s.failures.record(ip)
			writeError(w, http.StatusForbidden, "invalid_setup_token")
			return
		}
	}
	if !s.acquireHash(w, r) {
		return
	}
	hash, err := auth.HashPassword(in.Password)
	<-s.hashSlots
	if err != nil {
		s.internal(w, err)
		return
	}
	id, err := s.store.CreateFirstUser(r.Context(), in.Username, hash)
	if errors.Is(err, store.ErrAlreadySetup) {
		writeError(w, http.StatusConflict, "already_setup")
		return
	}
	if err != nil {
		s.internal(w, err)
		return
	}
	if tokenRequired {
		if err := os.Remove(s.SetupTokenFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.log.Warn("no se pudo eliminar el código de instalación", "error", err)
		}
	}
	s.startSession(w, r, id, in.Username)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.failures.blocked(ip) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts")
		return
	}
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	if len(in.Password) > 1024 {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	u, err := s.store.UserByUsername(r.Context(), strings.TrimSpace(in.Username))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.internal(w, err)
		return
	}
	found := err == nil
	hash := u.PasswordHash
	if !found {
		// Mismo coste que con un usuario real, para no revelar cuáles existen.
		hash = s.dummyHash
	}
	if !s.acquireHash(w, r) {
		return
	}
	ok, verr := auth.VerifyPassword(hash, in.Password)
	<-s.hashSlots
	if verr != nil {
		s.internal(w, verr)
		return
	}
	if !found || !ok {
		s.failures.record(ip)
		s.log.Warn("inicio de sesión fallido", "ip", ip)
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	s.failures.reset(ip)
	s.startSession(w, r, u.ID, u.Username)
}

func (s *Server) acquireHash(w http.ResponseWriter, r *http.Request) bool {
	select {
	case s.hashSlots <- struct{}{}:
		return true
	case <-r.Context().Done():
		writeError(w, http.StatusServiceUnavailable, "busy")
		return false
	}
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64, username string) {
	token, err := auth.NewSessionToken()
	if err != nil {
		s.internal(w, err)
		return
	}
	expires := time.Now().Add(sessionLifetime)
	if err := s.store.CreateSession(r.Context(), auth.HashToken(token), userID, expires); err != nil {
		s.internal(w, err)
		return
	}
	http.SetCookie(w, s.cookie(r, token, expires))
	writeJSON(w, http.StatusOK, map[string]string{"user": username})
}

func (s *Server) cookie(r *http.Request, value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name: cookieName, Value: value, Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	}
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if err := s.store.DeleteSession(r.Context(), auth.HashToken(c.Value)); err != nil {
			s.internal(w, err)
			return
		}
	}
	http.SetCookie(w, s.cookie(r, "", time.Unix(0, 0)))
	w.WriteHeader(http.StatusNoContent)
}

// connectionTypes define qué credenciales exige cada integración.
var connectionTypes = map[string]struct {
	needsUsername bool
	options       []string
	hasEntities   bool
}{
	"proxmox":       {options: []string{proxmox.OptionShowGuests}},
	"homeassistant": {hasEntities: true, options: []string{homeassistant.OptionShowSummary}},
	"adguard":       {needsUsername: true, options: []string{adguard.OptionShowTopDomains}},
	"opnsense":      {needsUsername: true},
	"portainer":     {},
	"wgdashboard":   {},
}

// proxmoxToken valida el formato usuario@reino!nombre=valor.
var proxmoxToken = regexp.MustCompile(`^[^\s!=]+@[^\s!=]+![^\s!=]+=[^\s]+$`)

// entityID valida el formato dominio.objeto de Home Assistant.
var entityID = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)

type connectionIn struct {
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	BaseURL         string          `json:"base_url"`
	Username        string          `json:"username"`
	Secret          *string         `json:"secret"`
	IntervalSeconds int             `json:"interval_seconds"`
	TLSSkipVerify   bool            `json:"tls_skip_verify"`
	Options         map[string]bool `json:"options"`
	Entities        []string        `json:"entities"`
	LinkURL         string          `json:"link_url"`
}

type connectionOut struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	BaseURL         string          `json:"base_url"`
	Username        string          `json:"username"`
	IntervalSeconds int             `json:"interval_seconds"`
	TLSSkipVerify   bool            `json:"tls_skip_verify"`
	Options         map[string]bool `json:"options"`
	Entities        []string        `json:"entities"`
	LinkURL         string          `json:"link_url"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func toOut(c store.Connection) connectionOut {
	return connectionOut{c.ID, c.Name, c.Type, c.BaseURL, c.Username, c.IntervalSeconds, c.TLSSkipVerify, c.Options, c.Entities, c.LinkURL, c.UpdatedAt}
}

func validateConnection(in *connectionIn) string {
	in.Name = strings.TrimSpace(in.Name)
	in.Username = strings.TrimSpace(in.Username)
	if in.Name == "" || len(in.Name) > 80 {
		return "invalid_name"
	}
	spec, ok := connectionTypes[in.Type]
	if !ok {
		return "invalid_type"
	}
	u, err := url.Parse(strings.TrimSpace(in.BaseURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "invalid_url"
	}
	in.BaseURL = strings.TrimRight(u.String(), "/")
	in.LinkURL = strings.TrimSpace(in.LinkURL)
	if in.LinkURL != "" {
		// El enlace se muestra en el panel: solo http o https, nunca javascript: ni similares.
		link, err := url.Parse(in.LinkURL)
		if err != nil || (link.Scheme != "http" && link.Scheme != "https") || link.Host == "" || link.User != nil || len(in.LinkURL) > 2048 {
			return "invalid_link"
		}
	}
	if in.Type == "opnsense" {
		// El archivo de OPNsense trae las líneas key=… y secret=…: se admite pegarlas enteras.
		in.Username = strings.TrimPrefix(in.Username, "key=")
		if strings.ContainsFunc(in.Username, unicode.IsSpace) {
			return "invalid_api_key"
		}
	}
	if spec.needsUsername && in.Username == "" {
		return "invalid_username"
	}
	if !spec.needsUsername {
		in.Username = ""
	}
	for key := range in.Options {
		if !slices.Contains(spec.options, key) {
			return "invalid_options"
		}
	}
	if len(in.Entities) > 0 && !spec.hasEntities {
		return "invalid_entities"
	}
	if len(in.Entities) > maxEntities {
		return "too_many_entities"
	}
	seen := make(map[string]bool, len(in.Entities))
	for _, id := range in.Entities {
		if !entityID.MatchString(id) || seen[id] {
			return "invalid_entities"
		}
		seen[id] = true
	}
	if in.IntervalSeconds < 5 || in.IntervalSeconds > 3600 {
		return "invalid_interval"
	}
	if in.Secret != nil {
		// Los tokens se suelen pegar con un salto de línea, que no sería válido en una cabecera.
		if in.Type != "adguard" {
			trimmed := strings.TrimSpace(*in.Secret)
			if in.Type == "opnsense" {
				trimmed = strings.TrimPrefix(trimmed, "secret=")
			}
			in.Secret = &trimmed
		}
		if *in.Secret == "" || len(*in.Secret) > 4096 || strings.ContainsAny(*in.Secret, "\r\n") {
			return "invalid_secret"
		}
		if in.Type == "proxmox" && !proxmoxToken.MatchString(*in.Secret) {
			return "invalid_proxmox_token"
		}
	}
	return ""
}

func (s *Server) listConnections(w http.ResponseWriter, r *http.Request) {
	conns, err := s.store.ListConnections(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	out := make([]connectionOut, len(conns))
	for i, c := range conns {
		out[i] = toOut(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getConnection(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.GetConnection(r.Context(), chi.URLParam(r, "id"))
	if s.storeError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toOut(c))
}

func (s *Server) createConnection(w http.ResponseWriter, r *http.Request) {
	var in connectionIn
	if !decode(w, r, &in) {
		return
	}
	if in.Secret == nil {
		writeError(w, http.StatusBadRequest, "invalid_secret")
		return
	}
	if code := validateConnection(&in); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	c, err := s.store.CreateConnection(r.Context(), store.Connection{
		Name: in.Name, Type: in.Type, BaseURL: in.BaseURL, Username: in.Username,
		IntervalSeconds: in.IntervalSeconds, TLSSkipVerify: in.TLSSkipVerify, Options: in.Options, Entities: in.Entities, LinkURL: in.LinkURL,
	}, *in.Secret)
	if err != nil {
		s.internal(w, err)
		return
	}
	s.refresh(r, c.ID)
	writeJSON(w, http.StatusCreated, toOut(c))
}

func (s *Server) updateConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	current, err := s.store.GetConnection(r.Context(), id)
	if s.storeError(w, err) {
		return
	}
	var in connectionIn
	if !decode(w, r, &in) {
		return
	}
	// El tipo no cambia: el secreto guardado pertenece a esa integración.
	in.Type = current.Type
	if in.Secret != nil && *in.Secret == "" {
		in.Secret = nil
	}
	if code := validateConnection(&in); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	// Otro destino exige volver a escribir el secreto, para no enviarlo a un servidor distinto.
	if in.BaseURL != current.BaseURL && in.Secret == nil {
		writeError(w, http.StatusBadRequest, "secret_required_for_url_change")
		return
	}
	c, err := s.store.UpdateConnection(r.Context(), store.Connection{
		ID: id, Name: in.Name, BaseURL: in.BaseURL, Username: in.Username,
		IntervalSeconds: in.IntervalSeconds, TLSSkipVerify: in.TLSSkipVerify, Options: in.Options, Entities: in.Entities, LinkURL: in.LinkURL,
	}, in.Secret)
	if s.storeError(w, err) {
		return
	}
	s.refresh(r, id)
	writeJSON(w, http.StatusOK, toOut(c))
}

func (s *Server) refresh(r *http.Request, id string) {
	if err := s.collector.Refresh(r.Context(), id); err != nil {
		s.log.Error("no se pudo actualizar el recolector", "conexion", id, "error", err)
	}
}

// connectionFromRequest prepara una integración con los datos del formulario, sin
// guardar nada. Solo reutiliza la credencial guardada si la URL no ha cambiado.
func (s *Server) connectionFromRequest(w http.ResponseWriter, r *http.Request) (integrations.Integration, bool) {
	var in struct {
		ID string `json:"id"`
		connectionIn
	}
	if !decode(w, r, &in) {
		return nil, false
	}
	if in.Secret != nil && *in.Secret == "" {
		in.Secret = nil
	}
	var current store.Connection
	if in.ID != "" {
		var err error
		current, err = s.store.GetConnection(r.Context(), in.ID)
		if s.storeError(w, err) {
			return nil, false
		}
		in.Type = current.Type
	}
	if code := validateConnection(&in.connectionIn); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return nil, false
	}
	var secret string
	switch {
	case in.Secret != nil:
		secret = *in.Secret
	case in.ID == "":
		writeError(w, http.StatusBadRequest, "invalid_secret")
		return nil, false
	case in.BaseURL != current.BaseURL:
		writeError(w, http.StatusBadRequest, "secret_required_for_url_change")
		return nil, false
	default:
		var err error
		secret, err = s.store.ConnectionSecret(r.Context(), in.ID)
		if s.storeError(w, err) {
			return nil, false
		}
	}
	factory, supported := s.registry[in.Type]
	if !supported {
		writeError(w, http.StatusBadRequest, "not_supported")
		return nil, false
	}
	return factory(integrations.Config{
		BaseURL: in.BaseURL, Username: in.Username, Secret: secret, TLSSkipVerify: in.TLSSkipVerify,
		Options: in.Options, Entities: in.Entities,
	}), true
}

// testConnection consulta el servicio una vez, sin guardar nada.
func (s *Server) testConnection(w http.ResponseWriter, r *http.Request) {
	integration, ok := s.connectionFromRequest(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	data, err := integration.Collect(ctx)
	if err != nil {
		s.log.Info("prueba de conexión fallida", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": integrations.Code(err)})
		return
	}
	response := map[string]any{"ok": true}
	if noter, ok := data.(integrations.Noter); ok {
		if notes := noter.Notes(); len(notes) > 0 {
			response["notes"] = notes
		}
	}
	writeJSON(w, http.StatusOK, response)
}

// listEntities devuelve las entidades que se pueden elegir en una conexión.
func (s *Server) listEntities(w http.ResponseWriter, r *http.Request) {
	integration, ok := s.connectionFromRequest(w, r)
	if !ok {
		return
	}
	lister, ok := integration.(integrations.EntityLister)
	if !ok {
		writeError(w, http.StatusBadRequest, "not_supported")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	entities, err := lister.ListEntities(ctx)
	if err != nil {
		s.log.Info("no se pudieron listar las entidades", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": integrations.Code(err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "entities": entities})
}

// dashboardState devuelve las páginas y el estado de las conexiones colocadas en
// ellas. Lo que no está en el panel no sale por la API pública.
func (s *Server) dashboardState(ctx context.Context) (map[string]any, error) {
	pages, err := s.store.Panel(ctx)
	if err != nil {
		return nil, err
	}
	placed := map[string]bool{}
	for _, page := range pages {
		for _, widget := range page.Widgets {
			placed[widget.ConnectionID] = true
		}
	}
	connections := []collector.Snapshot{}
	for _, snapshot := range s.collector.Snapshots() {
		if placed[snapshot.ID] {
			connections = append(connections, snapshot)
		}
	}
	return map[string]any{"pages": pages, "connections": connections}, nil
}

// canViewDashboard indica si la petición puede ver el panel según los ajustes.
func (s *Server) canViewDashboard(r *http.Request) (bool, error) {
	settings, err := s.store.GetSettings(r.Context())
	if err != nil {
		return false, err
	}
	if !settings.RequireLogin {
		return true, nil
	}
	_, err = s.currentUser(r)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	allowed, err := s.canViewDashboard(r)
	if err != nil {
		s.internal(w, err)
		return
	}
	if !allowed {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	state, err := s.dashboardState(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// adminStatus devuelve el estado de todas las conexiones, estén o no en el panel.
func (s *Server) adminStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"connections": s.collector.Snapshots()})
}

// dashboardEvents envía el estado completo al conectar y en cada cambio.
func (s *Server) dashboardEvents(w http.ResponseWriter, r *http.Request) {
	allowed, err := s.canViewDashboard(r)
	if err != nil {
		s.internal(w, err)
		return
	}
	if !allowed {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	changes, unsubscribe, ok := s.collector.Subscribe()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "busy")
		return
	}
	defer unsubscribe()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	// Evita que un proxy como Nginx retenga los eventos.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	send := func(payload []byte) bool {
		if err := rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return false
		}
		if _, err := w.Write(payload); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	sendSnapshot := func() bool {
		// Los ajustes o la sesión pueden cambiar con el flujo abierto.
		if allowed, err := s.canViewDashboard(r); err != nil || !allowed {
			return false
		}
		state, err := s.dashboardState(r.Context())
		if err != nil {
			s.log.Error("no se pudo leer el panel", "error", err)
			return false
		}
		data, err := json.Marshal(state)
		if err != nil {
			s.log.Error("no se pudo serializar el estado", "error", err)
			return false
		}
		return send(append(append([]byte("event: snapshot\ndata: "), data...), "\n\n"...))
	}
	if !sendSnapshot() {
		return
	}
	heartbeat := time.NewTicker(sseHeartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-changes:
			if !sendSnapshot() {
				return
			}
		case <-heartbeat.C:
			if !send([]byte(": ping\n\n")) {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) deleteConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if s.storeError(w, s.store.DeleteConnection(r.Context(), id)) {
		return
	}
	s.collector.Remove(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) storeError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		s.internal(w, err)
	}
	return true
}

func (s *Server) serveWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name != "" {
		if info, err := fs.Stat(s.web, name); err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFileFS(w, r, s.web, name)
			return
		}
		// Un archivo inexistente no es una ruta de la aplicación.
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
	}
	index, err := fs.ReadFile(s.web, "index.html")
	if err != nil {
		http.Error(w, "La web no está compilada: ejecuta make build", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(index)
}

func (s *Server) internal(w http.ResponseWriter, err error) {
	s.log.Error("error interno", "error", err)
	writeError(w, http.StatusInternalServerError, "internal")
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "json_required")
		return false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_json")
		}
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type failureEntry struct {
	count int
	reset time.Time
}

type failureLimiter struct {
	mu      sync.Mutex
	entries map[string]*failureEntry
}

func (l *failureLimiter) blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok {
		return false
	}
	if time.Now().After(e.reset) {
		delete(l.entries, ip)
		return false
	}
	return e.count >= maxFailures
}

func (l *failureLimiter) record(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, e := range l.entries {
		if now.After(e.reset) {
			delete(l.entries, k)
		}
	}
	e, ok := l.entries[ip]
	if !ok {
		e = &failureEntry{reset: now.Add(failureWindow)}
		l.entries[ip] = e
	}
	e.count++
}

func (l *failureLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}
