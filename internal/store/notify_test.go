package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func newChannel(t *testing.T, s *Store, secret string) Channel {
	t.Helper()
	c, err := s.CreateChannel(context.Background(), Channel{
		Name: "Móvil", Type: "telegram", Enabled: true, Config: map[string]string{"chat_id": "12345"},
	}, secret)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestChannelSecretEncrypted(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c := newChannel(t, s, `{"token":"123:token-secreto"}`)
	var raw []byte
	if err := s.db.QueryRow(`SELECT secret FROM notification_channels WHERE id = ?`, c.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "token-secreto") {
		t.Fatal("el secreto se guarda en claro")
	}
	got, err := s.ChannelSecret(ctx, c.ID)
	if err != nil || got != `{"token":"123:token-secreto"}` {
		t.Fatalf("ChannelSecret = %q, %v", got, err)
	}
	read, err := s.GetChannel(ctx, c.ID)
	if err != nil || read.Name != "Móvil" || !read.Enabled || read.Config["chat_id"] != "12345" || read.Type != "telegram" {
		t.Fatalf("GetChannel = %+v, %v", read, err)
	}
}

func TestUpdateChannelKeepsSecret(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c := newChannel(t, s, "uno")
	c.Name, c.Enabled = "Otro", false
	if _, err := s.UpdateChannel(ctx, c, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ChannelSecret(ctx, c.ID); got != "uno" {
		t.Fatalf("secreto = %q", got)
	}
	two := "dos"
	updated, err := s.UpdateChannel(ctx, c, &two)
	if err != nil || updated.Name != "Otro" || updated.Enabled {
		t.Fatalf("UpdateChannel = %+v, %v", updated, err)
	}
	if got, _ := s.ChannelSecret(ctx, c.ID); got != "dos" {
		t.Fatalf("secreto = %q", got)
	}
	if err := s.DeleteChannel(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetChannel(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tras borrar: %v", err)
	}
	if list, _ := s.ListChannels(ctx); len(list) != 0 {
		t.Fatalf("ListChannels = %+v", list)
	}
}

func TestNotifiedToneCascade(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c, err := s.CreateConnection(ctx, Connection{Name: "AG", Type: "adguard", BaseURL: "http://x", IntervalSeconds: 30}, "k")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetNotifiedTone(ctx, c.ID, "warn"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetNotifiedTone(ctx, c.ID, "bad"); err != nil {
		t.Fatal(err)
	}
	if tones, _ := s.NotifiedTones(ctx); tones[c.ID] != "bad" {
		t.Fatalf("tonos = %v", tones)
	}
	if err := s.DeleteConnection(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if tones, _ := s.NotifiedTones(ctx); len(tones) != 0 {
		t.Fatalf("tras borrar la conexión: %v", tones)
	}
}

func TestDeleteNotifiedTone(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c, _ := s.CreateConnection(ctx, Connection{Name: "AG", Type: "adguard", BaseURL: "http://x", IntervalSeconds: 30}, "k")
	_ = s.SetNotifiedTone(ctx, c.ID, "warn")
	if err := s.DeleteNotifiedTone(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if tones, _ := s.NotifiedTones(ctx); len(tones) != 0 {
		t.Fatalf("tonos = %v", tones)
	}
}

func TestLogKeepsLatest50(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	for i := range 55 {
		e := LogEntry{Channel: fmt.Sprintf("c%d", i), ChannelType: "ntfy", Event: "change", Result: "ok", Attempts: 1,
			Services: []LogService{{Name: "AdGuard", Tone: "warn", Previous: "good"}}}
		if err := s.AddLog(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	log, err := s.ListLog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 50 || log[0].Channel != "c54" || log[49].Channel != "c5" {
		t.Fatalf("registro: %d entradas, primera %q", len(log), log[0].Channel)
	}
	if log[0].Services[0].Name != "AdGuard" || log[0].At.IsZero() {
		t.Fatalf("entrada = %+v", log[0])
	}
}

func TestSettingsNotifyDefaults(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	got, err := s.GetSettings(ctx)
	if err != nil || got.NotifyLanguage != "" || got.NotifyDelaySeconds != 120 {
		t.Fatalf("ajustes = %+v, %v", got, err)
	}
	if err := s.SetNotifyLanguageIfUnset(ctx, "en"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetNotifyLanguageIfUnset(ctx, "es"); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.GetSettings(ctx)
	saved.NotifyDelaySeconds = 0
	if err := s.SaveSettings(ctx, saved); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetSettings(ctx); got.NotifyLanguage != "en" || got.NotifyDelaySeconds != 0 {
		t.Fatalf("ajustes = %+v", got)
	}
}

func TestConnectionNotifyDefaultTrue(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c, err := s.CreateConnection(ctx, Connection{Name: "AG", Type: "adguard", BaseURL: "http://x", IntervalSeconds: 30, Notify: true}, "k")
	if err != nil || !c.Notify {
		t.Fatalf("CreateConnection = %+v, %v", c, err)
	}
	c.Notify = false
	if c, err = s.UpdateConnection(ctx, c, nil); err != nil || c.Notify {
		t.Fatalf("UpdateConnection = %+v, %v", c, err)
	}
	// Una conexión anterior a la migración avisa.
	if _, err := s.db.Exec(`INSERT INTO connections(id, name, type, base_url, secret, interval_seconds, created_at, updated_at) VALUES ('vieja', 'V', 'adguard', 'http://x', x'00', 30, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if old, err := s.GetConnection(ctx, "vieja"); err != nil || !old.Notify {
		t.Fatalf("vieja = %+v, %v", old, err)
	}
}
