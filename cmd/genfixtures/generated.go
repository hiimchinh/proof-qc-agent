package main

import (
	"fmt"
	"image/color"
	"math/rand/v2"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
)

// comment is one customer comment with its hand-assigned effect. askCustomer
// comments need the customer whatever the artwork; the others leave the decision
// to the artwork (recreate under 150 dpi, otherwise ready).
type comment struct {
	kind        string
	text        func(r recipe) string
	askCustomer bool
}

func literal(s string) func(recipe) string { return func(recipe) string { return s } }

var comments = []comment{
	// No effect on the decision.
	{"none", literal(""), false},
	{"rush", literal("Need these by Friday if possible, thanks!"), false},
	{"praise", literal("Love your stickers, ordered from you three times now."), false},
	{"same-size", func(r recipe) string { return fmt.Sprintf("Please print at %gx%g inches.", r.w, r.h) }, false},
	{"vibrant", literal("Make the colors as bright as you can."), false},
	// Designer fixes.
	{"remove-bg", literal("Please remove the white background."), false},
	{"remove-bg-fr", literal("Merci d'enlever le fond blanc."), false},
	{"remove-bg-de", literal("Bitte den weißen Hintergrund entfernen."), false},
	{"remove-bg-vi", literal("Vui lòng xoá nền trắng giúp mình."), false},
	{"add-text", literal("Add our website, example.com, under the logo."), false},
	{"change-text", literal("Change the word to all lowercase please."), false},
	{"thicker", literal("The thin lines look weak, can you make them bolder?"), false},
	{"center", literal("Center the logo please."), false},
	{"bigger-text", literal("Make the text a bit larger."), false},
	{"redraw-ok", literal("Feel free to redraw it if the file is not good enough."), false},
	// Only the customer can unblock.
	{"size-up", func(r recipe) string { return fmt.Sprintf("Can you make these %g inches wide instead?", r.w+2) }, true},
	{"size-down", func(r recipe) string { return fmt.Sprintf("Actually I need them %gx%g inches.", r.w/2, r.h/2) }, true},
	{"to-square", literal("Keep the background and make it a square sticker, don't cut around the shape."), true},
	{"to-diecut", literal("Please cut around the logo instead of a rectangle."), true},
	{"holo", literal("Please print these on holographic vinyl."), true},
	{"clear", literal("I want these on clear vinyl so the laptop shows through."), true},
	{"glitter", literal("Can we do glitter for this one?"), true},
	{"other-file", literal("Use the green version of the logo instead, I'll send it."), true},
	{"email-later", literal("This is a placeholder, final art coming by email tomorrow."), true},
	{"hires-later", literal("The file is low quality, I'll upload a better one tonight."), true},
	{"contradict", literal("Remove the background but keep the white box around it."), true},
	{"contradict-size", literal("Make it bigger but keep it the same size as the order."), true},
}

var (
	names = []string{"José García", "Zoë Hill", "Nguyễn Văn An", "Łukasz Nowak", "Aisha Bello", "Mei Chen",
		"Sam Carter", "Fatima Khan", "Jonas Müller", "Chloé Dubois", "Raj Mehta", "Kate O'Brien"}
	words   = []string{"MULE", "TACO", "SURF", "BREW", "PIXEL", "BLOOM", "FORGE", "OWL", "DASH", "KIWI", "NOVA", "BOLT"}
	palette = []color.NRGBA{red, navy, green, orange, {120, 40, 160, 255}, {20, 120, 200, 255}}
	sizes   = [][2]float64{{2, 2}, {3, 3}, {4, 2}, {3, 2}, {2.5, 2.5}}
	dpis    = []float64{300, 300, 300, 240, 200, 130, 100}
)

// generated combines artwork and comments deterministically; the label follows
// the README rules from the comment's hand-assigned effect and the dpi.
func generated(n int) []recipe {
	rng := rand.New(rand.NewPCG(2026, 10))
	pick := func(k int) int { return rng.IntN(k) }
	var out []recipe
	for i := range n {
		c := comments[i%len(comments)]
		size := sizes[pick(len(sizes))]
		r := recipe{
			id:       fmt.Sprintf("gen-%03d-%s", i+1, c.kind),
			dpi:      dpis[pick(len(dpis))],
			product:  artwork.DieCut,
			w:        size[0],
			h:        size[1],
			word:     words[pick(len(words))],
			fill:     palette[pick(len(palette))],
			customer: names[pick(len(names))],
			hairline: pick(5) == 0,
		}
		if pick(3) == 0 {
			r.product = artwork.Rectangle
		}
		if r.product == artwork.Rectangle || c.kind == "remove-bg" || pick(4) == 0 {
			r.bg = whiteBG
		}
		switch c.kind {
		case "remove-bg-fr", "remove-bg-de", "remove-bg-vi", "contradict", "to-square":
			r.product, r.bg = artwork.DieCut, whiteBG
		case "to-diecut":
			r.product, r.bg = artwork.Rectangle, whiteBG
		case "hires-later":
			r.dpi = 110
		}
		r.comment = c.text(r)
		switch {
		case c.askCustomer:
			r.want, r.why = agent.AskCustomer, "comment: "+c.kind
		case r.dpi < 150:
			r.want, r.why = agent.RecreateArtwork, fmt.Sprintf("%.0f dpi", r.dpi)
		default:
			r.want, r.why = agent.ReadyForProof, "comment: "+c.kind
		}
		out = append(out, r)
	}
	return out
}
