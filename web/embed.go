// Package web serves the demo page. The TypeScript is built into dist/app.js
// and committed, so the Go binary embeds everything and needs no Node at run
// or build time.
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
	"time"
)

//go:embed index.html style.css dist/app.js
var files embed.FS

type asset struct {
	contentType string
	body        []byte
	etag        string
}

// routes maps each URL path to the one file it serves; anything else is a 404.
var routes = map[string]string{
	"/":          "index.html",
	"/app.js":    "dist/app.js",
	"/style.css": "style.css",
}

var contentTypes = map[string]string{
	"index.html":  "text/html; charset=utf-8",
	"dist/app.js": "text/javascript; charset=utf-8",
	"style.css":   "text/css; charset=utf-8",
}

// csp allows only this origin's script, style and API, plus uploaded previews.
const csp = "default-src 'self'; img-src 'self' data: blob:; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// Handler serves the page and its two assets with ETags, so a reload costs a 304.
func Handler() http.Handler {
	assets := make(map[string]asset, len(routes))
	for path, name := range routes {
		body, err := files.ReadFile(name)
		if err != nil {
			panic("web: embedded file missing: " + name) // a build defect, caught by the tests
		}
		sum := sha256.Sum256(body)
		assets[path] = asset{contentTypes[name], body, `"` + hex.EncodeToString(sum[:8]) + `"`}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a, ok := assets[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", a.contentType)
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-cache")
		h.Set("ETag", a.etag)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(a.body))
	})
}
