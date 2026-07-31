package store

import (
	"path/filepath"
	"testing"
)

func TestCreatePresetPersistsStructuredRuleJSON(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	rule := `{"schema_version":1,"headers":{"X-Nested":{"value":{"a":{"b":"c"}},"fill_missing":true}},"remove_headers":[],"generators":{}}`
	p, err := st.CreatePreset(PresetInput{
		Name:          "structured-new",
		VersionLabel:  "1.0",
		HeadersJSON:   "{}",
		RemoveHeaders: "[]",
		RuleJSON:      rule,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.RuleJSON != rule {
		t.Fatalf("create response lost rule_json: %q", p.RuleJSON)
	}
	got, err := st.GetPreset(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RuleJSON != rule {
		t.Fatalf("stored rule_json mismatch: %q", got.RuleJSON)
	}
}
