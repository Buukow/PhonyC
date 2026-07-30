package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phonyg/phonyg/internal/body"
	"github.com/phonyg/phonyg/internal/capture"
	"github.com/phonyg/phonyg/internal/protocol"
	"github.com/phonyg/phonyg/internal/snapshot"
	"github.com/phonyg/phonyg/internal/store"
	"github.com/phonyg/phonyg/internal/usage"
)

type Handler struct {
	Snap         *snapshot.Manager
	Store        *store.Store
	Capture      *capture.Manager
	MaxBodyBytes int64
	Client       *http.Client
}

func NewHandler(snap *snapshot.Manager, st *store.Store, cap *capture.Manager, maxBody int64) *Handler {
	return &Handler{
		Snap:         snap,
		Store:        st,
		Capture:      cap,
		MaxBodyBytes: maxBody,
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ResponseHeaderTimeout: 0,
			},
			// timeout per-request via context
			Timeout: 0,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type gwError struct {
	HTTP    int
	Code    string
	Message string
}

func (e gwError) Error() string { return e.Message }

func writeGatewayError(c *gin.Context, e gwError) {
	c.Header("Content-Type", "application/json")
	c.JSON(e.HTTP, gin.H{
		"error": gin.H{
			"message": e.Message,
			"type":    "gateway_error",
			"code":    e.Code,
		},
	})
}

func (h *Handler) Handle(c *gin.Context) {
	start := time.Now()
	reqID := c.GetHeader("X-Request-Id")
	if reqID == "" {
		reqID = uuid.NewString()
	}
	c.Header("X-Request-Id", reqID)

	snap := h.Snap.Current()
	if snap == nil {
		writeGatewayError(c, gwError{500, "internal_error", "config not ready"})
		return
	}

	// auth user key
	userKey, err := h.authenticate(c, snap)
	if err != nil {
		var ge gwError
		if errors.As(err, &ge) {
			writeGatewayError(c, ge)
			h.logMeta(reqID, nil, "", "", nil, c.Request.Method, c.Request.URL.Path, ge.HTTP, 0, time.Since(start), ge.Message, "", usage.Tokens{})
			return
		}
		writeGatewayError(c, gwError{401, "invalid_api_key", "invalid api key"})
		return
	}

	path := c.Request.URL.Path

	// capture-only mode: armed system key never enters routing (3.1 B+C)
	if userKey.Name == "header-capture" {
		h.handleCaptureOnly(c, reqID, path)
		return
	}

	// models catalog
	if protocol.IsModelsPath(path) {
		h.handleModels(c, snap, userKey, reqID, start)
		return
	}

	// read body with limit
	bodyBytes, err := h.readBody(c)
	if err != nil {
		var ge gwError
		if errors.As(err, &ge) {
			writeGatewayError(c, ge)
			h.logMeta(reqID, userKeyIDPtr(userKey), "", "", nil, c.Request.Method, path, ge.HTTP, 0, time.Since(start), ge.Message, userKey.ImpersonationMode, usage.Tokens{})
			return
		}
		writeGatewayError(c, gwError{400, "invalid_request_body", "failed to read body"})
		return
	}

	clientModel, _, _, err := body.PeekTopModel(bodyBytes)
	if err != nil {
		code := "model_required"
		msg := "top-level model is required"
		if errors.Is(err, body.ErrNotJSON) || errors.Is(err, body.ErrModelType) {
			code = "invalid_request_body"
			msg = err.Error()
		}
		writeGatewayError(c, gwError{400, code, msg})
		h.logMeta(reqID, userKeyIDPtr(userKey), "", "", nil, c.Request.Method, path, 400, 0, time.Since(start), msg, userKey.ImpersonationMode, usage.Tokens{})
		return
	}

	reqProtocol := protocol.ProtocolForPath(c.Request.Method, path)

	// Keep original body for multi-channel retry (each candidate may rewrite differently).
	origBody := append([]byte(nil), bodyBytes...)

	retryEnabled := h.Store.GetSettingBool(store.SettingAutoRetryEnabled, false)
	maxRetries := h.Store.GetSettingInt(store.SettingAutoRetryMax, 2)
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 20 {
		maxRetries = 20
	}
	retryCodes := store.ParseStatusCodeList(
		h.Store.GetSettingOr(store.SettingAutoRetryStatusCodes, "429,500,502,503,504"),
		[]int{429, 500, 502, 503, 504},
	)
	disableCodes := store.ParseStatusCodeList(
		h.Store.GetSettingOr(store.SettingAutoTestDisableCodes, "401,403,404,503"),
		[]int{401, 403, 404, 503},
	)

	exclude := map[int64]struct{}{}
	retriesLeft := 0
	if retryEnabled {
		retriesLeft = maxRetries
	}
	hadAttempt := false

	for {
		cand, ok := SelectChannelExcluding(snap, reqProtocol, clientModel, exclude)
		if !ok {
			msg := fmt.Sprintf("no enabled channel for model %q protocol %s", clientModel, reqProtocol)
			if hadAttempt {
				msg = fmt.Sprintf("all candidate channels failed for model %q protocol %s", clientModel, reqProtocol)
			}
			writeGatewayError(c, gwError{404, "model_not_found", msg})
			h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, "", nil, c.Request.Method, path, 404, 0, time.Since(start), msg, userKey.ImpersonationMode, usage.Tokens{})
			if hadAttempt {
				h.stats(userKey.ID, true)
			}
			return
		}
		hadAttempt = true

		bodyForUp := origBody
		metaUpstream := cand.Model.UpstreamModel
		if metaUpstream == "" {
			metaUpstream = clientModel
		}
		if cand.Model.RewriteModel && cand.Model.UpstreamModel != "" {
			rewritten, rerr := body.RewriteTopModel(origBody, cand.Model.UpstreamModel)
			if rerr != nil {
				exclude[cand.Channel.ID] = struct{}{}
				if retryEnabled && retriesLeft > 0 {
					retriesLeft--
					continue
				}
				writeGatewayError(c, gwError{500, "internal_error", "model rewrite failed"})
				chID := cand.Channel.ID
				h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, metaUpstream, &chID, c.Request.Method, path, 500, 0, time.Since(start), "model rewrite failed", userKey.ImpersonationMode, usage.Tokens{})
				h.stats(userKey.ID, true)
				return
			}
			bodyForUp = rewritten
		}

		base := strings.TrimRight(cand.Channel.BaseURL, "/")
		u, err := url.Parse(base + path)
		if err != nil {
			exclude[cand.Channel.ID] = struct{}{}
			if retryEnabled && retriesLeft > 0 {
				retriesLeft--
				continue
			}
			writeGatewayError(c, gwError{500, "internal_error", "invalid upstream url"})
			return
		}
		if c.Request.URL.RawQuery != "" {
			u.RawQuery = c.Request.URL.RawQuery
		}

		timeout := time.Duration(cand.Channel.TimeoutMS) * time.Millisecond
		if timeout <= 0 {
			timeout = 10 * time.Minute
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)

		upReq, err := http.NewRequestWithContext(ctx, c.Request.Method, u.String(), bytes.NewReader(bodyForUp))
		if err != nil {
			cancel()
			exclude[cand.Channel.ID] = struct{}{}
			if retryEnabled && retriesLeft > 0 {
				retriesLeft--
				continue
			}
			writeGatewayError(c, gwError{500, "internal_error", "build upstream request failed"})
			return
		}
		upHeaders := BuildUpstreamHeaders(c.Request.Header, cand.Channel, *userKey, snap, int64(len(bodyForUp)))
		upReq.Header = upHeaders
		upReq.Host = u.Host
		upReq.ContentLength = int64(len(bodyForUp))

		ttfbStart := time.Now()
		resp, err := h.Client.Do(upReq)
		ttfb := time.Since(ttfbStart)
		if err != nil {
			cancel()
			msg := "upstream request failed"
			if errors.Is(err, context.Canceled) {
				msg = "client canceled"
				writeGatewayError(c, gwError{502, "upstream_error", msg})
				chID := cand.Channel.ID
				h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, metaUpstream, &chID, c.Request.Method, path, 502, ttfb.Milliseconds(), time.Since(start), msg, userKey.ImpersonationMode, usage.Tokens{})
				h.stats(userKey.ID, true)
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				msg = "upstream timeout"
			}
			exclude[cand.Channel.ID] = struct{}{}
			chID := cand.Channel.ID
			if retryEnabled && retriesLeft > 0 && store.StatusCodeListContains(retryCodes, 502) {
				h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, metaUpstream, &chID, c.Request.Method, path, 502, ttfb.Milliseconds(), time.Since(start), msg+"; retrying", userKey.ImpersonationMode, usage.Tokens{})
				retriesLeft--
				continue
			}
			writeGatewayError(c, gwError{502, "upstream_error", msg})
			h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, metaUpstream, &chID, c.Request.Method, path, 502, ttfb.Milliseconds(), time.Since(start), msg, userKey.ImpersonationMode, usage.Tokens{})
			h.stats(userKey.ID, true)
			return
		}

		// Live traffic hits temp-disable codes => same as auto healthcheck ban.
		if store.StatusCodeListContains(disableCodes, resp.StatusCode) {
			_ = h.Store.SetChannelTempDisabled(cand.Channel.ID, true)
			_ = h.Snap.Reload()
			if s2 := h.Snap.Current(); s2 != nil {
				snap = s2
			}
		}

		if retryEnabled && retriesLeft > 0 && store.StatusCodeListContains(retryCodes, resp.StatusCode) {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2<<20))
			_ = resp.Body.Close()
			cancel()
			exclude[cand.Channel.ID] = struct{}{}
			chID := cand.Channel.ID
			h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, metaUpstream, &chID, c.Request.Method, path, resp.StatusCode, ttfb.Milliseconds(), time.Since(start), fmt.Sprintf("upstream status %d; retrying", resp.StatusCode), userKey.ImpersonationMode, usage.Tokens{})
			retriesLeft--
			continue
		}

		h.finishProxyResponse(c, reqID, userKey, clientModel, metaUpstream, path, start, cand, resp, ttfb, cancel)
		return
	}
}

func (h *Handler) finishProxyResponse(c *gin.Context, reqID string, userKey *store.UserKey, clientModel, metaUpstream, path string, start time.Time, cand snapshot.ModelCandidate, resp *http.Response, ttfb time.Duration, cancel context.CancelFunc) {
	defer cancel()
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		if isHop(k) {
			continue
		}
		for _, v := range vv {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.Header().Set("X-Request-Id", reqID)
	c.Writer.WriteHeader(resp.StatusCode)

	ct := resp.Header.Get("Content-Type")
	isSSE := strings.Contains(strings.ToLower(ct), "text/event-stream") || strings.Contains(strings.ToLower(ct), "event-stream")
	sniffer := usage.NewSniffer(isSSE)
	buf := make([]byte, 32*1024)
	flusher, canFlush := c.Writer.(http.Flusher)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = sniffer.Write(chunk)
			if _, werr := c.Writer.Write(chunk); werr != nil {
				break
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if rerr != nil {
			break
		}
	}

	chID := cand.Channel.ID
	errSummary := ""
	tok := sniffer.Result()
	if resp.StatusCode >= 400 {
		errSummary = fmt.Sprintf("upstream status %d", resp.StatusCode)
		tok = usage.Tokens{}
	}
	h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, metaUpstream, &chID, c.Request.Method, path, resp.StatusCode, ttfb.Milliseconds(), time.Since(start), errSummary, userKey.ImpersonationMode, tok)
	h.stats(userKey.ID, resp.StatusCode >= 400)
}

func (h *Handler) handleCaptureOnly(c *gin.Context, reqID, path string) {
	model := ""
	var bodyBytes []byte
	// optional peek model from body for non-GET; never reject on model name
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		b, err := h.readBody(c)
		if err == nil && len(b) > 0 {
			bodyBytes = b
			if m, _, _, perr := body.PeekTopModel(bodyBytes); perr == nil {
				model = m
			}
		}
	}
	if model == "" {
		model = "phonyg-capture"
	}
	captured := false
	if h.Capture != nil {
		captured = h.Capture.TryCapture(c.Request, model)
	}
	msg := "【PhonyG 请求捕获】捕获成功：已记录客户端请求头，未转发上游。可在管理台「请求捕获」查看并一键保存为预设。"
	if !captured {
		msg = "【PhonyG 请求捕获】当前未布防或未捕获到新请求（可能已捕获过）。请在管理台重新布防后再试。请求仍返回成功，未转发上游。"
	}
	c.Header("X-Request-Id", reqID)
	writeCaptureClientSuccess(c, path, model, msg, bodyBytes, captured)
	// no user key stats for capture
}

func requestWantsStream(bodyBytes []byte, h http.Header) bool {
	if strings.Contains(strings.ToLower(h.Get("Accept")), "text/event-stream") {
		return true
	}
	if len(bodyBytes) == 0 {
		return false
	}
	var tmp struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(bodyBytes, &tmp); err != nil {
		return false
	}
	return tmp.Stream
}

// writeCaptureClientSuccess returns a protocol-shaped 200 so clients accept any model name.
func writeCaptureClientSuccess(c *gin.Context, path, model, msg string, bodyBytes []byte, captured bool) {
	stream := requestWantsStream(bodyBytes, c.Request.Header)
	now := time.Now().Unix()

	switch {
	case path == "/v1/models" || strings.HasPrefix(path, "/v1/models/"):
		// Keep catalog shape so SDKs that probe models with capture key still succeed.
		if path == "/v1/models" {
			c.JSON(200, gin.H{
				"object": "list",
				"data": []gin.H{{
					"id": model, "object": "model", "created": now, "owned_by": "phonyg-capture",
					"captured": captured, "phonyg_message": msg,
				}},
			})
			return
		}
		c.JSON(200, gin.H{"id": model, "object": "model", "created": now, "owned_by": "phonyg-capture", "captured": captured, "phonyg_message": msg})
		return

	case path == "/v1/messages":
		if stream {
			writeCaptureAnthropicStream(c, model, msg, captured)
			return
		}
		c.JSON(200, gin.H{
			"id": "msg_phonyg_capture", "type": "message", "role": "assistant", "model": model,
			"content":     []gin.H{{"type": "text", "text": msg}},
			"stop_reason": "end_turn", "stop_sequence": nil,
			"usage":    gin.H{"input_tokens": 0, "output_tokens": 0},
			"captured": captured,
		})
		return

	case path == "/v1/responses" || strings.HasPrefix(path, "/v1/responses"):
		if stream {
			writeCaptureResponsesStream(c, model, msg, captured)
			return
		}
		c.JSON(200, gin.H{
			"id": "resp_phonyg_capture", "object": "response", "created_at": now, "status": "completed",
			"model": model, "output_text": msg,
			"output": []gin.H{{
				"type": "message", "id": "msg_phonyg_capture", "role": "assistant",
				"content": []gin.H{{"type": "output_text", "text": msg}},
			}},
			"usage":    gin.H{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0},
			"captured": captured,
		})
		return

	default:
		// OpenAI chat/completions and other openai-shaped endpoints
		if stream {
			writeCaptureChatStream(c, model, msg, captured)
			return
		}
		c.JSON(200, gin.H{
			"id": "chatcmpl-phonyg-capture", "object": "chat.completion", "created": now, "model": model,
			"choices": []gin.H{{
				"index":         0,
				"message":       gin.H{"role": "assistant", "content": msg},
				"finish_reason": "stop",
			}},
			"usage":    gin.H{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
			"captured": captured,
		})
	}
}

func writeCaptureChatStream(c *gin.Context, model, msg string, captured bool) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(200)
	flusher, _ := c.Writer.(http.Flusher)
	writeSSE := func(v any) {
		b, _ := json.Marshal(v)
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(b)
		_, _ = c.Writer.Write([]byte("\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}
	writeSSE(gin.H{
		"id": "chatcmpl-phonyg-capture", "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model,
		"choices":  []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": msg}, "finish_reason": nil}},
		"captured": captured,
	})
	writeSSE(gin.H{
		"id": "chatcmpl-phonyg-capture", "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model,
		"choices": []gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}},
	})
	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
}

func writeCaptureAnthropicStream(c *gin.Context, model, msg string, captured bool) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(200)
	flusher, _ := c.Writer.(http.Flusher)
	writeEvent := func(event string, v any) {
		b, _ := json.Marshal(v)
		_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	writeEvent("message_start", gin.H{
		"type": "message_start",
		"message": gin.H{
			"id": "msg_phonyg_capture", "type": "message", "role": "assistant", "model": model,
			"content": []any{}, "stop_reason": nil, "usage": gin.H{"input_tokens": 0, "output_tokens": 0},
		},
	})
	writeEvent("content_block_start", gin.H{"type": "content_block_start", "index": 0, "content_block": gin.H{"type": "text", "text": ""}})
	writeEvent("content_block_delta", gin.H{"type": "content_block_delta", "index": 0, "delta": gin.H{"type": "text_delta", "text": msg}})
	writeEvent("content_block_stop", gin.H{"type": "content_block_stop", "index": 0})
	writeEvent("message_delta", gin.H{"type": "message_delta", "delta": gin.H{"stop_reason": "end_turn"}, "usage": gin.H{"output_tokens": 0}, "captured": captured})
	writeEvent("message_stop", gin.H{"type": "message_stop"})
}

func writeCaptureResponsesStream(c *gin.Context, model, msg string, captured bool) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(200)
	flusher, _ := c.Writer.(http.Flusher)
	writeSSE := func(v any) {
		b, _ := json.Marshal(v)
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(b)
		_, _ = c.Writer.Write([]byte("\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}
	writeSSE(gin.H{"type": "response.created", "response": gin.H{"id": "resp_phonyg_capture", "object": "response", "status": "in_progress", "model": model}})
	writeSSE(gin.H{"type": "response.output_text.delta", "delta": msg})
	writeSSE(gin.H{
		"type": "response.completed",
		"response": gin.H{
			"id": "resp_phonyg_capture", "object": "response", "status": "completed", "model": model,
			"output_text": msg, "captured": captured,
			"output": []gin.H{{"type": "message", "role": "assistant", "content": []gin.H{{"type": "output_text", "text": msg}}}},
		},
	})
}

func userKeyIDPtr(k *store.UserKey) *int64 {
	if k == nil || k.ID <= 0 {
		return nil
	}
	id := k.ID
	return &id
}

func (h *Handler) authenticate(c *gin.Context, snap *snapshot.Snapshot) (*store.UserKey, error) {
	auth := c.GetHeader("Authorization")
	key := ""
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		key = strings.TrimSpace(auth[7:])
	}
	if key == "" {
		key = strings.TrimSpace(c.GetHeader("x-api-key"))
	}
	if key == "" {
		return nil, gwError{401, "invalid_api_key", "missing api key"}
	}
	// header capture system key (only when capture feature enabled)
	if h.Capture != nil && h.Capture.IsCaptureKey(key) {
		if !h.Capture.Enabled() {
			return nil, gwError{401, "invalid_api_key", "capture key disabled"}
		}
		if !h.Capture.Armed() {
			return nil, gwError{403, "capture_not_armed", "capture key is not armed; re-arm to capture next request"}
		}
		return &store.UserKey{
			ID:                0,
			Name:              "header-capture",
			Key:               key,
			Enabled:           true,
			ImpersonationMode: "passthrough",
		}, nil
	}
	uk := snap.KeyByValue[key]
	if uk == nil {
		return nil, gwError{401, "invalid_api_key", "invalid api key"}
	}
	if !uk.Enabled {
		return nil, gwError{401, "invalid_api_key", "api key disabled"}
	}
	return uk, nil
}

func (h *Handler) readBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, gwError{400, "invalid_request_body", "empty body"}
	}
	limit := h.MaxBodyBytes
	if limit <= 0 {
		limit = 64 << 20
	}
	lr := io.LimitReader(c.Request.Body, limit+1)
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, gwError{413, "request_too_large", "request body too large"}
	}
	return b, nil
}

func (h *Handler) handleModels(c *gin.Context, snap *snapshot.Snapshot, userKey *store.UserKey, reqID string, start time.Time) {
	path := c.Request.URL.Path
	anthropicStyle := c.GetHeader("x-api-key") != "" && (c.GetHeader("anthropic-version") != "" || c.GetHeader("Anthropic-Version") != "")
	// If authenticated via x-api-key only for our gateway, still check anthropic-version for format.
	// Spec: 同时存在 x-api-key 与 anthropic-version

	models := snap.ClientModels
	if models == nil {
		models = []string{}
	}

	if path == "/v1/models" {
		if anthropicStyle {
			data := make([]gin.H, 0, len(models))
			for _, id := range models {
				data = append(data, gin.H{
					"type":         "model",
					"id":           id,
					"display_name": id,
					"created_at":   "2021-07-20T00:00:00Z",
				})
			}
			first, last := "", ""
			if len(models) > 0 {
				first = models[0]
				last = models[len(models)-1]
			}
			c.JSON(200, gin.H{"data": data, "first_id": first, "last_id": last, "has_more": false})
		} else {
			data := make([]gin.H, 0, len(models))
			for _, id := range models {
				data = append(data, gin.H{
					"id": id, "object": "model", "created": 1626777600, "owned_by": "phonyg",
				})
			}
			c.JSON(200, gin.H{"object": "list", "data": data})
		}
		h.logMeta(reqID, userKeyIDPtr(userKey), "", "", nil, c.Request.Method, path, 200, 0, time.Since(start), "", userKey.ImpersonationMode, usage.Tokens{})
		h.stats(userKey.ID, false)
		return
	}

	// /v1/models/:id
	id := strings.TrimPrefix(path, "/v1/models/")
	found := false
	for _, m := range models {
		if m == id {
			found = true
			break
		}
	}
	if !found {
		writeGatewayError(c, gwError{404, "model_not_found", "model not found"})
		h.logMeta(reqID, userKeyIDPtr(userKey), id, "", nil, c.Request.Method, path, 404, 0, time.Since(start), "model_not_found", userKey.ImpersonationMode, usage.Tokens{})
		h.stats(userKey.ID, true)
		return
	}
	if anthropicStyle {
		c.JSON(200, gin.H{"type": "model", "id": id, "display_name": id, "created_at": "2021-07-20T00:00:00Z"})
	} else {
		c.JSON(200, gin.H{"id": id, "object": "model", "created": 1626777600, "owned_by": "phonyg"})
	}
	h.logMeta(reqID, userKeyIDPtr(userKey), id, "", nil, c.Request.Method, path, 200, 0, time.Since(start), "", userKey.ImpersonationMode, usage.Tokens{})
	h.stats(userKey.ID, false)
}

func (h *Handler) logMeta(reqID string, keyID *int64, clientModel, upstreamModel string, chID *int64, method, path string, status int, ttfbMs int64, total time.Duration, errSummary, mode string, tok usage.Tokens) {
	if status >= 400 {
		tok = usage.Tokens{}
	}
	m := &store.RequestMeta{
		RequestID:         reqID,
		UserKeyID:         keyID,
		ClientModel:       clientModel,
		UpstreamModel:     upstreamModel,
		ChannelID:         chID,
		Method:            method,
		Path:              path,
		StatusCode:        status,
		TTFBms:            ttfbMs,
		TotalMs:           total.Milliseconds(),
		ErrorSummary:      errSummary,
		ImpersonationMode: mode,
		PromptTokens:      tok.PromptTokens,
		CompletionTokens:  tok.CompletionTokens,
		TotalTokens:       tok.TotalTokens,
		CachedTokens:      tok.CachedTokens,
		ReasoningTokens:   tok.ReasoningTokens,
	}
	go func() { _ = h.Store.InsertRequestMeta(m) }()
}

func (h *Handler) stats(keyID int64, isErr bool) {
	if keyID <= 0 {
		return
	}
	day := time.Now().Format("2006-01-02") // system local day boundary
	go func() { _ = h.Store.IncrKeyStats(keyID, day, isErr) }()
}

// Ensure unused import quiet for httputil if we don't use reverse proxy directly
var _ = httputil.ReverseProxy{}
var _ = json.Marshal
