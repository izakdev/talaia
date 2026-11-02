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

func TestMigratesExistingDatabase(t *testing.T) {
	s, path := newStore(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`ALTER TABLE connections DROP COLUMN options; PRAGMA user_version = 1;`); err != nil {
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
	if err != nil || c.Name != "antigua" || c.Options["show_top_domains"] {
		t.Fatalf("c = %+v, err = %v", c, err)
	}
}
