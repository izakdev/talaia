package store

import "context"

// Link es un enlace guardado: un sitio que Talaia no vigila. Icon vacío, sin icono.
type Link struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Icon string `json:"icon"`
}

// LinkGroup es un grupo con nombre de enlaces guardados, en su orden.
type LinkGroup struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Links []Link `json:"links"`
}

// LinkGroups devuelve los grupos y sus enlaces en orden.
func (s *Store) LinkGroups(ctx context.Context) ([]LinkGroup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM link_groups ORDER BY position`)
	if err != nil {
		return nil, err
	}
	groups := []LinkGroup{}
	index := map[string]int{}
	for rows.Next() {
		g := LinkGroup{Links: []Link{}}
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			rows.Close()
			return nil, err
		}
		index[g.ID] = len(groups)
		groups = append(groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT id, group_id, name, url, icon FROM links ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l Link
		var group string
		if err := rows.Scan(&l.ID, &group, &l.Name, &l.URL, &l.Icon); err != nil {
			return nil, err
		}
		if i, ok := index[group]; ok {
			groups[i].Links = append(groups[i].Links, l)
		}
	}
	return groups, rows.Err()
}

// SaveLinkGroups reemplaza todos los grupos y enlaces; la posición es su orden.
func (s *Store) SaveLinkGroups(ctx context.Context, groups []LinkGroup) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM link_groups`); err != nil {
		return err
	}
	for i, g := range groups {
		if _, err := tx.ExecContext(ctx, `INSERT INTO link_groups(id, name, position) VALUES (?, ?, ?)`, g.ID, g.Name, i); err != nil {
			return err
		}
		for j, l := range g.Links {
			if _, err := tx.ExecContext(ctx, `INSERT INTO links(id, group_id, name, url, icon, position) VALUES (?, ?, ?, ?, ?, ?)`, l.ID, g.ID, l.Name, l.URL, l.Icon, j); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
