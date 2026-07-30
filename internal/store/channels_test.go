package store

import (
	"path/filepath"
	"testing"
)

func TestUpdateChannelNormalizesManualState(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	enabled := true
	ch, err := st.CreateChannel(ChannelInput{Name: "channel", Enabled: &enabled, Protocol: "openai", BaseURL: "http://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetChannelTempDisabled(ch.ID, true); err != nil {
		t.Fatal(err)
	}

	disabled := false
	ch, err = st.UpdateChannel(ch.ID, ChannelInput{Enabled: &disabled})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Enabled || ch.TempDisabled {
		t.Fatalf("manual disable must clear temporary state: %+v", ch)
	}

	tempDisabled := true
	ch, err = st.UpdateChannel(ch.ID, ChannelInput{TempDisabled: &tempDisabled})
	if err != nil {
		t.Fatal(err)
	}
	if ch.TempDisabled {
		t.Fatalf("disabled channel must not acquire temporary state: %+v", ch)
	}

	clearTemp := false
	ch, err = st.UpdateChannel(ch.ID, ChannelInput{Enabled: &enabled, TempDisabled: &clearTemp})
	if err != nil {
		t.Fatal(err)
	}
	if !ch.Enabled || ch.TempDisabled {
		t.Fatalf("manual enable must produce normal enabled state: %+v", ch)
	}
}
