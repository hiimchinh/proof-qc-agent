package api

import (
	"context"
	"fmt"
	"image"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

// models are the reviewers the eval kept; the first is the cheapest and the default.
var models = []string{
	"anthropic/claude-haiku-5.5",
	"anthropic/claude-sonnet-5.5",
	"openai/gpt-6.1-terra",
	"x-ai/grok-4.7",
}

const (
	maxCustomerRunes = 100
	maxCommentRunes  = 1000
	modelTimeout     = 90 * time.Second
)

type reviewInput struct {
	Customer    string
	Product     string
	WidthIn     float64
	HeightIn    float64
	Comment     string // defaulted by the schema, so never null
	Model       *string
	ImageBase64 *string
	SampleID    *string
}

func badInput(format string, args ...any) error {
	return &apiError{codeBadInput, fmt.Sprintf(format, args...)}
}

// ReviewArtwork validates everything a visitor sent before anything costs
// money, then checks the visitor's hourly quota and the demo's daily budget.
func (r *resolver) ReviewArtwork(ctx context.Context, args struct{ Input reviewInput }) (*reviewView, error) {
	in := args.Input
	model := models[0]
	if in.Model != nil && *in.Model != "" {
		model = *in.Model
	}
	if !slices.Contains(models, model) {
		return nil, badInput("model %q is not offered; pick one of %s", model, strings.Join(models, ", "))
	}
	o := artwork.Order{
		Customer: strings.TrimSpace(in.Customer), Product: artwork.Product(in.Product),
		WidthIn: in.WidthIn, HeightIn: in.HeightIn, Comment: strings.TrimSpace(in.Comment),
	}
	if err := checkOrder(o); err != nil {
		return nil, err
	}
	img, err := r.image(in)
	if err != nil {
		if _, ok := err.(*apiError); ok {
			return nil, err
		}
		r.log.ErrorContext(ctx, "load artwork", "err", err)
		return nil, errInternal
	}

	ip := clientIPFrom(ctx)
	if !r.limiter.allow(ip) {
		return nil, &apiError{codeRateLimited, fmt.Sprintf("at most %d reviews an hour from one address; try again later", reviewsPerHour)}
	}
	spent, err := r.store.SpentSince(ctx, utcMidnight(r.now()))
	if err != nil {
		r.log.ErrorContext(ctx, "daily spend", "err", err)
		return nil, errInternal
	}
	if spent >= r.capUSD {
		return nil, &apiError{codeDailyCap, fmt.Sprintf("the demo's model budget of $%.2f for today is spent; it resets at 00:00 UTC", r.capUSD)}
	}

	callCtx, cancel := context.WithTimeout(ctx, modelTimeout)
	defer cancel()
	res, callErr := agent.Agent{Model: model, LLM: r.llm}.Review(callCtx, img, o)
	saved := store.Review{ClientIP: ip, Model: model, Order: o, Result: res, CostUSD: res.CostUSD, CreatedAt: r.now()}
	if callErr == nil || res.CostUSD > 0 {
		// Record the spend even if the visitor has gone, so the cap stays honest.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if s, err := r.store.Save(saveCtx, saved); err != nil {
			r.log.ErrorContext(ctx, "save review", "err", err, "model", model, "cost_usd", res.CostUSD)
		} else {
			saved = s
		}
	}
	if callErr != nil {
		r.log.WarnContext(ctx, "model call failed", "err", callErr, "model", model)
		return nil, &apiError{codeModelFailed, "the model did not return a usable answer; try again or pick another model"}
	}
	v := newReviewView(saved)
	return &v, nil
}

func checkOrder(o artwork.Order) error {
	if err := o.Validate(); err != nil {
		return badInput("order: %v", err)
	}
	if o.Customer == "" || utf8.RuneCountInString(o.Customer) > maxCustomerRunes {
		return badInput("customer name is required, at most %d characters", maxCustomerRunes)
	}
	if utf8.RuneCountInString(o.Comment) > maxCommentRunes {
		return badInput("comment is at most %d characters", maxCommentRunes)
	}
	return nil
}

// image returns the uploaded artwork or the sample's, exactly one of which must be given.
func (r *resolver) image(in reviewInput) (image.Image, error) {
	upload, sample := deref(in.ImageBase64), deref(in.SampleID)
	if (upload == "") == (sample == "") {
		return nil, badInput("give either imageBase64 or sampleId")
	}
	if upload != "" {
		return decodeUpload(upload)
	}
	c, ok := r.cases[sample]
	if !ok {
		return nil, badInput("no sample with id %q", sample)
	}
	f, err := r.art.Open(c.File)
	if err != nil {
		return nil, fmt.Errorf("sample %s: %w", c.ID, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("sample %s: %w", c.ID, err)
	}
	return img, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
