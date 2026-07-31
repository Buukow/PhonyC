package preset

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGeneratorRandomTypesCharsetsAndExclusions(t *testing.T) {
	allowed := map[string]string{
		"digits": "0123456789", "lowercase": "abcdefghijklmnopqrstuvwxyz",
		"uppercase": "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "letters": "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"alnum": "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
	}
	for charset, chars := range allowed {
		t.Run(charset, func(t *testing.T) {
			value, err := generateValue(GeneratorRule{Type: "random", Charset: charset, Length: 256})
			if err != nil {
				t.Fatal(err)
			}
			if len(value) != 256 {
				t.Fatalf("length=%d", len(value))
			}
			for _, char := range value {
				if !strings.ContainsRune(chars, char) {
					t.Fatalf("unexpected character %q in %s", char, charset)
				}
			}
		})
	}

	value, err := generateValue(GeneratorRule{Type: "random", Charset: "alnum", Length: 256, ExcludeAmbiguous: true, Exclude: []string{"x", "Y", "9"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(value, "0O1IlxY9") {
		t.Fatalf("excluded character generated: %q", value)
	}

	value, err = generateValue(GeneratorRule{Type: "uuid"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(value); err != nil {
		t.Fatalf("invalid UUID %q: %v", value, err)
	}
}

func TestGeneratorUUIDVersions(t *testing.T) {
	for _, version := range []int{4, 7} {
		value, err := generateValue(GeneratorRule{Type: "uuid", Version: version})
		if err != nil {
			t.Fatal(err)
		}
		if got := int(uuid.MustParse(value).Version()); got != version {
			t.Fatalf("version=%d generated UUID v%d: %s", version, got, value)
		}
	}
}

func TestNumericTimeTemplateKeepsJSONNumberType(t *testing.T) {
	doc := Document{
		SchemaVersion: SchemaVersion,
		Headers: map[string]HeaderRule{
			"X-Metadata": {Value: map[string]any{"started_at": "{{time_number:unix_ms}}"}},
		},
		RemoveHeaders: []string{},
		Generators:    map[string]GeneratorRule{},
	}
	resolved, _, err := (Resolver{Generators: NewGeneratorManager()}).Resolve(1, "", doc, http.Header{}, http.Header{}, time.UnixMilli(1785500416427))
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(resolved.Get("X-Metadata")), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["started_at"] != float64(1785500416427) {
		t.Fatalf("numeric time became %T(%v)", metadata["started_at"], metadata["started_at"])
	}
}

func TestGeneratorPerRequestMode(t *testing.T) {
	m := NewGeneratorManager()
	rule := GeneratorRule{Type: "uuid", Mode: "request"}
	req1 := NewRequestContext(time.Now())
	first, err := m.Value(1, "request_id", rule, req1)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := m.Value(1, "request_id", rule, req1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Value(1, "request_id", rule, NewRequestContext(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if first != repeated {
		t.Fatalf("one request received two values: %q / %q", first, repeated)
	}
	if first == second {
		t.Fatalf("different requests reused request value: %q", first)
	}
	if len(m.states) != 0 {
		t.Fatalf("request mode unexpectedly created persistent state: %+v", m.states)
	}
}

func TestGeneratorRuntimeFixedRefreshRuleChangeAndRestart(t *testing.T) {
	now := time.Now()
	m := NewGeneratorManager()
	rule := GeneratorRule{Type: "uuid", Mode: "fixed"}
	first, err := m.Value(2, "install_id", rule, NewRequestContext(now))
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Value(2, "install_id", rule, NewRequestContext(now.Add(24*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("fixed value changed without refresh: %q / %q", first, second)
	}
	state := m.states["2:install_id"]
	if state == nil || state.Value != first || state.Generated != now {
		t.Fatalf("fixed value was not stored in memory: %+v", state)
	}

	refreshed, err := m.Refresh(2, "install_id", rule)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed == first {
		t.Fatalf("manual refresh did not replace fixed UUID: %q", refreshed)
	}
	afterRefresh, _ := m.Value(2, "install_id", rule, NewRequestContext(now))
	if afterRefresh != refreshed {
		t.Fatalf("refreshed value not reused: %q / %q", refreshed, afterRefresh)
	}
	if m.states["2:install_id"].Value != refreshed {
		t.Fatalf("manual refresh did not update memory: %+v", m.states["2:install_id"])
	}

	changedRule := GeneratorRule{Type: "random", Charset: "uppercase", Length: 12, Mode: "fixed"}
	changed, err := m.Value(2, "install_id", changedRule, NewRequestContext(now))
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 12 || changed == refreshed {
		t.Fatalf("rule change did not reset state: %q", changed)
	}

	restarted, err := NewGeneratorManager().Value(2, "install_id", rule, NewRequestContext(now))
	if err != nil {
		t.Fatal(err)
	}
	if restarted == first {
		t.Fatalf("new manager unexpectedly reused pre-restart UUID: %q", restarted)
	}
}

func TestGeneratorIntervalModeRefreshesAtDeadline(t *testing.T) {
	m := NewGeneratorManager()
	rule := GeneratorRule{Type: "uuid", Mode: "interval", Interval: "30m"}
	t0 := time.Date(2026, 7, 31, 10, 0, 0, 0, time.Local)
	first, err := m.Value(3, "window_id", rule, NewRequestContext(t0))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := m.Value(3, "window_id", rule, NewRequestContext(t0.Add(30*time.Minute-time.Nanosecond)))
	if before != first {
		t.Fatalf("interval refreshed early: %q / %q", first, before)
	}
	state := m.states["3:window_id"]
	if state == nil || state.Value != first || !state.Generated.Equal(t0) || !state.Next.Equal(t0.Add(30*time.Minute)) {
		t.Fatalf("initial interval state not stored correctly: %+v", state)
	}
	atDeadline, _ := m.Value(3, "window_id", rule, NewRequestContext(t0.Add(30*time.Minute)))
	if atDeadline == first {
		t.Fatalf("interval did not refresh at deadline: %q", first)
	}
	after, _ := m.Value(3, "window_id", rule, NewRequestContext(t0.Add(59*time.Minute)))
	if after != atDeadline {
		t.Fatalf("interval value was not retained for the new period: %q / %q", atDeadline, after)
	}
	state = m.states["3:window_id"]
	if state.Value != atDeadline || !state.Generated.Equal(t0.Add(30*time.Minute)) || !state.Next.Equal(t0.Add(time.Hour)) {
		t.Fatalf("refreshed interval state not stored correctly: %+v", state)
	}
}

func TestScheduledIntervalRefreshesMemoryWithoutRequest(t *testing.T) {
	m := NewScheduledGeneratorManager()
	defer m.Close()
	rule := GeneratorRule{Type: "uuid", Mode: "interval", Interval: "1s"}
	first, err := m.Value(30, "scheduled", rule, NewRequestContext(time.Now()))
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	var refreshed string
	for time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		m.mu.Lock()
		state := m.states["30:scheduled"]
		if state != nil && state.Value != first {
			refreshed = state.Value
			if state.Generated.IsZero() || !state.Next.After(state.Generated) || state.Timer == nil {
				m.mu.Unlock()
				t.Fatalf("scheduled refresh produced invalid memory state: %+v", state)
			}
		}
		m.mu.Unlock()
		if refreshed != "" {
			break
		}
	}
	if refreshed == "" {
		t.Fatal("background timer did not refresh interval state")
	}
	readBack, err := m.Value(30, "scheduled", rule, NewRequestContext(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if readBack != refreshed {
		t.Fatalf("request did not read refreshed memory value: got=%q want=%q", readBack, refreshed)
	}
}

func TestGeneratorIncrementModeSupportsLongValuesAndConcurrency(t *testing.T) {
	m := NewGeneratorManager()
	rule := GeneratorRule{Type: "random", Charset: "digits", Length: 64, Mode: "increment", Step: 7, Overflow: "wrap"}
	req := NewRequestContext(time.Now())
	first, err := m.Value(4, "sequence", rule, req)
	if err != nil {
		t.Fatal(err)
	}
	repeated, _ := m.Value(4, "sequence", rule, req)
	if repeated != first {
		t.Fatalf("increment advanced twice in one request: %q / %q", first, repeated)
	}
	second, err := m.Value(4, "sequence", rule, NewRequestContext(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	firstNumber, ok := new(big.Int).SetString(first, 10)
	if !ok {
		t.Fatalf("invalid first value %q", first)
	}
	want := new(big.Int).Add(firstNumber, big.NewInt(7))
	want.Mod(want, new(big.Int).Exp(big.NewInt(10), big.NewInt(64), nil))
	if second != formatCounter(want, 64) {
		t.Fatalf("increment mismatch: first=%s second=%s want=%s", first, second, formatCounter(want, 64))
	}
	state := m.states["4:sequence"]
	wantStored := new(big.Int).Add(want, big.NewInt(7))
	wantStored.Mod(wantStored, new(big.Int).Exp(big.NewInt(10), big.NewInt(64), nil))
	if state == nil || state.Value != formatCounter(wantStored, 64) || state.Counter.Cmp(wantStored) != 0 {
		t.Fatalf("incremented value was not written to memory: %+v", state)
	}

	concurrent := NewGeneratorManager()
	concurrentRule := GeneratorRule{Type: "random", Charset: "digits", Length: 32, Mode: "increment", Step: 1, Overflow: "wrap"}
	const count = 128
	values := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := concurrent.Value(5, "counter", concurrentRule, NewRequestContext(time.Now()))
			if err != nil {
				errs <- err
				return
			}
			values <- value
		}()
	}
	wg.Wait()
	close(values)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for value := range values {
		if seen[value] {
			t.Fatalf("concurrent increment returned duplicate %q", value)
		}
		seen[value] = true
	}
	if len(seen) != count {
		t.Fatalf("received %d unique values, want %d", len(seen), count)
	}
	concurrent.mu.Lock()
	stored := concurrent.states["5:counter"]
	concurrent.mu.Unlock()
	if stored == nil || stored.Counter == nil || len(stored.Value) != 32 {
		t.Fatalf("concurrent increment state missing from memory: %+v", stored)
	}
}

func TestGeneratorIncrementOverflowModes(t *testing.T) {
	rule := GeneratorRule{Type: "random", Charset: "digits", Length: 2, Mode: "increment", Step: 1}
	for _, test := range []struct {
		mode      string
		wantValue string
		wantWidth int
		wantError bool
	}{
		{mode: "wrap", wantValue: "00", wantWidth: 2},
		{mode: "expand", wantValue: "100", wantWidth: 3},
		{mode: "error", wantValue: "99", wantWidth: 2, wantError: true},
	} {
		t.Run(test.mode, func(t *testing.T) {
			state := &generatorState{Value: "99", Counter: big.NewInt(99), Width: 2}
			rule.Overflow = test.mode
			err := incrementState(state, rule)
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v wantError=%v", err, test.wantError)
			}
			if state.Value != test.wantValue || state.Width != test.wantWidth {
				t.Fatalf("state=%+v want value=%s width=%d", state, test.wantValue, test.wantWidth)
			}
		})
	}

	state := &generatorState{Value: "0007", Counter: big.NewInt(7), Width: 4}
	rule.Length, rule.Step, rule.Overflow = 4, 1, "wrap"
	if err := incrementState(state, rule); err != nil {
		t.Fatal(err)
	}
	if state.Value != "0008" {
		t.Fatalf("leading zeroes lost: %q", state.Value)
	}

	state = &generatorState{Value: "99", Counter: big.NewInt(99), Width: 2}
	rule.Length, rule.Step, rule.Overflow = 2, 1, "regenerate"
	if err := incrementState(state, rule); err != nil {
		t.Fatal(err)
	}
	if len(state.Value) != 2 || state.Counter == nil {
		t.Fatalf("regenerate produced invalid state: %+v", state)
	}
}

func TestResolverReusesGeneratorsInsideNestedHeader(t *testing.T) {
	m := NewGeneratorManager()
	doc := Document{
		SchemaVersion: 1,
		Headers: map[string]HeaderRule{
			"X-Generator-Matrix": {Value: map[string]any{
				"request":   []any{"{{generator:per_request}}", "{{generator:per_request}}"},
				"fixed":     map[string]any{"a": "{{generator:fixed}}", "b": "{{generator:fixed}}"},
				"interval":  "{{generator:interval}}",
				"increment": map[string]any{"a": "{{generator:increment}}", "b": "{{generator:increment}}"},
			}},
		},
		Generators: map[string]GeneratorRule{
			"per_request": {Type: "uuid", Mode: "request"},
			"fixed":       {Type: "uuid", Mode: "fixed"},
			"interval":    {Type: "uuid", Mode: "interval", Interval: "1h"},
			"increment":   {Type: "random", Charset: "digits", Length: 32, Mode: "increment", Step: 1, Overflow: "wrap"},
		},
	}

	resolve := func(now time.Time) map[string]any {
		t.Helper()
		out, _, err := (Resolver{Generators: m}).Resolve(9, "1.0", doc, http.Header{}, http.Header{}, now)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(out.Get("X-Generator-Matrix")), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}

	t0 := time.Date(2026, 7, 31, 10, 0, 0, 0, time.Local)
	first := resolve(t0)
	second := resolve(t0.Add(30 * time.Minute))
	third := resolve(t0.Add(time.Hour))
	requestValues := first["request"].([]any)
	if requestValues[0] != requestValues[1] || requestValues[0] == second["request"].([]any)[0] {
		t.Fatalf("per-request nested reuse failed: first=%v second=%v", first["request"], second["request"])
	}
	fixedValues := first["fixed"].(map[string]any)
	if fixedValues["a"] != fixedValues["b"] || fixedValues["a"] != third["fixed"].(map[string]any)["a"] {
		t.Fatalf("fixed nested reuse failed: first=%v third=%v", first["fixed"], third["fixed"])
	}
	if first["interval"] != second["interval"] || first["interval"] == third["interval"] {
		t.Fatalf("interval nested refresh failed: %v / %v / %v", first["interval"], second["interval"], third["interval"])
	}
	firstIncrement := first["increment"].(map[string]any)
	secondIncrement := second["increment"].(map[string]any)
	if firstIncrement["a"] != firstIncrement["b"] || firstIncrement["a"] == secondIncrement["a"] {
		t.Fatalf("increment nested reuse failed: first=%v second=%v", firstIncrement, secondIncrement)
	}
	if len(firstIncrement["a"].(string)) != 32 {
		t.Fatalf("long increment width lost: %v", firstIncrement)
	}
}

func TestGeneratorValidationCoversAllModes(t *testing.T) {
	base := Document{SchemaVersion: 1, Headers: map[string]HeaderRule{"X-Test": {Value: "{{generator:g}}"}}}
	valid := []GeneratorRule{
		{Type: "random", Charset: "alnum", Length: 16, Mode: "request"},
		{Type: "random", Charset: "alnum", Length: 16, Mode: "fixed"},
		{Type: "random", Charset: "alnum", Length: 16, Mode: "interval", Interval: "30m"},
		{Type: "random", Charset: "digits", Length: 64, Mode: "increment", Step: 1, Overflow: "wrap"},
	}
	for _, rule := range valid {
		t.Run(rule.Mode, func(t *testing.T) {
			doc := base
			doc.Generators = map[string]GeneratorRule{"g": rule}
			if err := Validate(doc); err != nil {
				t.Fatalf("valid %s generator rejected: %v", rule.Mode, err)
			}
		})
	}

	invalid := []GeneratorRule{
		{Type: "random", Charset: "alnum", Length: 16, Mode: "interval", Interval: "500ms"},
		{Type: "random", Charset: "alnum", Length: 16, Mode: "increment", Step: 1},
		{Type: "random", Charset: "digits", Length: 16, Mode: "increment", Step: 0},
	}
	for index, rule := range invalid {
		t.Run(fmt.Sprintf("invalid-%d", index), func(t *testing.T) {
			doc := base
			doc.Generators = map[string]GeneratorRule{"g": rule}
			if err := Validate(doc); err == nil {
				t.Fatalf("invalid generator accepted: %+v", rule)
			}
		})
	}
}
