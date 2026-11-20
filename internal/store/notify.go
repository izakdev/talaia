package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// maxLog es el número de envíos que se conservan en el registro.
const maxLog = 50

// Channel es un canal de avisos. Nunca incluye el secreto.
type Channel struct {
	ID            string
	Name          string
	Type          string // telegram, ntfy o webhook
	Enabled       bool
	TLSSkipVerify bool
	// Config guarda lo que no es secreto: chat_id de Telegram o servidor de ntfy.
	Config    map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const channelColumns = `id, name, type, enabled, tls_skip_verify, config, created_at, updated_at`

func scanChannel(row scanner) (Channel, error) {
	var c Channel
	var config string
	var created, updated int64
	if err := row.Scan(&c.ID, &c.Name, &c.Type, &c.Enabled, &c.TLSSkipVerify, &config, &created, &updated); err != nil {
		return c, err
	}
	c.CreatedAt, c.UpdatedAt = time.Unix(created, 0).UTC(), time.Unix(updated, 0).UTC()
	if err := json.Unmarshal([]byte(config), &c.Config); err != nil {
		return c, fmt.Errorf("leer la configuración del canal %s: %w", c.ID, err)
	}
	return c, nil
}

func encodeConfig(c Channel) (string, error) {
	if c.Config == nil {
		c.Config = map[string]string{}
	}
	raw, err := json.Marshal(c.Config)
	return string(raw), err
}

// ListChannels devuelve los canales por nombre.
func (s *Store) ListChannels(ctx context.Context) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+channelColumns+` FROM notification_channels ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Channel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetChannel devuelve un canal.
func (s *Store) GetChannel(ctx context.Context, id string) (Channel, error) {
	c, err := scanChannel(s.db.QueryRowContext(ctx, `SELECT `+channelColumns+` FROM notification_channels WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// CreateChannel crea un canal con su secreto cifrado.
func (s *Store) CreateChannel(ctx context.Context, c Channel, secret string) (Channel, error) {
	idb := make([]byte, 16)
	if _, err := rand.Read(idb); err != nil {
		return c, err
	}
	c.ID = hex.EncodeToString(idb)
	sealed, err := s.box.Seal([]byte(secret), []byte(c.ID))
	if err != nil {
		return c, err
	}
	config, err := encodeConfig(c)
	if err != nil {
		return c, err
	}
	now := time.Now().Unix()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO notification_channels(id, name, type, enabled, config, secret, tls_skip_verify, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Type, c.Enabled, config, sealed, c.TLSSkipVerify, now, now); err != nil {
		return c, err
	}
	return s.GetChannel(ctx, c.ID)
}

// UpdateChannel actualiza un canal. Si secret es nil, conserva el secreto actual.
// El tipo no cambia.
func (s *Store) UpdateChannel(ctx context.Context, c Channel, secret *string) (Channel, error) {
	config, err := encodeConfig(c)
	if err != nil {
		return c, err
	}
	query := `UPDATE notification_channels SET name = ?, enabled = ?, config = ?, tls_skip_verify = ?, updated_at = ?`
	args := []any{c.Name, c.Enabled, config, c.TLSSkipVerify, time.Now().Unix()}
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
	return s.GetChannel(ctx, c.ID)
}

// DeleteChannel elimina un canal.
func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM notification_channels WHERE id = ?`, id)
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

// ChannelSecret descifra el secreto de un canal.
func (s *Store) ChannelSecret(ctx context.Context, id string) (string, error) {
	var sealed []byte
	err := s.db.QueryRowContext(ctx, `SELECT secret FROM notification_channels WHERE id = ?`, id).Scan(&sealed)
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

// SetNotifyLanguageIfUnset elige el idioma de los avisos si aún no se ha elegido.
func (s *Store) SetNotifyLanguageIfUnset(ctx context.Context, lang string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings(key, value) VALUES ('notify_language', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value WHERE settings.value = ''`, lang)
	return err
}

// NotifiedTones devuelve el último color avisado de cada conexión.
func (s *Store) NotifiedTones(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT connection_id, tone FROM notify_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, tone string
		if err := rows.Scan(&id, &tone); err != nil {
			return nil, err
		}
		out[id] = tone
	}
	return out, rows.Err()
}

// SetNotifiedTone guarda el último color avisado de una conexión.
func (s *Store) SetNotifiedTone(ctx context.Context, connectionID, tone string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notify_state(connection_id, tone) VALUES (?, ?)
		 ON CONFLICT(connection_id) DO UPDATE SET tone = excluded.tone`, connectionID, tone)
	return err
}

// DeleteNotifiedTone olvida el último color avisado de una conexión.
func (s *Store) DeleteNotifiedTone(ctx context.Context, connectionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notify_state WHERE connection_id = ?`, connectionID)
	return err
}

// LogService es un servicio de un envío, con su color nuevo y el anterior.
type LogService struct {
	Name     string `json:"name"`
	Tone     string `json:"tone"`
	Previous string `json:"previous"`
}

// LogEntry es un envío del registro. No guarda direcciones, secretos ni respuestas.
type LogEntry struct {
	ID          int64        `json:"id"`
	At          time.Time    `json:"at"`
	Channel     string       `json:"channel"`
	ChannelType string       `json:"channel_type"`
	Event       string       `json:"event"` // change o test
	Services    []LogService `json:"services"`
	Result      string       `json:"result"` // ok o error
	Code        string       `json:"code,omitempty"`
	Attempts    int          `json:"attempts"`
}

// AddLog añade un envío al registro y deja solo los más recientes.
func (s *Store) AddLog(ctx context.Context, e LogEntry) error {
	if e.Services == nil {
		e.Services = []LogService{}
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	services, err := json.Marshal(e.Services)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notification_log(at, channel, channel_type, event, services, result, code, attempts) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.At.UnixMilli(), e.Channel, e.ChannelType, e.Event, string(services), e.Result, e.Code, e.Attempts); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM notification_log WHERE id NOT IN (SELECT id FROM notification_log ORDER BY id DESC LIMIT ?)`, maxLog); err != nil {
		return err
	}
	return tx.Commit()
}

// ListLog devuelve el registro, del envío más reciente al más antiguo.
func (s *Store) ListLog(ctx context.Context) ([]LogEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, at, channel, channel_type, event, services, result, code, attempts FROM notification_log ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		var at int64
		var services string
		if err := rows.Scan(&e.ID, &at, &e.Channel, &e.ChannelType, &e.Event, &services, &e.Result, &e.Code, &e.Attempts); err != nil {
			return nil, err
		}
		e.At = time.UnixMilli(at).UTC()
		if err := json.Unmarshal([]byte(services), &e.Services); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
