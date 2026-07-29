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

	"github.com/gin-gonic/gin"
	"github.com/phonyc/phonyc/internal/seed"
	"github.com/phonyc/phonyc/internal/snapshot"
	"github.com/phonyc/phonyc/internal/store"
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
	k, err := st.CreateUserKey(store.UserKeyInput{Name: "k1", Key: "sk-test-1", Enabled: &en, ImpersonationMode: "passthrough"})
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
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
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
	_, _ = st.CreateUserKey(store.UserKeyInput{Name: "k", Key: "sk-abc", Enabled: &en, ImpersonationMode: "passthrough"})
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
