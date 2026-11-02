package store

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/secrets"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	box, err := secrets.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "t.db")
	s, err := Open(path, box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestFirstUserOnlyOnce(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	if _, err := s.CreateFirstUser(ctx, "admin", "h"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFirstUser(ctx, "otro", "h"); !errors.Is(err, ErrAlreadySetup) {
		t.Fatalf("err = %v", err)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	id, err := s.CreateFirstUser(ctx, "admin", "h")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, "old", id, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestConnectionSecretIsEncryptedAndKept(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c, err := s.CreateConnection(ctx, Connection{Name: "a", Type: "adguard", BaseURL: "http://x", IntervalSeconds: 30}, "muy-secreto")
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := s.db.QueryRow(`SELECT secret FROM connections WHERE id = ?`, c.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("muy-secreto")) {
		t.Fatal("el secreto está en claro")
	}
	c.Name = "b"
	if _, err := s.UpdateConnection(ctx, c, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ConnectionSecret(ctx, c.ID); err != nil || got != "muy-secreto" {
		t.Fatalf("secreto = %q, %v", got, err)
	}
	nuevo := "nuevo"
	if _, err := s.UpdateConnection(ctx, c, &nuevo); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ConnectionSecret(ctx, c.ID); got != "nuevo" {
		t.Fatalf("secreto = %q", got)
	}
}

func TestReopenKeepsData(t *testing.T) {
	s, path := newStore(t)
	if _, err := s.CreateFirstUser(context.Background(), "admin", "h"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	box, _ := secrets.New(bytes.Repeat([]byte{7}, 32))
	s2, err := Open(path, box)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if need, err := s2.SetupRequired(context.Background()); err != nil || need {
		t.Fatalf("need = %v, %v", need, err)
	}
}

func TestOptionsArePersisted(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c, err := s.CreateConnection(ctx, Connection{Name: "a", Type: "adguard", BaseURL: "http://x", IntervalSeconds: 30}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Options) != 0 {
		t.Fatalf("opciones por defecto = %v", c.Options)
	}
	c.Options = map[string]bool{"show_top_domains": true}
	c, err = s.UpdateConnection(ctx, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Options["show_top_domains"] {
		t.Fatalf("opciones = %v", c.Options)
	}
}

func TestEntitiesArePersisted(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c, err := s.CreateConnection(ctx, Connection{Name: "a", Type: "homeassistant", BaseURL: "http://x", IntervalSeconds: 30, Entities: []string{"light.salon"}}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entities) != 1 || c.Entities[0] != "light.salon" {
		t.Fatalf("entidades = %v", c.Entities)
	}
	c.Entities = nil
	if c, err = s.UpdateConnection(ctx, c, nil); err != nil || len(c.Entities) != 0 {
		t.Fatalf("entidades = %v, err = %v", c.Entities, err)
	}
}

func TestLinkFallsBackToConnectionURL(t *testing.T) {
	s, _ := newStore(t)
	c, err := s.CreateConnection(context.Background(), Connection{Name: "a", Type: "adguard", BaseURL: "http://interna", IntervalSeconds: 30}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if c.Link() != "http://interna" {
		t.Fatalf("enlace = %s", c.Link())
	}
	c.LinkURL = "https://servicio.example/panel"
	if c, err = s.UpdateConnection(context.Background(), c, nil); err != nil || c.Link() != "https://servicio.example/panel" {
		t.Fatalf("enlace = %s, err = %v", c.Link(), err)
	}
}

func TestMigratesExistingDatabase(t *testing.T) {
	s, path := newStore(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`DROP TABLE widgets; DROP TABLE pages; DROP TABLE settings; ALTER TABLE connections DROP COLUMN link_url; ALTER TABLE connections DROP COLUMN entities; ALTER TABLE connections DROP COLUMN options; PRAGMA user_version = 1;`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO connections(id, name, type, base_url, secret, interval_seconds, created_at, updated_at)
		VALUES ('v1', 'antigua', 'adguard', 'http://x', x'00', 30, 1, 1)`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	box, _ := secrets.New(bytes.Repeat([]byte{7}, 32))
	s2, err := Open(path, box)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	c, err := s2.GetConnection(ctx, "v1")
	if err != nil || c.Name != "antigua" || c.Options["show_top_domains"] || len(c.Entities) != 0 {
		t.Fatalf("c = %+v, err = %v", c, err)
	}
	pages, err := s2.Panel(ctx)
	if err != nil || len(pages) != 1 || len(pages[0].Widgets) != 1 || pages[0].Widgets[0].ConnectionID != "v1" {
		t.Fatalf("las conexiones existentes deben quedar en el panel: %+v, err = %v", pages, err)
	}
}

func TestNewConnectionsFillTwoColumns(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	for _, name := range []string{"a", "b", "c"} {
		if _, err := s.CreateConnection(ctx, Connection{Name: name, Type: "adguard", BaseURL: "http://x", IntervalSeconds: 30}, "s"); err != nil {
			t.Fatal(err)
		}
	}
	pages, err := s.Panel(ctx)
	if err != nil || len(pages) != 1 || len(pages[0].Widgets) != 3 {
		t.Fatalf("panel = %+v, err = %v", pages, err)
	}
	w := pages[0].Widgets
	if w[0].X != 0 || w[0].Y != 0 || w[1].X != DefaultWidgetWidth || w[1].Y != 0 || w[2].X != 0 || w[2].Y != DefaultWidgetHeight {
		t.Fatalf("widgets = %+v", w)
	}
}

func TestPanelSaveAndCascade(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	c, err := s.CreateConnection(ctx, Connection{Name: "a", Type: "adguard", BaseURL: "http://x", IntervalSeconds: 30}, "s")
	if err != nil {
		t.Fatal(err)
	}
	saved := []Page{
		{ID: "p2", Name: "Red", Widgets: []Widget{{ID: "w1", ConnectionID: c.ID, X: 3, Y: 1, W: 4, H: 5}}},
		{ID: "p1", Name: "Vacía", Widgets: []Widget{}},
	}
	if err := s.SavePanel(ctx, saved); err != nil {
		t.Fatal(err)
	}
	pages, err := s.Panel(ctx)
	if err != nil || len(pages) != 2 || pages[0].ID != "p2" || pages[0].Widgets[0].X != 3 {
		t.Fatalf("panel = %+v, err = %v", pages, err)
	}
	if err := s.DeleteConnection(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if pages, _ = s.Panel(ctx); len(pages[0].Widgets) != 0 {
		t.Fatalf("eliminar la conexión debe quitar su widget: %+v", pages)
	}
}

func TestSettingsDefaultsAndSave(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	got, err := s.GetSettings(ctx)
	if err != nil || got != (Settings{Theme: "auto", Accent: "amber"}) {
		t.Fatalf("por defecto = %+v, err = %v", got, err)
	}
	want := Settings{Theme: "dark", Accent: "teal", RequireLogin: true}
	if err := s.SaveSettings(ctx, want); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.GetSettings(ctx); got != want {
		t.Fatalf("guardado = %+v", got)
	}
}

func TestPasswordChangeAndReset(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	if _, err := s.ResetPassword(ctx, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sin usuarios: %v", err)
	}
	id, err := s.CreateFirstUser(ctx, "admin", "h1")
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	for _, token := range []string{"actual", "otra"} {
		if err := s.CreateSession(ctx, token, id, expires); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ChangePassword(ctx, id, "h2", "actual"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, "otra"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("debe revocar las demás sesiones: %v", err)
	}
	if u, err := s.UserBySession(ctx, "actual"); err != nil || u.PasswordHash != "h2" {
		t.Fatalf("u = %+v, err = %v", u, err)
	}
	name, err := s.ResetPassword(ctx, "h3")
	if err != nil || name != "admin" {
		t.Fatalf("name = %s, err = %v", name, err)
	}
	if _, err := s.UserBySession(ctx, "actual"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("debe revocar todas las sesiones: %v", err)
	}
}
