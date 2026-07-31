package preset

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

const SchemaVersion = 1

type Document struct {
	SchemaVersion int                      `json:"schema_version"`
	Headers       map[string]HeaderRule    `json:"headers"`
	RemoveHeaders []string                 `json:"remove_headers"`
	Generators    map[string]GeneratorRule `json:"generators"`
}

type HeaderRule struct {
	Value               any             `json:"value"`
	FillMissing         bool            `json:"fill_missing"`
	ChildrenFillMissing map[string]bool `json:"children_fill_missing,omitempty"`
}

type GeneratorRule struct {
	Type             string   `json:"type"`
	Charset          string   `json:"charset,omitempty"`
	Length           int      `json:"length,omitempty"`
	ExcludeAmbiguous bool     `json:"exclude_ambiguous,omitempty"`
	Exclude          []string `json:"exclude,omitempty"`
	Mode             string   `json:"mode"`
	Interval         string   `json:"interval,omitempty"`
	Step             int64    `json:"step,omitempty"`
	Overflow         string   `json:"overflow,omitempty"`
}

var generatorNameRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var templateRE = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

func LegacyDocument(headersJSON, removeJSON string) (Document, error) {
	var headers map[string]any
	if strings.TrimSpace(headersJSON) == "" {
		headersJSON = "{}"
	}
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		return Document{}, fmt.Errorf("旧版 Headers JSON 无效: %w", err)
	}
	var remove []string
	if strings.TrimSpace(removeJSON) == "" {
		removeJSON = "[]"
	}
	if err := json.Unmarshal([]byte(removeJSON), &remove); err != nil {
		return Document{}, fmt.Errorf("旧版 Remove Headers JSON 无效: %w", err)
	}
	doc := Document{SchemaVersion: SchemaVersion, Headers: map[string]HeaderRule{}, RemoveHeaders: remove, Generators: map[string]GeneratorRule{}}
	for name, value := range headers {
		doc.Headers[name] = HeaderRule{Value: value, FillMissing: false}
	}
	return doc, Validate(doc)
}

func Parse(raw string) (Document, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("预设规则 JSON 无效: %w", err)
	}
	if doc.Headers == nil {
		doc.Headers = map[string]HeaderRule{}
	}
	if doc.RemoveHeaders == nil {
		doc.RemoveHeaders = []string{}
	}
	if doc.Generators == nil {
		doc.Generators = map[string]GeneratorRule{}
	}
	if err := Validate(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func Normalize(raw string) (string, Document, error) {
	doc, err := Parse(raw)
	if err != nil {
		return "", Document{}, err
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return string(b), doc, nil
}

func Marshal(doc Document) string {
	b, _ := json.MarshalIndent(doc, "", "  ")
	return string(b)
}

func Validate(doc Document) error {
	if doc.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version 必须为 %d", SchemaVersion)
	}
	seen := map[string]string{}
	for name, rule := range doc.Headers {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return fmt.Errorf("headers 不能包含空名称")
		}
		lower := strings.ToLower(trimmed)
		if old, ok := seen[lower]; ok {
			return fmt.Errorf("Header 名称大小写重复: %s / %s", old, name)
		}
		seen[lower] = name
		if IsProtected(name) {
			return fmt.Errorf("受保护 Header 不能由预设写入: %s", name)
		}
		if err := validateValue(rule.Value, "headers."+name+".value", doc); err != nil {
			return err
		}
		for path := range rule.ChildrenFillMissing {
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("headers.%s.children_fill_missing 包含空路径", name)
			}
		}
	}
	removeSeen := map[string]bool{}
	for _, name := range doc.RemoveHeaders {
		lower := strings.ToLower(strings.TrimSpace(name))
		if lower == "" {
			return fmt.Errorf("remove_headers 不能包含空名称")
		}
		if removeSeen[lower] {
			return fmt.Errorf("remove_headers 大小写重复: %s", name)
		}
		removeSeen[lower] = true
		if IsProtected(name) {
			return fmt.Errorf("受保护 Header 不能删除: %s", name)
		}
	}
	for name, rule := range doc.Generators {
		if !generatorNameRE.MatchString(name) {
			return fmt.Errorf("generators.%s 名称无效", name)
		}
		if err := validateGenerator(name, rule); err != nil {
			return err
		}
	}
	if err := validateResolvedDependencies(doc); err != nil {
		return err
	}
	return nil
}

func validateGenerator(name string, g GeneratorRule) error {
	if g.Type != "uuid" && g.Type != "random" {
		return fmt.Errorf("generators.%s.type 必须是 uuid 或 random", name)
	}
	if g.Mode != "request" && g.Mode != "interval" && g.Mode != "increment" && g.Mode != "fixed" {
		return fmt.Errorf("generators.%s.mode 无效", name)
	}
	if g.Type == "random" {
		if g.Length < 1 || g.Length > 256 {
			return fmt.Errorf("generators.%s.length 必须在 1 到 256 之间", name)
		}
		validCharset := map[string]bool{"digits": true, "lowercase": true, "uppercase": true, "letters": true, "alnum": true}
		if !validCharset[g.Charset] {
			return fmt.Errorf("generators.%s.charset 无效", name)
		}
	}
	if g.Mode == "increment" {
		if g.Type != "random" || g.Charset != "digits" {
			return fmt.Errorf("generators.%s 递增模式仅支持数字随机生成器", name)
		}
		if g.Step <= 0 {
			return fmt.Errorf("generators.%s.step 必须大于 0", name)
		}
		if g.Overflow == "" {
			g.Overflow = "wrap"
		}
		if g.Overflow != "wrap" && g.Overflow != "regenerate" && g.Overflow != "expand" && g.Overflow != "error" {
			return fmt.Errorf("generators.%s.overflow 无效", name)
		}
	}
	if g.Mode == "interval" {
		d, err := parseDuration(g.Interval)
		if err != nil || d < minInterval || d > maxInterval {
			return fmt.Errorf("generators.%s.interval 必须在 1s 到 365d 之间", name)
		}
	}
	return nil
}

func validateValue(value any, path string, doc Document) error {
	switch v := value.(type) {
	case string:
		for _, m := range templateRE.FindAllStringSubmatch(v, -1) {
			expr := m[1]
			switch {
			case expr == "version":
			case strings.HasPrefix(expr, "client_header:"):
				if IsProtected(strings.TrimPrefix(expr, "client_header:")) {
					return fmt.Errorf("%s 不能引用受保护 Header", path)
				}
			case strings.HasPrefix(expr, "resolved_header:"):
				if IsProtected(strings.TrimPrefix(expr, "resolved_header:")) {
					return fmt.Errorf("%s 不能引用受保护 Header", path)
				}
			case strings.HasPrefix(expr, "generator:"):
				name := strings.TrimPrefix(expr, "generator:")
				if _, ok := doc.Generators[name]; !ok {
					return fmt.Errorf("%s 引用了不存在的生成器 %s", path, name)
				}
			case strings.HasPrefix(expr, "time:"):
				valid := map[string]bool{"year": true, "month": true, "day": true, "hour": true, "minute": true, "second": true, "millisecond": true, "unix": true, "unix_ms": true}
				if !valid[strings.TrimPrefix(expr, "time:")] {
					return fmt.Errorf("%s 包含未知时间变量 %s", path, expr)
				}
			default:
				return fmt.Errorf("%s 包含未知模板变量 %s", path, expr)
			}
		}
	case []any:
		for i, child := range v {
			if err := validateValue(child, fmt.Sprintf("%s.%d", path, i), doc); err != nil {
				return err
			}
		}
	case map[string]any:
		for key, child := range v {
			if err := validateValue(child, path+"."+key, doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateResolvedDependencies(doc Document) error {
	deps := map[string][]string{}
	canonical := map[string]string{}
	for name := range doc.Headers {
		canonical[strings.ToLower(name)] = name
	}
	for name, rule := range doc.Headers {
		refs := collectResolvedRefs(rule.Value)
		for _, ref := range refs {
			resolved, ok := canonical[strings.ToLower(ref)]
			if !ok {
				return fmt.Errorf("Header %s 引用了不存在的 resolved_header:%s", name, ref)
			}
			deps[name] = append(deps[name], resolved)
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("resolved_header 存在循环引用: %s", name)
		}
		if done[name] {
			return nil
		}
		visiting[name] = true
		for _, dep := range deps[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[name] = false
		done[name] = true
		return nil
	}
	for name := range doc.Headers {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func HeaderOrder(doc Document) []string {
	canonical := map[string]string{}
	for name := range doc.Headers {
		canonical[strings.ToLower(name)] = name
	}
	deps := map[string][]string{}
	for name, rule := range doc.Headers {
		for _, ref := range collectResolvedRefs(rule.Value) {
			if dep, ok := canonical[strings.ToLower(ref)]; ok {
				deps[name] = append(deps[name], dep)
			}
		}
	}
	var out []string
	done := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if done[name] {
			return
		}
		for _, dep := range deps[name] {
			visit(dep)
		}
		done[name] = true
		out = append(out, name)
	}
	names := make([]string, 0, len(doc.Headers))
	for name := range doc.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		visit(name)
	}
	return out
}

func collectResolvedRefs(value any) []string {
	var refs []string
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			for _, m := range templateRE.FindAllStringSubmatch(x, -1) {
				if strings.HasPrefix(m[1], "resolved_header:") {
					refs = append(refs, strings.TrimPrefix(m[1], "resolved_header:"))
				}
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		case map[string]any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(value)
	return refs
}

var protected = map[string]bool{
	"authorization": true, "x-api-key": true, "host": true, "content-length": true, "accept-encoding": true,
	"connection": true, "keep-alive": true, "proxy-authenticate": true, "proxy-authorization": true,
	"te": true, "trailer": true, "trailers": true, "transfer-encoding": true, "upgrade": true, "proxy-connection": true,
}

func IsProtected(name string) bool { return protected[strings.ToLower(strings.TrimSpace(name))] }

func CanonicalHeaderName(name string) string { return http.CanonicalHeaderKey(strings.TrimSpace(name)) }
