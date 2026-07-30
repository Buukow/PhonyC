package store

import (
	"strconv"
	"strings"
)

func (s *Store) GetSettingOr(key, def string) string {
	v, err := s.GetSetting(key)
	if err != nil || v == "" {
		return def
	}
	return v
}

func (s *Store) GetSettingBool(key string, def bool) bool {
	v, err := s.GetSetting(key)
	if err != nil || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func (s *Store) GetSettingInt(key string, def int) int {
	v, err := s.GetSetting(key)
	if err != nil || v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

func ParseStatusCodeList(s string, def []int) []int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err == nil {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func StatusCodeListContains(codes []int, code int) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// Setting keys
const (
	SettingAutoTestEnabled      = "auto_test_enabled"
	SettingAutoTestIntervalMin  = "auto_test_interval_minutes"
	SettingAutoTestRandomOffset = "auto_test_random_offset_minutes"
	SettingAutoTestPrompt       = "auto_test_prompt"
	SettingAutoTestModel        = "auto_test_model"
	SettingAutoTestDisableCodes = "auto_test_disable_status_codes"
	SettingAutoTestEnhanced     = "auto_test_enhanced_enabled"
	SettingAutoTestLexicon      = "auto_test_enhanced_lexicon"
	SettingHeaderCaptureEnabled = "header_capture_enabled"
	SettingHeaderCaptureKey     = "header_capture_key"
	SettingHeaderCaptureArmed   = "header_capture_armed" // "1" waiting for first request
	SettingHeaderCapturePayload = "header_capture_payload"
	SettingAutoRetryEnabled     = "auto_retry_enabled"
	SettingAutoRetryMax         = "auto_retry_max"
	SettingAutoRetryStatusCodes = "auto_retry_status_codes"
)
