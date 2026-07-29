package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const channelSelectCols = `id, name, enabled, temp_disabled, protocol, base_url, api_key, priority, extra_headers_json, timeout_ms, test_model, last_test_at, last_test_status, last_test_ms, last_test_error, created_at, updated_at`

func (s *Store) ListChannels() ([]Channel, error) {
	rows, err := s.db.Query(`SELECT ` + channelSelectCols + ` FROM channels ORDER BY priority DESC, id ASC`)
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
	row := s.db.QueryRow(`SELECT `+channelSelectCols+` FROM channels WHERE id=?`, id)
	return scanChannel(row)
}

func scanChannel(row *sql.Row) (*Channel, error) {
	var c Channel
	var en, td int
	var lastAt, cAt, uAt string
	if err := row.Scan(&c.ID, &c.Name, &en, &td, &c.Protocol, &c.BaseURL, &c.APIKey, &c.Priority, &c.ExtraHeadersJSON, &c.TimeoutMS, &c.TestModel, &lastAt, &c.LastTestStatus, &c.LastTestMs, &c.LastTestError, &cAt, &uAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	c.Enabled = intToBool(en)
	c.TempDisabled = intToBool(td)
	if lastAt != "" {
		t := parseTime(lastAt)
		c.LastTestAt = &t
	}
	c.CreatedAt = parseTime(cAt)
	c.UpdatedAt = parseTime(uAt)
	return &c, nil
}

func scanChannelRows(rows *sql.Rows) (*Channel, error) {
	var c Channel
	var en, td int
	var lastAt, cAt, uAt string
	if err := rows.Scan(&c.ID, &c.Name, &en, &td, &c.Protocol, &c.BaseURL, &c.APIKey, &c.Priority, &c.ExtraHeadersJSON, &c.TimeoutMS, &c.TestModel, &lastAt, &c.LastTestStatus, &c.LastTestMs, &c.LastTestError, &cAt, &uAt); err != nil {
		return nil, err
	}
	c.Enabled = intToBool(en)
	c.TempDisabled = intToBool(td)
	if lastAt != "" {
		t := parseTime(lastAt)
		c.LastTestAt = &t
	}
	c.CreatedAt = parseTime(cAt)
	c.UpdatedAt = parseTime(uAt)
	return &c, nil
}

type ChannelInput struct {
	Name             string `json:"name"`
	Enabled          *bool  `json:"enabled"`
	TempDisabled     *bool  `json:"temp_disabled"`
	Protocol         string `json:"protocol"`
	BaseURL          string `json:"base_url"`
	APIKey           string `json:"api_key"`
	Priority         *int   `json:"priority"`
	ExtraHeadersJSON string `json:"extra_headers_json"`
	TimeoutMS        *int   `json:"timeout_ms"`
	TestModel        *string `json:"test_model"`
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
	testModel := ""
	if in.TestModel != nil {
		testModel = *in.TestModel
	}
	res, err := s.db.Exec(`INSERT INTO channels(name, enabled, temp_disabled, protocol, base_url, api_key, priority, extra_headers_json, timeout_ms, test_model, last_test_at, last_test_status, last_test_ms, last_test_error, created_at, updated_at)
VALUES(?,?,0,?,?,?,?,?,?,?,'',0,0,'',?,?)`, in.Name, en, in.Protocol, strings.TrimRight(in.BaseURL, "/"), in.APIKey, pri, extra, to, testModel, now, now)
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
	if in.TempDisabled != nil {
		cur.TempDisabled = *in.TempDisabled
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
	if in.TestModel != nil {
		cur.TestModel = *in.TestModel
	}
	now := formatTime(nowUTC())
	_, err = s.db.Exec(`UPDATE channels SET name=?, enabled=?, temp_disabled=?, protocol=?, base_url=?, api_key=?, priority=?, extra_headers_json=?, timeout_ms=?, test_model=?, updated_at=? WHERE id=?`,
		cur.Name, boolToInt(cur.Enabled), boolToInt(cur.TempDisabled), cur.Protocol, cur.BaseURL, cur.APIKey, cur.Priority, cur.ExtraHeadersJSON, cur.TimeoutMS, cur.TestModel, now, id)
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

func (s *Store) SetChannelTempDisabled(id int64, temp bool) error {
	now := formatTime(nowUTC())
	res, err := s.db.Exec(`UPDATE channels SET temp_disabled=?, updated_at=? WHERE id=?`, boolToInt(temp), now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateChannelTestResult(id int64, status int, ms int64, errSummary string, tempDisabled *bool) error {
	now := time.Now().UTC()
	nowStr := formatTime(now)
	if tempDisabled != nil {
		_, err := s.db.Exec(`UPDATE channels SET last_test_at=?, last_test_status=?, last_test_ms=?, last_test_error=?, temp_disabled=?, updated_at=? WHERE id=?`,
			nowStr, status, ms, errSummary, boolToInt(*tempDisabled), nowStr, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE channels SET last_test_at=?, last_test_status=?, last_test_ms=?, last_test_error=?, updated_at=? WHERE id=?`,
		nowStr, status, ms, errSummary, nowStr, id)
	return err
}

// ListEnabledChannelsForAutoTest returns manually-enabled channels (incl. temp-disabled for recovery).
func (s *Store) ListEnabledChannelsForAutoTest() ([]Channel, error) {
	rows, err := s.db.Query(`SELECT ` + channelSelectCols + ` FROM channels WHERE enabled=1 ORDER BY priority DESC, id ASC`)
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
