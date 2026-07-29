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
	"github.com/phonyc/phonyc/internal/body"
	"github.com/phonyc/phonyc/internal/capture"
	"github.com/phonyc/phonyc/internal/protocol"
	"github.com/phonyc/phonyc/internal/snapshot"
	"github.com/phonyc/phonyc/internal/store"
	"github.com/phonyc/phonyc/internal/usage"
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
	cand, ok := SelectChannel(snap, reqProtocol, clientModel)
	if !ok {
		writeGatewayError(c, gwError{404, "model_not_found", fmt.Sprintf("no enabled channel for model %q protocol %s", clientModel, reqProtocol)})
		h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, "", nil, c.Request.Method, path, 404, 0, time.Since(start), "model_not_found", userKey.ImpersonationMode, usage.Tokens{})
		return
	}

	upstreamModel := clientModel
	if cand.Model.RewriteModel {
		upstreamModel = cand.Model.UpstreamModel
		bodyBytes, err = body.RewriteTopModel(bodyBytes, cand.Model.UpstreamModel)
		if err != nil {
			writeGatewayError(c, gwError{500, "internal_error", "model rewrite failed"})
			return
		}
	} else {
		upstreamModel = cand.Model.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = clientModel
		}
		// when rewrite off, body stays as client; upstream_model recorded for meta
		if !cand.Model.RewriteModel {
			upstreamModel = clientModel
		}
	}
	// Fix upstream model recording:
	// - rewrite on: body becomes UpstreamModel, meta upstream = UpstreamModel
	// - rewrite off: body stays client, meta upstream_model field still stores mapping target for observability? Spec says client_model/upstream_model in meta.
	// Use mapping's upstream_model always for meta.upstream_model, and client for client_model.
	metaUpstream := cand.Model.UpstreamModel
	if metaUpstream == "" {
		metaUpstream = clientModel
	}
	if cand.Model.RewriteModel {
		upstreamModel = cand.Model.UpstreamModel
	} else {
		upstreamModel = clientModel
	}
	_ = upstreamModel

	// build upstream request
	base := strings.TrimRight(cand.Channel.BaseURL, "/")
	u, err := url.Parse(base + path)
	if err != nil {
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
	defer cancel()

	upReq, err := http.NewRequestWithContext(ctx, c.Request.Method, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		writeGatewayError(c, gwError{500, "internal_error", "build upstream request failed"})
		return
	}
	upHeaders := BuildUpstreamHeaders(c.Request.Header, cand.Channel, *userKey, snap, int64(len(bodyBytes)))
	upReq.Header = upHeaders
	upReq.Host = u.Host
	upReq.ContentLength = int64(len(bodyBytes))

	ttfbStart := time.Now()
	resp, err := h.Client.Do(upReq)
	if err != nil {
		msg := "upstream request failed"
		if errors.Is(err, context.Canceled) {
			msg = "client canceled"
		} else if errors.Is(err, context.DeadlineExceeded) {
			msg = "upstream timeout"
		}
		writeGatewayError(c, gwError{502, "upstream_error", msg})
		chID := cand.Channel.ID
		h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, metaUpstream, &chID, c.Request.Method, path, 502, 0, time.Since(start), msg, userKey.ImpersonationMode, usage.Tokens{})
		h.stats(userKey.ID, true)
		return
	}
	defer resp.Body.Close()

	ttfb := time.Since(ttfbStart)
	// copy response headers
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

	// stream copy with flush for SSE; tee-sniff usage from response
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
		tok = usage.Tokens{} // failed requests always 0
	}
	h.logMeta(reqID, userKeyIDPtr(userKey), clientModel, metaUpstream, &chID, c.Request.Method, path, resp.StatusCode, ttfb.Milliseconds(), time.Since(start), errSummary, userKey.ImpersonationMode, tok)
	h.stats(userKey.ID, resp.StatusCode >= 400)
}


func (h *Handler) handleCaptureOnly(c *gin.Context, reqID, path string) {
	model := ""
	// optional peek model from body for non-GET
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		bodyBytes, err := h.readBody(c)
		if err == nil && len(bodyBytes) > 0 {
			if m, _, _, perr := body.PeekTopModel(bodyBytes); perr == nil {
				model = m
			}
		}
	}
	captured := false
	if h.Capture != nil {
		captured = h.Capture.TryCapture(c.Request, model)
	}
	c.Header("X-Request-Id", reqID)
	c.JSON(200, gin.H{
		"captured": captured,
		"message":  "request headers recorded; not proxied",
		"path":     path,
		"model":    model,
	})
	// no user key stats for capture
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
					"id": id, "object": "model", "created": 1626777600, "owned_by": "phonyc",
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
		c.JSON(200, gin.H{"id": id, "object": "model", "created": 1626777600, "owned_by": "phonyc"})
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
