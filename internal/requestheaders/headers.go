package requestheaders

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/phonyg/phonyg/internal/preset"
	"github.com/phonyg/phonyg/internal/protocol"
	"github.com/phonyg/phonyg/internal/snapshot"
	"github.com/phonyg/phonyg/internal/store"
)

var hopHeaders = map[string]bool{
	"connection": true, "keep-alive": true, "proxy-authenticate": true,
	"proxy-authorization": true, "te": true, "trailers": true,
	"transfer-encoding": true, "upgrade": true, "proxy-connection": true,
}

func IsHop(name string) bool { return hopHeaders[strings.ToLower(name)] }

func isAuthHeader(name string) bool {
	l := strings.ToLower(name)
	return l == "authorization" || l == "x-api-key"
}

// BuildUpstreamHeaders builds final upstream headers for proxy and healthcheck requests.
func BuildUpstreamHeaders(client http.Header, ch store.Channel, key store.UserKey, snap *snapshot.Snapshot, contentLength int64) http.Header {
	out := make(http.Header)
	mode := key.ImpersonationMode
	if mode == "" {
		mode = store.ImpersonationModePassthrough
	}

	if mode == store.ImpersonationModePassthrough {
		for k, vv := range client {
			if IsHop(k) || isAuthHeader(k) || strings.EqualFold(k, "host") ||
				strings.EqualFold(k, "content-length") || strings.EqualFold(k, "accept-encoding") {
				continue
			}
			for _, v := range vv {
				out.Add(k, v)
			}
		}
	}

	protocol.Get(ch.Protocol).ApplyAuth(out, ch.APIKey)

	for k, v := range parseHeaderMap(ch.ExtraHeadersJSON) {
		if IsHop(k) || strings.EqualFold(k, "host") || strings.EqualFold(k, "content-length") {
			continue
		}
		out.Set(k, v)
	}

	if mode == store.ImpersonationModePreset || mode == store.ImpersonationModeCustom {
		var tmpl map[string]string
		var remove []string
		if mode == store.ImpersonationModePreset && key.PresetID != nil && snap != nil {
			if p := snap.PresetByID[*key.PresetID]; p != nil {
				if strings.TrimSpace(p.RuleJSON) != "" {
					doc, err := preset.Parse(p.RuleJSON)
					if err == nil {
						resolved, _, resolveErr := (preset.Resolver{}).Resolve(p.ID, p.VersionLabel, doc, client, out, time.Now())
						if resolveErr == nil {
							if contentLength >= 0 {
								resolved.Set("Content-Length", strconv.FormatInt(contentLength, 10))
							}
							resolved.Del("Accept-Encoding")
							return resolved
						}
					}
				}
				tmpl = renderTemplateMap(parseHeaderMap(p.HeadersJSON), p.VersionLabel)
				_ = json.Unmarshal([]byte(p.RemoveHeaders), &remove)
			}
		} else if mode == store.ImpersonationModeCustom {
			tmpl = parseHeaderMap(key.CustomHeadersJSON)
		}
		for _, r := range remove {
			if !isAuthHeader(r) {
				out.Del(r)
			}
		}
		for k, v := range tmpl {
			if IsHop(k) || isAuthHeader(k) || strings.EqualFold(k, "host") || strings.EqualFold(k, "content-length") {
				continue
			}
			out.Set(k, v)
		}
	}

	if contentLength >= 0 {
		out.Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	out.Del("Accept-Encoding")
	return out
}

func parseHeaderMap(s string) map[string]string {
	out := map[string]string{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func renderTemplateMap(in map[string]string, version string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = strings.ReplaceAll(v, "{{version}}", version)
	}
	return out
}
