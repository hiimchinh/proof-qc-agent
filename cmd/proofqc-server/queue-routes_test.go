package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func TestConfiguredQueueReachesTickAndDeskQuery(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"DATABASE_URL": "unused", "OPENROUTER_API_KEY": "unused", "DAILY_USD_CAP": "0", "TICK_TOKEN": "test", "CHALLENGER_QUEUE": "vendor/next,vendor/later"}))
	if err != nil {
		t.Fatal(err)
	}
	st := storetest.Open(t)
	mux, err := newMux(st, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Retire both initial models, then let the real route fill the vacant seat.
	if _, err := st.InitFleet(t.Context(), "a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := st.ApplyFleet(t.Context(), store.FleetState{}, nil); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/internal/tick", nil)
	req.Header.Set("X-Tick-Token", "test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("tick: %d %s", rec.Code, rec.Body)
	}
	req = httptest.NewRequest("POST", "/graphql", strings.NewReader(`{"query":"{ desk { challenger challengerQueue } }"}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if got := strings.TrimSpace(rec.Body.String()); got != `{"data":{"desk":{"challenger":"vendor/next","challengerQueue":["vendor/later"]}}}` {
		t.Fatalf("response=%s", got)
	}
}
