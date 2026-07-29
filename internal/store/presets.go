package store

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) ListPresets() ([]ClientPreset, error) {
	rows, err := s.db.Query(`SELECT id, name, description, version_label, headers_json, remove_headers, builtin, created_at, updated_at FROM client_presets ORDER BY builtin DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClientPreset
	for rows.Next() {
		p, err := scanPresetRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Store) GetPreset(id int64) (*ClientPreset, error) {
	row := s.db.QueryRow(`SELECT id, name, description, version_label, headers_json, remove_headers, builtin, created_at, updated_at FROM client_presets WHERE id=?`, id)
	return scanPreset(row)
}

func (s *Store) GetPresetByName(name string) (*ClientPreset, error) {
	row := s.db.QueryRow(`SELECT id, name, description, version_label, headers_json, remove_headers, builtin, created_at, updated_at FROM client_presets WHERE name=?`, name)
	return scanPreset(row)
}

func scanPreset(row *sql.Row) (*ClientPreset, error) {
	var p ClientPreset
	var bi int
	var cAt, uAt string
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.VersionLabel, &p.HeadersJSON, &p.RemoveHeaders, &bi, &cAt, &uAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Builtin = intToBool(bi)
	p.CreatedAt = parseTime(cAt)
	p.UpdatedAt = parseTime(uAt)
	return &p, nil
}

func scanPresetRows(rows *sql.Rows) (*ClientPreset, error) {
	var p ClientPreset
	var bi int
	var cAt, uAt string
	if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.VersionLabel, &p.HeadersJSON, &p.RemoveHeaders, &bi, &cAt, &uAt); err != nil {
		return nil, err
	}
	p.Builtin = intToBool(bi)
	p.CreatedAt = parseTime(cAt)
	p.UpdatedAt = parseTime(uAt)
	return &p, nil
}

type PresetInput struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	VersionLabel  string `json:"version_label"`
	HeadersJSON   string `json:"headers_json"`
	RemoveHeaders string `json:"remove_headers"`
	Builtin       bool   `json:"builtin"`
}

func (s *Store) CreatePreset(in PresetInput) (*ClientPreset, error) {
	now := formatTime(nowUTC())
	hj := in.HeadersJSON
	if strings.TrimSpace(hj) == "" {
		hj = "{}"
	}
	rh := in.RemoveHeaders
	if strings.TrimSpace(rh) == "" {
		rh = "[]"
	}
	res, err := s.db.Exec(`INSERT INTO client_presets(name, description, version_label, headers_json, remove_headers, builtin, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		in.Name, in.Description, in.VersionLabel, hj, rh, boolToInt(in.Builtin), now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetPreset(id)
}

func (s *Store) UpsertBuiltinPreset(in PresetInput) (*ClientPreset, error) {
	cur, err := s.GetPresetByName(in.Name)
	if err == nil {
		// keep user edits for non-first run: only seed if missing
		return cur, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	in.Builtin = true
	return s.CreatePreset(in)
}

func (s *Store) UpdatePreset(id int64, in PresetInput) (*ClientPreset, error) {
	cur, err := s.GetPreset(id)
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		cur.Name = in.Name
	}
	if in.Description != "" || in.Description == "" && in.HeadersJSON != "" {
		// allow empty description only when other fields provided via explicit set below
	}
	// always apply provided fields
	if in.Name != "" {
		cur.Name = in.Name
	}
	cur.Description = in.Description
	if in.VersionLabel != "" {
		cur.VersionLabel = in.VersionLabel
	}
	if in.HeadersJSON != "" {
		cur.HeadersJSON = in.HeadersJSON
	}
	if in.RemoveHeaders != "" {
		cur.RemoveHeaders = in.RemoveHeaders
	}
	now := formatTime(nowUTC())
	_, err = s.db.Exec(`UPDATE client_presets SET name=?, description=?, version_label=?, headers_json=?, remove_headers=?, updated_at=? WHERE id=?`,
		cur.Name, cur.Description, cur.VersionLabel, cur.HeadersJSON, cur.RemoveHeaders, now, id)
	if err != nil {
		return nil, err
	}
	return s.GetPreset(id)
}

func (s *Store) DeletePreset(id int64) error {
	cur, err := s.GetPreset(id)
	if err != nil {
		return err
	}
	if cur.Builtin {
		return errors.New("cannot delete builtin preset")
	}
	res, err := s.db.Exec(`DELETE FROM client_presets WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
