package upstream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/phonyg/phonyg/internal/protocol"
)

type FetchRequest struct {
	BaseURL          string
	APIKey           string
	Protocol         string
	ExtraHeadersJSON string
	Timeout          time.Duration
}

// FetchModelIDs calls OpenAI-compatible GET {base}/v1/models and returns model ids.
func FetchModelIDs(req FetchRequest) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("base_url required")
	}
	url := base + "/v1/models"
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	proto := strings.ToLower(strings.TrimSpace(req.Protocol))
	if proto == "" {
		proto = protocol.OpenAI
	}
	protocol.Get(proto).ApplyAuth(httpReq.Header, req.APIKey)
	httpReq.Header.Set("Accept", "application/json")
	if strings.TrimSpace(req.ExtraHeadersJSON) != "" && req.ExtraHeadersJSON != "{}" {
		var extra map[string]string
		if err := json.Unmarshal([]byte(req.ExtraHeadersJSON), &extra); err == nil {
			for k, v := range extra {
				if strings.TrimSpace(k) != "" {
					httpReq.Header.Set(k, v)
				}
			}
		}
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 300 {
			msg = msg[:300] + "..."
		}
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, msg)
	}
	ids, err := parseModelIDs(body)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func parseModelIDs(body []byte) ([]string, error) {
	var openai struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openai); err == nil && len(openai.Data) > 0 {
		ids := make([]string, 0, len(openai.Data))
		for _, d := range openai.Data {
			if d.ID != "" {
				ids = append(ids, d.ID)
			}
		}
		return normalize(ids), nil
	}

	var alt struct {
		Models json.RawMessage `json:"models"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &alt); err == nil {
		if ids := parseFlexibleList(alt.Models); len(ids) > 0 {
			return normalize(ids), nil
		}
		if ids := parseFlexibleList(alt.Data); len(ids) > 0 {
			return normalize(ids), nil
		}
	}
	if ids := parseFlexibleList(json.RawMessage(body)); len(ids) > 0 {
		return normalize(ids), nil
	}
	return nil, fmt.Errorf("unrecognized models response")
}

func parseFlexibleList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil {
		return strs
	}
	var objs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &objs); err == nil {
		out := make([]string, 0, len(objs))
		for _, o := range objs {
			id := o.ID
			if id == "" {
				id = o.Name
			}
			if id != "" {
				out = append(out, id)
			}
		}
		return out
	}
	return nil
}

func normalize(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		id = strings.TrimPrefix(id, "models/")
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
