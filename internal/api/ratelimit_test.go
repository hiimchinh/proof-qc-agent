package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIPTrustsOnlyTheHopCloudRunAdds(t *testing.T) {
	r := httptest.NewRequest("POST", "/graphql", nil)
	r.RemoteAddr = "10.0.0.9:443"
	if got := clientIP(r); got != "10.0.0.9" {
		t.Fatalf("no header: %q, want the remote address", got)
	}
	// A visitor can send any X-Forwarded-For; Google's front end appends the
	// address it actually saw, so only the last hop is trustworthy.
	r.Header.Set("X-Forwarded-For", "203.0.113.66, 198.51.100.4")
	if got := clientIP(r); got != "198.51.100.4" {
		t.Fatalf("forwarded: %q, want the last hop", got)
	}
}

func TestLimiterWindowSlides(t *testing.T) {
	now := time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC)
	l := newLimiter(2, time.Hour, func() time.Time { return now })
	if !l.allow("a") || !l.allow("a") || l.allow("a") {
		t.Fatal("want two calls allowed, the third refused")
	}
	if !l.allow("b") {
		t.Fatal("another client shares no quota")
	}
	now = now.Add(time.Hour + time.Second)
	if !l.allow("a") {
		t.Fatal("calls older than the window must stop counting")
	}
}
