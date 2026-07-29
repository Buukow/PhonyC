package healthcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/phonyc/phonyc/internal/protocol"
	"github.com/phonyc/phonyc/internal/snapshot"
	"github.com/phonyc/phonyc/internal/store"
)

type Worker struct {
	Store  *store.Store
	Snap   *snapshot.Manager
	Client *http.Client

	mu          sync.Mutex
	running     bool
	stopCh      chan struct{}
	wakeCh      chan struct{}
	lastRun     time.Time
	lastSummary RunSummary
}

type RunSummary struct {
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt time.Time       `json:"finished_at"`
	Total      int             `json:"total"`
	OK         int             `json:"ok"`
	Failed     int             `json:"failed"`
	Disabled   int             `json:"disabled"`
	Recovered  int             `json:"recovered"`
	Results    []ChannelResult `json:"results"`
}

type ChannelResult struct {
	ChannelID    int64  `json:"channel_id"`
	Name         string `json:"name"`
	Model        string `json:"model"`
	StatusCode   int    `json:"status_code"`
	LatencyMs    int64  `json:"latency_ms"`
	Error        string `json:"error"`
	TempDisabled bool   `json:"temp_disabled"`
	Action       string `json:"action"` // none|disable|recover
}

func New(st *store.Store, snap *snapshot.Manager) *Worker {
	return &Worker{
		Store: st,
		Snap:  snap,
		Client: &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		stopCh: make(chan struct{}),
		wakeCh: make(chan struct{}, 1),
	}
}

func (w *Worker) Start() { go w.loop() }

func (w *Worker) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

func (w *Worker) TriggerNow() {
	select {
	case w.wakeCh <- struct{}{}:
	default:
	}
}

func (w *Worker) LastSummary() RunSummary {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastSummary
}

func (w *Worker) loop() {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.wakeCh:
			w.runOnce(true)
			resetTimer(timer, w.nextDelay())
		case <-timer.C:
			if w.Store.GetSettingBool(store.SettingAutoTestEnabled, false) {
				w.runOnce(true)
			}
			resetTimer(timer, w.nextDelay())
		}
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

func (w *Worker) nextDelay() time.Duration {
	interval := w.Store.GetSettingInt(store.SettingAutoTestIntervalMin, 10)
	if interval < 1 {
		interval = 1
	}
	offset := w.Store.GetSettingInt(store.SettingAutoTestRandomOffset, 0)
	if offset < 0 {
		offset = 0
	}
	extra := 0
	if offset > 0 {
		extra = rand.Intn(offset + 1)
	}
	return time.Duration(interval+extra) * time.Minute
}

// runOnce tests all manually-enabled channels. applyBan controls temp disable/recover.
func (w *Worker) runOnce(applyBan bool) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	summary := RunSummary{StartedAt: time.Now().UTC()}
	channels, err := w.Store.ListEnabledChannelsForAutoTest()
	if err != nil {
		log.Printf("healthcheck list channels: %v", err)
		return
	}
	prompt := w.Store.GetSettingOr(store.SettingAutoTestPrompt, "hi")
	disableCodes := store.ParseStatusCodeList(
		w.Store.GetSettingOr(store.SettingAutoTestDisableCodes, "401,403,404,503"),
		[]int{401, 403, 404, 503},
	)

	changed := false
	for _, ch := range channels {
		res := w.testChannel(ch, prompt, disableCodes, applyBan)
		summary.Total++
		summary.Results = append(summary.Results, res)
		if res.Error == "" && res.StatusCode > 0 && res.StatusCode < 400 {
			summary.OK++
		} else {
			summary.Failed++
		}
		if res.Action == "disable" {
			summary.Disabled++
			changed = true
		}
		if res.Action == "recover" {
			summary.Recovered++
			changed = true
		}
	}
	summary.FinishedAt = time.Now().UTC()
	w.mu.Lock()
	w.lastRun = summary.FinishedAt
	w.lastSummary = summary
	w.mu.Unlock()
	if changed {
		_ = w.Snap.Reload()
	}
	log.Printf("healthcheck done total=%d ok=%d failed=%d disabled=%d recovered=%d applyBan=%v",
		summary.Total, summary.OK, summary.Failed, summary.Disabled, summary.Recovered, applyBan)
}

func firstChannelModel(st *store.Store, channelID int64) string {
	models, err := st.ListChannelModels(channelID)
	if err != nil {
		return ""
	}
	for _, m := range models {
		if !m.Enabled {
			continue
		}
		// first enabled mapping: use client_model as upstream request model name
		// (glass body uses whatever we send; channel mapping is for proxy path)
		if m.RewriteModel && m.UpstreamModel != "" {
			return m.UpstreamModel
		}
		if m.ClientModel != "" {
			return m.ClientModel
		}
	}
	return ""
}

func (w *Worker) testChannel(ch store.Channel, prompt string, disableCodes []int, applyBan bool) ChannelResult {
	res := ChannelResult{ChannelID: ch.ID, Name: ch.Name, TempDisabled: ch.TempDisabled, Action: "none"}
	// Decision 2.7: always each channel's first enabled model
	modelName := firstChannelModel(w.Store, ch.ID)
	if modelName == "" {
		res.Error = "no enabled model on channel"
		_ = w.Store.UpdateChannelTestResult(ch.ID, 0, 0, res.Error, nil)
		return res
	}
	res.Model = modelName

	path, body, err := buildTestBody(ch.Protocol, modelName, prompt)
	if err != nil {
		res.Error = err.Error()
		_ = w.Store.UpdateChannelTestResult(ch.ID, 0, 0, res.Error, nil)
		return res
	}

	base := strings.TrimRight(ch.BaseURL, "/")
	url := base + path
	timeout := time.Duration(ch.TimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > 2*time.Minute {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		res.Error = err.Error()
		_ = w.Store.UpdateChannelTestResult(ch.ID, 0, 0, res.Error, nil)
		return res
	}
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	protocol.Get(ch.Protocol).ApplyAuth(req.Header, ch.APIKey)
	var extra map[string]string
	_ = json.Unmarshal([]byte(ch.ExtraHeadersJSON), &extra)
	for k, v := range extra {
		if k != "" {
			req.Header.Set(k, v)
		}
	}

	start := time.Now()
	resp, err := w.Client.Do(req)
	res.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
		_ = w.Store.UpdateChannelTestResult(ch.ID, 0, res.LatencyMs, res.Error, nil)
		return res
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	res.StatusCode = resp.StatusCode

	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	var tempPtr *bool
	if ok {
		res.Error = ""
		if applyBan && ch.TempDisabled {
			f := false
			tempPtr = &f
			res.Action = "recover"
			res.TempDisabled = false
		}
	} else {
		res.Error = fmt.Sprintf("upstream status %d", resp.StatusCode)
		if applyBan && store.StatusCodeListContains(disableCodes, resp.StatusCode) {
			t := true
			tempPtr = &t
			res.Action = "disable"
			res.TempDisabled = true
		}
	}
	_ = w.Store.UpdateChannelTestResult(ch.ID, res.StatusCode, res.LatencyMs, res.Error, tempPtr)
	return res
}

func buildTestBody(proto, model, prompt string) (path string, body []byte, err error) {
	if prompt == "" {
		prompt = "hi"
	}
	switch proto {
	case protocol.Anthropic:
		path = "/v1/messages"
		payload := map[string]any{
			"model": model, "max_tokens": 16, "stream": false,
			"messages": []map[string]any{{"role": "user", "content": prompt}},
		}
		body, err = json.Marshal(payload)
		return path, body, err
	default:
		path = "/v1/chat/completions"
		payload := map[string]any{
			"model": model, "max_tokens": 16, "stream": false,
			"messages": []map[string]any{{"role": "user", "content": prompt}},
		}
		body, err = json.Marshal(payload)
		return path, body, err
	}
}

// TestOne manual: report only, never change temp_disabled (decision 2.9B).
func (w *Worker) TestOne(channelID int64) (ChannelResult, error) {
	ch, err := w.Store.GetChannel(channelID)
	if err != nil {
		return ChannelResult{}, err
	}
	if !ch.Enabled {
		return ChannelResult{}, fmt.Errorf("channel is manually disabled")
	}
	prompt := w.Store.GetSettingOr(store.SettingAutoTestPrompt, "hi")
	disableCodes := store.ParseStatusCodeList(
		w.Store.GetSettingOr(store.SettingAutoTestDisableCodes, "401,403,404,503"),
		[]int{401, 403, 404, 503},
	)
	res := w.testChannel(*ch, prompt, disableCodes, false)
	return res, nil
}
