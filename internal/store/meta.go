package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *Store) InsertRequestMeta(m *RequestMeta) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = nowUTC()
	}
	res, err := s.db.Exec(`INSERT INTO request_meta(
request_id, created_at, user_key_id, client_model, upstream_model, channel_id, method, path,
status_code, ttfb_ms, total_ms, error_summary, impersonation_mode,
prompt_tokens, completion_tokens, total_tokens, cached_tokens, reasoning_tokens)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.RequestID, formatTime(m.CreatedAt), m.UserKeyID, m.ClientModel, m.UpstreamModel, m.ChannelID, m.Method, m.Path,
		m.StatusCode, m.TTFBms, m.TotalMs, m.ErrorSummary, m.ImpersonationMode,
		m.PromptTokens, m.CompletionTokens, m.TotalTokens, m.CachedTokens, m.ReasoningTokens)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.ID = id

	// aggregate using system local time boundary
	local := m.CreatedAt.In(time.Local)
	if m.CreatedAt.IsZero() {
		local = nowLocal()
	}
	isErr := m.StatusCode >= 400 || m.ErrorSummary != ""
	_ = s.IncrUsageHourly(hourBucket(local), isErr, m.PromptTokens, m.CompletionTokens, m.TotalTokens, m.CachedTokens, m.ReasoningTokens)
	if strings.TrimSpace(m.ClientModel) != "" {
		_ = s.IncrModelDaily(dayBucket(local), m.ClientModel, m.PromptTokens, m.CompletionTokens, m.TotalTokens)
	}
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

func (s *Store) IncrUsageHourly(hour string, isError bool, prompt, completion, total, cached, reasoning int) error {
	errInc := 0
	if isError {
		errInc = 1
	}
	_, err := s.db.Exec(`INSERT INTO usage_stats_hourly(hour, requests, errors, prompt_tokens, completion_tokens, total_tokens, cached_tokens, reasoning_tokens)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(hour) DO UPDATE SET
  requests = requests + 1,
  errors = errors + excluded.errors,
  prompt_tokens = prompt_tokens + excluded.prompt_tokens,
  completion_tokens = completion_tokens + excluded.completion_tokens,
  total_tokens = total_tokens + excluded.total_tokens,
  cached_tokens = cached_tokens + excluded.cached_tokens,
  reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens`,
		hour, 1, errInc, prompt, completion, total, cached, reasoning)
	return err
}

func (s *Store) IncrModelDaily(day, model string, prompt, completion, total int) error {
	_, err := s.db.Exec(`INSERT INTO model_stats_daily(day, client_model, requests, prompt_tokens, completion_tokens, total_tokens)
VALUES(?,?,1,?,?,?)
ON CONFLICT(day, client_model) DO UPDATE SET
  requests = requests + 1,
  prompt_tokens = prompt_tokens + excluded.prompt_tokens,
  completion_tokens = completion_tokens + excluded.completion_tokens,
  total_tokens = total_tokens + excluded.total_tokens`,
		day, model, prompt, completion, total)
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
	rows, err := s.db.Query(`SELECT id, request_id, created_at, user_key_id, client_model, upstream_model, channel_id, method, path, status_code, ttfb_ms, total_ms, error_summary, impersonation_mode,
prompt_tokens, completion_tokens, total_tokens, cached_tokens, reasoning_tokens
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
		if err := rows.Scan(&m.ID, &m.RequestID, &cAt, &uk, &m.ClientModel, &m.UpstreamModel, &ch, &m.Method, &m.Path, &m.StatusCode, &m.TTFBms, &m.TotalMs, &m.ErrorSummary, &m.ImpersonationMode,
			&m.PromptTokens, &m.CompletionTokens, &m.TotalTokens, &m.CachedTokens, &m.ReasoningTokens); err != nil {
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

type TokenUsageTotal struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	CachedTokens     int64 `json:"cached_tokens"`
	ReasoningTokens  int64 `json:"reasoning_tokens"`
}

type DashboardSummary struct {
	RequestsToday int64              `json:"requests_today"`
	ErrorsToday   int64              `json:"errors_today"`
	Requests7d    int64              `json:"requests_7d"`
	ErrorRate7d   float64            `json:"error_rate_7d"`
	Period        string             `json:"period"`
	TokenUsage    TokenUsageTotal    `json:"token_usage"`
	Series        []UsageSeriesPoint `json:"series"`
	TopModels     []ModelStat        `json:"top_models"`
	RecentErrors  []RequestMeta      `json:"recent_errors"`
}

type NameCount struct {
	ID    int64  `json:"id,omitempty"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (s *Store) DashboardSummary(period string) (*DashboardSummary, error) {
	period = strings.ToLower(strings.TrimSpace(period))
	if period == "" {
		period = "7d"
	}
	switch period {
	case "24h", "7d", "30d":
	default:
		period = "7d"
	}

	now := nowLocal()
	today := dayBucket(now)
	day7 := dayBucket(now.AddDate(0, 0, -6))

	out := &DashboardSummary{
		Period:       period,
		TopModels:    []ModelStat{},
		Series:       []UsageSeriesPoint{},
		RecentErrors: []RequestMeta{},
	}

	_ = s.db.QueryRow(`SELECT COALESCE(SUM(requests),0), COALESCE(SUM(errors),0) FROM key_stats_daily WHERE day=?`, today).Scan(&out.RequestsToday, &out.ErrorsToday)
	var err7 int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(requests),0), COALESCE(SUM(errors),0) FROM key_stats_daily WHERE day>=?`, day7).Scan(&out.Requests7d, &err7)
	if out.Requests7d > 0 {
		out.ErrorRate7d = float64(err7) / float64(out.Requests7d)
	}

	// series + token totals for selected period
	switch period {
	case "24h":
		start := now.Add(-23 * time.Hour).Truncate(time.Hour)
		hours := make([]string, 0, 24)
		for i := 0; i < 24; i++ {
			h := start.Add(time.Duration(i) * time.Hour)
			hours = append(hours, hourBucket(h))
		}
		byHour := map[string]UsageSeriesPoint{}
		if len(hours) > 0 {
			placeholders := strings.TrimRight(strings.Repeat("?,", len(hours)), ",")
			args := make([]any, len(hours))
			for i, h := range hours {
				args[i] = h
			}
			rows, err := s.db.Query(`SELECT hour, requests, errors, prompt_tokens, completion_tokens, total_tokens, cached_tokens, reasoning_tokens
FROM usage_stats_hourly WHERE hour IN (`+placeholders+`)`, args...)
			if err == nil {
				for rows.Next() {
					var hour string
					var p UsageSeriesPoint
					if err := rows.Scan(&hour, &p.Requests, &p.Errors, &p.PromptTokens, &p.CompletionTokens, &p.TotalTokens, &p.CachedTokens, &p.ReasoningTokens); err != nil {
						break
					}
					p.Start = hour + ":00:00"
					end, _ := time.ParseInLocation("2006-01-02T15", hour, time.Local)
					p.End = end.Add(time.Hour).Format("2006-01-02T15:04:05")
					byHour[hour] = p
				}
				_ = rows.Close()
			}
		}
		for i, h := range hours {
			pt, ok := byHour[h]
			if !ok {
				end := start.Add(time.Duration(i+1) * time.Hour)
				pt = UsageSeriesPoint{
					Start: start.Add(time.Duration(i) * time.Hour).Format("2006-01-02T15:04:05"),
					End:   end.Format("2006-01-02T15:04:05"),
				}
			} else if pt.Start == "" {
				pt.Start = start.Add(time.Duration(i) * time.Hour).Format("2006-01-02T15:04:05")
				pt.End = start.Add(time.Duration(i+1) * time.Hour).Format("2006-01-02T15:04:05")
			}
			out.Series = append(out.Series, pt)
			out.TokenUsage.PromptTokens += pt.PromptTokens
			out.TokenUsage.CompletionTokens += pt.CompletionTokens
			out.TokenUsage.TotalTokens += pt.TotalTokens
			out.TokenUsage.CachedTokens += pt.CachedTokens
			out.TokenUsage.ReasoningTokens += pt.ReasoningTokens
		}
	default:
		days := 7
		if period == "30d" {
			days = 30
		}
		startDay := now.AddDate(0, 0, -(days - 1))
		dayList := make([]string, 0, days)
		for i := 0; i < days; i++ {
			dayList = append(dayList, dayBucket(startDay.AddDate(0, 0, i)))
		}
		byDay := map[string]UsageSeriesPoint{}
		placeholders := strings.TrimRight(strings.Repeat("?,", len(dayList)), ",")
		args := make([]any, len(dayList))
		for i, d := range dayList {
			args[i] = d
		}
		// aggregate hourly rows by day prefix
		rows, err := s.db.Query(`SELECT substr(hour,1,10) AS day,
  COALESCE(SUM(requests),0), COALESCE(SUM(errors),0),
  COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(total_tokens),0),
  COALESCE(SUM(cached_tokens),0), COALESCE(SUM(reasoning_tokens),0)
FROM usage_stats_hourly
WHERE substr(hour,1,10) IN (`+placeholders+`)
GROUP BY substr(hour,1,10)`, args...)
		if err == nil {
			for rows.Next() {
				var day string
				var p UsageSeriesPoint
				if err := rows.Scan(&day, &p.Requests, &p.Errors, &p.PromptTokens, &p.CompletionTokens, &p.TotalTokens, &p.CachedTokens, &p.ReasoningTokens); err != nil {
					break
				}
				p.Start = day + "T00:00:00"
				p.End = day + "T23:59:59"
				byDay[day] = p
			}
			_ = rows.Close()
		}
		for _, d := range dayList {
			pt, ok := byDay[d]
			if !ok {
				pt = UsageSeriesPoint{Start: d + "T00:00:00", End: d + "T23:59:59"}
			}
			out.Series = append(out.Series, pt)
			out.TokenUsage.PromptTokens += pt.PromptTokens
			out.TokenUsage.CompletionTokens += pt.CompletionTokens
			out.TokenUsage.TotalTokens += pt.TotalTokens
			out.TokenUsage.CachedTokens += pt.CachedTokens
			out.TokenUsage.ReasoningTokens += pt.ReasoningTokens
		}
	}

	// top models: last 7 local days, show both requests and tokens
	rows2, err := s.db.Query(`SELECT client_model,
  COALESCE(SUM(requests),0), COALESCE(SUM(total_tokens),0),
  COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0)
FROM model_stats_daily WHERE day>=? AND client_model!=''
GROUP BY client_model
ORDER BY SUM(requests) DESC, SUM(total_tokens) DESC
LIMIT 8`, day7)
	if err == nil {
		for rows2.Next() {
			var m ModelStat
			if err := rows2.Scan(&m.Name, &m.Count, &m.Tokens, &m.PromptTokens, &m.CompletionTokens); err != nil {
				break
			}
			out.TopModels = append(out.TopModels, m)
		}
		_ = rows2.Close()
	}

	rows3, err := s.db.Query(`SELECT id, request_id, created_at, user_key_id, client_model, upstream_model, channel_id, method, path, status_code, ttfb_ms, total_ms, error_summary, impersonation_mode,
prompt_tokens, completion_tokens, total_tokens, cached_tokens, reasoning_tokens
FROM request_meta WHERE status_code>=400 OR error_summary!='' ORDER BY id DESC LIMIT 10`)
	if err == nil {
		for rows3.Next() {
			var m RequestMeta
			var cAt string
			var uk, ch sql.NullInt64
			if err := rows3.Scan(&m.ID, &m.RequestID, &cAt, &uk, &m.ClientModel, &m.UpstreamModel, &ch, &m.Method, &m.Path, &m.StatusCode, &m.TTFBms, &m.TotalMs, &m.ErrorSummary, &m.ImpersonationMode,
				&m.PromptTokens, &m.CompletionTokens, &m.TotalTokens, &m.CachedTokens, &m.ReasoningTokens); err != nil {
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
		_ = rows3.Close()
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

func (s *Store) SetSettings(values map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range values {
		if _, err := tx.Exec(`INSERT INTO app_settings(key, value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
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
