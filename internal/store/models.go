package store

import "time"

type AdminUser struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Channel struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Enabled          bool      `json:"enabled"`
	Protocol         string    `json:"protocol"` // openai | anthropic
	BaseURL          string    `json:"base_url"`
	APIKey           string    `json:"api_key"`
	Priority         int       `json:"priority"`
	ExtraHeadersJSON string    `json:"extra_headers_json"`
	TimeoutMS        int       `json:"timeout_ms"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ChannelModel struct {
	ID            int64     `json:"id"`
	ChannelID     int64     `json:"channel_id"`
	ClientModel   string    `json:"client_model"`
	UpstreamModel string    `json:"upstream_model"`
	RewriteModel  bool      `json:"rewrite_model"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UserKey struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	Key                string    `json:"key"`
	Enabled            bool      `json:"enabled"`
	Remark             string    `json:"remark"`
	ImpersonationMode  string    `json:"impersonation_mode"` // passthrough | preset | custom
	PresetID           *int64    `json:"preset_id"`
	CustomHeadersJSON  string    `json:"custom_headers_json"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ClientPreset struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	VersionLabel  string    `json:"version_label"`
	HeadersJSON   string    `json:"headers_json"`
	RemoveHeaders string    `json:"remove_headers"` // JSON array
	Builtin       bool      `json:"builtin"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RequestMeta struct {
	ID                int64     `json:"id"`
	RequestID         string    `json:"request_id"`
	CreatedAt         time.Time `json:"created_at"`
	UserKeyID         *int64    `json:"user_key_id"`
	ClientModel       string    `json:"client_model"`
	UpstreamModel     string    `json:"upstream_model"`
	ChannelID         *int64    `json:"channel_id"`
	Method            string    `json:"method"`
	Path              string    `json:"path"`
	StatusCode        int       `json:"status_code"`
	TTFBms            int64     `json:"ttfb_ms"`
	TotalMs           int64     `json:"total_ms"`
	ErrorSummary      string    `json:"error_summary"`
	ImpersonationMode string    `json:"impersonation_mode"`
}

type KeyStatsDaily struct {
	UserKeyID int64  `json:"user_key_id"`
	Day       string `json:"day"` // YYYY-MM-DD
	Requests  int64  `json:"requests"`
	Errors    int64  `json:"errors"`
}
