package store

import (
	"database/sql"
	"errors"
)

func (s *Store) ListChannelModels(channelID int64) ([]ChannelModel, error) {
	rows, err := s.db.Query(`SELECT id, channel_id, client_model, upstream_model, rewrite_model, enabled, created_at, updated_at FROM channel_models WHERE channel_id=? ORDER BY id ASC`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChannelModel
	for rows.Next() {
		m, err := scanChannelModelRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) ListAllChannelModels() ([]ChannelModel, error) {
	rows, err := s.db.Query(`SELECT id, channel_id, client_model, upstream_model, rewrite_model, enabled, created_at, updated_at FROM channel_models ORDER BY channel_id, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChannelModel
	for rows.Next() {
		m, err := scanChannelModelRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) GetChannelModel(id int64) (*ChannelModel, error) {
	row := s.db.QueryRow(`SELECT id, channel_id, client_model, upstream_model, rewrite_model, enabled, created_at, updated_at FROM channel_models WHERE id=?`, id)
	return scanChannelModel(row)
}

func scanChannelModel(row *sql.Row) (*ChannelModel, error) {
	var m ChannelModel
	var rw, en int
	var cAt, uAt string
	if err := row.Scan(&m.ID, &m.ChannelID, &m.ClientModel, &m.UpstreamModel, &rw, &en, &cAt, &uAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	m.RewriteModel = intToBool(rw)
	m.Enabled = intToBool(en)
	m.CreatedAt = parseTime(cAt)
	m.UpdatedAt = parseTime(uAt)
	return &m, nil
}

func scanChannelModelRows(rows *sql.Rows) (*ChannelModel, error) {
	var m ChannelModel
	var rw, en int
	var cAt, uAt string
	if err := rows.Scan(&m.ID, &m.ChannelID, &m.ClientModel, &m.UpstreamModel, &rw, &en, &cAt, &uAt); err != nil {
		return nil, err
	}
	m.RewriteModel = intToBool(rw)
	m.Enabled = intToBool(en)
	m.CreatedAt = parseTime(cAt)
	m.UpdatedAt = parseTime(uAt)
	return &m, nil
}

type ChannelModelInput struct {
	ClientModel   string `json:"client_model"`
	UpstreamModel string `json:"upstream_model"`
	RewriteModel  *bool  `json:"rewrite_model"`
	Enabled       *bool  `json:"enabled"`
}

func (s *Store) CreateChannelModel(channelID int64, in ChannelModelInput) (*ChannelModel, error) {
	now := formatTime(nowUTC())
	rw, en := 0, 1
	if in.RewriteModel != nil && *in.RewriteModel {
		rw = 1
	}
	if in.Enabled != nil && !*in.Enabled {
		en = 0
	}
	up := in.UpstreamModel
	if up == "" {
		up = in.ClientModel
	}
	res, err := s.db.Exec(`INSERT INTO channel_models(channel_id, client_model, upstream_model, rewrite_model, enabled, created_at, updated_at) VALUES(?,?,?,?,?,?,?)`,
		channelID, in.ClientModel, up, rw, en, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetChannelModel(id)
}

func (s *Store) UpdateChannelModel(id int64, in ChannelModelInput) (*ChannelModel, error) {
	cur, err := s.GetChannelModel(id)
	if err != nil {
		return nil, err
	}
	if in.ClientModel != "" {
		cur.ClientModel = in.ClientModel
	}
	if in.UpstreamModel != "" {
		cur.UpstreamModel = in.UpstreamModel
	}
	if in.RewriteModel != nil {
		cur.RewriteModel = *in.RewriteModel
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	now := formatTime(nowUTC())
	_, err = s.db.Exec(`UPDATE channel_models SET client_model=?, upstream_model=?, rewrite_model=?, enabled=?, updated_at=? WHERE id=?`,
		cur.ClientModel, cur.UpstreamModel, boolToInt(cur.RewriteModel), boolToInt(cur.Enabled), now, id)
	if err != nil {
		return nil, err
	}
	return s.GetChannelModel(id)
}

func (s *Store) DeleteChannelModel(id int64) error {
	res, err := s.db.Exec(`DELETE FROM channel_models WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
