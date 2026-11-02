// Package store persiste los datos de Talaia en SQLite.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/izakdev/talaia/internal/secrets"
	_ "modernc.org/sqlite" // driver
)

var (
	// ErrNotFound indica que el registro no existe.
	ErrNotFound = errors.New("no encontrado")
	// ErrAlreadySetup indica que ya existe un administrador.
	ErrAlreadySetup = errors.New("ya configurado")
)

var migrations = []string{`
CREATE TABLE users (
	id            INTEGER PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	created_at    INTEGER NOT NULL
);
CREATE TABLE sessions (
	token_hash TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE INDEX sessions_expires_at ON sessions(expires_at);
CREATE TABLE connections (
	id               TEXT PRIMARY KEY,
	name             TEXT NOT NULL,
	type             TEXT NOT NULL,
	base_url         TEXT NOT NULL,
	username         TEXT NOT NULL DEFAULT '',
	secret           BLOB NOT NULL,
	interval_seconds INTEGER NOT NULL,
	tls_skip_verify  INTEGER NOT NULL DEFAULT 0,
	created_at       INTEGER NOT NULL,
	updated_at       INTEGER NOT NULL
);`}

// Store accede a la base de datos.
type Store struct {
	db  *sql.DB
	box *secrets.Box
}

// Open abre la base de datos y aplica las migraciones pendientes.
func Open(path string, box *secrets.Box) (*Store, error) {
	// SQLite hereda estos permisos en los archivos WAL y SHM.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, box: box}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrar base de datos: %w", err)
	}
	return s, nil
}

// Close cierra la base de datos.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("la base de datos es de una versión más nueva (%d)", version)
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// User es un administrador.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
}

// SetupRequired indica si todavía no existe ningún administrador.
func (s *Store) SetupRequired(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n == 0, err
}

// CreateFirstUser crea el primer administrador o devuelve ErrAlreadySetup.
func (s *Store) CreateFirstUser(ctx context.Context, username, hash string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users(username, password_hash, created_at)
		 SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM users)`,
		username, hash, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, ErrAlreadySetup
	}
	return res.LastInsertId()
}

// UserByUsername busca un administrador por nombre.
func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// CreateSession guarda una sesión.
func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, userID, expires.Unix())
	return err
}

// UserBySession devuelve el administrador de una sesión vigente.
func (s *Store) UserBySession(ctx context.Context, tokenHash string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT u.id, u.username, u.password_hash FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, time.Now().Unix()).
		Scan(&u.ID, &u.Username, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// DeleteSession elimina una sesión.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteExpiredSessions elimina las sesiones caducadas.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, time.Now().Unix())
	return err
}

// Connection es la configuración de un servicio. Nunca incluye el secreto.
type Connection struct {
	ID              string
	Name            string
	Type            string
	BaseURL         string
	Username        string
	IntervalSeconds int
	TLSSkipVerify   bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const connectionColumns = `id, name, type, base_url, username, interval_seconds, tls_skip_verify, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scanConnection(row scanner) (Connection, error) {
	var c Connection
	var created, updated int64
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.BaseURL, &c.Username, &c.IntervalSeconds, &c.TLSSkipVerify, &created, &updated)
	c.CreatedAt, c.UpdatedAt = time.Unix(created, 0).UTC(), time.Unix(updated, 0).UTC()
	return c, err
}

// ListConnections devuelve todas las conexiones por nombre.
func (s *Store) ListConnections(ctx context.Context) ([]Connection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+connectionColumns+` FROM connections ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetConnection devuelve una conexión.
func (s *Store) GetConnection(ctx context.Context, id string) (Connection, error) {
	c, err := scanConnection(s.db.QueryRowContext(ctx, `SELECT `+connectionColumns+` FROM connections WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// CreateConnection crea una conexión con su secreto cifrado.
func (s *Store) CreateConnection(ctx context.Context, c Connection, secret string) (Connection, error) {
	idb := make([]byte, 16)
	if _, err := rand.Read(idb); err != nil {
		return c, err
	}
	c.ID = hex.EncodeToString(idb)
	sealed, err := s.box.Seal([]byte(secret), []byte(c.ID))
	if err != nil {
		return c, err
	}
	now := time.Now().Unix()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO connections(id, name, type, base_url, username, secret, interval_seconds, tls_skip_verify, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Type, c.BaseURL, c.Username, sealed, c.IntervalSeconds, c.TLSSkipVerify, now, now); err != nil {
		return c, err
	}
	return s.GetConnection(ctx, c.ID)
}

// UpdateConnection actualiza una conexión. Si secret es nil, conserva el secreto actual.
func (s *Store) UpdateConnection(ctx context.Context, c Connection, secret *string) (Connection, error) {
	query := `UPDATE connections SET name = ?, base_url = ?, username = ?, interval_seconds = ?, tls_skip_verify = ?, updated_at = ?`
	args := []any{c.Name, c.BaseURL, c.Username, c.IntervalSeconds, c.TLSSkipVerify, time.Now().Unix()}
	if secret != nil {
		sealed, err := s.box.Seal([]byte(*secret), []byte(c.ID))
		if err != nil {
			return c, err
		}
		query += `, secret = ?`
		args = append(args, sealed)
	}
	res, err := s.db.ExecContext(ctx, query+` WHERE id = ?`, append(args, c.ID)...)
	if err != nil {
		return c, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return c, err
	} else if n == 0 {
		return c, ErrNotFound
	}
	return s.GetConnection(ctx, c.ID)
}

// DeleteConnection elimina una conexión.
func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM connections WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ConnectionSecret descifra el secreto de una conexión.
func (s *Store) ConnectionSecret(ctx context.Context, id string) (string, error) {
	var sealed []byte
	err := s.db.QueryRowContext(ctx, `SELECT secret FROM connections WHERE id = ?`, id).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	plain, err := s.box.Open(sealed, []byte(id))
	if err != nil {
		return "", fmt.Errorf("descifrar secreto: %w", err)
	}
	return string(plain), nil
}
