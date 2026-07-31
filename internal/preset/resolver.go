package preset

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Diff struct {
	Added      []string          `json:"added"`
	Overridden []string          `json:"overridden"`
	Preserved  []string          `json:"preserved"`
	Removed    []string          `json:"removed"`
	Skipped    []string          `json:"skipped"`
	Generated  map[string]string `json:"generated"`
}

type Resolver struct{ Generators *GeneratorManager }

func (r Resolver) Resolve(presetID int64, version string, doc Document, client http.Header, initial http.Header, now time.Time) (http.Header, Diff, error) {
	if r.Generators == nil {
		r.Generators = DefaultGenerators
	}
	out := initial.Clone()
	diff := Diff{Generated: map[string]string{}}
	for _, name := range doc.RemoveHeaders {
		if out.Get(name) != "" {
			diff.Removed = append(diff.Removed, CanonicalHeaderName(name))
		}
		out.Del(name)
	}
	reqCtx := NewRequestContext(now)
	for _, name := range HeaderOrder(doc) {
		rule := doc.Headers[name]
		value, err := r.resolveValue(presetID, version, rule.Value, client, out, doc, reqCtx, &diff)
		if err != nil {
			return nil, diff, fmt.Errorf("Header %s: %w", name, err)
		}
		clientRaw, exists := headerLookup(client, name)
		final := value
		if objectValue, ok := value.(map[string]any); ok && exists {
			var clientObject map[string]any
			if err := json.Unmarshal([]byte(clientRaw), &clientObject); err != nil {
				return nil, diff, fmt.Errorf("客户端 Header 不是合法 JSON: %w", err)
			}
			final = mergeValue(objectValue, clientObject, rule.FillMissing, rule.ChildrenFillMissing, "")
		} else if arrayValue, ok := value.([]any); ok && exists {
			var clientArray []any
			if err := json.Unmarshal([]byte(clientRaw), &clientArray); err != nil {
				return nil, diff, fmt.Errorf("客户端 Header 不是合法 JSON: %w", err)
			}
			final = mergeValue(arrayValue, clientArray, rule.FillMissing, rule.ChildrenFillMissing, "")
		} else if rule.FillMissing && exists {
			final = clientRaw
			diff.Preserved = append(diff.Preserved, CanonicalHeaderName(name))
		}
		serialized, err := serializeHeaderValue(final)
		if err != nil {
			return nil, diff, err
		}
		if exists {
			if !rule.FillMissing || serialized != clientRaw {
				diff.Overridden = append(diff.Overridden, CanonicalHeaderName(name))
			}
		} else {
			diff.Added = append(diff.Added, CanonicalHeaderName(name))
		}
		out.Set(name, serialized)
	}
	return out, diff, nil
}

func (r Resolver) resolveValue(presetID int64, version string, value any, client, resolved http.Header, doc Document, req *RequestContext, diff *Diff) (any, error) {
	switch v := value.(type) {
	case string:
		if matches := templateRE.FindStringSubmatch(v); len(matches) == 2 && matches[0] == v && strings.HasPrefix(matches[1], "time_number:") {
			switch strings.TrimPrefix(matches[1], "time_number:") {
			case "unix":
				return req.Now.Unix(), nil
			case "unix_ms":
				return req.Now.UnixMilli(), nil
			}
		}
		var resolveErr error
		result := templateRE.ReplaceAllStringFunc(v, func(token string) string {
			expr := strings.TrimSuffix(strings.TrimPrefix(token, "{{"), "}}")
			switch {
			case expr == "version":
				return version
			case strings.HasPrefix(expr, "client_header:"):
				return client.Get(strings.TrimPrefix(expr, "client_header:"))
			case strings.HasPrefix(expr, "resolved_header:"):
				return resolved.Get(strings.TrimPrefix(expr, "resolved_header:"))
			case strings.HasPrefix(expr, "generator:"):
				name := strings.TrimPrefix(expr, "generator:")
				generated, err := r.Generators.Value(presetID, name, doc.Generators[name], req)
				if err != nil {
					resolveErr = err
					return ""
				}
				diff.Generated[name] = generated
				return generated
			case strings.HasPrefix(expr, "time:"):
				return formatTimeExpression(req.Now, strings.TrimPrefix(expr, "time:"))
			}
			return token
		})
		return result, resolveErr
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			resolvedChild, err := r.resolveValue(presetID, version, child, client, resolved, doc, req, diff)
			if err != nil {
				return nil, err
			}
			out[i] = resolvedChild
		}
		return out, nil
	case map[string]any:
		out := map[string]any{}
		for key, child := range v {
			resolvedChild, err := r.resolveValue(presetID, version, child, client, resolved, doc, req, diff)
			if err != nil {
				return nil, err
			}
			out[key] = resolvedChild
		}
		return out, nil
	default:
		return value, nil
	}
}

func mergeValue(presetValue, clientValue any, inherited bool, overrides map[string]bool, path string) any {
	fill := inherited
	if explicit, ok := overrides[path]; ok && path != "" {
		fill = explicit
	}
	switch p := presetValue.(type) {
	case map[string]any:
		c, ok := clientValue.(map[string]any)
		if !ok {
			if fill && clientValue != nil {
				return clientValue
			}
			return p
		}
		out := map[string]any{}
		for key, value := range c {
			out[key] = value
		}
		for key, value := range p {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			clientChild, exists := c[key]
			if !exists {
				out[key] = value
				continue
			}
			out[key] = mergeValue(value, clientChild, fill, overrides, childPath)
		}
		return out
	case []any:
		c, ok := clientValue.([]any)
		if !ok {
			if fill && clientValue != nil {
				return clientValue
			}
			return p
		}
		length := len(c)
		if len(p) > length {
			length = len(p)
		}
		out := make([]any, length)
		copy(out, c)
		for i, value := range p {
			childPath := strconv.Itoa(i)
			if path != "" {
				childPath = path + "." + childPath
			}
			if i >= len(c) {
				out[i] = value
			} else {
				out[i] = mergeValue(value, c[i], fill, overrides, childPath)
			}
		}
		return out
	default:
		if fill && clientValue != nil {
			return clientValue
		}
		return presetValue
	}
}

func serializeHeaderValue(value any) (string, error) {
	if s, ok := value.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(value)
	return string(b), err
}

func headerLookup(h http.Header, name string) (string, bool) {
	for key, values := range h {
		if strings.EqualFold(key, name) {
			if len(values) == 0 {
				return "", true
			}
			return values[0], true
		}
	}
	return "", false
}

func formatTimeExpression(t time.Time, kind string) string {
	switch kind {
	case "year":
		return t.Format("2006")
	case "month":
		return t.Format("01")
	case "day":
		return t.Format("02")
	case "hour":
		return t.Format("15")
	case "minute":
		return t.Format("04")
	case "second":
		return t.Format("05")
	case "millisecond":
		return fmt.Sprintf("%03d", t.Nanosecond()/int(time.Millisecond))
	case "unix":
		return strconv.FormatInt(t.Unix(), 10)
	case "unix_ms":
		return strconv.FormatInt(t.UnixMilli(), 10)
	}
	return ""
}
