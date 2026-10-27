package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func TestRoutes(t *testing.T) {
	mux, err := newMux(storetest.Open(t), "unused-key", 3)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method, path string
		status       int
		contentType  string
	}{
		{"GET", "/healthz", 200, "text/plain"},
		{"GET", "/", 200, "text/html"},
		{"GET", "/app.js", 200, "text/javascript"},
		{"GET", "/samples/clean-diecut.png", 200, "image/png"},
		{"GET", "/samples/cases.json", 404, ""},
		{"GET", "/graphql", 405, "application/json"},
		{"POST", "/graphql", 200, "application/json"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(`{"query":"{ spentToday }"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != c.status || !strings.HasPrefix(rec.Header().Get("Content-Type"), c.contentType) {
			t.Errorf("%s %s: %d %q, want %d %q", c.method, c.path, rec.Code, rec.Header().Get("Content-Type"), c.status, c.contentType)
		}
	}
}
