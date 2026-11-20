package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/izakdev/talaia/internal/store"
)

var (
	channelTypes   = []string{"telegram", "ntfy", "webhook"}
	notifyDelays   = []int{0, 60, 120, 300, 600, 900}
	notifyLangs    = []string{"", "es", "en"}
	telegramToken  = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]+$`)
	telegramChatID = regexp.MustCompile(`^(-?\d+|@[A-Za-z0-9_]{5,})$`)
	ntfyTopic      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// channelIn es un canal tal como llega de la administración. Los campos secretos
// vacíos o ausentes conservan el valor guardado.
type channelIn struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Enabled       bool    `json:"enabled"`
	TLSSkipVerify bool    `json:"tls_skip_verify"`
	ChatID        string  `json:"chat_id"`
	Server        string  `json:"server"`
	Token         *string `json:"token"`
	Topic         *string `json:"topic"`
	URL           *string `json:"url"`
	Language      string  `json:"language"`
}

// channelOut nunca lleva el token, el tópico ni la URL del webhook.
type channelOut struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Enabled       bool      `json:"enabled"`
	TLSSkipVerify bool      `json:"tls_skip_verify"`
	ChatID        string    `json:"chat_id,omitempty"`
	Server        string    `json:"server,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toChannelOut(c store.Channel) channelOut {
	return channelOut{c.ID, c.Name, c.Type, c.Enabled, c.TLSSkipVerify, c.Config["chat_id"], c.Config["server"], c.UpdatedAt}
}

type channelSecret struct {
	Token string `json:"token,omitempty"`
	Topic string `json:"topic,omitempty"`
	URL   string `json:"url,omitempty"`
}

func trimmed(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func httpURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && !strings.ContainsAny(raw, " \r\n")
}

// buildChannel valida un canal y le añade los secretos guardados que no cambian.
// Devuelve el canal, su secreto en JSON y si el secreto ha cambiado.
func buildChannel(in channelIn, stored *channelSecret) (store.Channel, string, bool, bool) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 60 || !slices.Contains(channelTypes, in.Type) {
		return store.Channel{}, "", false, false
	}
	secret := channelSecret{}
	if stored != nil {
		secret = *stored
	}
	changed := false
	set := func(dst *string, p *string) {
		if v := trimmed(p); v != "" {
			*dst, changed = v, true
		}
	}
	c := store.Channel{ID: in.ID, Name: in.Name, Type: in.Type, Enabled: in.Enabled, Config: map[string]string{}}
	switch in.Type {
	case "telegram":
		set(&secret.Token, in.Token)
		c.Config["chat_id"] = strings.TrimSpace(in.ChatID)
		if !telegramToken.MatchString(secret.Token) || !telegramChatID.MatchString(c.Config["chat_id"]) {
			return c, "", false, false
		}
	case "ntfy":
		set(&secret.Topic, in.Topic)
		// El token es opcional: solo cambia si se escribe uno.
		set(&secret.Token, in.Token)
		server := strings.TrimRight(strings.TrimSpace(in.Server), "/")
		if server != "" && !httpURL(server) || !ntfyTopic.MatchString(secret.Topic) || strings.ContainsAny(secret.Token, " \r\n") {
			return c, "", false, false
		}
		if server != "" {
			c.Config["server"] = server
		}
		c.TLSSkipVerify = in.TLSSkipVerify
	case "webhook":
		set(&secret.URL, in.URL)
		if !httpURL(secret.URL) {
			return c, "", false, false
		}
		c.TLSSkipVerify = in.TLSSkipVerify
	}
	raw, _ := json.Marshal(secret)
	return c, string(raw), changed, true
}

// ntfyServer normaliza el servidor de ntfy: vacío es ntfy.sh.
func ntfyServer(server string) string {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	if server == "" {
		return "https://ntfy.sh"
	}
	return server
}

// secretsMissingForMove indica si un canal de ntfy cambia de servidor sin volver a
// escribir sus secretos: los guardados no se envían a otro servidor.
func secretsMissingForMove(current store.Channel, in channelIn, stored *channelSecret) bool {
	if current.Type != "ntfy" || ntfyServer(in.Server) == ntfyServer(current.Config["server"]) {
		return false
	}
	tokenStored := stored != nil && stored.Token != ""
	return trimmed(in.Topic) == "" || (tokenStored && trimmed(in.Token) == "")
}

func (s *Server) storedSecret(ctx context.Context, id string) (*channelSecret, error) {
	plain, err := s.store.ChannelSecret(ctx, id)
	if err != nil {
		return nil, err
	}
	var secret channelSecret
	if err := json.Unmarshal([]byte(plain), &secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func (s *Server) listChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.store.ListChannels(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	out := make([]channelOut, len(channels))
	for i, c := range channels {
		out[i] = toChannelOut(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createChannel(w http.ResponseWriter, r *http.Request) {
	var in channelIn
	if !decode(w, r, &in) {
		return
	}
	c, secret, _, ok := buildChannel(in, nil)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_channel")
		return
	}
	created, err := s.store.CreateChannel(r.Context(), c, secret)
	if err != nil {
		s.internal(w, err)
		return
	}
	// El primer canal fija el idioma de los avisos con el de la administración.
	if in.Language == "es" || in.Language == "en" {
		if err := s.store.SetNotifyLanguageIfUnset(r.Context(), in.Language); err != nil {
			s.internal(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, toChannelOut(created))
}

func (s *Server) updateChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	current, err := s.store.GetChannel(r.Context(), id)
	if s.storeError(w, err) {
		return
	}
	var in channelIn
	if !decode(w, r, &in) {
		return
	}
	// El tipo no cambia: el secreto guardado pertenece a ese canal.
	in.ID, in.Type = id, current.Type
	stored, err := s.storedSecret(r.Context(), id)
	if err != nil {
		// Con el secreto ilegible solo se puede guardar si se escribe entero.
		stored = nil
	}
	if secretsMissingForMove(current, in, stored) {
		writeError(w, http.StatusBadRequest, "secret_required_for_url_change")
		return
	}
	c, secret, changed, ok := buildChannel(in, stored)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_channel")
		return
	}
	var newSecret *string
	if changed {
		newSecret = &secret
	}
	updated, err := s.store.UpdateChannel(r.Context(), c, newSecret)
	if s.storeError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toChannelOut(updated))
}

func (s *Server) deleteChannel(w http.ResponseWriter, r *http.Request) {
	if s.storeError(w, s.store.DeleteChannel(r.Context(), chi.URLParam(r, "id"))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sendTest(w http.ResponseWriter, r *http.Request, c store.Channel, secret string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, s.notifier.SendTest(ctx, c, secret))
}

// testSavedChannel envía una prueba por un canal guardado.
func (s *Server) testSavedChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := s.store.GetChannel(r.Context(), id)
	if s.storeError(w, err) {
		return
	}
	secret, err := s.store.ChannelSecret(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "secret_unreadable")
		return
	}
	s.sendTest(w, r, c, secret)
}

// testChannel envía una prueba con los datos del formulario, sin guardar nada.
func (s *Server) testChannel(w http.ResponseWriter, r *http.Request) {
	var in channelIn
	if !decode(w, r, &in) {
		return
	}
	var stored *channelSecret
	if in.ID != "" {
		current, err := s.store.GetChannel(r.Context(), in.ID)
		if s.storeError(w, err) {
			return
		}
		in.Type = current.Type
		if stored, err = s.storedSecret(r.Context(), in.ID); err != nil {
			stored = nil
		}
		if secretsMissingForMove(current, in, stored) {
			writeError(w, http.StatusBadRequest, "secret_required_for_url_change")
			return
		}
	}
	c, secret, _, ok := buildChannel(in, stored)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_channel")
		return
	}
	// El secreto ya está en claro: la prueba no lo vuelve a leer de la base de datos.
	c.ID = ""
	s.sendTest(w, r, c, secret)
}

func (s *Server) notificationLog(w http.ResponseWriter, r *http.Request) {
	log, err := s.store.ListLog(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}
