package healthcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"
)

type EnhancedLexicon struct {
	Prefix     []string `json:"prefix"`
	Modifier   []string `json:"modifier"`
	ModalWords []string `json:"modal_words"`
	ShortRules []string `json:"short_rules"`
	Targets    []string `json:"targets"`
}

var defaultEnhancedLexicon = EnhancedLexicon{
	Prefix: []string{
		"简单介绍一下", "简要讲讲", "麻烦简单说明", "粗略介绍下", "能不能简要说明", "简单概括一下", "简要介绍",
		"简单说说", "麻烦简单概括下", "简单阐述下", "能不能简单聊一聊", "方便简单介绍下", "说说看", "简单讲一讲",
	},
	Modifier:   []string{"", "大致", "大体上", "通俗来讲"},
	ModalWords: []string{"", "吧", "呢", "呀", "喔"},
	ShortRules: []string{"简短作答", "精简回答", "不要长篇介绍", "只用几句话说明", "尽量简短回复", "概括即可", "简洁说明"},
	Targets: []string{
		"c++", "java", "docker", "redis", "http", "mysql", "linux", "vue", "git", "python", "浏览器", "流媒体", "开源", "虚拟机",
		"二维码", "数据库", "域名", "缓存", "nginx", "react", "typescript", "element plus", "tailwindcss", "shadcn-ui", "springboot",
		"mybatis", "rabbitmq", "websocket", "https", "axios", "vite", "webpack", "jwt", "orm", "restful", "thread", "golang", "跨域问题",
		"内存泄漏", "死锁", "事务隔离级别", "数据库索引", "防抖", "节流", "cookie", "localstorage", "session", "消息队列", "分页查询",
		"乐观锁", "悲观锁", "分布式锁", "请求重试", "轮询", "代码热更新",
	},
}

func DefaultEnhancedLexiconJSON() string {
	b, _ := json.MarshalIndent(defaultEnhancedLexicon, "", "  ")
	return string(b)
}

func ParseEnhancedLexicon(raw string) (EnhancedLexicon, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var lex EnhancedLexicon
	if err := dec.Decode(&lex); err != nil {
		return EnhancedLexicon{}, fmt.Errorf("增强测活词库 JSON 无效: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return EnhancedLexicon{}, err
	}
	if err := validateLexicon(lex); err != nil {
		return EnhancedLexicon{}, err
	}
	return lex, nil
}

func ensureJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("增强测活词库 JSON 只能包含一个对象")
		}
		return fmt.Errorf("增强测活词库 JSON 无效: %w", err)
	}
	return nil
}

func validateLexicon(lex EnhancedLexicon) error {
	if len(lex.Prefix) == 0 || len(lex.ModalWords) == 0 || len(lex.ShortRules) == 0 || len(lex.Targets) == 0 {
		return fmt.Errorf("prefix、modal_words、short_rules、targets 必须是非空数组")
	}
	if len(lex.Modifier) == 0 {
		return fmt.Errorf("modifier 必须是非空数组")
	}
	for name, values := range map[string][]string{
		"prefix": lex.Prefix, "short_rules": lex.ShortRules, "targets": lex.Targets,
	} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s 不能包含空字符串", name)
			}
		}
	}
	return nil
}

type RandomSource interface {
	Float64() float64
	Intn(int) int
}

type globalRandom struct{}

func (globalRandom) Float64() float64 { return rand.Float64() }
func (globalRandom) Intn(n int) int   { return rand.Intn(n) }

func GenerateEnhancedPrompt(lex EnhancedLexicon, random RandomSource) string {
	if random == nil {
		random = globalRandom{}
	}
	prefix := choose(lex.Prefix, random)
	modifier := choose(lex.Modifier, random)
	target := choose(lex.Targets, random)

	segments := make([]string, 0, 4)
	if random.Intn(2) == 0 {
		segments = appendNonEmpty(segments, prefix, modifier, target)
	} else {
		segments = appendNonEmpty(segments, modifier, target, prefix)
	}
	if random.Float64() < 0.4 {
		rule := choose(lex.ShortRules, random)
		if random.Intn(2) == 0 {
			segments = append([]string{rule}, segments...)
		} else {
			segments = append(segments, rule)
		}
	}
	for i := range segments {
		if random.Float64() < 0.3 {
			segments[i] += choose(lex.ModalWords, random)
		}
	}
	var out strings.Builder
	for i, segment := range segments {
		if i > 0 && random.Float64() < 0.6 {
			out.WriteString("，")
		}
		out.WriteString(segment)
	}
	if random.Float64() < 0.3 {
		out.WriteString("。")
	}
	return out.String()
}

func choose(values []string, random RandomSource) string {
	return values[random.Intn(len(values))]
}

func appendNonEmpty(dst []string, values ...string) []string {
	for _, value := range values {
		if value != "" {
			dst = append(dst, value)
		}
	}
	return dst
}

func streamHasContent(contentType string, body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") || bytes.Contains(trimmed, []byte("data:")) {
		for _, line := range bytes.Split(body, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
				continue
			}
			var value any
			if json.Unmarshal(payload, &value) == nil && eventHasContent(value) {
				return true
			}
		}
		return false
	}
	return !bytes.Equal(trimmed, []byte("[DONE]"))
}

func eventHasContent(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			switch key {
			case "content", "text", "delta", "output_text":
				if contentValueNonEmpty(child) {
					return true
				}
			}
			if eventHasContent(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if eventHasContent(child) {
				return true
			}
		}
	}
	return false
}

func contentValueNonEmpty(value any) bool {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case map[string]any, []any:
		return eventHasContent(v)
	}
	return false
}
