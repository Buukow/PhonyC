package store

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) ListChannels() ([]Channel, error) {
	rows, err := s.db.Query(`SELECT id, name, enabled, protocol, base_url, api_key, priority, extra_headers_json, timeout_ms, created_at, updated_at FROM channels ORDER BY priority DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		c, err := scanChannelRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) GetChannel(id int64) (*Channel, error) {
	row := s.db.QueryRow(`SELECT id, name, enabled, protocol, base_url, api_key, priority, extra_headers_json, timeout_ms, created_at, updated_at FROM channels WHERE id=?`, id)
	return scanChannel(row)
}

func scanChannel(row *sql.Row) (*Channel, error) {
	var c Channel
	var en int
	var cAt, uAt string
	if err := row.Scan(&c.ID, &c.Name, &en, &c.Protocol, &c.BaseURL, &c.APIKey, &c.Priority, &c.ExtraHeadersJSON, &c.TimeoutMS, &cAt, &uAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	c.Enabled = intToBool(en)
	c.CreatedAt = parseTime(cAt)
	c.UpdatedAt = parseTime(uAt)
	return &c, nil
}

func scanChannelRows(rows *sql.Rows) (*Channel, error) {
	var c Channel
	var en int
	var cAt, uAt string
	if err := rows.Scan(&c.ID, &c.Name, &en, &c.Protocol, &c.BaseURL, &c.APIKey, &c.Priority, &c.ExtraHeadersJSON, &c.TimeoutMS, &cAt, &uAt); err != nil {
		return nil, err
	}
	c.Enabled = intToBool(en)
	c.CreatedAt = parseTime(cAt)
	c.UpdatedAt = parseTime(uAt)
	return &c, nil
}

type ChannelInput struct {
	Name             string `json:"name"`
	Enabled          *bool  `json:"enabled"`
	Protocol         string `json:"protocol"`
	BaseURL          string `json:"base_url"`
	APIKey           string `json:"api_key"`
	Priority         *int   `json:"priority"`
	ExtraHeadersJSON string `json:"extra_headers_json"`
	TimeoutMS        *int   `json:"timeout_ms"`
}

func (s *Store) CreateChannel(in ChannelInput) (*Channel, error) {
	now := formatTime(nowUTC())
	en := 1
	if in.Enabled != nil && !*in.Enabled {
		en = 0
	}
	pri := 0
	if in.Priority != nil {
		pri = *in.Priority
	}
	to := 600000
	if in.TimeoutMS != nil {
		to = *in.TimeoutMS
	}
	extra := in.ExtraHeadersJSON
	if strings.TrimSpace(extra) == "" {
		extra = "{}"
	}
	res, err := s.db.Exec(`INSERT INTO channels(name, enabled, protocol, base_url, api_key, priority, extra_headers_json, timeout_ms, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?)`, in.Name, en, in.Protocol, strings.TrimRight(in.BaseURL, "/"), in.APIKey, pri, extra, to, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetChannel(id)
}

func (s *Store) UpdateChannel(id int64, in ChannelInput) (*Channel, error) {
	cur, err := s.GetChannel(id)
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		cur.Name = in.Name
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.Protocol != "" {
		cur.Protocol = in.Protocol
	}
	if in.BaseURL != "" {
		cur.BaseURL = strings.TrimRight(in.BaseURL, "/")
	}
	if in.APIKey != "" {
		cur.APIKey = in.APIKey
	}
	if in.Priority != nil {
		cur.Priority = *in.Priority
	}
	if in.ExtraHeadersJSON != "" {
		cur.ExtraHeadersJSON = in.ExtraHeadersJSON
	}
	if in.TimeoutMS != nil {
		cur.TimeoutMS = *in.TimeoutMS
	}
	now := formatTime(nowUTC())
	_, err = s.db.Exec(`UPDATE channels SET name=?, enabled=?, protocol=?, base_url=?, api_key=?, priority=?, extra_headers_json=?, timeout_ms=?, updated_at=? WHERE id=?`,
		cur.Name, boolToInt(cur.Enabled), cur.Protocol, cur.BaseURL, cur.APIKey, cur.Priority, cur.ExtraHeadersJSON, cur.TimeoutMS, now, id)
	if err != nil {
		return nil, err
	}
	return s.GetChannel(id)
}

func (s *Store) DeleteChannel(id int64) error {
	res, err := s.db.Exec(`DELETE FROM channels WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
