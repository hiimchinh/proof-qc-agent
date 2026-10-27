package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type config struct {
	addr        string
	databaseURL string
	apiKey      string
	dailyCapUSD float64
}

// loadConfig reads the environment once at startup and fails on anything
// missing or malformed, so a bad deploy stops before it serves traffic.
func loadConfig(getenv func(string) string) (config, error) {
	c := config{addr: ":8080", dailyCapUSD: 3}
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
