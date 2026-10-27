package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
	"github.com/hiimchinh/proof-qc-agent/results"
)

type testServer struct {
	http.Handler
	llm   *fakeLLM
	store *store.Store
}

func newTestServer(t *testing.T, capUSD float64) testServer {
	t.Helper()
	cases, art, err := proofqc.Cases()
	if err != nil {
		t.Fatal(err)
	}
	board, err := results.Leaderboard(results.Embedded(), cases)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeLLM{cost: 0.0004569}
	st := storetest.Open(t)
	h, err := New(Config{Store: st, LLM: fake, Cases: cases, Artwork: art, Leaderboard: board, DailyCapUSD: capUSD})
	if err != nil {
		t.Fatal(err)
	}
	return testServer{Handler: h, llm: fake, store: st}
}

type gqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

// do posts one GraphQL request from the given client address.
func (s testServer) do(t *testing.T, from, query string, vars map[string]any) gqlResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = from + ":5555"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out gqlResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%v: %s", err, rec.Body)
	}
	return out
}

func (r gqlResponse) code() string {
	if len(r.Errors) == 0 {
		return ""
	}
	return r.Errors[0].Extensions.Code
}

const reviewMutation = `mutation($in: ReviewInput!) { reviewArtwork(input: $in) {
  decision reasons designerNote customerNote orderChanges
  preflight { widthPx heightPx effectiveDpi findings { flag message } }
  lint { rule detail } model costUsd latencyMs createdAt } }`

type reviewData struct {
	ReviewArtwork struct {
		Decision     string
		CustomerNote string
		OrderChanges []string
		Preflight    struct {
			WidthPx      int
			EffectiveDpi float64
		}
		Model     string
		CostUsd   float64
		LatencyMs int
		CreatedAt string
	}
}

func sampleInput() map[string]any {
	return map[string]any{
		"customer": "Brian Smith", "product": "die_cut", "widthIn": 3, "heightIn": 3,
		"comment": "", "sampleId": "clean-diecut",
	}
}

func TestReviewArtworkWithASample(t *testing.T) {
	s := newTestServer(t, 3)
	resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": sampleInput()})
	if len(resp.Errors) > 0 {
		t.Fatalf("errors: %+v", resp.Errors)
	}
	var d reviewData
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatal(err)
	}
	r := d.ReviewArtwork
	// clean-diecut is drawn at 300 dpi for 3 in: 900 px.
	if r.Decision != "ready_for_proof" || r.Preflight.EffectiveDpi != 300 || r.Preflight.WidthPx != 900 ||
		r.Model != "anthropic/claude-haiku-5.5" || r.CostUsd != 0.0004569 || r.LatencyMs != 1200 || r.OrderChanges == nil {
		t.Fatalf("review = %+v", r)
	}
	if _, err := time.Parse(time.RFC3339, r.CreatedAt); err != nil {
		t.Fatalf("createdAt %q: %v", r.CreatedAt, err)
	}

	resp = s.do(t, "198.51.100.1", `{ recentReviews(limit: 5) { customerNote model } spentToday }`, nil)
	var q struct {
		RecentReviews []struct{ CustomerNote, Model string }
		SpentToday    float64
	}
	if err := json.Unmarshal(resp.Data, &q); err != nil || len(resp.Errors) > 0 {
		t.Fatalf("%v %+v", err, resp.Errors)
	}
	if len(q.RecentReviews) != 1 || q.RecentReviews[0].CustomerNote != "Thanks Brian. Your proof is on the way." || q.SpentToday != 0.0004569 {
		t.Fatalf("after one review: %+v", q)
	}
}

func TestModelFailureIsAGraphQLErrorAndItsCostCounts(t *testing.T) {
	s := newTestServer(t, 3)
	s.llm.err, s.llm.cost = errors.New("upstream 502"), 0.002
	resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": sampleInput()})
	if resp.code() != "MODEL_FAILED" {
		t.Fatalf("errors = %+v, want MODEL_FAILED", resp.Errors)
	}
	resp = s.do(t, "198.51.100.1", `{ spentToday recentReviews { model } }`, nil)
	if string(resp.Data) != `{"spentToday":0.002,"recentReviews":[]}` {
		t.Fatalf("data = %s, want the failed call billed but not listed", resp.Data)
	}
}

func TestSamplesAndLeaderboard(t *testing.T) {
	s := newTestServer(t, 3)
	resp := s.do(t, "198.51.100.1", `{ samples { id imageUrl product widthIn want } leaderboard { model variant accuracy verdict } }`, nil)
	var q struct {
		Samples []struct {
			ID, ImageURL, Product, Want string
			WidthIn                     float64
		}
		Leaderboard []struct{ Model, Variant, Verdict string }
	}
	if err := json.Unmarshal(resp.Data, &q); err != nil || len(resp.Errors) > 0 {
		t.Fatalf("%v %+v", err, resp.Errors)
	}
	first := q.Samples[0]
	if len(q.Samples) < 26 || first.ID != "clean-diecut" || first.ImageURL != "/samples/clean-diecut.png" ||
		first.Product != "die_cut" || first.WidthIn != 3 || first.Want != "ready_for_proof" {
		t.Fatalf("samples[0] = %+v of %d", first, len(q.Samples))
	}
	if q.Leaderboard == nil {
		t.Fatal("leaderboard is null, want a list")
	}
}
