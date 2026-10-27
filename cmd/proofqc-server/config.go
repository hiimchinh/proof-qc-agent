package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hiimchinh/proof-qc-agent/internal/desk"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
)

type config struct {
	challengerQueue                 []string
	addr                            string
	databaseURL                     string
	apiKey                          string
	dailyCapUSD                     float64
	tickToken, champion, challenger string
}

// loadConfig reads the environment once at startup and fails on anything
// missing or malformed, so a bad deploy stops before it serves traffic.
func loadConfig(getenv func(string) string) (config, error) {
	c := config{addr: ":8080", dailyCapUSD: 3, champion: desk.DefaultChampion, challenger: desk.DefaultChallenger}
	queue := strings.TrimSpace(getenv("CHALLENGER_QUEUE"))
	if queue == "" {
		queue = fleet.DefaultQueue
	}
	c.challengerQueue = []string{}
	seen := map[string]bool{}
	for part := range strings.SplitSeq(queue, ",") {
		model := strings.TrimSpace(part)
		if model != "" && !seen[model] {
			c.challengerQueue = append(c.challengerQueue, model)
			seen[model] = true
		}
	}
	c.tickToken = getenv("TICK_TOKEN")
	if model := strings.TrimSpace(getenv("CHAMPION_MODEL")); model != "" {
		c.champion = model
	}
	if model := strings.TrimSpace(getenv("CHALLENGER_MODEL")); model != "" {
		c.challenger = model
	}
	var errs []error
	if p := strings.TrimSpace(getenv("PORT")); p != "" {
		if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
			errs = append(errs, fmt.Errorf("PORT %q is not a port number", p))
		} else {
			c.addr = ":" + p
		}
	}
	if c.databaseURL = strings.TrimSpace(getenv("DATABASE_URL")); c.databaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.apiKey = strings.TrimSpace(getenv("OPENROUTER_API_KEY")); c.apiKey == "" {
		errs = append(errs, errors.New("OPENROUTER_API_KEY is required"))
	}
	if s := strings.TrimSpace(getenv("DAILY_USD_CAP")); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 || math.IsInf(v, 0) || math.IsNaN(v) {
			errs = append(errs, fmt.Errorf("DAILY_USD_CAP %q must be a dollar amount of 0 or more", s))
		} else {
			c.dailyCapUSD = v
		}
	}
	return c, errors.Join(errs...)
}
