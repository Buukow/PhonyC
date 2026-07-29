package protocol

import "net/http"

const (
	OpenAI    = "openai"
	Anthropic = "anthropic"
)

// Plugin applies protocol-specific auth skeleton headers.
type Plugin interface {
	Name() string
	ApplyAuth(h http.Header, apiKey string)
}

type openAI struct{}

func (openAI) Name() string { return OpenAI }
func (openAI) ApplyAuth(h http.Header, apiKey string) {
	h.Set("Authorization", "Bearer "+apiKey)
}

type anthropic struct{}

func (anthropic) Name() string { return Anthropic }
func (anthropic) ApplyAuth(h http.Header, apiKey string) {
	h.Set("x-api-key", apiKey)
	if h.Get("anthropic-version") == "" && h.Get("Anthropic-Version") == "" {
		h.Set("anthropic-version", "2023-06-01")
	}
}

func Get(name string) Plugin {
	switch name {
	case Anthropic:
		return anthropic{}
	default:
		return openAI{}
	}
}

// ProtocolForPath returns required channel protocol for a request path.
// models listing is special-cased by caller.
func ProtocolForPath(method, path string) string {
	switch path {
	case "/v1/messages":
		return Anthropic
	case "/v1/responses", "/v1/responses/compact", "/v1/chat/completions", "/v1/completions":
		return OpenAI
	default:
		// unknown /v1/* : treat as openai for model peek routing
		return OpenAI
	}
}

func IsModelsPath(path string) bool {
	if path == "/v1/models" {
		return true
	}
	if len(path) > len("/v1/models/") && path[:len("/v1/models/")] == "/v1/models/" {
		return true
	}
	return false
}
