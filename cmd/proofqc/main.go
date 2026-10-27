// Command proofqc reviews sticker artwork before proofing and benchmarks the
// models that do it.
//
//	proofqc check -product die_cut -w 3 -h 3 -customer "Brian Smith" -comment "..." art.png
//	proofqc eval -models anthropic/claude-sonnet-5.5,x-ai/grok-4.7 -max-usd 5
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

const usage = `usage:
  proofqc check [flags] ARTWORK   review one file (model "baseline" uses rules only)
  proofqc eval  [flags]           score models on testdata/cases and write results/`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "check":
		err = runCheck(os.Args[2:])
	case "eval":
		err = runEval(os.Args[2:])
	default:
		err = errors.New(usage)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	model := fs.String("model", "anthropic/claude-sonnet-5.5", `OpenRouter model id, or "baseline"`)
	product := fs.String("product", string(artwork.DieCut), "die_cut or rectangle")
	w := fs.Float64("w", 3, "width in inches")
	h := fs.Float64("h", 3, "height in inches")
	customer := fs.String("customer", "", "customer name")
	comment := fs.String("comment", "", "customer comment")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("check needs one artwork file")
	}
	img, err := loadImage(fs.Arg(0))
	if err != nil {
		return err
	}
	o := artwork.Order{Customer: *customer, Product: artwork.Product(*product), WidthIn: *w, HeightIn: *h, Comment: *comment}

	var res agent.Result
	if *model == "baseline" {
		res = agent.Baseline(img, o)
	} else {
		client, err := newClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if res, err = (agent.Agent{Model: *model, LLM: client}).Review(ctx, img, o); err != nil {
			return err
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func newClient() (*llm.Client, error) {
	key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if key == "" {
		return nil, errors.New("set OPENROUTER_API_KEY")
	}
	return &llm.Client{APIKey: key}, nil
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}
