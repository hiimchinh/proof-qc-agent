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
}

// Response is the JSON answer plus what it cost.
type Response struct {
	JSON         json.RawMessage
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

type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends the request and returns the model's JSON answer.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	user := []part{{Type: "text", Text: req.Text}}
	if len(req.ImagePNG) > 0 {
		url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(req.ImagePNG)
		user = append(user, part{Type: "image_url", ImageURL: &imageURL{URL: url}})
	}
	msgs := []message{}
	if req.System != "" {
		msgs = append(msgs, message{Role: "system", Content: req.System})
	}
	msgs = append(msgs, message{Role: "user", Content: user})

	body := map[string]any{
		"model":    req.Model,
		"messages": msgs,
		"usage":    map[string]bool{"include": true},
		// Route only to providers that honour the schema instead of silently dropping it.
		"provider": map[string]bool{"require_parameters": true},
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
		return Response{}, fmt.Errorf("%s: status %d: %.200s", req.Model, resp.StatusCode, data)
	}
	if resp.StatusCode != http.StatusOK || cr.Error != nil {
		msg := fmt.Sprintf("status %d", resp.StatusCode)
		if cr.Error != nil {
			msg = cr.Error.Message
		}
		return Response{}, fmt.Errorf("%s: %s", req.Model, msg)
	}
	if len(cr.Choices) == 0 {
		return Response{}, fmt.Errorf("%s: no choices", req.Model)
	}

	out := Response{
		CostUSD:      cr.Usage.Cost,
		InputTokens:  cr.Usage.PromptTokens,
		OutputTokens: cr.Usage.CompletionTokens,
		Latency:      latency,
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
