package capture

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/phonyg/phonyg/internal/store"
)

func TestArmRotatesKeyAndClearsCapture(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "capture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetSetting(store.SettingHeaderCaptureKey, "sk-old-capture-key"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(store.SettingHeaderCapturePayload, `{"old":true}`); err != nil {
		t.Fatal(err)
	}

	m := New(st)
	if err := m.Arm(); err != nil {
		t.Fatal(err)
	}
	if m.Key() == "sk-old-capture-key" || !strings.HasPrefix(m.Key(), "sk-phonyg-capture-") {
		t.Fatalf("unexpected rotated key: %q", m.Key())
	}
	if !m.Armed() {
		t.Fatal("capture was not armed")
	}
	if _, err := m.GetCaptured(); err != store.ErrNotFound {
		t.Fatalf("captured payload was not cleared: %v", err)
	}
}
