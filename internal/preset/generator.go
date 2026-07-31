package preset

import (
	cryptorand "crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	minInterval = time.Second
	maxInterval = 365 * 24 * time.Hour
)

type generatorState struct {
	Value     string
	Generated time.Time
	Next      time.Time
	Counter   *big.Int
	Width     int
	RuleKey   string
	Timer     *time.Timer
}

type GeneratorManager struct {
	mu        sync.Mutex
	states    map[string]*generatorState
	scheduled bool
}

func NewGeneratorManager() *GeneratorManager {
	return &GeneratorManager{states: map[string]*generatorState{}}
}

func NewScheduledGeneratorManager() *GeneratorManager {
	return &GeneratorManager{states: map[string]*generatorState{}, scheduled: true}
}

var DefaultGenerators = NewScheduledGeneratorManager()

type RequestContext struct {
	Values map[string]string
	Now    time.Time
}

func NewRequestContext(now time.Time) *RequestContext {
	return &RequestContext{Values: map[string]string{}, Now: now}
}

func (m *GeneratorManager) Value(presetID int64, name string, rule GeneratorRule, req *RequestContext) (string, error) {
	if value, ok := req.Values[name]; ok {
		return value, nil
	}
	if rule.Mode == "request" {
		value, err := generateValue(rule)
		if err == nil {
			req.Values[name] = value
		}
		return value, err
	}
	key := fmt.Sprintf("%d:%s", presetID, name)
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.states[key]
	ruleKey := fmt.Sprintf("%+v", rule)
	if state == nil || state.RuleKey != ruleKey {
		if state != nil && state.Timer != nil {
			state.Timer.Stop()
		}
		var err error
		state, err = initializeState(rule, req.Now, ruleKey)
		if err != nil {
			return "", err
		}
		m.states[key] = state
		m.scheduleIntervalLocked(key, state, rule)
	}
	if rule.Mode == "interval" && !req.Now.Before(state.Next) {
		if state.Timer != nil {
			state.Timer.Stop()
			state.Timer = nil
		}
		value, err := generateValue(rule)
		if err != nil {
			return "", err
		}
		d, _ := parseDuration(rule.Interval)
		state.Value, state.Generated, state.Next = value, req.Now, req.Now.Add(d)
		m.scheduleIntervalLocked(key, state, rule)
	}
	value := state.Value
	if rule.Mode == "increment" {
		value = formatCounter(state.Counter, state.Width)
		if err := incrementState(state, rule); err != nil {
			return "", err
		}
	}
	req.Values[name] = value
	return value, nil
}

func (m *GeneratorManager) Refresh(presetID int64, name string, rule GeneratorRule) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	key := fmt.Sprintf("%d:%s", presetID, name)
	if current := m.states[key]; current != nil && current.Timer != nil {
		current.Timer.Stop()
	}
	state, err := initializeState(rule, now, fmt.Sprintf("%+v", rule))
	if err != nil {
		return "", err
	}
	m.states[key] = state
	m.scheduleIntervalLocked(key, state, rule)
	return state.Value, nil
}

func (m *GeneratorManager) ResetPreset(presetID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := fmt.Sprintf("%d:", presetID)
	for key := range m.states {
		if strings.HasPrefix(key, prefix) {
			if m.states[key].Timer != nil {
				m.states[key].Timer.Stop()
			}
			delete(m.states, key)
		}
	}
}

func (m *GeneratorManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, state := range m.states {
		if state.Timer != nil {
			state.Timer.Stop()
		}
		delete(m.states, key)
	}
}

func (m *GeneratorManager) scheduleIntervalLocked(key string, state *generatorState, rule GeneratorRule) {
	if !m.scheduled || rule.Mode != "interval" {
		return
	}
	delay := time.Until(state.Next)
	if delay < 0 {
		delay = 0
	}
	state.Timer = time.AfterFunc(delay, func() {
		m.refreshScheduledInterval(key, state, rule)
	})
}

func (m *GeneratorManager) refreshScheduledInterval(key string, expected *generatorState, rule GeneratorRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.states[key]
	if state == nil || state != expected || state.RuleKey != fmt.Sprintf("%+v", rule) {
		return
	}
	now := time.Now()
	value, err := generateValue(rule)
	d, durationErr := parseDuration(rule.Interval)
	if durationErr != nil {
		return
	}
	if err == nil {
		state.Value = value
		state.Generated = now
	}
	state.Next = now.Add(d)
	state.Timer = nil
	m.scheduleIntervalLocked(key, state, rule)
}

func initializeState(rule GeneratorRule, now time.Time, ruleKey string) (*generatorState, error) {
	value, err := generateValue(rule)
	if err != nil {
		return nil, err
	}
	state := &generatorState{Value: value, Generated: now, RuleKey: ruleKey, Width: rule.Length}
	if rule.Mode == "interval" {
		d, _ := parseDuration(rule.Interval)
		state.Next = now.Add(d)
	}
	if rule.Mode == "increment" {
		state.Counter = new(big.Int)
		if _, ok := state.Counter.SetString(value, 10); !ok {
			return nil, fmt.Errorf("递增生成器初始值无效")
		}
	}
	return state, nil
}

func incrementState(state *generatorState, rule GeneratorRule) error {
	if state.Counter == nil {
		return fmt.Errorf("递增生成器状态无效")
	}
	next := new(big.Int).Add(state.Counter, big.NewInt(rule.Step))
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(state.Width)), nil)
	if next.Cmp(max) < 0 {
		state.Counter = next
		state.Value = formatCounter(next, state.Width)
		return nil
	}
	switch rule.Overflow {
	case "", "wrap":
		state.Counter = new(big.Int).Mod(next, max)
	case "regenerate":
		value, err := generateValue(rule)
		if err != nil {
			return err
		}
		state.Counter = new(big.Int)
		if _, ok := state.Counter.SetString(value, 10); !ok {
			return fmt.Errorf("递增生成器刷新值无效")
		}
	case "expand":
		state.Counter = next
		if digits := len(next.String()); digits > state.Width {
			state.Width = digits
		}
	case "error":
		return fmt.Errorf("递增生成器已溢出")
	}
	state.Value = formatCounter(state.Counter, state.Width)
	return nil
}

func formatCounter(value *big.Int, width int) string {
	raw := value.String()
	if len(raw) >= width {
		return raw
	}
	return strings.Repeat("0", width-len(raw)) + raw
}

func generateValue(rule GeneratorRule) (string, error) {
	if rule.Type == "uuid" {
		if rule.Version == 7 {
			value, err := uuid.NewV7()
			return value.String(), err
		}
		return uuid.NewString(), nil
	}
	charsets := map[string]string{
		"digits": "0123456789", "lowercase": "abcdefghijklmnopqrstuvwxyz", "uppercase": "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"letters": "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", "alnum": "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
	}
	chars := charsets[rule.Charset]
	exclude := strings.Join(rule.Exclude, "")
	if rule.ExcludeAmbiguous {
		exclude += "0O1Il"
	}
	for _, ch := range exclude {
		chars = strings.ReplaceAll(chars, string(ch), "")
	}
	if chars == "" {
		return "", fmt.Errorf("随机字符集不能为空")
	}
	out := make([]byte, rule.Length)
	for i := range out {
		n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		out[i] = chars[n.Int64()]
	}
	return string(out), nil
}

func parseDuration(raw string) (time.Duration, error) {
	if strings.HasSuffix(raw, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		return time.Duration(n) * 24 * time.Hour, err
	}
	return time.ParseDuration(raw)
}
