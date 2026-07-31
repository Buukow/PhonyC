package preset

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLegacyDocumentAndProtectedHeaders(t *testing.T) {
	doc, err := LegacyDocument(`{"User-Agent":"client/{{version}}"}`, `["X-Old"]`)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Headers["User-Agent"].FillMissing {
		t.Fatal("legacy header must force override")
	}
	doc.Headers["Authorization"] = HeaderRule{Value: "bad"}
	if err := Validate(doc); err == nil {
		t.Fatal("expected protected header validation error")
	}
}

func TestResolveFillMissingAndNestedJSON(t *testing.T) {
	doc := Document{
		SchemaVersion: 1,
		Headers: map[string]HeaderRule{
			"User-Agent": {Value: "preset", FillMissing: true},
			"X-Force":    {Value: "forced", FillMissing: false},
			"X-Meta": {
				Value:               map[string]any{"session": "preset-session", "turn": "preset-turn"},
				FillMissing:         true,
				ChildrenFillMissing: map[string]bool{"turn": false},
			},
		},
		RemoveHeaders: []string{"X-Remove"}, Generators: map[string]GeneratorRule{},
	}
	client := http.Header{
		"User-Agent": []string{"client"}, "X-Force": []string{"client-force"},
		"X-Meta": []string{`{"session":"client-session","turn":"client-turn"}`}, "X-Remove": []string{"old"},
	}
	out, _, err := (Resolver{Generators: NewGeneratorManager()}).Resolve(1, "1.0", doc, client, http.Header{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if out.Get("User-Agent") != "client" || out.Get("X-Force") != "forced" {
		t.Fatalf("unexpected headers: %v", out)
	}
	if got := out.Get("X-Meta"); !strings.Contains(got, `"session":"client-session"`) || !strings.Contains(got, `"turn":"preset-turn"`) {
		t.Fatalf("unexpected merged JSON: %s", got)
	}
	if out.Get("X-Remove") != "" {
		t.Fatal("removed header retained")
	}
}

func TestGeneratorRequestReuseAndIncrement(t *testing.T) {
	m := NewGeneratorManager()
	req := NewRequestContext(time.Now())
	rule := GeneratorRule{Type: "random", Charset: "digits", Length: 4, Mode: "request"}
	a, err := m.Value(1, "code", rule, req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.Value(1, "code", rule, req)
	if a != b || len(a) != 4 {
		t.Fatalf("request generator mismatch %q %q", a, b)
	}
	inc := GeneratorRule{Type: "random", Charset: "digits", Length: 4, Mode: "increment", Step: 1, Overflow: "wrap"}
	x, _ := m.Value(2, "seq", inc, NewRequestContext(time.Now()))
	y, _ := m.Value(2, "seq", inc, NewRequestContext(time.Now()))
	if x == y {
		t.Fatalf("increment did not advance: %s", x)
	}
}
