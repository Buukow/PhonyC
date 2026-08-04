package store

import (
	"path/filepath"
	"testing"
)

func TestCreateUserKeyDefaultsToChinesePassthroughMode(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "keys.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	enabled := true
	key, err := st.CreateUserKey(UserKeyInput{Name: "test", Key: "sk-test", Enabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	if key.ImpersonationMode != ImpersonationModePassthrough {
		t.Fatalf("mode=%q", key.ImpersonationMode)
	}
}
