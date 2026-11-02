package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSealOpen(t *testing.T) {
	key, err := LoadOrCreateKey(filepath.Join(t.TempDir(), "k"))
	if err != nil {
		t.Fatal(err)
	}
	box, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal([]byte("token"), []byte("conn-1"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := box.Open(sealed, []byte("conn-1"))
	if err != nil || string(got) != "token" {
		t.Fatalf("Open = %q, %v", got, err)
	}
	if _, err := box.Open(sealed, []byte("conn-2")); err == nil {
		t.Fatal("un secreto no debe descifrarse con otra conexión")
	}
}

func TestKeyIsStableAndProtected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k")
	a, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreateKey(path)
	if err != nil || string(a) != string(b) {
		t.Fatalf("la clave cambió: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateKey(path); err == nil {
		t.Fatal("debe rechazar una clave con permisos abiertos")
	}
}
