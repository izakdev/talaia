package store

import (
	"context"
	"database/sql"
	"errors"
)

// Widget es el bloque de una conexión colocado en una página.
type Widget struct {
	ID           string `json:"id"`
	ConnectionID string `json:"connection_id"`
	X            int    `json:"x"`
	Y            int    `json:"y"`
	W            int    `json:"w"`
	H            int    `json:"h"`
}

// Page es una página del panel.
type Page struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Widgets []Widget `json:"widgets"`
}

// Panel devuelve las páginas en orden, con sus widgets.
func (s *Store) Panel(ctx context.Context) ([]Page, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM pages ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pages := []Page{}
	index := map[string]int{}
	for rows.Next() {
		page := Page{Widgets: []Widget{}}
		if err := rows.Scan(&page.ID, &page.Name); err != nil {
			return nil, err
		}
		index[page.ID] = len(pages)
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	widgets, err := s.db.QueryContext(ctx, `SELECT id, page_id, connection_id, x, y, w, h FROM widgets ORDER BY y, x, id`)
	if err != nil {
		return nil, err
	}
	defer widgets.Close()
	for widgets.Next() {
		var w Widget
		var pageID string
		if err := widgets.Scan(&w.ID, &pageID, &w.ConnectionID, &w.X, &w.Y, &w.W, &w.H); err != nil {
			return nil, err
		}
		if i, ok := index[pageID]; ok {
			pages[i].Widgets = append(pages[i].Widgets, w)
		}
	}
	return pages, widgets.Err()
}

// SavePanel sustituye de forma atómica todas las páginas y sus widgets.
func (s *Store) SavePanel(ctx context.Context, pages []Page) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM pages`); err != nil {
		return err
	}
	for position, page := range pages {
		if _, err := tx.ExecContext(ctx, `INSERT INTO pages(id, name, position) VALUES (?, ?, ?)`, page.ID, page.Name, position); err != nil {
			return err
		}
		for _, w := range page.Widgets {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO widgets(id, page_id, connection_id, x, y, w, h) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				w.ID, page.ID, w.ConnectionID, w.X, w.Y, w.W, w.H); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// Settings son los ajustes generales.
type Settings struct {
	Theme        string `json:"theme"`
	Accent       string `json:"accent"`
	RequireLogin bool   `json:"require_login"`
}

// GetSettings devuelve los ajustes, con sus valores por defecto si no se han guardado.
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	settings := Settings{Theme: "auto", Accent: "amber"}
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return settings, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return settings, err
		}
		switch key {
		case "theme":
			settings.Theme = value
		case "accent":
			settings.Accent = value
		case "require_login":
			settings.RequireLogin = value == "1"
		}
	}
	return settings, rows.Err()
}

// SaveSettings guarda los ajustes.
func (s *Store) SaveSettings(ctx context.Context, settings Settings) error {
	requireLogin := "0"
	if settings.RequireLogin {
		requireLogin = "1"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range map[string]string{"theme": settings.Theme, "accent": settings.Accent, "require_login": requireLogin} {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ChangePassword cambia la contraseña y revoca las demás sesiones del usuario.
func (s *Store) ChangePassword(ctx context.Context, userID int64, hash, keepTokenHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`, userID, keepTokenHash); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetPassword cambia la contraseña del administrador, revoca todas sus sesiones y
// devuelve su nombre. Se usa desde la línea de comandos, con acceso al equipo.
func (s *Store) ResetPassword(ctx context.Context, hash string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var id int64
	var username string
	err = tx.QueryRowContext(ctx, `SELECT id, username FROM users ORDER BY id LIMIT 1`).Scan(&id, &username)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return "", err
	}
	return username, tx.Commit()
}

// placeNewConnection coloca una conexión nueva en la primera página, alternando dos
// columnas, debajo de lo que ya ocupe ese espacio.
func placeNewConnection(ctx context.Context, tx *sql.Tx, widgetID, connectionID string) error {
	var pageID string
	var count int
	err := tx.QueryRowContext(ctx, `SELECT p.id, (SELECT COUNT(*) FROM widgets WHERE page_id = p.id)
		FROM pages p ORDER BY p.position, p.id LIMIT 1`).Scan(&pageID, &count)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	x := (count % 2) * DefaultWidgetWidth
	var y int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(y + h), 0) FROM widgets WHERE page_id = ? AND x < ? AND x + w > ?`,
		pageID, x+DefaultWidgetWidth, x).Scan(&y)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO widgets(id, page_id, connection_id, x, y, w, h) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		widgetID, pageID, connectionID, x, y, DefaultWidgetWidth, DefaultWidgetHeight)
	return err
}

// Tamaño inicial de un widget nuevo, en columnas y filas de la rejilla.
const (
	DefaultWidgetWidth  = 6
	DefaultWidgetHeight = 10
)
