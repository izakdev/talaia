package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/secrets"
	"github.com/izakdev/talaia/internal/store"
)

type fake struct {
	fail  *atomic.Bool
	calls *atomic.Int64
}

func (f fake) Collect(context.Context) (any, error) {
	n := f.calls.Add(1)
	if f.fail.Load() {
		return nil, &integrations.Error{Code: integrations.CodeUnauthorized, Err: errors.New("no")}
	}
	return map[string]int64{"call": n}, nil
}

type env struct {
	st    *store.Store
	m     *Manager
	fail  *atomic.Bool
	cfgs  chan integrations.Config
	calls *atomic.Int64
}

func setup(t *testing.T) *env {
	t.Helper()
	box, _ := secrets.New(bytes.Repeat([]byte{3}, 32))
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{st: st, fail: &atomic.Bool{}, cfgs: make(chan integrations.Config, 16), calls: &atomic.Int64{}}
	registry := integrations.Registry{"adguard": func(c integrations.Config) integrations.Integration {
		e.cfgs <- c
		return fake{e.fail, e.calls}
	}}
	e.m = New(st, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	if err := e.m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); e.m.Wait(); st.Close() })
	return e
}

func (e *env) create(t *testing.T, typ string) store.Connection {
	t.Helper()
	c, err := e.st.CreateConnection(context.Background(), store.Connection{
		Name: "AG", Type: typ, BaseURL: "http://interno.example", Username: "usuario-secreto", IntervalSeconds: 3600,
	}, "clave-secreta")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.m.Refresh(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	return c
}

func (e *env) waitFor(t *testing.T, cond func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := e.m.Snapshots(); len(s) == 1 && cond(s[0]) {
			return s[0]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("estado no alcanzado: %+v", e.m.Snapshots())
	return Snapshot{}
}

func TestCollectsAndPassesDecryptedSecret(t *testing.T) {
	e := setup(t)
	e.create(t, "adguard")
	cfg := <-e.cfgs
	if cfg.Secret != "clave-secreta" || cfg.Username != "usuario-secreto" {
		t.Fatalf("config = %+v", cfg)
	}
	s := e.waitFor(t, func(s Snapshot) bool { return s.Status == StatusOK })
	raw, _ := json.Marshal(s)
	for _, leak := range []string{"clave-secreta", "usuario-secreto", "interno.example"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("el snapshot filtra %q: %s", leak, raw)
		}
	}
}

func TestErrorKeepsLastDataAndRecovers(t *testing.T) {
	e := setup(t)
	c := e.create(t, "adguard")
	first := e.waitFor(t, func(s Snapshot) bool { return s.Status == StatusOK })

	e.fail.Store(true)
	if err := e.m.Refresh(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	failed := e.waitFor(t, func(s Snapshot) bool { return s.Status == StatusError })
	if failed.Error != integrations.CodeUnauthorized || failed.Data == nil {
		t.Fatalf("snapshot = %+v", failed)
	}
	if !failed.LastSuccessAt.Equal(*first.LastSuccessAt) {
		t.Fatal("un fallo no debe cambiar la fecha del último dato")
	}

	e.fail.Store(false)
	if err := e.m.Refresh(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.waitFor(t, func(s Snapshot) bool { return s.Status == StatusOK }); got.Error != "" {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestChangingTargetDropsOldData(t *testing.T) {
	e := setup(t)
	c := e.create(t, "adguard")
	e.waitFor(t, func(s Snapshot) bool { return s.Status == StatusOK })
	e.fail.Store(true)
	c.BaseURL = "http://otro.example"
	if _, err := e.st.UpdateConnection(context.Background(), c, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.m.Refresh(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	if s := e.waitFor(t, func(s Snapshot) bool { return s.Status == StatusError }); s.Data != nil || s.LastSuccessAt != nil {
		t.Fatalf("conserva datos del destino anterior: %+v", s)
	}
}

func TestRemoveAndUnsupported(t *testing.T) {
	e := setup(t)
	c := e.create(t, "adguard")
	e.waitFor(t, func(Snapshot) bool { return true })
	e.m.Remove(c.ID)
	if s := e.m.Snapshots(); len(s) != 0 {
		t.Fatalf("snapshots = %+v", s)
	}
	e.create(t, "proxmox")
	if s := e.m.Snapshots(); len(s) != 0 {
		t.Fatalf("un tipo sin integración no debe aparecer: %+v", s)
	}
}

func TestSubscribersAreNotified(t *testing.T) {
	e := setup(t)
	ch, cancel, ok := e.m.Subscribe()
	if !ok {
		t.Fatal("suscripción rechazada")
	}
	defer cancel()
	e.create(t, "adguard")
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("sin aviso")
	}
}
