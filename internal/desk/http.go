package desk

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// TickHandler is disabled without a token. Cloud Scheduler supplies X-Tick-Token.
func TickHandler(token string, s *Service) http.Handler {
	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if token == "" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "POST required", 405)
			return
		}
		supplied := sha256.Sum256([]byte(r.Header.Get("X-Tick-Token")))
		if subtle.ConstantTimeCompare(supplied[:], expected[:]) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		out, err := s.Tick(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "desk tick failed", "err", err)
			http.Error(w, "tick failed; inspect server logs", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
}
