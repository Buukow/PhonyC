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
	for _, name := range []string{"codex-tui", "codex-enhanced", "claude-cli", "claude-enhanced"} {
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
}

func TestEnsureBuiltinPresetsRefreshesEnhancedRulesOnly(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := EnsureBuiltinPresets(st); err != nil {
		t.Fatal(err)
	}
	base, _ := st.GetPresetByName("codex-tui")
	enhanced, _ := st.GetPresetByName("codex-enhanced")
	if _, err := st.UpdatePreset(base.ID, store.PresetInput{RuleJSON: `{"legacy":"keep"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdatePreset(enhanced.ID, store.PresetInput{RuleJSON: `{"legacy":"replace"}`}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureBuiltinPresets(st); err != nil {
		t.Fatal(err)
	}
	base, _ = st.GetPresetByName("codex-tui")
	enhanced, _ = st.GetPresetByName("codex-enhanced")
	if base.RuleJSON != `{"legacy":"keep"}` {
		t.Fatalf("base preset was unexpectedly refreshed: %s", base.RuleJSON)
	}
	if !strings.Contains(enhanced.RuleJSON, "time_number:unix_ms") {
		t.Fatalf("enhanced preset was not refreshed: %s", enhanced.RuleJSON)
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

func TestEnhancedPresetsFillOnlyMissingHeaders(t *testing.T) {
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
			if resolved.Get(tc.header) != "client-owned-value" {
				t.Fatalf("client header was overwritten: %q", resolved.Get(tc.header))
			}
			if strings.TrimSpace(resolved.Get("User-Agent")) == "" {
				t.Fatal("missing user agent completion")
			}
		})
	}
}
