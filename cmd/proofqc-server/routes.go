package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/api"
	"github.com/hiimchinh/proof-qc-agent/internal/desk"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/results"
	"github.com/hiimchinh/proof-qc-agent/web"
)

// newMux wires the page, the GraphQL API, the sample artwork and the health check.
func newMux(st *store.Store, cfg config) (http.Handler, error) {
	cases, art, err := proofqc.Cases()
	if err != nil {
		return nil, err
	}
	board, err := results.Leaderboard(results.Embedded(), cases)
	if err != nil {
		// The demo still works without the table; CI fails on this, so it is rare.
		slog.Error("leaderboard unavailable", "err", err)
	}
	client := &llm.Client{APIKey: cfg.apiKey}
	service, err := desk.New(desk.Config{Store: st, LLM: client, Cases: cases, Artwork: art, DailyCapUSD: cfg.dailyCapUSD, Champion: cfg.champion, Challenger: cfg.challenger, ChallengerQueue: cfg.challengerQueue})
	if err != nil {
		return nil, err
	}
	gql, err := api.New(api.Config{
		Store: st, LLM: client, Cases: cases, Artwork: art,
		Leaderboard: board, DailyCapUSD: cfg.dailyCapUSD, ChallengerQueue: cfg.challengerQueue,
	})
	if err != nil {
		return nil, err
	}
	files := make(map[string]bool, len(cases))
	for _, c := range cases {
		files[c.File] = true
	}

	mux := http.NewServeMux()
	mux.Handle("/graphql", gql)
	mux.Handle("/internal/tick", desk.TickHandler(cfg.tickToken, service))
	mux.Handle("GET /samples/{file}", samples(art, files))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if err := st.Ping(ctx); err != nil {
			slog.Warn("health check", "err", err)
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	mux.Handle("/", web.Handler())
	return mux, nil
}

// samples serves only the artwork the cases name, never the rest of the folder.
func samples(art fs.FS, files map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("file")
		if !files[name] || !strings.HasSuffix(name, ".png") {
			http.NotFound(w, r)
			return
		}
		body, err := fs.ReadFile(art, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(body)
	})
}
