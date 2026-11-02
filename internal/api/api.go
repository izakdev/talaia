// Package api expone la API HTTP y sirve la web embebida.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/izakdev/talaia/internal/auth"
	"github.com/izakdev/talaia/internal/collector"
	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/integrations/adguard"
	"github.com/izakdev/talaia/internal/integrations/proxmox"
	"github.com/izakdev/talaia/internal/store"
)

const (
	cookieName      = "talaia_session"
	sessionLifetime = 30 * 24 * time.Hour
	maxBody         = 64 << 10
	maxFailures     = 10
	failureWindow   = 15 * time.Minute
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
			r.Get("/connections", s.listConnections)
			r.Post("/connections", s.createConnection)
			r.Post("/connections/test", s.testConnection)
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
	resp := map[string]any{"setup_required": needSetup, "version": s.version, "user": nil}
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
}{
	"proxmox":       {options: []string{proxmox.OptionShowGuests}},
	"homeassistant": {},
	"adguard":       {needsUsername: true, options: []string{adguard.OptionShowTopDomains}},
}

// proxmoxToken valida el formato usuario@reino!nombre=valor.
var proxmoxToken = regexp.MustCompile(`^[^\s!=]+@[^\s!=]+![^\s!=]+=[^\s]+$`)

type connectionIn struct {
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	BaseURL         string          `json:"base_url"`
	Username        string          `json:"username"`
	Secret          *string         `json:"secret"`
	IntervalSeconds int             `json:"interval_seconds"`
	TLSSkipVerify   bool            `json:"tls_skip_verify"`
	Options         map[string]bool `json:"options"`
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
	UpdatedAt       time.Time       `json:"updated_at"`
}

func toOut(c store.Connection) connectionOut {
	return connectionOut{c.ID, c.Name, c.Type, c.BaseURL, c.Username, c.IntervalSeconds, c.TLSSkipVerify, c.Options, c.UpdatedAt}
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
	if in.IntervalSeconds < 5 || in.IntervalSeconds > 3600 {
		return "invalid_interval"
	}
	if in.Secret != nil {
		if *in.Secret == "" || len(*in.Secret) > 4096 {
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
		IntervalSeconds: in.IntervalSeconds, TLSSkipVerify: in.TLSSkipVerify, Options: in.Options,
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
		IntervalSeconds: in.IntervalSeconds, TLSSkipVerify: in.TLSSkipVerify, Options: in.Options,
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

// testConnection consulta el servicio una vez, sin guardar nada.
func (s *Server) testConnection(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID string `json:"id"`
		connectionIn
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Secret != nil && *in.Secret == "" {
		in.Secret = nil
	}
	var current store.Connection
	if in.ID != "" {
		var err error
		current, err = s.store.GetConnection(r.Context(), in.ID)
		if s.storeError(w, err) {
			return
		}
		in.Type = current.Type
	}
	if code := validateConnection(&in.connectionIn); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	var secret string
	switch {
	case in.Secret != nil:
		secret = *in.Secret
	case in.ID == "":
		writeError(w, http.StatusBadRequest, "invalid_secret")
		return
	case in.BaseURL != current.BaseURL:
		writeError(w, http.StatusBadRequest, "secret_required_for_url_change")
		return
	default:
		var err error
		secret, err = s.store.ConnectionSecret(r.Context(), in.ID)
		if s.storeError(w, err) {
			return
		}
	}
	factory, supported := s.registry[in.Type]
	if !supported {
		writeError(w, http.StatusBadRequest, "not_supported")
		return
	}
	integration := factory(integrations.Config{
		BaseURL: in.BaseURL, Username: in.Username, Secret: secret, TLSSkipVerify: in.TLSSkipVerify, Options: in.Options,
	})
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if _, err := integration.Collect(ctx); err != nil {
		s.log.Info("prueba de conexión fallida", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": integrations.Code(err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) dashboard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"connections": s.collector.Snapshots()})
}

// dashboardEvents envía el estado completo al conectar y en cada cambio.
func (s *Server) dashboardEvents(w http.ResponseWriter, r *http.Request) {
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
		data, err := json.Marshal(map[string]any{"connections": s.collector.Snapshots()})
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
