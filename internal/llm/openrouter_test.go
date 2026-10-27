package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeServer(t *testing.T, status int, body string, seen *map[string]any) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if seen != nil {
			_ = json.Unmarshal(raw, seen)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, APIKey: "test", HTTP: srv.Client()}
}

const okBody = `{"choices":[{"message":{"content":"{\"decision\":\"ready_for_proof\"}"}}],
 "usage":{"prompt_tokens":1200,"completion_tokens":80,"cost":0.00042}}`

func TestCompleteReturnsJSONAndCost(t *testing.T) {
	c := fakeServer(t, 200, okBody, nil)
	res, err := c.Complete(context.Background(), Request{Model: "x-ai/grok-4.7", Text: "hi", Schema: json.RawMessage(`{"type":"object"}`), SchemaName: "proof"})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.JSON) != `{"decision":"ready_for_proof"}` {
		t.Fatalf("JSON = %s", res.JSON)
	}
	if res.CostUSD != 0.00042 || res.InputTokens != 1200 || res.OutputTokens != 80 {
		t.Fatalf("usage = %+v", res)
	}
}

func TestCompleteSendsTheImage(t *testing.T) {
	var seen map[string]any
	c := fakeServer(t, 200, okBody, &seen)
	_, err := c.Complete(context.Background(), Request{Model: "m", Text: "hi", ImagePNG: []byte{0x89, 'P', 'N', 'G'}, Schema: json.RawMessage(`{}`), SchemaName: "proof"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(seen["messages"])
	if !strings.Contains(string(raw), "data:image/png;base64,iVBORw") {
		t.Fatalf("image not sent: %s", raw)
	}
}

func TestCompleteStripsCodeFences(t *testing.T) {
	body := `{"choices":[{"message":{"content":"` + "```json\\n{\\\"a\\\":1}\\n```" + `"}}],"usage":{}}`
	c := fakeServer(t, 200, body, nil)
	res, err := c.Complete(context.Background(), Request{Model: "m", Text: "hi", Schema: json.RawMessage(`{}`), SchemaName: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.JSON) != `{"a":1}` {
		t.Fatalf("JSON = %s", res.JSON)
	}
}

func TestCompleteFailsOnHTTPError(t *testing.T) {
	c := fakeServer(t, 429, `{"error":{"message":"rate limited"}}`, nil)
	if _, err := c.Complete(context.Background(), Request{Model: "m", Text: "hi"}); err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("err = %v, want rate limited", err)
	}
}

func TestCompleteFailsOnProseAnswer(t *testing.T) {
	body := `{"choices":[{"message":{"content":"Looks good to me."}}],"usage":{}}`
	c := fakeServer(t, 200, body, nil)
	if _, err := c.Complete(context.Background(), Request{Model: "m", Text: "hi", Schema: json.RawMessage(`{}`), SchemaName: "s"}); err == nil {
		t.Fatal("want an error for a non-JSON answer")
	}
}

func TestCompleteFailsOnATruncatedAnswer(t *testing.T) {
	body := `{"choices":[{"finish_reason":"length","message":{"content":"{\"a\":1}"}}],"usage":{"cost":0.01}}`
	c := fakeServer(t, 200, body, nil)
	res, err := c.Complete(context.Background(), Request{Model: "m", Text: "hi", Schema: json.RawMessage(`{}`), SchemaName: "s"})
	if err == nil || res.CostUSD != 0.01 {
		t.Fatalf("err = %v cost = %v; want a truncation error that still reports the cost", err, res.CostUSD)
	}
}
