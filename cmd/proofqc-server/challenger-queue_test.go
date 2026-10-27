package main

import (
	"reflect"
	"testing"
)

func TestConfigChallengerQueue(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		want        []string
	}{
		{"default", "", []string{"google/gemma-3-27b-it", "x-ai/grok-4.7"}},
		{"custom ordered", " x-ai/grok-4.7 , google/gemma-3-27b-it , x-ai/grok-4.7 ", []string{"x-ai/grok-4.7", "google/gemma-3-27b-it"}},
		{"empty entries", ",, google/gemma-3-27b-it, ,", []string{"google/gemma-3-27b-it"}},
		{"explicit empty", ",", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := loadConfig(env(map[string]string{"DATABASE_URL": "postgres://x", "OPENROUTER_API_KEY": "k", "CHALLENGER_QUEUE": tc.value}))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.challengerQueue, tc.want) {
				t.Fatalf("queue=%v; want %v", c.challengerQueue, tc.want)
			}
		})
	}
}
