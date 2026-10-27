package desk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func TestTickAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, token, supplied, method string
		status                        int
	}{
		{"disabled", "", "", "POST", 404}, {"missing", "desk-secret", "", "POST", 401},
		{"wrong", "desk-secret", "wrong", "POST", 401}, {"method", "desk-secret", "desk-secret", "GET", 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/internal/tick", nil)
			req.Header.Set("X-Tick-Token", tc.supplied)
			rec := httptest.NewRecorder()
			TickHandler(tc.token, nil).ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d; want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestCappedTicksEnqueueWithoutCallingModels(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); t.Error("capped tick called LLM") }))
	defer upstream.Close()
	st := storetest.Open(t)
	cases, art, err := proofqc.Cases()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := New(Config{Store: st, LLM: &llm.Client{BaseURL: upstream.URL}, Cases: cases, Artwork: art, DailyCapUSD: 0})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		req := httptest.NewRequest("POST", "/internal/tick", nil)
		req.Header.Set("X-Tick-Token", "test-token")
		rec := httptest.NewRecorder()
		TickHandler("test-token", svc).ServeHTTP(rec, req)
		var result TickResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || !result.Capped || result.Processed != 0 {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
		}
	}
	orders, err := st.RecentOrders(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || len(orders) != 2 || orders[0].Status != "queued" || orders[1].Status != "queued" || orders[0].CaseID == orders[1].CaseID {
		t.Fatalf("calls=%d orders=%+v", calls.Load(), orders)
	}
}
