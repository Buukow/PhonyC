package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/phonyc/phonyc/internal/snapshot"
	"github.com/phonyc/phonyc/internal/store"
)

func healthTestWorker(t *testing.T, status *int) (*Worker, *store.Store, int64) {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(*status)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(up.Close)

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	enabled := true
	ch, err := st.CreateChannel(store.ChannelInput{Name: "test", Enabled: &enabled, Protocol: "openai", BaseURL: up.URL})
	if err != nil {
		t.Fatal(err)
	}
	rewrite := false
	if _, err := st.CreateChannelModel(ch.ID, store.ChannelModelInput{ClientModel: "model", UpstreamModel: "model", RewriteModel: &rewrite, Enabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	snap := snapshot.NewManager(st)
	if err := snap.Reload(); err != nil {
		t.Fatal(err)
	}
	return New(st, snap), st, ch.ID
}

func TestManualHealthcheckStateTransitions(t *testing.T) {
	status := http.StatusServiceUnavailable
	w, st, id := healthTestWorker(t, &status)
	disabled := false
	if _, err := st.UpdateChannel(id, store.ChannelInput{Enabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	res, err := w.TestOne(id)
	if err != nil {
		t.Fatal(err)
	}
	ch, _ := st.GetChannel(id)
	if res.StatusCode != status || ch.Enabled || ch.TempDisabled {
		t.Fatalf("disabled manual test changed state: result=%+v channel=%+v", res, ch)
	}

	enabled := true
	if _, err := st.UpdateChannel(id, store.ChannelInput{Enabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.TestOne(id); err != nil {
		t.Fatal(err)
	}
	ch, _ = st.GetChannel(id)
	if !ch.Enabled || ch.TempDisabled {
		t.Fatalf("enabled manual failure changed state: %+v", ch)
	}

	if err := st.SetChannelTempDisabled(id, true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.TestOne(id); err != nil {
		t.Fatal(err)
	}
	ch, _ = st.GetChannel(id)
	if !ch.TempDisabled {
		t.Fatalf("failed manual test recovered channel: %+v", ch)
	}

	status = http.StatusOK
	res, err = w.TestOne(id)
	if err != nil {
		t.Fatal(err)
	}
	ch, _ = st.GetChannel(id)
	if res.Action != "recover" || ch.TempDisabled || !ch.Enabled {
		t.Fatalf("successful manual test did not recover: result=%+v channel=%+v", res, ch)
	}
}

func TestAutomaticHealthcheckUsesConfiguredDisableCodes(t *testing.T) {
	status := http.StatusTeapot
	w, st, id := healthTestWorker(t, &status)
	if err := st.SetSetting(store.SettingAutoTestDisableCodes, "418"); err != nil {
		t.Fatal(err)
	}
	w.runOnce(true)
	ch, _ := st.GetChannel(id)
	if !ch.TempDisabled {
		t.Fatalf("configured code did not temporarily disable: %+v", ch)
	}

	status = http.StatusBadGateway
	if err := st.SetChannelTempDisabled(id, false); err != nil {
		t.Fatal(err)
	}
	w.runOnce(true)
	ch, _ = st.GetChannel(id)
	if ch.TempDisabled {
		t.Fatalf("non-configured code temporarily disabled: %+v", ch)
	}

	if err := st.SetChannelTempDisabled(id, true); err != nil {
		t.Fatal(err)
	}
	status = http.StatusOK
	w.runOnce(true)
	ch, _ = st.GetChannel(id)
	if ch.TempDisabled {
		t.Fatalf("successful automatic check did not recover: %+v", ch)
	}

	disabled := false
	if _, err := st.UpdateChannel(id, store.ChannelInput{Enabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	status = http.StatusTeapot
	w.runOnce(true)
	ch, _ = st.GetChannel(id)
	if ch.Enabled || ch.TempDisabled {
		t.Fatalf("automatic check changed manually disabled channel: %+v", ch)
	}
}
