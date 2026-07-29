package body

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	ErrNotJSON      = errors.New("not json object")
	ErrModelMissing = errors.New("model missing")
	ErrModelType    = errors.New("model not string")
)

// PeekTopModel finds the top-level "model" string without full unmarshal.
// Returns logical (unescaped) model name and the raw JSON string value byte range [start,end) inside body
// where start points to opening quote and end is after closing quote.
func PeekTopModel(body []byte) (model string, valueStart, valueEnd int, err error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return "", 0, 0, ErrNotJSON
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return "", 0, 0, ErrNotJSON
	}
	depth := 1
	for dec.More() || depth > 0 {
		if depth == 1 && !dec.More() {
			// closing
			tok, err = dec.Token()
			if err != nil {
				return "", 0, 0, err
			}
			if d, ok := tok.(json.Delim); ok && d == '}' {
				depth--
				break
			}
			return "", 0, 0, ErrNotJSON
		}
		// key
		tok, err = dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", 0, 0, err
		}
		key, ok := tok.(string)
		if !ok {
			// could be closing delim when More is false handled above
			if d, ok := tok.(json.Delim); ok && d == '}' {
				depth--
				if depth == 0 {
					break
				}
				continue
			}
			return "", 0, 0, ErrNotJSON
		}
		offsetBeforeValue := dec.InputOffset()
		// value token
		tok, err = dec.Token()
		if err != nil {
			return "", 0, 0, err
		}
		if depth == 1 && key == "model" {
			str, ok := tok.(string)
			if !ok {
				return "", 0, 0, ErrModelType
			}
			// Find raw string span in body starting near offsetBeforeValue.
			// Decoder.InputOffset after Token for a string is past the closing quote.
			end := int(dec.InputOffset())
			start := findJSONStringStart(body, int(offsetBeforeValue), end)
			if start < 0 {
				// fallback: re-encode
				raw, _ := json.Marshal(str)
				idx := bytes.Index(body, raw)
				if idx < 0 {
					return str, 0, 0, nil
				}
				return str, idx, idx + len(raw), nil
			}
			return str, start, end, nil
		}
		// skip nested structures fully via decoder state
		switch v := tok.(type) {
		case json.Delim:
			if v == '{' || v == '[' {
				if err := skipRest(dec, v); err != nil {
					return "", 0, 0, err
				}
			}
		}
		_ = key
	}
	return "", 0, 0, ErrModelMissing
}

func skipRest(dec *json.Decoder, open json.Delim) error {
	depth := 1
	var close json.Delim = '}'
	if open == '[' {
		close = ']'
	}
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			if d == '{' || d == '[' {
				depth++
			} else if d == '}' || d == ']' {
				depth--
				if depth == 0 && d != close {
					// mismatched but tolerate
				}
			}
		}
	}
	return nil
}

func findJSONStringStart(body []byte, from, end int) int {
	if end <= 0 || end > len(body) {
		end = len(body)
	}
	if from < 0 {
		from = 0
	}
	// walk back from end-1 which should be closing quote
	i := end - 1
	if i < 0 || i >= len(body) || body[i] != '"' {
		// search nearby
		for j := end - 1; j >= from && j >= end-8; j-- {
			if body[j] == '"' {
				i = j
				break
			}
		}
	}
	if i < 0 || i >= len(body) || body[i] != '"' {
		return -1
	}
	// walk back to opening quote accounting for escapes
	for j := i - 1; j >= 0; j-- {
		if body[j] != '"' {
			continue
		}
		// count preceding backslashes
		bs := 0
		for k := j - 1; k >= 0 && body[k] == '\\'; k-- {
			bs++
		}
		if bs%2 == 0 {
			return j
		}
	}
	return -1
}

// RewriteTopModel replaces the first top-level model string value with upstream.
// Returns new body bytes (may be same underlying if unchanged).
func RewriteTopModel(body []byte, upstream string) ([]byte, error) {
	_, start, end, err := PeekTopModel(body)
	if err != nil {
		return nil, err
	}
	if start <= 0 || end <= start {
		return nil, fmt.Errorf("cannot locate model value span")
	}
	raw, err := json.Marshal(upstream)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(body)- (end-start) + len(raw))
	out = append(out, body[:start]...)
	out = append(out, raw...)
	out = append(out, body[end:]...)
	return out, nil
}
