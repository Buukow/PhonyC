package capture

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/phonyc/phonyc/internal/store"
)

type Manager struct {
	Store *store.Store
	mu    sync.Mutex
}

type CapturedRequest struct {
	CapturedAt time.Time         `json:"captured_at"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      string            `json:"query"`
	Headers    map[string]string `json:"headers"`
	Model      string            `json:"model,omitempty"`
}

// hop / transport / auth headers excluded from capture & preset save (3.2B)
var skipHeader = map[string]bool{
	"authorization": true, "x-api-key": true, "cookie": true, "host": true,
	"content-length": true, "connection": true, "keep-alive": true,
	"transfer-encoding": true, "te": true, "trailer": true, "upgrade": true,
	"proxy-connection": true, "proxy-authenticate": true, "proxy-authorization": true,
	"accept-encoding": true,
}

func New(st *store.Store) *Manager { return &Manager{Store: st} }

func (m *Manager) Enabled() bool {
	return m.Store.GetSettingBool(store.SettingHeaderCaptureEnabled, false)
}

func (m *Manager) Armed() bool {
	return m.Store.GetSettingBool(store.SettingHeaderCaptureArmed, false)
}

func (m *Manager) Key() string {
	return m.Store.GetSettingOr(store.SettingHeaderCaptureKey, "")
}

// IsCaptureKey reports whether key equals the system capture key (regardless of enabled).
func (m *Manager) IsCaptureKey(key string) bool {
	ck := m.Key()
	return ck != "" && key == ck
}

func FilterHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for k, vv := range h {
		if skipHeader[strings.ToLower(k)] {
			continue
		}
		if len(vv) > 0 {
			out[k] = vv[0]
		}
	}
	return out
}

// TryCapture records the first request when armed. Returns true if captured.
func (m *Manager) TryCapture(r *http.Request, model string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.Enabled() || !m.Armed() {
		return false
	}
	cap := CapturedRequest{
		CapturedAt: time.Now().UTC(),
		Method:     r.Method,
		Path:       r.URL.Path,
		Query:      r.URL.RawQuery,
		Headers:    FilterHeaders(r.Header),
		Model:      model,
	}
	b, err := json.Marshal(cap)
	if err != nil {
		return false
	}
	_ = m.Store.SetSetting(store.SettingHeaderCapturePayload, string(b))
	_ = m.Store.SetSetting(store.SettingHeaderCaptureArmed, "false")
	return true
}

func (m *Manager) GetCaptured() (*CapturedRequest, error) {
	raw, err := m.Store.GetSetting(store.SettingHeaderCapturePayload)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, store.ErrNotFound
	}
	var c CapturedRequest
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return nil, err
	}
	// re-filter in case older payloads contained hop headers
	cleaned := map[string]string{}
	for k, v := range c.Headers {
		if skipHeader[strings.ToLower(k)] {
			continue
		}
		cleaned[k] = v
	}
	c.Headers = cleaned
	return &c, nil
}

func (m *Manager) ClearCaptured() error {
	_ = m.Store.SetSetting(store.SettingHeaderCapturePayload, "")
	return nil
}

func (m *Manager) Arm() error {
	_ = m.Store.SetSetting(store.SettingHeaderCaptureArmed, "true")
	_ = m.Store.SetSetting(store.SettingHeaderCapturePayload, "")
	return nil
}

func (m *Manager) SetEnabled(on bool) error {
	v := "false"
	if on {
		v = "true"
	}
	if err := m.Store.SetSetting(store.SettingHeaderCaptureEnabled, v); err != nil {
		return err
	}
	if on {
		return m.Arm()
	}
	_ = m.Store.SetSetting(store.SettingHeaderCaptureArmed, "false")
	return nil
}
