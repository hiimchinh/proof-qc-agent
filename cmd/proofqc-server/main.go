// Command proofqc-server runs the live demo: the page, the GraphQL API and a
// Postgres log of every review, with a per-visitor rate limit and a daily
// spend cap in front of the paid model call.
//
// Environment: PORT (8080), DATABASE_URL, OPENROUTER_API_KEY, DAILY_USD_CAP (3).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	openCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	st, err := store.Open(openCtx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	mux, err := newMux(st, cfg.apiKey, cfg.dailyCapUSD)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute, // a model call may take up to 90 s
		IdleTimeout:       2 * time.Minute,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.addr, "daily_usd_cap", cfg.dailyCapUSD)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	// Cloud Run allows 10 s after SIGTERM; let in-flight requests finish within it.
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
