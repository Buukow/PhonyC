package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/phonyg/phonyg/internal/capture"
	"github.com/phonyg/phonyg/internal/seed"
	"github.com/phonyg/phonyg/internal/snapshot"
	"github.com/phonyg/phonyg/internal/store"
)

func setupTest(t *testing.T) (*Handler, *store.Store, *snapshot.Manager, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	_ = seed.EnsureBuiltinPresets(st)
	en := true
	pri := 10
	ch, err := st.CreateChannel(store.ChannelInput{
		Name: "mock", Enabled: &en, Protocol: "openai", BaseURL: "http://127.0.0.1:9", APIKey: "up-key", Priority: &pri,
	})
	if err != nil {
		t.Fatal(err)
	}
	rw := false
	_, err = st.CreateChannelModel(ch.ID, store.ChannelModelInput{ClientModel: "test-model", UpstreamModel: "up-model", RewriteModel: &rw, Enabled: &en})
	if err != nil {
		t.Fatal(err)
	}
	k, err := st.CreateUserKey(store.UserKeyInput{Name: "k1", Key: "sk-test-1", Enabled: &en, ImpersonationMode: store.ImpersonationModePassthrough})
	if err != nil {
		t.Fatal(err)
	}
	_ = k
	snap := snapshot.NewManager(st)
	if err := snap.Reload(); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(snap, st, nil, 1<<20)
	return h, st, snap, dir
}

func TestAuthAndModelRequired(t *testing.T) {
	h, _, _, _ := setupTest(t)
	r := gin.New()
	r.Any("/v1/*path", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"test-model"}`))
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}

	// missing model
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Authorization", "Bearer sk-test-1")
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestModelsList(t *testing.T) {
	h, _, _, _ := setupTest(t)
	r := gin.New()
	r.Any("/v1/*path", h.Handle)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-test-1")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["object"] != "list" {
		t.Fatalf("resp=%v", resp)
	}
}

func TestProxyRewriteAndHeaders(t *testing.T) {
	// mock upstream
	var gotBody []byte
	var gotAuth string
	var gotUA string
	var gotAcceptEncoding string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotAcceptEncoding = r.Header.Get("Accept-Encoding")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer up.Close()

	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = seed.EnsureBuiltinPresets(st)
	en := true
	pri := 1
	ch, _ := st.CreateChannel(store.ChannelInput{Name: "u", Enabled: &en, Protocol: "openai", BaseURL: up.URL, APIKey: "UPSTREAMKEY", Priority: &pri})
	rw := true
	_, _ = st.CreateChannelModel(ch.ID, store.ChannelModelInput{ClientModel: "client-m", UpstreamModel: "upstream-m", RewriteModel: &rw, Enabled: &en})
	_, _ = st.CreateUserKey(store.UserKeyInput{Name: "k", Key: "sk-abc", Enabled: &en, ImpersonationMode: store.ImpersonationModePassthrough})
	snap := snapshot.NewManager(st)
	_ = snap.Reload()
	h := NewHandler(snap, st, nil, 1<<20)
	r := gin.New()
	r.Any("/v1/*path", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"client-m","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer sk-abc")
	req.Header.Set("User-Agent", "my-sdk/1.0")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer UPSTREAMKEY" {
		t.Fatalf("auth=%q", gotAuth)
	}
	if gotUA != "my-sdk/1.0" {
		t.Fatalf("ua=%q", gotUA)
	}
	if gotAcceptEncoding != "" {
		t.Fatalf("unexpected accept-encoding=%q", gotAcceptEncoding)
	}
	if !strings.Contains(string(gotBody), `"model":"upstream-m"`) {
		t.Fatalf("body=%s", gotBody)
	}
	if strings.Contains(string(gotBody), "client-m") {
		t.Fatalf("client model should be rewritten: %s", gotBody)
	}
}

func TestDisabledKey(t *testing.T) {
	h, st, snap, _ := setupTest(t)
	en := false
	_, _ = st.UpdateUserKey(1, store.UserKeyInput{Enabled: &en})
	_ = snap.Reload()
	r := gin.New()
	r.Any("/v1/*path", h.Handle)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"test-model"}`))
	req.Header.Set("Authorization", "Bearer sk-test-1")
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("code=%d", w.Code)
	}
}

var _ = os.TempDir

func TestCaptureAnyModelSuccessShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = seed.EnsureBuiltinPresets(st)
	_ = st.SetSetting(store.SettingHeaderCaptureEnabled, "true")
	_ = st.SetSetting(store.SettingHeaderCaptureArmed, "true")
	_ = st.SetSetting(store.SettingHeaderCaptureKey, "sk-capture-test")
	snap := snapshot.NewManager(st)
	_ = snap.Reload()
	capMgr := capture.New(st)
	h := NewHandler(snap, st, capMgr, 1<<20)
	r := gin.New()
	r.Any("/v1/*path", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"totally-unknown-model-xyz","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer sk-capture-test")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Codex-Test/1.0")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["object"] != "chat.completion" {
		t.Fatalf("want chat.completion got %v body=%s", resp["object"], w.Body.String())
	}
	if resp["model"] != "totally-unknown-model-xyz" {
		t.Fatalf("model=%v", resp["model"])
	}
	if resp["captured"] != true {
		t.Fatalf("captured=%v", resp["captured"])
	}
	choices, _ := resp["choices"].([]any)
	if len(choices) == 0 {
		t.Fatalf("no choices: %v", resp)
	}
	var logs []store.RequestMeta
	var total int
	for i := 0; i < 100; i++ {
		logs, total, err = st.ListRequestMeta(store.LogFilter{Path: "/v1/chat/completions", Limit: 10})
		if err == nil && total == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("list request metadata: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("request metadata rows: total=%d rows=%d", total, len(logs))
	}
	meta := logs[0]
	if meta.StatusCode != 200 || meta.ErrorSummary != "" ||
		meta.ClientModel != "totally-unknown-model-xyz" || meta.UpstreamModel != "" ||
		meta.ChannelID != nil || meta.UserKeyID != nil || meta.Method != "POST" ||
		meta.ImpersonationMode != store.ImpersonationModePassthrough {
		t.Fatalf("unexpected capture metadata: %+v", meta)
	}
}

func TestAutoRetryAndLiveTempDisable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var hits int
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":"busy"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true,"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer up.Close()

	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = seed.EnsureBuiltinPresets(st)
	en := true
	pri1, pri2 := 10, 5
	ch1, _ := st.CreateChannel(store.ChannelInput{Name: "c1", Enabled: &en, Protocol: "openai", BaseURL: up.URL, APIKey: "k1", Priority: &pri1})
	ch2, _ := st.CreateChannel(store.ChannelInput{Name: "c2", Enabled: &en, Protocol: "openai", BaseURL: up.URL, APIKey: "k2", Priority: &pri2})
	rw := false
	_, _ = st.CreateChannelModel(ch1.ID, store.ChannelModelInput{ClientModel: "m1", UpstreamModel: "m1", RewriteModel: &rw, Enabled: &en})
	_, _ = st.CreateChannelModel(ch2.ID, store.ChannelModelInput{ClientModel: "m1", UpstreamModel: "m1", RewriteModel: &rw, Enabled: &en})
	_, _ = st.CreateUserKey(store.UserKeyInput{Name: "k", Key: "sk-retry", Enabled: &en, ImpersonationMode: store.ImpersonationModePassthrough})
	_ = st.SetSetting(store.SettingAutoRetryEnabled, "true")
	_ = st.SetSetting(store.SettingAutoRetryMax, "2")
	_ = st.SetSetting(store.SettingAutoRetryStatusCodes, "503")
	_ = st.SetSetting(store.SettingAutoTestDisableCodes, "503")

	snap := snapshot.NewManager(st)
	_ = snap.Reload()
	h := NewHandler(snap, st, nil, 1<<20)
	r := gin.New()
	r.Any("/v1/*path", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer sk-retry")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s hits=%d", w.Code, w.Body.String(), hits)
	}
	if hits < 2 {
		t.Fatalf("expected retry hits>=2 got %d", hits)
	}
	c1, _ := st.GetChannel(ch1.ID)
	if !c1.TempDisabled {
		// first channel was higher priority and returned 503, should be temp disabled
		// Depending on random within same priority — priorities differ so ch1 first.
		t.Fatalf("channel1 should be temp disabled, got %+v", c1)
	}
}
