package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/phonyg/phonyg/internal/capture"
	"github.com/phonyg/phonyg/internal/healthcheck"
	"github.com/phonyg/phonyg/internal/snapshot"
	"github.com/phonyg/phonyg/internal/store"
	"github.com/phonyg/phonyg/internal/upstream"
)

type API struct {
	Store   *store.Store
	Auth    *Auth
	Snap    *snapshot.Manager
	Health  *healthcheck.Worker
	Capture *capture.Manager
}

func (a *API) Register(r *gin.Engine) {
	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	api.GET("/setup/status", a.setupStatus)
	api.POST("/setup", a.setup)
	api.POST("/auth/login", a.login)

	authz := api.Group("")
	authz.Use(a.Auth.Middleware())
	authz.POST("/auth/change-password", a.changePassword)
	authz.PATCH("/auth/profile", a.profile)

	authz.GET("/channels", a.listChannels)
	authz.POST("/channels", a.createChannel)
	authz.GET("/channels/:id", a.getChannel)
	authz.PATCH("/channels/:id", a.updateChannel)
	authz.DELETE("/channels/:id", a.deleteChannel)

	authz.POST("/channels/probe-models", a.probeModels)
	authz.POST("/channels/:id/fetch-models", a.fetchChannelModels)
	authz.GET("/channels/:id/models", a.listModels)
	authz.POST("/channels/:id/models", a.createModel)
	authz.PUT("/channels/:id/models", a.replaceModels)
	authz.PATCH("/channel-models/:id", a.updateModel)
	authz.DELETE("/channel-models/:id", a.deleteModel)

	authz.GET("/keys", a.listKeys)
	authz.POST("/keys", a.createKey)
	authz.GET("/keys/:id", a.getKey)
	authz.PATCH("/keys/:id", a.updateKey)
	authz.DELETE("/keys/:id", a.deleteKey)
	authz.GET("/keys/:id/stats", a.keyStats)

	authz.GET("/presets", a.listPresets)
	authz.POST("/presets", a.createPreset)
	authz.GET("/presets/:id", a.getPreset)
	authz.PATCH("/presets/:id", a.updatePreset)
	authz.DELETE("/presets/:id", a.deletePreset)

	authz.GET("/logs", a.listLogs)
	authz.GET("/dashboard/summary", a.dashboard)
	authz.GET("/settings", a.getSettings)
	authz.PATCH("/settings", a.patchSettings)

	// health check / 测活
	authz.POST("/channels/:id/test", a.testChannel)
	authz.POST("/healthcheck/run", a.runHealthcheck)
	authz.GET("/healthcheck/status", a.healthcheckStatus)
	authz.POST("/healthcheck/enhanced-preview", a.previewEnhancedHealthcheck)

	// header capture
	authz.GET("/capture", a.getCapture)
	authz.POST("/capture/enable", a.enableCapture)
	authz.POST("/capture/disable", a.disableCapture)
	authz.POST("/capture/arm", a.armCapture)
	authz.POST("/capture/clear", a.clearCapture)
	authz.POST("/capture/save-preset", a.saveCapturePreset)
}

func (a *API) reload() {
	_ = a.Snap.Reload()
}

func (a *API) setupStatus(c *gin.Context) {
	n, err := a.Store.AdminCount()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"initialized": n > 0})
}

func (a *API) setup(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Password) < 6 {
		c.JSON(400, gin.H{"error": "username required and password min 6 chars"})
		return
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	admin, err := a.Store.CreateAdmin(req.Username, hash)
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	tok, err := a.Auth.IssueToken(admin)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"token": tok, "admin": gin.H{"id": admin.ID, "username": admin.Username}})
}

func (a *API) login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	admin, err := a.Store.GetAdminByUsername(strings.TrimSpace(req.Username))
	if err != nil || !CheckPassword(admin.PasswordHash, req.Password) {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	tok, err := a.Auth.IssueToken(admin)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"token": tok, "admin": gin.H{"id": admin.ID, "username": admin.Username}})
}

func (a *API) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BindJSON(&req); err != nil || len(req.NewPassword) < 6 {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	id := c.GetInt64("admin_id")
	admin, err := a.Store.GetAdminByID(id)
	if err != nil || !CheckPassword(admin.PasswordHash, req.OldPassword) {
		c.JSON(400, gin.H{"error": "old password incorrect"})
		return
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if err := a.Store.UpdateAdminPassword(id, hash); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (a *API) profile(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
	}
	if err := c.BindJSON(&req); err != nil || strings.TrimSpace(req.Username) == "" {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	id := c.GetInt64("admin_id")
	if err := a.Store.UpdateAdminUsername(id, strings.TrimSpace(req.Username)); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "username": strings.TrimSpace(req.Username)})
}

func (a *API) listChannels(c *gin.Context) {
	list, err := a.Store.ListChannels()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []store.Channel{}
	}
	c.JSON(200, gin.H{"items": list})
}

func (a *API) createChannel(c *gin.Context) {
	var req struct {
		store.ChannelInput
		Models []store.ChannelModelInput `json:"models"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	in := req.ChannelInput
	in.Protocol = strings.ToLower(strings.TrimSpace(in.Protocol))
	if in.Name == "" || (in.Protocol != "openai" && in.Protocol != "anthropic") || in.BaseURL == "" {
		c.JSON(400, gin.H{"error": "name, protocol(openai|anthropic), base_url required"})
		return
	}
	if in.Priority != nil && *in.Priority < 0 {
		c.JSON(400, gin.H{"error": "priority must be >= 0 (0 is lowest, higher is preferred)"})
		return
	}
	ch, err := a.Store.CreateChannel(in)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if len(req.Models) > 0 {
		if _, err := a.Store.ReplaceChannelModels(ch.ID, req.Models); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	a.reload()
	c.JSON(200, ch)
}

func (a *API) getChannel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ch, err := a.Store.GetChannel(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, ch)
}

func (a *API) updateChannel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in store.ChannelInput
	if err := c.BindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	if in.Protocol != "" {
		in.Protocol = strings.ToLower(in.Protocol)
	}
	if in.Priority != nil && *in.Priority < 0 {
		c.JSON(400, gin.H{"error": "priority must be >= 0 (0 is lowest, higher is preferred)"})
		return
	}
	ch, err := a.Store.UpdateChannel(id, in)
	if err != nil {
		status := 400
		if errors.Is(err, store.ErrNotFound) {
			status = 404
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, ch)
}

func (a *API) deleteChannel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := a.Store.DeleteChannel(id); err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	a.reload()
	c.JSON(200, gin.H{"ok": true})
}

func (a *API) listModels(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := a.Store.ListChannelModels(id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []store.ChannelModel{}
	}
	c.JSON(200, gin.H{"items": list})
}

func (a *API) createModel(c *gin.Context) {
	cid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in store.ChannelModelInput
	if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.ClientModel) == "" {
		c.JSON(400, gin.H{"error": "client_model required"})
		return
	}
	m, err := a.Store.CreateChannelModel(cid, in)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, m)
}

func (a *API) updateModel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in store.ChannelModelInput
	if err := c.BindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	m, err := a.Store.UpdateChannelModel(id, in)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, m)
}

func (a *API) deleteModel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := a.Store.DeleteChannelModel(id); err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	a.reload()
	c.JSON(200, gin.H{"ok": true})
}

func (a *API) probeModels(c *gin.Context) {
	var req struct {
		BaseURL          string `json:"base_url"`
		APIKey           string `json:"api_key"`
		Protocol         string `json:"protocol"`
		ExtraHeadersJSON string `json:"extra_headers_json"`
		ChannelID        int64  `json:"channel_id"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	if strings.TrimSpace(req.APIKey) == "" && req.ChannelID > 0 {
		if ch, err := a.Store.GetChannel(req.ChannelID); err == nil {
			req.APIKey = ch.APIKey
			if strings.TrimSpace(req.BaseURL) == "" {
				req.BaseURL = ch.BaseURL
			}
			if strings.TrimSpace(req.Protocol) == "" {
				req.Protocol = ch.Protocol
			}
			if strings.TrimSpace(req.ExtraHeadersJSON) == "" || req.ExtraHeadersJSON == "{}" {
				req.ExtraHeadersJSON = ch.ExtraHeadersJSON
			}
		}
	}
	ids, err := upstream.FetchModelIDs(upstream.FetchRequest{
		BaseURL: req.BaseURL, APIKey: req.APIKey, Protocol: req.Protocol, ExtraHeadersJSON: req.ExtraHeadersJSON,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": ids})
}

func (a *API) fetchChannelModels(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ch, err := a.Store.GetChannel(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	ids, err := upstream.FetchModelIDs(upstream.FetchRequest{
		BaseURL: ch.BaseURL, APIKey: ch.APIKey, Protocol: ch.Protocol, ExtraHeadersJSON: ch.ExtraHeadersJSON,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": ids})
}

func (a *API) replaceModels(c *gin.Context) {
	cid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if _, err := a.Store.GetChannel(cid); err != nil {
		c.JSON(404, gin.H{"error": "channel not found"})
		return
	}
	var req struct {
		Items []store.ChannelModelInput `json:"items"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	list, err := a.Store.ReplaceChannelModels(cid, req.Items)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []store.ChannelModel{}
	}
	a.reload()
	c.JSON(200, gin.H{"items": list})
}

func (a *API) listKeys(c *gin.Context) {
	list, err := a.Store.ListUserKeys()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []store.UserKey{}
	}
	c.JSON(200, gin.H{"items": list})
}

func (a *API) createKey(c *gin.Context) {
	var in store.UserKeyInput
	if err := c.BindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		c.JSON(400, gin.H{"error": "name required"})
		return
	}
	if strings.TrimSpace(in.Key) == "" {
		in.Key = RandomAPIKey()
	}
	if in.ImpersonationMode == "" {
		in.ImpersonationMode = "passthrough"
	}
	k, err := a.Store.CreateUserKey(in)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, k)
}

func (a *API) getKey(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	k, err := a.Store.GetUserKey(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, k)
}

func (a *API) updateKey(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in store.UserKeyInput
	if err := c.BindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	k, err := a.Store.UpdateUserKey(id, in)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, k)
}

func (a *API) deleteKey(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := a.Store.DeleteUserKey(id); err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	a.reload()
	c.JSON(200, gin.H{"ok": true})
}

func (a *API) keyStats(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rangeQ := c.DefaultQuery("range", "7d")
	days := 7
	if rangeQ == "30d" {
		days = 30
	}
	from := time.Now().UTC().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	list, err := a.Store.KeyStats(id, from)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []store.KeyStatsDaily{}
	}
	c.JSON(200, gin.H{"items": list})
}

func (a *API) listPresets(c *gin.Context) {
	list, err := a.Store.ListPresets()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []store.ClientPreset{}
	}
	c.JSON(200, gin.H{"items": list})
}

func (a *API) createPreset(c *gin.Context) {
	var in store.PresetInput
	if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.Name) == "" {
		c.JSON(400, gin.H{"error": "name required"})
		return
	}
	p, err := a.Store.CreatePreset(in)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, p)
}

func (a *API) getPreset(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	p, err := a.Store.GetPreset(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, p)
}

func (a *API) updatePreset(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in store.PresetInput
	if err := c.BindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	p, err := a.Store.UpdatePreset(id, in)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, p)
}

func (a *API) deletePreset(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := a.Store.DeletePreset(id); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, gin.H{"ok": true})
}

func (a *API) listLogs(c *gin.Context) {
	f := store.LogFilter{
		Path:   c.Query("path"),
		Q:      c.Query("q"),
		Limit:  atoiDefault(c.Query("limit"), 50),
		Offset: atoiDefault(c.Query("offset"), 0),
	}
	if v := c.Query("user_key_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		f.UserKeyID = &id
	}
	if v := c.Query("channel_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		f.ChannelID = &id
	}
	if v := c.Query("status_min"); v != "" {
		n, _ := strconv.Atoi(v)
		f.StatusMin = &n
	}
	if v := c.Query("status_max"); v != "" {
		n, _ := strconv.Atoi(v)
		f.StatusMax = &n
	}
	items, total, err := a.Store.ListRequestMeta(f)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []store.RequestMeta{}
	}
	c.JSON(200, gin.H{"items": items, "total": total})
}

func (a *API) dashboard(c *gin.Context) {
	period := c.DefaultQuery("period", "7d")
	s, err := a.Store.DashboardSummary(period)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, s)
}

func (a *API) getSettings(c *gin.Context) {
	m, err := a.Store.ListSettings()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if m[store.SettingAutoTestLexicon] == "" {
		m[store.SettingAutoTestLexicon] = healthcheck.DefaultEnhancedLexiconJSON()
	}
	c.JSON(200, gin.H{"settings": m, "enhanced_lexicon_default": healthcheck.DefaultEnhancedLexiconJSON()})
}

func (a *API) patchSettings(c *gin.Context) {
	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	if raw, ok := req.Settings[store.SettingAutoTestLexicon]; ok {
		if _, err := healthcheck.ParseEnhancedLexicon(raw); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	if err := a.Store.SetSettings(req.Settings); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (a *API) previewEnhancedHealthcheck(c *gin.Context) {
	var req struct {
		Lexicon string `json:"lexicon"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	lex, err := healthcheck.ParseEnhancedLexicon(req.Lexicon)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"prompt": healthcheck.GenerateEnhancedPrompt(lex, nil)})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func (a *API) testChannel(c *gin.Context) {
	if a.Health == nil {
		c.JSON(500, gin.H{"error": "healthcheck not available"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := a.Health.TestOne(id)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, res)
}

func (a *API) runHealthcheck(c *gin.Context) {
	if a.Health == nil {
		c.JSON(500, gin.H{"error": "healthcheck not available"})
		return
	}
	a.Health.TriggerNow()
	c.JSON(200, gin.H{"ok": true, "message": "healthcheck triggered"})
}

func (a *API) healthcheckStatus(c *gin.Context) {
	if a.Health == nil {
		c.JSON(200, gin.H{"enabled": false})
		return
	}
	sum := a.Health.LastSummary()
	c.JSON(200, gin.H{
		"enabled":               a.Store.GetSettingBool(store.SettingAutoTestEnabled, false),
		"interval_minutes":      a.Store.GetSettingInt(store.SettingAutoTestIntervalMin, 10),
		"random_offset_minutes": a.Store.GetSettingInt(store.SettingAutoTestRandomOffset, 0),
		"prompt":                a.Store.GetSettingOr(store.SettingAutoTestPrompt, "hi"),
		"model":                 a.Store.GetSettingOr(store.SettingAutoTestModel, ""),
		"disable_status_codes":  a.Store.GetSettingOr(store.SettingAutoTestDisableCodes, "401,403,404,503"),
		"enhanced_enabled":      a.Store.GetSettingBool(store.SettingAutoTestEnhanced, false),
		"last_summary":          sum,
	})
}

func (a *API) getCapture(c *gin.Context) {
	if a.Capture == nil {
		c.JSON(500, gin.H{"error": "capture not available"})
		return
	}
	var captured any
	if cap, err := a.Capture.GetCaptured(); err == nil {
		captured = cap
	}
	c.JSON(200, gin.H{
		"enabled":  a.Capture.Enabled(),
		"armed":    a.Capture.Armed(),
		"key":      a.Capture.Key(),
		"captured": captured,
	})
}

func (a *API) enableCapture(c *gin.Context) {
	if a.Capture == nil {
		c.JSON(500, gin.H{"error": "capture not available"})
		return
	}
	if err := a.Capture.SetEnabled(true); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "enabled": true, "armed": true, "key": a.Capture.Key()})
}

func (a *API) disableCapture(c *gin.Context) {
	if a.Capture == nil {
		c.JSON(500, gin.H{"error": "capture not available"})
		return
	}
	if err := a.Capture.SetEnabled(false); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "enabled": false})
}

func (a *API) armCapture(c *gin.Context) {
	if a.Capture == nil {
		c.JSON(500, gin.H{"error": "capture not available"})
		return
	}
	if !a.Capture.Enabled() {
		c.JSON(400, gin.H{"error": "capture is disabled"})
		return
	}
	if err := a.Capture.Arm(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "armed": true})
}

func (a *API) clearCapture(c *gin.Context) {
	if a.Capture == nil {
		c.JSON(500, gin.H{"error": "capture not available"})
		return
	}
	_ = a.Capture.ClearCaptured()
	c.JSON(200, gin.H{"ok": true})
}

func (a *API) saveCapturePreset(c *gin.Context) {
	if a.Capture == nil {
		c.JSON(500, gin.H{"error": "capture not available"})
		return
	}
	cap, err := a.Capture.GetCaptured()
	if err != nil {
		c.JSON(400, gin.H{"error": "no captured request"})
		return
	}
	var req struct {
		Name         string `json:"name"`
		VersionLabel string `json:"version_label"`
		Description  string `json:"description"`
		Overwrite    bool   `json:"overwrite"`
	}
	_ = c.BindJSON(&req)
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "captured-" + time.Now().Format("20060102-1504")
	}
	if req.Description == "" {
		req.Description = "从请求捕获保存的客户端指纹"
	}
	hj, _ := json.Marshal(cap.Headers)
	if req.VersionLabel == "" {
		if ua, ok := cap.Headers["User-Agent"]; ok {
			req.VersionLabel = ua
			if len(req.VersionLabel) > 64 {
				req.VersionLabel = req.VersionLabel[:64]
			}
		}
	}
	// overwrite existing by name when requested (3.4B)
	if existing, err := a.Store.GetPresetByName(req.Name); err == nil {
		if !req.Overwrite {
			c.JSON(409, gin.H{"error": "preset name exists; set overwrite=true to replace", "id": existing.ID})
			return
		}
		p, err := a.Store.UpdatePreset(existing.ID, store.PresetInput{
			Name: req.Name, Description: req.Description, VersionLabel: req.VersionLabel,
			HeadersJSON: string(hj), RemoveHeaders: "[]",
		})
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		a.reload()
		c.JSON(200, p)
		return
	}
	p, err := a.Store.CreatePreset(store.PresetInput{
		Name: req.Name, Description: req.Description, VersionLabel: req.VersionLabel,
		HeadersJSON: string(hj), RemoveHeaders: "[]",
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a.reload()
	c.JSON(200, p)
}

var _ = http.StatusOK
