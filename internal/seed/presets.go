package seed

import (
	"encoding/json"

	"github.com/phonyc/phonyc/internal/store"
)

func EnsureBuiltinPresets(st *store.Store) error {
	codexHeaders := map[string]string{
		"User-Agent":            "codex-tui/{{version}} (Debian 12.0.0; x86_64) xterm-256color (codex-tui; {{version}})",
		"Originator":            "codex-tui",
		"Accept":                "text/event-stream",
		"Content-Type":          "application/json",
		"X-Codex-Beta-Features": "remote_compaction_v2",
	}
	claudeHeaders := map[string]string{
		"User-Agent":            "claude-cli/{{version}} (external, cli)",
		"X-App":                 "cli",
		"Anthropic-Version":     "2023-06-01",
		"Anthropic-Beta":        "claude-code-20250219,interleaved-thinking-2025-05-14,redact-thinking-2026-02-12,thinking-token-count-2026-05-13,context-management-2025-06-27,prompt-caching-scope-2026-01-05,mid-conversation-system-2026-04-07,effort-2025-11-24",
		"Content-Type":          "application/json",
		"Accept":                "application/json",
		"X-Stainless-Arch":      "x64",
		"X-Stainless-Lang":      "js",
		"X-Stainless-Os":        "Linux",
		"X-Stainless-Package-Version": "0.94.0",
		"X-Stainless-Runtime":   "node",
		"X-Stainless-Runtime-Version": "v26.3.0",
		"X-Stainless-Retry-Count": "0",
		"X-Stainless-Timeout":   "600",
	}
	ch, _ := json.Marshal(codexHeaders)
	cl, _ := json.Marshal(claudeHeaders)
	if _, err := st.UpsertBuiltinPreset(store.PresetInput{
		Name: "codex-tui", Description: "OpenAI Codex TUI client fingerprint",
		VersionLabel: "0.145.0", HeadersJSON: string(ch), RemoveHeaders: "[]", Builtin: true,
	}); err != nil {
		return err
	}
	if _, err := st.UpsertBuiltinPreset(store.PresetInput{
		Name: "claude-cli", Description: "Anthropic Claude Code CLI fingerprint",
		VersionLabel: "2.1.220", HeadersJSON: string(cl), RemoveHeaders: "[]", Builtin: true,
	}); err != nil {
		return err
	}
	// default settings
	if _, err := st.GetSetting("log_retention_days"); err != nil {
		_ = st.SetSetting("log_retention_days", "30")
	}
	return nil
}
