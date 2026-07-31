package seed

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"reflect"
	"sort"

	"github.com/phonyg/phonyg/internal/healthcheck"
	"github.com/phonyg/phonyg/internal/preset"
	"github.com/phonyg/phonyg/internal/store"
)

func EnsureBuiltinPresets(st *store.Store) error {
	legacyCodexHeaders := map[string]string{
		"User-Agent":            "codex-tui/{{version}} (Debian 12.0.0; x86_64) xterm-256color (codex-tui; {{version}})",
		"Originator":            "codex-tui",
		"Accept":                "text/event-stream",
		"Content-Type":          "application/json",
		"X-Codex-Beta-Features": "remote_compaction_v2",
	}
	legacyClaudeHeaders := map[string]string{
		"User-Agent":                  "claude-cli/{{version}} (external, cli)",
		"X-App":                       "cli",
		"Anthropic-Version":           "2023-06-01",
		"Anthropic-Beta":              "claude-code-20250219,interleaved-thinking-2025-05-14,redact-thinking-2026-02-12,thinking-token-count-2026-05-13,context-management-2025-06-27,prompt-caching-scope-2026-01-05,mid-conversation-system-2026-04-07,effort-2025-11-24",
		"Content-Type":                "application/json",
		"Accept":                      "application/json",
		"X-Stainless-Arch":            "x64",
		"X-Stainless-Lang":            "js",
		"X-Stainless-Os":              "Linux",
		"X-Stainless-Package-Version": "0.94.0",
		"X-Stainless-Runtime":         "node",
		"X-Stainless-Runtime-Version": "v26.3.0",
		"X-Stainless-Retry-Count":     "0",
		"X-Stainless-Timeout":         "600",
	}
	codexHeaders := map[string]string{
		"User-Agent":            "codex_exec/{{version}} (Debian 12.0.0; x86_64) dumb (codex_exec; {{version}})",
		"Originator":            "codex_exec",
		"Accept":                "text/event-stream",
		"Content-Type":          "application/json",
		"X-Codex-Beta-Features": "remote_compaction_v2",
	}
	claudeHeaders := map[string]string{
		"User-Agent":                  "claude-cli/{{version}} (external, sdk-cli)",
		"X-App":                       "cli",
		"Anthropic-Version":           "2023-06-01",
		"Anthropic-Beta":              "claude-code-20250219,interleaved-thinking-2025-05-14,thinking-token-count-2026-05-13,context-management-2025-06-27,prompt-caching-scope-2026-01-05,mid-conversation-system-2026-04-07,effort-2025-11-24",
		"Content-Type":                "application/json",
		"Accept":                      "application/json",
		"X-Stainless-Arch":            "x64",
		"X-Stainless-Lang":            "js",
		"X-Stainless-Os":              "Linux",
		"X-Stainless-Package-Version": "0.94.0",
		"X-Stainless-Runtime":         "node",
		"X-Stainless-Runtime-Version": "v26.3.0",
		"X-Stainless-Retry-Count":     "0",
		"X-Stainless-Timeout":         "600",
	}
	codexBasic := basicPreset("codex-tui", "OpenAI Codex CLI basic fingerprint", "0.145.0", codexHeaders)
	claudeBasic := basicPreset("claude-cli", "Anthropic Claude Code CLI basic fingerprint", "2.1.220", claudeHeaders)
	legacyCodex := legacyBuiltinPreset("codex-tui", "OpenAI Codex TUI client fingerprint", "0.145.0", legacyCodexHeaders)
	legacyClaude := legacyBuiltinPreset("claude-cli", "Anthropic Claude Code CLI fingerprint", "2.1.220", legacyClaudeHeaders)
	builtinPresets := []struct {
		input  store.PresetInput
		legacy *store.PresetInput
		force  bool
	}{{input: codexBasic, legacy: &legacyCodex}, {input: claudeBasic, legacy: &legacyClaude}, {input: codexEnhancedPreset(), force: true}, {input: claudeEnhancedPreset(), force: true}}
	for _, item := range builtinPresets {
		if err := ensureBuiltinPreset(st, item.input, item.legacy, item.force); err != nil {
			return err
		}
	}
	defaults := map[string]string{
		"log_retention_days":              "30",
		store.SettingAutoTestEnabled:      "false",
		store.SettingAutoTestIntervalMin:  "10",
		store.SettingAutoTestRandomOffset: "0",
		store.SettingAutoTestPrompt:       "hi",
		store.SettingAutoTestModel:        "",
		store.SettingAutoTestDisableCodes: "401,403,404,503",
		store.SettingAutoTestEnhanced:     "false",
		store.SettingAutoTestLexicon:      healthcheck.DefaultEnhancedLexiconJSON(),
		store.SettingHeaderCaptureEnabled: "false",
		store.SettingHeaderCaptureArmed:   "false",
		store.SettingHeaderCapturePayload: "",
	}
	for k, v := range defaults {
		if _, err := st.GetSetting(k); err != nil {
			_ = st.SetSetting(k, v)
		}
	}
	if raw, err := st.GetSetting(store.SettingAutoTestLexicon); err == nil {
		normalized, changes, normalizeErr := healthcheck.NormalizeEnhancedLexiconJSON(raw)
		if normalizeErr != nil {
			log.Printf("enhanced healthcheck lexicon migration skipped: %v", normalizeErr)
		} else if changes.Changed() {
			sort.Strings(changes.Added)
			sort.Strings(changes.Removed)
			if err := st.SetSetting(store.SettingAutoTestLexicon, normalized); err != nil {
				return err
			}
			log.Printf("enhanced healthcheck lexicon migrated: added=%v removed=%v schema_version_changed=%t", changes.Added, changes.Removed, changes.SchemaVersionChanged)
		}
	}
	if key, err := st.GetSetting(store.SettingHeaderCaptureKey); err != nil || key == "" {
		_ = st.SetSetting(store.SettingHeaderCaptureKey, "sk-phonyg-capture-"+randomHex(12))
	}
	if presets, err := st.ListPresets(); err == nil {
		for _, item := range presets {
			if item.RuleJSON != "" {
				continue
			}
			doc, migrateErr := preset.LegacyDocument(item.HeadersJSON, item.RemoveHeaders)
			if migrateErr != nil {
				log.Printf("client preset %d legacy migration skipped: %v", item.ID, migrateErr)
				continue
			}
			if _, updateErr := st.UpdatePreset(item.ID, store.PresetInput{RuleJSON: preset.Marshal(doc)}); updateErr != nil {
				return updateErr
			}
		}
	}
	return nil
}

func ensureBuiltinPreset(st *store.Store, in store.PresetInput, legacy *store.PresetInput, force bool) error {
	current, err := st.GetPresetByName(in.Name)
	if err == nil {
		if current.Builtin && (force || (legacy != nil && builtinPresetMatches(current, *legacy))) {
			_, err = st.UpdatePreset(current.ID, in)
		}
		return err
	}
	if err != store.ErrNotFound {
		return err
	}
	_, err = st.UpsertBuiltinPreset(in)
	return err
}

func basicPreset(name, description, version string, headers map[string]string) store.PresetInput {
	doc := preset.Document{SchemaVersion: preset.SchemaVersion, Headers: map[string]preset.HeaderRule{}, RemoveHeaders: []string{}, Generators: map[string]preset.GeneratorRule{}}
	for header, value := range headers {
		doc.Headers[header] = preset.HeaderRule{Value: value, FillMissing: true}
	}
	raw, _ := json.Marshal(headers)
	return store.PresetInput{Name: name, Description: description, VersionLabel: version, HeadersJSON: string(raw), RemoveHeaders: "[]", RuleJSON: preset.Marshal(doc), Builtin: true}
}

func legacyBuiltinPreset(name, description, version string, headers map[string]string) store.PresetInput {
	raw, _ := json.Marshal(headers)
	doc, _ := preset.LegacyDocument(string(raw), "[]")
	return store.PresetInput{Name: name, Description: description, VersionLabel: version, HeadersJSON: string(raw), RemoveHeaders: "[]", RuleJSON: preset.Marshal(doc), Builtin: true}
}

func builtinPresetMatches(current *store.ClientPreset, legacy store.PresetInput) bool {
	if current.Description != legacy.Description || current.VersionLabel != legacy.VersionLabel || current.RemoveHeaders != legacy.RemoveHeaders {
		return false
	}
	var currentHeaders, legacyHeaders map[string]string
	if json.Unmarshal([]byte(current.HeadersJSON), &currentHeaders) != nil ||
		json.Unmarshal([]byte(legacy.HeadersJSON), &legacyHeaders) != nil || !reflect.DeepEqual(currentHeaders, legacyHeaders) {
		return false
	}
	if current.RuleJSON != "" {
		currentDoc, currentErr := preset.Parse(current.RuleJSON)
		legacyDoc, legacyErr := preset.Parse(legacy.RuleJSON)
		return currentErr == nil && legacyErr == nil && reflect.DeepEqual(currentDoc, legacyDoc)
	}
	return true
}

func codexEnhancedPreset() store.PresetInput {
	fill := func(value any) preset.HeaderRule { return preset.HeaderRule{Value: value, FillMissing: true} }
	doc := preset.Document{
		SchemaVersion: preset.SchemaVersion,
		Headers: map[string]preset.HeaderRule{
			"Accept":                fill("text/event-stream"),
			"Content-Type":          fill("application/json"),
			"Originator":            fill("codex_exec"),
			"Session-Id":            fill("{{generator:session_id}}"),
			"Thread-Id":             fill("{{generator:session_id}}"),
			"User-Agent":            fill("codex_exec/{{version}} (Debian 12.0.0; x86_64) dumb (codex_exec; {{version}})"),
			"X-Client-Request-Id":   fill("{{generator:session_id}}"),
			"X-Codex-Beta-Features": fill("remote_compaction_v2"),
			"X-Codex-Window-Id":     fill("{{generator:session_id}}:0"),
			"X-Codex-Turn-Metadata": fill(map[string]any{
				"installation_id":         "{{generator:installation_id}}",
				"session_id":              "{{generator:session_id}}",
				"thread_id":               "{{generator:session_id}}",
				"turn_id":                 "{{generator:turn_id}}",
				"window_id":               "{{generator:session_id}}:0",
				"request_kind":            "turn",
				"thread_source":           "user",
				"sandbox":                 "seccomp",
				"workspaces":              map[string]any{"/workspace": map[string]any{"has_changes": false}},
				"turn_started_at_unix_ms": "{{time_number:unix_ms}}",
			}),
		},
		RemoveHeaders: []string{},
		Generators: map[string]preset.GeneratorRule{
			"installation_id": {Type: "uuid", Version: 4, Mode: "fixed"},
			"session_id":      {Type: "uuid", Version: 7, Mode: "interval", Interval: "30m"},
			"turn_id":         {Type: "uuid", Version: 7, Mode: "request"},
		},
	}
	return store.PresetInput{
		Name: "codex-enhanced", Description: "OpenAI Codex CLI enhanced fingerprint with missing-header completion",
		VersionLabel: "0.145.0", HeadersJSON: "{}", RemoveHeaders: "[]", RuleJSON: preset.Marshal(doc), Builtin: true,
	}
}

func claudeEnhancedPreset() store.PresetInput {
	fill := func(value any) preset.HeaderRule { return preset.HeaderRule{Value: value, FillMissing: true} }
	doc := preset.Document{
		SchemaVersion: preset.SchemaVersion,
		Headers: map[string]preset.HeaderRule{
			"Accept":         fill("application/json"),
			"Anthropic-Beta": fill("claude-code-20250219,interleaved-thinking-2025-05-14,thinking-token-count-2026-05-13,context-management-2025-06-27,prompt-caching-scope-2026-01-05,mid-conversation-system-2026-04-07,effort-2025-11-24"),
			"Anthropic-Dangerous-Direct-Browser-Access": fill("true"),
			"Anthropic-Version":                         fill("2023-06-01"),
			"Content-Type":                              fill("application/json"),
			"User-Agent":                                fill("claude-cli/{{version}} (external, sdk-cli)"),
			"X-App":                                     fill("cli"),
			"X-Claude-Code-Session-Id":                  fill("{{generator:session_id}}"),
			"X-Stainless-Arch":                          fill("x64"),
			"X-Stainless-Lang":                          fill("js"),
			"X-Stainless-Os":                            fill("Linux"),
			"X-Stainless-Package-Version":               fill("0.94.0"),
			"X-Stainless-Retry-Count":                   fill("0"),
			"X-Stainless-Runtime":                       fill("node"),
			"X-Stainless-Runtime-Version":               fill("v26.3.0"),
			"X-Stainless-Timeout":                       fill("600"),
		},
		RemoveHeaders: []string{},
		Generators: map[string]preset.GeneratorRule{
			"session_id": {Type: "uuid", Version: 4, Mode: "interval", Interval: "30m"},
		},
	}
	return store.PresetInput{
		Name: "claude-enhanced", Description: "Anthropic Claude Code enhanced fingerprint with missing-header completion",
		VersionLabel: "2.1.220", HeadersJSON: "{}", RemoveHeaders: "[]", RuleJSON: preset.Marshal(doc), Builtin: true,
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
