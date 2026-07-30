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

	"github.com/google/uuid"
	"github.com/phonyg/phonyg/internal/protocol"
	"github.com/phonyg/phonyg/internal/snapshot"
	"github.com/phonyg/phonyg/internal/store"
	"github.com/phonyg/phonyg/internal/usage"
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
	ChannelID      int64  `json:"channel_id"`
	Name           string `json:"name"`
	Model          string `json:"model"`
	StatusCode     int    `json:"status_code"`
	LatencyMs      int64  `json:"latency_ms"`
	Error          string `json:"error"`
	TempDisabled   bool   `json:"temp_disabled"`
	Action         string `json:"action"` // none|disable|recover
	StreamFallback bool   `json:"stream_fallback"`
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
	disableCodes := store.ParseStatusCodeList(
		w.Store.GetSettingOr(store.SettingAutoTestDisableCodes, "401,403,404,503"),
		[]int{401, 403, 404, 503},
	)

	changed := false
	for _, ch := range channels {
		prompt, enhanced := w.promptForCheck()
		res := w.testChannel(ch, prompt, enhanced, disableCodes, applyBan, applyBan)
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

func (w *Worker) promptForCheck() (string, bool) {
	if !w.Store.GetSettingBool(store.SettingAutoTestEnhanced, false) {
		return w.Store.GetSettingOr(store.SettingAutoTestPrompt, "hi"), false
	}
	raw := w.Store.GetSettingOr(store.SettingAutoTestLexicon, DefaultEnhancedLexiconJSON())
	lex, err := ParseEnhancedLexicon(raw)
	if err != nil {
		log.Printf("enhanced healthcheck lexicon invalid, using default: %v", err)
		lex = defaultEnhancedLexicon
	}
	return GenerateEnhancedPrompt(lex, globalRandom{}), true
}

type requestAttempt struct {
	StatusCode  int
	LatencyMs   int64
	Error       string
	Body        []byte
	ContentType string
}

func (w *Worker) testChannel(ch store.Channel, prompt string, enhanced bool, disableCodes []int, allowDisable, allowRecover bool) ChannelResult {
	res := ChannelResult{ChannelID: ch.ID, Name: ch.Name, TempDisabled: ch.TempDisabled, Action: "none"}
	// Decision 2.7: always each channel's first enabled model
	modelName := firstChannelModel(w.Store, ch.ID)
	if modelName == "" {
		res.Error = "no enabled model on channel"
		_ = w.Store.UpdateChannelTestResult(ch.ID, 0, 0, res.Error, nil)
		return res
	}
	res.Model = modelName

	path, body, err := buildTestBody(ch.Protocol, modelName, prompt, enhanced)
	if err != nil {
		res.Error = err.Error()
		_ = w.Store.UpdateChannelTestResult(ch.ID, 0, 0, res.Error, nil)
		return res
	}

	start := time.Now()
	attempt := w.performRequest(ch, path, body)
	fallbackDetail := ""
	if enhanced && !streamAttemptSucceeded(attempt) {
		res.StreamFallback = true
		fallbackDetail = streamFailureSummary(attempt)
		_, fallbackBody, buildErr := buildTestBody(ch.Protocol, modelName, prompt, false)
		if buildErr != nil {
			attempt = requestAttempt{LatencyMs: time.Since(start).Milliseconds(), Error: buildErr.Error()}
		} else {
			attempt = w.performRequest(ch, path, fallbackBody)
			attempt.LatencyMs = time.Since(start).Milliseconds()
		}
	}
	res.LatencyMs = attempt.LatencyMs
	res.StatusCode = attempt.StatusCode
	bodyBytes := attempt.Body
	if attempt.Error != "" {
		res.Error = attempt.Error
		_ = w.Store.UpdateChannelTestResult(ch.ID, 0, res.LatencyMs, res.Error, nil)
		metaError := res.Error
		if fallbackDetail != "" {
			metaError = "stream fallback (" + fallbackDetail + "): " + metaError
		}
		w.recordHealthcheckMeta(ch, path, modelName, 0, res.LatencyMs, metaError, usage.Tokens{})
		return res
	}

	ok := attempt.StatusCode >= 200 && attempt.StatusCode < 400
	var tempPtr *bool
	if ok {
		res.Error = ""
		if allowRecover && ch.TempDisabled {
			f := false
			tempPtr = &f
			res.Action = "recover"
			res.TempDisabled = false
		}
	} else {
		res.Error = fmt.Sprintf("upstream status %d", attempt.StatusCode)
		if allowDisable && store.StatusCodeListContains(disableCodes, attempt.StatusCode) {
			t := true
			tempPtr = &t
			res.Action = "disable"
			res.TempDisabled = true
		}
	}
	_ = w.Store.UpdateChannelTestResult(ch.ID, res.StatusCode, res.LatencyMs, res.Error, tempPtr)
	tok := usage.Tokens{}
	if ok {
		tok = usage.ParseTopLevelJSON(bodyBytes)
	}
	metaError := res.Error
	if fallbackDetail != "" {
		if metaError == "" {
			metaError = "stream fallback: " + fallbackDetail
		} else {
			metaError = "stream fallback (" + fallbackDetail + "): " + metaError
		}
	}
	w.recordHealthcheckMeta(ch, path, modelName, res.StatusCode, res.LatencyMs, metaError, tok)
	return res
}

func (w *Worker) performRequest(ch store.Channel, path string, body []byte) requestAttempt {
	timeout := time.Duration(ch.TimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > 2*time.Minute {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	url := strings.TrimRight(ch.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return requestAttempt{Error: err.Error()}
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
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return requestAttempt{LatencyMs: latency, Error: err.Error()}
	}
	defer resp.Body.Close()
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return requestAttempt{StatusCode: resp.StatusCode, LatencyMs: latency, Error: readErr.Error(), ContentType: resp.Header.Get("Content-Type")}
	}
	return requestAttempt{StatusCode: resp.StatusCode, LatencyMs: latency, Body: bodyBytes, ContentType: resp.Header.Get("Content-Type")}
}

func streamAttemptSucceeded(attempt requestAttempt) bool {
	return attempt.Error == "" && attempt.StatusCode >= 200 && attempt.StatusCode < 400 && streamHasContent(attempt.ContentType, attempt.Body)
}

func streamFailureSummary(attempt requestAttempt) string {
	if attempt.Error != "" {
		return attempt.Error
	}
	if attempt.StatusCode < 200 || attempt.StatusCode >= 400 {
		return fmt.Sprintf("upstream status %d", attempt.StatusCode)
	}
	return "empty or unrecognized stream"
}

func (w *Worker) recordHealthcheckMeta(ch store.Channel, path, model string, status int, latencyMs int64, errSummary string, tok usage.Tokens) {
	if status >= 400 {
		tok = usage.Tokens{}
	}
	chID := ch.ID
	m := &store.RequestMeta{
		RequestID:         "hc-" + uuid.NewString(),
		ClientModel:       model,
		UpstreamModel:     model,
		ChannelID:         &chID,
		Method:            http.MethodPost,
		Path:              path,
		StatusCode:        status,
		TTFBms:            latencyMs,
		TotalMs:           latencyMs,
		ErrorSummary:      errSummary,
		ImpersonationMode: "healthcheck",
		PromptTokens:      tok.PromptTokens,
		CompletionTokens:  tok.CompletionTokens,
		TotalTokens:       tok.TotalTokens,
		CachedTokens:      tok.CachedTokens,
		ReasoningTokens:   tok.ReasoningTokens,
	}
	_ = w.Store.InsertRequestMeta(m)
}

func buildTestBody(proto, model, prompt string, stream bool) (path string, body []byte, err error) {
	if prompt == "" {
		prompt = "hi"
	}
	switch proto {
	case protocol.Anthropic:
		path = "/v1/messages"
		payload := map[string]any{
			"model": model, "max_tokens": 16, "stream": stream,
			"messages": []map[string]any{{"role": "user", "content": prompt}},
		}
		body, err = json.Marshal(payload)
		return path, body, err
	default:
		path = "/v1/chat/completions"
		payload := map[string]any{
			"model": model, "max_tokens": 16, "stream": stream,
			"messages": []map[string]any{{"role": "user", "content": prompt}},
		}
		body, err = json.Marshal(payload)
		return path, body, err
	}
}

// TestOne records results for every channel state. It only recovers a channel
// that was temporarily disabled before a successful manual test.
func (w *Worker) TestOne(channelID int64) (ChannelResult, error) {
	ch, err := w.Store.GetChannel(channelID)
	if err != nil {
		return ChannelResult{}, err
	}
	prompt, enhanced := w.promptForCheck()
	disableCodes := store.ParseStatusCodeList(
		w.Store.GetSettingOr(store.SettingAutoTestDisableCodes, "401,403,404,503"),
		[]int{401, 403, 404, 503},
	)
	res := w.testChannel(*ch, prompt, enhanced, disableCodes, false, ch.Enabled && ch.TempDisabled)
	if res.Action == "recover" {
		_ = w.Snap.Reload()
	}
	return res, nil
}
