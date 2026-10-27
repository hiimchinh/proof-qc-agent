// Package llm calls any model through OpenRouter's OpenAI-compatible API, so
// Claude, GPT, Grok and open-weight models run behind one interface.
package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is OpenRouter's API root.
const DefaultBaseURL = "https://openrouter.ai/api/v1"

// Request is one structured-output call, optionally with an image.
type Request struct {
	Model      string
	System     string
	Text       string
	ImagePNG   []byte
	Schema     json.RawMessage
	SchemaName string
	MaxTokens  int
	Messages   []Message
	Tools      []Tool
}

// Response is the JSON answer plus what it cost.
type Response struct {
	JSON         json.RawMessage
	ToolCalls    []ToolCall
	FinishReason string
	CostUSD      float64
	InputTokens  int
	OutputTokens int
	Latency      time.Duration
}

// Client talks to OpenRouter. One call is one billed attempt: it never retries.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

type part struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
	Error *providerError `json:"error"`
}

// Complete sends the request and returns the model's JSON answer.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	user := []part{{Type: "text", Text: req.Text}}
	if len(req.ImagePNG) > 0 {
		url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(req.ImagePNG)
		user = append(user, part{Type: "image_url", ImageURL: &imageURL{URL: url}})
	}
	msgs := []Message{}
	if req.System != "" {
		msgs = append(msgs, Message{Role: "system", Content: req.System})
	}
	if req.Text != "" || len(req.ImagePNG) > 0 || len(req.Messages) == 0 {
		msgs = append(msgs, Message{Role: "user", Content: user})
	}
	msgs = append(msgs, req.Messages...)

	body := map[string]any{
		"model":    req.Model,
		"messages": msgs,
		"usage":    map[string]bool{"include": true},
		// Require support for all parameters, including tools and JSON schemas.
		"provider": map[string]bool{"require_parameters": true},
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.Schema) > 0 {
		body["response_format"] = map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": req.SchemaName, "strict": true, "schema": req.Schema},
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}

	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Title", "proof-qc-agent")

	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 3 * time.Minute}
	}
	start := time.Now()
	resp, err := hc.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	latency := time.Since(start)

	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return Response{}, fmt.Errorf("%s: status %d: %s", req.Model, resp.StatusCode, safeErrorText(string(data), c.APIKey, 200))
	}
	out := Response{
		CostUSD: cr.Usage.Cost, InputTokens: cr.Usage.PromptTokens,
		OutputTokens: cr.Usage.CompletionTokens, Latency: latency,
	}
	if resp.StatusCode != http.StatusOK || cr.Error != nil {
		msg := fmt.Sprintf("status %d", resp.StatusCode)
		if cr.Error != nil {
			msg = cr.Error.detail(c.APIKey)
		}
		return out, fmt.Errorf("%s: %s", req.Model, msg)
	}
	if len(cr.Choices) == 0 {
		return out, fmt.Errorf("%s: no choices", req.Model)
	}
	out.FinishReason = cr.Choices[0].FinishReason
	out.ToolCalls = cr.Choices[0].Message.ToolCalls
	if out.FinishReason == "tool_calls" {
		if len(out.ToolCalls) == 0 {
			return out, fmt.Errorf("%s: no tool calls", req.Model)
		}
		return out, validateToolCalls(out.ToolCalls, req.Tools)
	}
	if out.FinishReason != "stop" && out.FinishReason != "length" {
		return out, fmt.Errorf("%s: unsupported finish_reason %q", req.Model, out.FinishReason)
	}
	if len(out.ToolCalls) > 0 {
		return out, fmt.Errorf("%s: tool calls without tool_calls finish_reason", req.Model)
	}
	if cr.Choices[0].FinishReason == "length" {
		return out, fmt.Errorf("%s: answer cut off at the token limit", req.Model)
	}
	content := stripFences(cr.Choices[0].Message.Content)
	if len(req.Schema) > 0 {
		if !json.Valid([]byte(content)) {
			return out, fmt.Errorf("%s: %w: %.200s", req.Model, ErrNotJSON, content)
		}
	}
	out.JSON = json.RawMessage(content)
	return out, nil
}

// ErrNotJSON means a schema was requested and the answer was not JSON.
var ErrNotJSON = errors.New("answer is not JSON")

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}
