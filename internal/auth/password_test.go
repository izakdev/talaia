package auth

import "testing"

func TestPassword(t *testing.T) {
	h, err := HashPassword("una-contraseña-larga")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyPassword(h, "una-contraseña-larga"); err != nil || !ok {
		t.Fatalf("correcta: %v %v", ok, err)
	}
	if ok, err := VerifyPassword(h, "otra"); err != nil || ok {
		t.Fatalf("incorrecta: %v %v", ok, err)
	}
	if _, err := VerifyPassword("basura", "x"); err == nil {
		t.Fatal("debe fallar con un hash no válido")
	}
}
