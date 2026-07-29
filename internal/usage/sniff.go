package usage

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Tokens mirrors the NewAPI-facing subset we persist for MVP.
// First version: real upstream usage only (no local estimation).
type Tokens struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CachedTokens     int `json:"cached_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens"`
}

func (t Tokens) Empty() bool {
	return t.PromptTokens == 0 && t.CompletionTokens == 0 && t.TotalTokens == 0 &&
		t.CachedTokens == 0 && t.ReasoningTokens == 0
}

func (t *Tokens) Normalize() {
	if t.TotalTokens == 0 {
		t.TotalTokens = t.PromptTokens + t.CompletionTokens
	}
}

// Merge prefers newer non-zero fields (stream deltas).
func (t *Tokens) Merge(other Tokens) {
	if other.PromptTokens > 0 {
		t.PromptTokens = other.PromptTokens
	}
	if other.CompletionTokens > 0 {
		t.CompletionTokens = other.CompletionTokens
	}
	if other.CachedTokens > 0 {
		t.CachedTokens = other.CachedTokens
	}
	if other.ReasoningTokens > 0 {
		t.ReasoningTokens = other.ReasoningTokens
	}
	sum := t.PromptTokens + t.CompletionTokens
	// Only keep an explicit total when it is at least the known sum (full final usage).
	if other.TotalTokens > sum {
		t.TotalTokens = other.TotalTokens
	} else {
		t.TotalTokens = sum
	}
}

type rawUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`

	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
	InputTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`

	// Anthropic cache fields (top-level usage object)
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

func fromRaw(u *rawUsage) Tokens {
	if u == nil {
		return Tokens{}
	}
	t := Tokens{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}
	if t.PromptTokens == 0 && u.InputTokens > 0 {
		t.PromptTokens = u.InputTokens
	}
	if t.CompletionTokens == 0 && u.OutputTokens > 0 {
		t.CompletionTokens = u.OutputTokens
	}
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		t.CachedTokens = u.PromptTokensDetails.CachedTokens
	}
	if u.InputTokensDetails != nil && u.InputTokensDetails.CachedTokens > 0 {
		t.CachedTokens = u.InputTokensDetails.CachedTokens
	}
	if u.CacheReadInputTokens > 0 {
		t.CachedTokens = u.CacheReadInputTokens
	}
	if u.CompletionTokensDetails != nil && u.CompletionTokensDetails.ReasoningTokens > 0 {
		t.ReasoningTokens = u.CompletionTokensDetails.ReasoningTokens
	}
	t.Normalize()
	return t
}

// ParseTopLevelJSON extracts usage from a full JSON body (non-stream).
// Only looks at top-level "usage" (and Anthropic message envelope is itself top-level).
func ParseTopLevelJSON(b []byte) Tokens {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return Tokens{}
	}
	var envelope struct {
		Usage   *rawUsage `json:"usage"`
		Type    string    `json:"type"`
		Message *struct {
			Usage *rawUsage `json:"usage"`
		} `json:"message"`
		// OpenAI responses API completed object sometimes nests response.usage
		Response *struct {
			Usage *rawUsage `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return Tokens{}
	}
	if envelope.Usage != nil {
		return fromRaw(envelope.Usage)
	}
	// Anthropic stream-style single event stored as JSON
	if envelope.Message != nil && envelope.Message.Usage != nil {
		return fromRaw(envelope.Message.Usage)
	}
	if envelope.Response != nil && envelope.Response.Usage != nil {
		return fromRaw(envelope.Response.Usage)
	}
	return Tokens{}
}

// ParseSSEData parses one SSE data payload (without the "data:" prefix).
func ParseSSEData(data string) Tokens {
	data = strings.TrimSpace(data)
	if data == "" || data == "[DONE]" {
		return Tokens{}
	}
	return ParseTopLevelJSON([]byte(data))
}

// Sniffer tees response bytes and extracts usage without blocking the client stream.
type Sniffer struct {
	sse    bool
	buf    []byte
	acc    Tokens
	has    bool
	maxBuf int
}

func NewSniffer(isSSE bool) *Sniffer {
	return &Sniffer{sse: isSSE, maxBuf: 8 << 20} // 8MiB cap for non-SSE accumulate
}

func (s *Sniffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if s.sse {
		s.buf = append(s.buf, p...)
		for {
			i := bytes.IndexByte(s.buf, '\n')
			if i < 0 {
				break
			}
			line := string(s.buf[:i])
			s.buf = s.buf[i+1:]
			s.consumeSSELine(line)
		}
		// keep buf bounded
		if len(s.buf) > 1<<20 {
			s.buf = s.buf[len(s.buf)-64*1024:]
		}
		return len(p), nil
	}
	// non-SSE: accumulate (capped) and parse at Result()
	if len(s.buf) < s.maxBuf {
		need := s.maxBuf - len(s.buf)
		if need > len(p) {
			need = len(p)
		}
		s.buf = append(s.buf, p[:need]...)
	}
	return len(p), nil
}

func (s *Sniffer) consumeSSELine(line string) {
	line = strings.TrimRight(line, "\r")
	if !strings.HasPrefix(line, "data:") {
		return
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	tok := ParseSSEData(data)
	if tok.Empty() {
		// still try to detect anthropic event types with only partial usage
		return
	}
	s.acc.Merge(tok)
	s.has = true
}

func (s *Sniffer) Result() Tokens {
	if s.sse {
		// flush remainder
		if len(s.buf) > 0 {
			s.consumeSSELine(string(s.buf))
			s.buf = nil
		}
		out := s.acc
		out.Normalize()
		return out
	}
	return ParseTopLevelJSON(s.buf)
}
