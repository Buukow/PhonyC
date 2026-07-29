package store

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) ListUserKeys() ([]UserKey, error) {
	rows, err := s.db.Query(`SELECT id, name, key, enabled, remark, impersonation_mode, preset_id, custom_headers_json, created_at, updated_at FROM user_keys ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserKey
	for rows.Next() {
		k, err := scanUserKeyRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (s *Store) GetUserKey(id int64) (*UserKey, error) {
	row := s.db.QueryRow(`SELECT id, name, key, enabled, remark, impersonation_mode, preset_id, custom_headers_json, created_at, updated_at FROM user_keys WHERE id=?`, id)
	return scanUserKey(row)
}

func (s *Store) GetUserKeyByKey(key string) (*UserKey, error) {
	row := s.db.QueryRow(`SELECT id, name, key, enabled, remark, impersonation_mode, preset_id, custom_headers_json, created_at, updated_at FROM user_keys WHERE key=?`, key)
	return scanUserKey(row)
}

func scanUserKey(row *sql.Row) (*UserKey, error) {
	var k UserKey
	var en int
	var preset sql.NullInt64
	var cAt, uAt string
	if err := row.Scan(&k.ID, &k.Name, &k.Key, &en, &k.Remark, &k.ImpersonationMode, &preset, &k.CustomHeadersJSON, &cAt, &uAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	k.Enabled = intToBool(en)
	if preset.Valid {
		v := preset.Int64
		k.PresetID = &v
	}
	k.CreatedAt = parseTime(cAt)
	k.UpdatedAt = parseTime(uAt)
	return &k, nil
}

func scanUserKeyRows(rows *sql.Rows) (*UserKey, error) {
	var k UserKey
	var en int
	var preset sql.NullInt64
	var cAt, uAt string
	if err := rows.Scan(&k.ID, &k.Name, &k.Key, &en, &k.Remark, &k.ImpersonationMode, &preset, &k.CustomHeadersJSON, &cAt, &uAt); err != nil {
		return nil, err
	}
	k.Enabled = intToBool(en)
	if preset.Valid {
		v := preset.Int64
		k.PresetID = &v
	}
	k.CreatedAt = parseTime(cAt)
	k.UpdatedAt = parseTime(uAt)
	return &k, nil
}

type UserKeyInput struct {
	Name              string `json:"name"`
	Key               string `json:"key"`
	Enabled           *bool  `json:"enabled"`
	Remark            *string `json:"remark"`
	ImpersonationMode string `json:"impersonation_mode"`
	PresetID          *int64 `json:"preset_id"`
	ClearPreset       bool   `json:"clear_preset"`
	CustomHeadersJSON string `json:"custom_headers_json"`
}

func (s *Store) CreateUserKey(in UserKeyInput) (*UserKey, error) {
	now := formatTime(nowUTC())
	en := 1
	if in.Enabled != nil && !*in.Enabled {
		en = 0
	}
	mode := in.ImpersonationMode
	if mode == "" {
		mode = "passthrough"
	}
	custom := in.CustomHeadersJSON
	if strings.TrimSpace(custom) == "" {
		custom = "{}"
	}
	remark := ""
	if in.Remark != nil {
		remark = *in.Remark
	}
	var preset any
	if in.PresetID != nil {
		preset = *in.PresetID
	}
	res, err := s.db.Exec(`INSERT INTO user_keys(name, key, enabled, remark, impersonation_mode, preset_id, custom_headers_json, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,?)`, in.Name, in.Key, en, remark, mode, preset, custom, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUserKey(id)
}

func (s *Store) UpdateUserKey(id int64, in UserKeyInput) (*UserKey, error) {
	cur, err := s.GetUserKey(id)
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		cur.Name = in.Name
	}
	if in.Key != "" {
		cur.Key = in.Key
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.Remark != nil {
		cur.Remark = *in.Remark
	}
	if in.ImpersonationMode != "" {
		cur.ImpersonationMode = in.ImpersonationMode
	}
	if in.ClearPreset {
		cur.PresetID = nil
	} else if in.PresetID != nil {
		cur.PresetID = in.PresetID
	}
	if in.CustomHeadersJSON != "" {
		cur.CustomHeadersJSON = in.CustomHeadersJSON
	}
	now := formatTime(nowUTC())
	var preset any
	if cur.PresetID != nil {
		preset = *cur.PresetID
	}
	_, err = s.db.Exec(`UPDATE user_keys SET name=?, key=?, enabled=?, remark=?, impersonation_mode=?, preset_id=?, custom_headers_json=?, updated_at=? WHERE id=?`,
		cur.Name, cur.Key, boolToInt(cur.Enabled), cur.Remark, cur.ImpersonationMode, preset, cur.CustomHeadersJSON, now, id)
	if err != nil {
		return nil, err
	}
	return s.GetUserKey(id)
}

func (s *Store) DeleteUserKey(id int64) error {
	res, err := s.db.Exec(`DELETE FROM user_keys WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
