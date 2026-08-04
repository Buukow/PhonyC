package seed

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"sort"

	"github.com/phonyg/phonyg/internal/healthcheck"
	"github.com/phonyg/phonyg/internal/preset"
	"github.com/phonyg/phonyg/internal/store"
)

func EnsureBuiltinPresets(st *store.Store) error {
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
	codexBasic := basicPreset("Codex 基础", "模拟 Codex 客户端的基础请求头", "0.145.0", codexHeaders)
	claudeBasic := basicPreset("Claude 基础", "模拟 Claude Code 客户端的基础请求头", "2.1.220", claudeHeaders)
	builtinPresets := []store.PresetInput{codexBasic, claudeBasic, codexEnhancedPreset(), claudeEnhancedPreset()}
	for _, item := range builtinPresets {
		if err := ensureBuiltinPreset(st, item); err != nil {
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

func ensureBuiltinPreset(st *store.Store, in store.PresetInput) error {
	current, err := st.GetPresetByName(in.Name)
	if err == nil {
		if current.Builtin {
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
		doc.Headers[header] = preset.HeaderRule{Value: value, FillMissing: false}
	}
	raw, _ := json.Marshal(headers)
	return store.PresetInput{Name: name, Description: description, VersionLabel: version, HeadersJSON: string(raw), RemoveHeaders: "[]", RuleJSON: preset.Marshal(doc), Builtin: true}
}

func codexEnhancedPreset() store.PresetInput {
	force := func(value any) preset.HeaderRule { return preset.HeaderRule{Value: value, FillMissing: false} }
	doc := preset.Document{
		SchemaVersion: preset.SchemaVersion,
		Headers: map[string]preset.HeaderRule{
			"Accept":                force("text/event-stream"),
			"Content-Type":          force("application/json"),
			"Originator":            force("codex_exec"),
			"Session-Id":            force("{{generator:session_id}}"),
			"Thread-Id":             force("{{generator:session_id}}"),
			"User-Agent":            force("codex_exec/{{version}} (Debian 12.0.0; x86_64) dumb (codex_exec; {{version}})"),
			"X-Client-Request-Id":   force("{{generator:session_id}}"),
			"X-Codex-Beta-Features": force("remote_compaction_v2"),
			"X-Codex-Window-Id":     force("{{generator:session_id}}:0"),
			"X-Codex-Turn-Metadata": force(map[string]any{
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
		Name: "Codex 增强", Description: "在基础版上增加动态会话信息，更真实但可能会出现问题",
		VersionLabel: "0.145.0", HeadersJSON: "{}", RemoveHeaders: "[]", RuleJSON: preset.Marshal(doc), Builtin: true,
	}
}

func claudeEnhancedPreset() store.PresetInput {
	force := func(value any) preset.HeaderRule { return preset.HeaderRule{Value: value, FillMissing: false} }
	doc := preset.Document{
		SchemaVersion: preset.SchemaVersion,
		Headers: map[string]preset.HeaderRule{
			"Accept":         force("application/json"),
			"Anthropic-Beta": force("claude-code-20250219,interleaved-thinking-2025-05-14,thinking-token-count-2026-05-13,context-management-2025-06-27,prompt-caching-scope-2026-01-05,mid-conversation-system-2026-04-07,effort-2025-11-24"),
			"Anthropic-Dangerous-Direct-Browser-Access": force("true"),
			"Anthropic-Version":                         force("2023-06-01"),
			"Content-Type":                              force("application/json"),
			"User-Agent":                                force("claude-cli/{{version}} (external, sdk-cli)"),
			"X-App":                                     force("cli"),
			"X-Claude-Code-Session-Id":                  force("{{generator:session_id}}"),
			"X-Stainless-Arch":                          force("x64"),
			"X-Stainless-Lang":                          force("js"),
			"X-Stainless-Os":                            force("Linux"),
			"X-Stainless-Package-Version":               force("0.94.0"),
			"X-Stainless-Retry-Count":                   force("0"),
			"X-Stainless-Runtime":                       force("node"),
			"X-Stainless-Runtime-Version":               force("v26.3.0"),
			"X-Stainless-Timeout":                       force("600"),
		},
		RemoveHeaders: []string{},
		Generators: map[string]preset.GeneratorRule{
			"session_id": {Type: "uuid", Version: 4, Mode: "interval", Interval: "30m"},
		},
	}
	return store.PresetInput{
		Name: "Claude 增强", Description: "在基础版上增加动态会话信息，更真实但可能会出现问题",
		VersionLabel: "2.1.220", HeadersJSON: "{}", RemoveHeaders: "[]", RuleJSON: preset.Marshal(doc), Builtin: true,
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
