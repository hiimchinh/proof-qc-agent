package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// limiter allows n calls per client in any sliding window. It lives in memory,
// which is enough for a demo that runs as one instance; the daily spend cap in
// Postgres is the guard that holds across instances.
type limiter struct {
	n      int
	window time.Duration
	now    func() time.Time
	mu     sync.Mutex
	calls  map[string][]time.Time
}

func newLimiter(n int, window time.Duration, now func() time.Time) *limiter {
	return &limiter{n: n, window: window, now: now, calls: map[string][]time.Time{}}
}

// maxClients bounds memory: past it, clients with no recent calls are dropped.
const maxClients = 10_000

// allow records a call for key and reports whether it is within the limit.
func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.calls) > maxClients {
		for k, ts := range l.calls {
			if len(l.recent(ts, now)) == 0 {
				delete(l.calls, k)
			}
		}
	}
	ts := l.recent(l.calls[key], now)
	if len(ts) >= l.n {
		l.calls[key] = ts
		return false
	}
	l.calls[key] = append(ts, now)
	return true
}

// recent drops calls that have left the window; ts is in call order.
func (l *limiter) recent(ts []time.Time, now time.Time) []time.Time {
	cut := now.Add(-l.window)
	i := 0
	for i < len(ts) && !ts[i].After(cut) {
		i++
	}
	return ts[i:]
}

// clientIP is the last X-Forwarded-For hop: Cloud Run's front end appends the
// address it saw, while anything before it came from the visitor and can be
// forged. Without the header it is the connection's address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		hops := strings.Split(xff, ",")
		if ip := strings.TrimSpace(hops[len(hops)-1]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type ipKey struct{}

func withClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ipKey{}, ip)
}

func clientIPFrom(ctx context.Context) string {
	ip, _ := ctx.Value(ipKey{}).(string)
	return ip
}
