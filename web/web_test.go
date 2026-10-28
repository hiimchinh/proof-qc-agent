package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestServesThePageAndItsAssets(t *testing.T) {
	cases := []struct{ path, contentType, contains string }{
		{"/", "text/html", `<script type="module" src="/app.js">`},
		{"/app.js", "text/javascript", "reviewArtwork"},
		{"/style.css", "text/css", "system-ui"},
	}
	for _, c := range cases {
		rec := get(t, c.path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", c.path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, c.contentType) {
			t.Errorf("%s: Content-Type %q, want %s", c.path, ct, c.contentType)
		}
		if !strings.Contains(rec.Body.String(), c.contains) {
			t.Errorf("%s: body lacks %q", c.path, c.contains)
		}
	}
}

func TestUnknownPathsAre404(t *testing.T) {
	for _, p := range []string{"/src/app.ts", "/package.json", "/index.html/x"} {
		if rec := get(t, p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", p, rec.Code)
		}
	}
}

func TestPageForbidsInlineAndThirdPartyCode(t *testing.T) {
	csp := get(t, "/").Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") || strings.Contains(csp, "unsafe-inline") {
		t.Fatalf("Content-Security-Policy = %q", csp)
	}
}

func TestOnlyReadsAreAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /: status %d, want 405", rec.Code)
	}
}

func TestLiveDeskPrecedesTheManualReview(t *testing.T) {
	body := get(t, "/").Body.String()
	desk, form := strings.Index(body, `id="desk-orders"`), strings.Index(body, `id="review"`)
	if desk < 0 || form < desk || !strings.Contains(body, "Simulated order stream") || !strings.Contains(body, `id="fleet-scoreboard"`) || !strings.Contains(body, `id="fleet-events"`) {
		t.Fatal("page must lead with a labelled simulated desk, scoreboard and fleet events, before the manual form")
	}
}

func TestFleetQueueStatusIsInsideTheFleetCard(t *testing.T) {
	body := get(t, "/").Body.String()
	card, queue, policy := strings.Index(body, `id="fleet-title"`), strings.Index(body, `id="fleet-queue"`), strings.Index(body, "Promotion and retirement policy")
	if card < 0 || queue < card || policy < queue || !strings.Contains(body, `id="fleet-queue" role="status"`) {
		t.Fatal("fleet card must expose queue status before the policy details")
	}
}

func TestChampionSummaryPrecedesTheOrderFeed(t *testing.T) {
	body := get(t, "/").Body.String()
	kpis, orders := strings.Index(body, `<dl id="desk-kpis"`), strings.Index(body, `id="desk-orders"`)
	if kpis < 0 || orders < kpis {
		t.Fatal("champion summary must come before the order feed")
	}
	if !strings.Contains(body, "rate-limited on 12 calls in a row") {
		t.Fatal("policy details must explain when a rate-limited challenger is skipped")
	}
}
