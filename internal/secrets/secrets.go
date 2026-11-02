// Package secrets cifra credenciales con AES-256-GCM usando una clave externa a la base de datos.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

const keySize = 32

// Box cifra y descifra valores.
type Box struct{ aead cipher.AEAD }

// LoadOrCreateKey lee la clave de path o la genera si no existe.
func LoadOrCreateKey(path string) ([]byte, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if info.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("la clave %s es accesible por otros usuarios (permisos %o): usa chmod 600", path, info.Mode().Perm())
		}
		return readKey(path)
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("comprobar clave: %w", err)
	}

	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generar clave: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("crear clave: %w", err)
	}
	_, werr := f.WriteString(hex.EncodeToString(key) + "\n")
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("escribir clave: %w", err)
	}
	return key, nil
}

func readKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leer clave: %w", err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != keySize {
		return nil, fmt.Errorf("la clave %s no es válida", path)
	}
	return key, nil
}

// New crea un Box a partir de una clave de 32 bytes.
func New(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("la clave debe tener %d bytes", keySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal cifra plain y vincula el resultado a aad.
func (b *Box) Seal(plain, aad []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, plain, aad), nil
}

// Open descifra un valor generado por Seal con el mismo aad.
func (b *Box) Open(sealed, aad []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("valor cifrado demasiado corto")
	}
	return b.aead.Open(nil, sealed[:n], sealed[n:], aad)
}
