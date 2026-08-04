package seed

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/phonyg/phonyg/internal/preset"
	"github.com/phonyg/phonyg/internal/store"
)

func TestEnsureBuiltinPresetsCreatesFourVariants(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := EnsureBuiltinPresets(st); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Codex 基础", "Codex 增强", "Claude 基础", "Claude 增强"} {
		item, err := st.GetPresetByName(name)
		if err != nil {
			t.Fatalf("missing preset %s: %v", name, err)
		}
		if !item.Builtin {
			t.Fatalf("preset %s is not builtin", name)
		}
		if _, err := preset.Parse(item.RuleJSON); err != nil {
			t.Fatalf("preset %s has invalid rule: %v", name, err)
		}
	}
	codexEnhanced, _ := st.GetPresetByName("Codex 增强")
	claudeEnhanced, _ := st.GetPresetByName("Claude 增强")
	for _, item := range []*store.ClientPreset{codexEnhanced, claudeEnhanced} {
		if item.Description != "在基础版上增加动态会话信息，更真实但可能会出现问题" {
			t.Fatalf("unexpected enhanced description: %q", item.Description)
		}
	}
	if got := st.GetSettingOr(store.SettingAutoTestPrompt, ""); got != "什么是codex？" {
		t.Fatalf("unexpected default auto-test prompt: %q", got)
	}
}

func TestEnsureBuiltinPresetsRefreshesAllBuiltinRules(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := EnsureBuiltinPresets(st); err != nil {
		t.Fatal(err)
	}
	base, _ := st.GetPresetByName("Codex 基础")
	enhanced, _ := st.GetPresetByName("Codex 增强")
	if _, err := st.UpdatePreset(base.ID, store.PresetInput{RuleJSON: `{"legacy":"keep"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdatePreset(enhanced.ID, store.PresetInput{RuleJSON: `{"legacy":"replace"}`}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureBuiltinPresets(st); err != nil {
		t.Fatal(err)
	}
	base, _ = st.GetPresetByName("Codex 基础")
	enhanced, _ = st.GetPresetByName("Codex 增强")
	if base.RuleJSON == `{"legacy":"keep"}` {
		t.Fatalf("base preset was not refreshed: %s", base.RuleJSON)
	}
	if !strings.Contains(enhanced.RuleJSON, "time_number:unix_ms") {
		t.Fatalf("enhanced preset was not refreshed: %s", enhanced.RuleJSON)
	}
}

func TestBasicPresetsUseCapturedFingerprintsAndForceOverride(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := EnsureBuiltinPresets(st); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name       string
		originator string
		userAgent  string
	}{
		{name: "Codex 基础", originator: "codex_exec", userAgent: "codex_exec/0.145.0 (Debian 12.0.0; x86_64) dumb (codex_exec; 0.145.0)"},
		{name: "Claude 基础", userAgent: "claude-cli/2.1.220 (external, sdk-cli)"},
	}
	for _, tc := range checks {
		item, err := st.GetPresetByName(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := preset.Parse(item.RuleJSON)
		if err != nil {
			t.Fatal(err)
		}
		resolved, _, err := (preset.Resolver{Generators: preset.NewGeneratorManager()}).Resolve(item.ID, item.VersionLabel, doc, http.Header{}, http.Header{}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Get("User-Agent") != tc.userAgent {
			t.Fatalf("%s user agent=%q", tc.name, resolved.Get("User-Agent"))
		}
		if tc.originator != "" && resolved.Get("Originator") != tc.originator {
			t.Fatalf("%s originator=%q", tc.name, resolved.Get("Originator"))
		}
		client := http.Header{"User-Agent": []string{"client-owned-agent"}}
		resolved, _, err = (preset.Resolver{Generators: preset.NewGeneratorManager()}).Resolve(item.ID, item.VersionLabel, doc, client, http.Header{}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Get("User-Agent") != tc.userAgent {
			t.Fatalf("%s did not force user agent: %q", tc.name, resolved.Get("User-Agent"))
		}
	}
}

func TestCodexEnhancedPresetResolvesConsistentIdentity(t *testing.T) {
	in := codexEnhancedPreset()
	doc, err := preset.Parse(in.RuleJSON)
	if err != nil {
		t.Fatal(err)
	}
	manager := preset.NewGeneratorManager()
	resolved, _, err := (preset.Resolver{Generators: manager}).Resolve(41, in.VersionLabel, doc, http.Header{}, http.Header{}, time.UnixMilli(1785500416427))
	if err != nil {
		t.Fatal(err)
	}
	sessionID := resolved.Get("Session-Id")
	if uuid.MustParse(sessionID).Version() != 7 {
		t.Fatalf("session id is not UUID v7: %s", sessionID)
	}
	for _, name := range []string{"Thread-Id", "X-Client-Request-Id"} {
		if resolved.Get(name) != sessionID {
			t.Fatalf("%s does not share session id", name)
		}
	}
	if resolved.Get("X-Codex-Window-Id") != sessionID+":0" {
		t.Fatalf("unexpected window id %q", resolved.Get("X-Codex-Window-Id"))
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(resolved.Get("X-Codex-Turn-Metadata")), &metadata); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"session_id", "thread_id"} {
		if metadata[name] != sessionID {
			t.Fatalf("metadata %s mismatch: %v", name, metadata[name])
		}
	}
	if metadata["window_id"] != sessionID+":0" || metadata["turn_started_at_unix_ms"] != float64(1785500416427) {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if uuid.MustParse(metadata["turn_id"].(string)).Version() != 7 {
		t.Fatalf("turn id is not UUID v7: %v", metadata["turn_id"])
	}
}

func TestEnhancedPresetsForceOverrideHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  store.PresetInput
		header string
	}{
		{name: "codex", input: codexEnhancedPreset(), header: "Session-Id"},
		{name: "claude", input: claudeEnhancedPreset(), header: "X-Claude-Code-Session-Id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := preset.Parse(tc.input.RuleJSON)
			if err != nil {
				t.Fatal(err)
			}
			client := http.Header{tc.header: []string{"client-owned-value"}}
			resolved, _, err := (preset.Resolver{Generators: preset.NewGeneratorManager()}).Resolve(42, tc.input.VersionLabel, doc, client, http.Header{}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Get(tc.header) == "client-owned-value" {
				t.Fatalf("client header was not overwritten: %q", resolved.Get(tc.header))
			}
			if strings.TrimSpace(resolved.Get("User-Agent")) == "" {
				t.Fatal("missing user agent completion")
			}
		})
	}
}
