package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) InsertRequestMeta(m *RequestMeta) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = nowUTC()
	}
	res, err := s.db.Exec(`INSERT INTO request_meta(request_id, created_at, user_key_id, client_model, upstream_model, channel_id, method, path, status_code, ttfb_ms, total_ms, error_summary, impersonation_mode)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.RequestID, formatTime(m.CreatedAt), m.UserKeyID, m.ClientModel, m.UpstreamModel, m.ChannelID, m.Method, m.Path, m.StatusCode, m.TTFBms, m.TotalMs, m.ErrorSummary, m.ImpersonationMode)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	return nil
}

func (s *Store) IncrKeyStats(userKeyID int64, day string, isError bool) error {
	errInc := 0
	if isError {
		errInc = 1
	}
	_, err := s.db.Exec(`INSERT INTO key_stats_daily(user_key_id, day, requests, errors) VALUES(?,?,1,?)
ON CONFLICT(user_key_id, day) DO UPDATE SET requests = requests + 1, errors = errors + excluded.errors`, userKeyID, day, errInc)
	return err
}

type LogFilter struct {
	UserKeyID *int64
	ChannelID *int64
	Path      string
	StatusMin *int
	StatusMax *int
	Q         string
	Limit     int
	Offset    int
}

func (s *Store) ListRequestMeta(f LogFilter) ([]RequestMeta, int, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.UserKeyID != nil {
		where = append(where, "user_key_id=?")
		args = append(args, *f.UserKeyID)
	}
	if f.ChannelID != nil {
		where = append(where, "channel_id=?")
		args = append(args, *f.ChannelID)
	}
	if f.Path != "" {
		where = append(where, "path LIKE ?")
		args = append(args, "%"+f.Path+"%")
	}
	if f.StatusMin != nil {
		where = append(where, "status_code>=?")
		args = append(args, *f.StatusMin)
	}
	if f.StatusMax != nil {
		where = append(where, "status_code<=?")
		args = append(args, *f.StatusMax)
	}
	if f.Q != "" {
		where = append(where, "(client_model LIKE ? OR upstream_model LIKE ? OR error_summary LIKE ? OR request_id LIKE ?)")
		q := "%" + f.Q + "%"
		args = append(args, q, q, q, q)
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM request_meta WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	qArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.Query(`SELECT id, request_id, created_at, user_key_id, client_model, upstream_model, channel_id, method, path, status_code, ttfb_ms, total_ms, error_summary, impersonation_mode
FROM request_meta WHERE `+w+` ORDER BY id DESC LIMIT ? OFFSET ?`, qArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []RequestMeta
	for rows.Next() {
		var m RequestMeta
		var cAt string
		var uk, ch sql.NullInt64
		if err := rows.Scan(&m.ID, &m.RequestID, &cAt, &uk, &m.ClientModel, &m.UpstreamModel, &ch, &m.Method, &m.Path, &m.StatusCode, &m.TTFBms, &m.TotalMs, &m.ErrorSummary, &m.ImpersonationMode); err != nil {
			return nil, 0, err
		}
		m.CreatedAt = parseTime(cAt)
		if uk.Valid {
			v := uk.Int64
			m.UserKeyID = &v
		}
		if ch.Valid {
			v := ch.Int64
			m.ChannelID = &v
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

type DashboardSummary struct {
	RequestsToday int64            `json:"requests_today"`
	ErrorsToday   int64            `json:"errors_today"`
	Requests7d    int64            `json:"requests_7d"`
	ErrorRate7d   float64          `json:"error_rate_7d"`
	TopKeys       []NameCount      `json:"top_keys"`
	TopModels     []NameCount      `json:"top_models"`
	RecentErrors  []RequestMeta    `json:"recent_errors"`
}

type NameCount struct {
	ID    int64  `json:"id,omitempty"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (s *Store) DashboardSummary() (*DashboardSummary, error) {
	today := time.Now().UTC().Format("2006-01-02")
	day7 := time.Now().UTC().AddDate(0, 0, -6).Format("2006-01-02")
	out := &DashboardSummary{TopKeys: []NameCount{}, TopModels: []NameCount{}, RecentErrors: []RequestMeta{}}

	_ = s.db.QueryRow(`SELECT COALESCE(SUM(requests),0), COALESCE(SUM(errors),0) FROM key_stats_daily WHERE day=?`, today).Scan(&out.RequestsToday, &out.ErrorsToday)
	var err7 int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(requests),0), COALESCE(SUM(errors),0) FROM key_stats_daily WHERE day>=?`, day7).Scan(&out.Requests7d, &err7)
	if out.Requests7d > 0 {
		out.ErrorRate7d = float64(err7) / float64(out.Requests7d)
	}

	rows, err := s.db.Query(`SELECT user_key_id, COALESCE(SUM(requests),0) AS c FROM key_stats_daily WHERE day>=? GROUP BY user_key_id ORDER BY c DESC LIMIT 5`, day7)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, c int64
			if err := rows.Scan(&id, &c); err != nil {
				break
			}
			name := fmt.Sprintf("key#%d", id)
			if k, err := s.GetUserKey(id); err == nil {
				name = k.Name
			}
			out.TopKeys = append(out.TopKeys, NameCount{ID: id, Name: name, Count: c})
		}
	}

	rows2, err := s.db.Query(`SELECT client_model, COUNT(1) AS c FROM request_meta WHERE created_at>=? AND client_model!='' GROUP BY client_model ORDER BY c DESC LIMIT 5`, day7+"T00:00:00Z")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var name string
			var c int64
			if err := rows2.Scan(&name, &c); err != nil {
				break
			}
			out.TopModels = append(out.TopModels, NameCount{Name: name, Count: c})
		}
	}

	rows3, err := s.db.Query(`SELECT id, request_id, created_at, user_key_id, client_model, upstream_model, channel_id, method, path, status_code, ttfb_ms, total_ms, error_summary, impersonation_mode
FROM request_meta WHERE status_code>=400 OR error_summary!='' ORDER BY id DESC LIMIT 10`)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var m RequestMeta
			var cAt string
			var uk, ch sql.NullInt64
			if err := rows3.Scan(&m.ID, &m.RequestID, &cAt, &uk, &m.ClientModel, &m.UpstreamModel, &ch, &m.Method, &m.Path, &m.StatusCode, &m.TTFBms, &m.TotalMs, &m.ErrorSummary, &m.ImpersonationMode); err != nil {
				break
			}
			m.CreatedAt = parseTime(cAt)
			if uk.Valid {
				v := uk.Int64
				m.UserKeyID = &v
			}
			if ch.Valid {
				v := ch.Int64
				m.ChannelID = &v
			}
			out.RecentErrors = append(out.RecentErrors, m)
		}
	}
	return out, nil
}

func (s *Store) KeyStats(userKeyID int64, fromDay string) ([]KeyStatsDaily, error) {
	rows, err := s.db.Query(`SELECT user_key_id, day, requests, errors FROM key_stats_daily WHERE user_key_id=? AND day>=? ORDER BY day ASC`, userKeyID, fromDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KeyStatsDaily
	for rows.Next() {
		var k KeyStatsDaily
		if err := rows.Scan(&k.UserKeyID, &k.Day, &k.Requests, &k.Errors); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO app_settings(key, value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) ListSettings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
