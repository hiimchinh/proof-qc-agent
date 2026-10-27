// Package api serves the demo's GraphQL API: review one artwork with a chosen
// model, and read the leaderboard, samples, recent reviews and today's spend.
package api

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	graphql "github.com/graph-gophers/graphql-go"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/results"
)

//go:embed schema.graphql
var schemaSDL string

// maxBody fits a 4 MB image after base64 (about 5.4 MB) plus the query.
const maxBody = 6 << 20

// Config is everything the API needs; Store and LLM are required.
type Config struct {
	Store       *store.Store
	LLM         agent.Completer
	Cases       []eval.Case // the samples a visitor can review without an upload
	Artwork     fs.FS       // the samples' artwork, keyed by Case.File
	Leaderboard []results.Score
	DailyCapUSD float64
	Logger      *slog.Logger // slog.Default() when nil
}

// New returns the GraphQL handler. It accepts POST requests with a JSON body.
func New(cfg Config) (http.Handler, error) {
	if cfg.Store == nil || cfg.LLM == nil || cfg.Artwork == nil {
		return nil, errors.New("api: Store, LLM and Artwork are required")
	}
	if cfg.DailyCapUSD < 0 {
		return nil, errors.New("api: DailyCapUSD must not be negative")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	r := newResolver(cfg)
	schema, err := graphql.ParseSchema(schemaSDL, r,
		graphql.UseFieldResolvers(),
		graphql.MaxDepth(6),
		graphql.MaxQueryLength(8<<10),
	)
	if err != nil {
		return nil, err
	}
	return &handler{schema: schema}, nil
}

type handler struct{ schema *graphql.Schema }

type request struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "send GraphQL requests as POST with a JSON body")
		return
	}
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body is over 6 MB")
			return
		}
		writeError(w, http.StatusBadRequest, "request body is not a GraphQL JSON request")
		return
	}
	ctx := withClientIP(r.Context(), clientIP(r))
	resp := h.schema.Exec(ctx, req.Query, req.OperationName, req.Variables)
	writeJSON(w, http.StatusOK, resp)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"errors": []*apiError{{code: codeBadInput, msg: msg}}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error codes a client can branch on, in extensions.code.
const (
	codeBadInput    = "BAD_INPUT"
	codeRateLimited = "RATE_LIMITED"
	codeDailyCap    = "DAILY_CAP"
	codeModelFailed = "MODEL_FAILED"
	codeInternal    = "INTERNAL"
)

// apiError is a resolver error the visitor can act on; graphql-go copies
// Extensions into the response.
type apiError struct{ code, msg string }

func (e *apiError) Error() string              { return e.msg }
func (e *apiError) Extensions() map[string]any { return map[string]any{"code": e.code} }

func (e *apiError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"message": e.msg, "extensions": e.Extensions()})
}

var errInternal = &apiError{codeInternal, "something went wrong on our side; try again"}

// utcMidnight is when the daily budget resets.
func utcMidnight(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
