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

func TestChannelHealthcheckPresetPersistsClearsAndNullsOnDelete(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	preset, err := st.CreatePreset(PresetInput{Name: "healthcheck", HeadersJSON: "{}", RemoveHeaders: "[]"})
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	channel, err := st.CreateChannel(ChannelInput{
		Name: "channel", Enabled: &enabled, Protocol: "openai", BaseURL: "http://example.test", HealthcheckPresetID: &preset.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if channel.HealthcheckPresetID == nil || *channel.HealthcheckPresetID != preset.ID {
		t.Fatalf("healthcheck preset was not stored: %+v", channel)
	}

	channel, err = st.UpdateChannel(channel.ID, ChannelInput{ClearHealthcheckPreset: true})
	if err != nil {
		t.Fatal(err)
	}
	if channel.HealthcheckPresetID != nil {
		t.Fatalf("healthcheck preset was not cleared: %+v", channel)
	}

	channel, err = st.UpdateChannel(channel.ID, ChannelInput{HealthcheckPresetID: &preset.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePreset(preset.ID); err != nil {
		t.Fatal(err)
	}
	channel, err = st.GetChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if channel.HealthcheckPresetID != nil {
		t.Fatalf("deleted preset did not null channel reference: %+v", channel)
	}
}
