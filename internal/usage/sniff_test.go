package usage

import "testing"

func TestParseOpenAI(t *testing.T) {
	in := []byte(`{"id":"x","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":2},"completion_tokens_details":{"reasoning_tokens":1}}}`)
	tok := ParseTopLevelJSON(in)
	if tok.PromptTokens != 10 || tok.CompletionTokens != 5 || tok.TotalTokens != 15 || tok.CachedTokens != 2 || tok.ReasoningTokens != 1 {
		t.Fatalf("got %+v", tok)
	}
}

func TestParseAnthropic(t *testing.T) {
	in := []byte(`{"id":"msg","type":"message","usage":{"input_tokens":8,"output_tokens":3,"cache_read_input_tokens":4}}`)
	tok := ParseTopLevelJSON(in)
	if tok.PromptTokens != 8 || tok.CompletionTokens != 3 || tok.CachedTokens != 4 || tok.TotalTokens != 11 {
		t.Fatalf("got %+v", tok)
	}
}

func TestStreamSnifferOpenAI(t *testing.T) {
	s := NewSniffer(true)
	payload := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: {\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\n" +
		"data: [DONE]\n\n"
	_, _ = s.Write([]byte(payload))
	tok := s.Result()
	if tok.TotalTokens != 5 || tok.PromptTokens != 3 {
		t.Fatalf("got %+v", tok)
	}
}

func TestStreamSnifferAnthropicMerge(t *testing.T) {
	s := NewSniffer(true)
	_, _ = s.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":9,\"output_tokens\":0}}}\n\n"))
	_, _ = s.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":4}}\n\n"))
	tok := s.Result()
	if tok.PromptTokens != 9 || tok.CompletionTokens != 4 || tok.TotalTokens != 13 {
		t.Fatalf("got %+v", tok)
	}
}
