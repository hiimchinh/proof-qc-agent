package main

import (
	"strings"
	"testing"
)

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestConfigDefaults(t *testing.T) {
	c, err := loadConfig(env(map[string]string{"DATABASE_URL": "postgres://x", "OPENROUTER_API_KEY": "k"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.addr != ":8080" || c.dailyCapUSD != 3 || c.databaseURL != "postgres://x" || c.apiKey != "k" {
		t.Fatalf("config = %+v", c)
	}
}

func TestConfigReadsEveryVariable(t *testing.T) {
	c, err := loadConfig(env(map[string]string{
		"PORT": "9090", "DATABASE_URL": "postgres://x", "OPENROUTER_API_KEY": "k", "DAILY_USD_CAP": "0",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.addr != ":9090" || c.dailyCapUSD != 0 {
		t.Fatalf("config = %+v, want port 9090 and a zero cap", c)
	}
}

func TestConfigRejectsBadValues(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "postgres://x", "OPENROUTER_API_KEY": "k"}
	cases := map[string]struct{ key, value, want string }{
		"no database":  {"DATABASE_URL", "", "DATABASE_URL"},
		"no key":       {"OPENROUTER_API_KEY", " ", "OPENROUTER_API_KEY"},
		"cap not a $":  {"DAILY_USD_CAP", "three", "DAILY_USD_CAP"},
		"negative cap": {"DAILY_USD_CAP", "-1", "DAILY_USD_CAP"},
		"bad port":     {"PORT", "http", "PORT"},
	}
	for name, c := range cases {
		kv := map[string]string{}
		for k, v := range base {
			kv[k] = v
		}
		kv[c.key] = c.value
		if _, err := loadConfig(env(kv)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one naming %s", name, err, c.want)
		}
	}
}
