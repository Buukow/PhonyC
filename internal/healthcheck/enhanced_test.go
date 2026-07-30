package healthcheck

import (
	"strings"
	"testing"
)

type sequenceRandom struct {
	ints   []int
	floats []float64
	i      int
	f      int
}

func (r *sequenceRandom) Intn(n int) int {
	v := r.ints[r.i]
	r.i++
	return v % n
}

func (r *sequenceRandom) Float64() float64 {
	v := r.floats[r.f]
	r.f++
	return v
}

func TestEnhancedLexiconValidation(t *testing.T) {
	if _, err := ParseEnhancedLexicon(DefaultEnhancedLexiconJSON()); err != nil {
		t.Fatal(err)
	}
	bad := []string{
		`{"prefix":[]}`,
		`{"prefix":["a"],"modifier":[],"modal_words":[""],"short_rules":["s"],"targets":["t"]}`,
		`{"prefix":["a"],"modifier":[],"modal_words":[""],"short_rules":["s"],"targets":["t"],"punctuation":["。"]}`,
		`{"prefix":[""],"modifier":[],"modal_words":[""],"short_rules":["s"],"targets":["t"]}`,
	}
	for _, raw := range bad {
		if _, err := ParseEnhancedLexicon(raw); err == nil {
			t.Fatalf("expected invalid lexicon: %s", raw)
		}
	}
}

func TestGenerateEnhancedPromptRules(t *testing.T) {
	lex := EnhancedLexicon{
		Prefix: []string{"介绍"}, Modifier: []string{"大致"}, ModalWords: []string{"吧"}, ShortRules: []string{"简短作答"}, Targets: []string{"docker"},
	}
	r := &sequenceRandom{
		// choices prefix/modifier/target; template 1; short choice; short at start; modal choices.
		ints: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// include short; modal on all four; commas between all; terminal period.
		floats: []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
	}
	got := GenerateEnhancedPrompt(lex, r)
	want := "简短作答吧，介绍吧，大致吧，docker吧。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestGenerateEnhancedPromptInvertedEmptyModifier(t *testing.T) {
	lex := EnhancedLexicon{
		Prefix: []string{"介绍"}, Modifier: []string{""}, ModalWords: []string{""}, ShortRules: []string{"简短"}, Targets: []string{"git"},
	}
	r := &sequenceRandom{
		ints:   []int{0, 0, 0, 1},
		floats: []float64{0.9, 0.9, 0.9, 0.9, 0.9},
	}
	if got := GenerateEnhancedPrompt(lex, r); got != "git介绍" {
		t.Fatalf("got %q", got)
	}
}

func TestStreamHasContent(t *testing.T) {
	invalid := []string{
		"", "data: [DONE]\n\n", ": heartbeat\n\n", "data:\n\n",
		`data: {"choices":[{"delta":{"content":""}}]}`,
	}
	for _, body := range invalid {
		if streamHasContent("text/event-stream", []byte(body)) {
			t.Fatalf("unexpected content in %q", body)
		}
	}
	valid := `data: {"choices":[{"delta":{"content":"hello"}}]}`
	if !streamHasContent("text/event-stream", []byte(valid)) {
		t.Fatal("valid stream content not recognized")
	}
	if !streamHasContent("application/octet-stream", []byte("chunk")) {
		t.Fatal("non-SSE chunk not recognized")
	}
	if streamHasContent("application/octet-stream", []byte(strings.Repeat(" ", 3))) {
		t.Fatal("whitespace chunk recognized")
	}
}
