// Command genfixtures draws the eval artwork and writes testdata/cases/cases.json.
// The expected decision of every case is typed by hand from the labelling rules
// in the README; nothing here runs a reviewer.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
)

type background int

const (
	transparentBG background = iota
	whiteBG
	creamBG
)

type recipe struct {
	id       string
	dpi      float64
	product  artwork.Product
	w, h     float64
	bg       background
	word     string
	fill     color.NRGBA
	empty    bool
	bleed    bool   // full-bleed gradient, edge to edge
	hairline bool   // 1/64 in strokes
	tinyText string // a tagline about 1/20 in tall
	nearEdge bool   // a bar running into the safe margin
	customer string
	comment  string
	want     agent.Decision
	why      string
}

var (
	red    = color.NRGBA{214, 40, 57, 255}
	navy   = color.NRGBA{29, 53, 87, 255}
	green  = color.NRGBA{42, 157, 143, 255}
	orange = color.NRGBA{244, 140, 6, 255}
	white  = color.NRGBA{255, 255, 255, 255}
)

const (
	ready    = agent.ReadyForProof
	recreate = agent.RecreateArtwork
	ask      = agent.AskCustomer
	die      = artwork.DieCut
	rect     = artwork.Rectangle
)

var recipes = []recipe{
	{id: "clean-diecut", dpi: 300, product: die, w: 3, h: 3, word: "MULE", fill: red, customer: "Brian Smith",
		want: ready, why: "300 dpi, transparent, no comment"},
	{id: "clean-rectangle", dpi: 300, product: rect, w: 3, h: 2, bg: whiteBG, word: "TACOS", fill: orange, customer: "Ana Lopez",
		want: ready, why: "rectangle keeps its white background"},
	{id: "white-box-diecut", dpi: 300, product: die, w: 3, h: 3, bg: whiteBG, word: "BREW", fill: navy, customer: "Kenji Sato",
		want: ready, why: "designer removes the white box so the cut follows the art"},
	{id: "keep-square-on-diecut", dpi: 300, product: die, w: 3, h: 3, bg: creamBG, word: "BAKERY", fill: navy, customer: "Olivia Brown",
		comment: "Please keep the cream background as a square, don't cut around the logo.",
		want:    ask, why: "asks for a square sticker on a die-cut order: product change"},
	{id: "lowres-diecut", dpi: 100, product: die, w: 3, h: 3, word: "ROCK", fill: green, customer: "Marcus Lee",
		want: recreate, why: "100 dpi"},
	{id: "lowres-rectangle-sharp", dpi: 120, product: rect, w: 4, h: 2, bg: whiteBG, word: "SURF CO", fill: navy, customer: "Priya Patel",
		comment: "Make sure it looks sharp please",
		want:    recreate, why: "120 dpi; the comment asks for what recreating gives"},
	{id: "medium-res", dpi: 200, product: die, w: 3, h: 3, word: "HIVE", fill: orange, customer: "Tom Becker",
		want: ready, why: "200 dpi prints"},
	{id: "hairlines", dpi: 300, product: die, w: 3, h: 3, word: "WAVE", fill: navy, hairline: true, customer: "Sofia Rossi",
		want: ready, why: "designer thickens hairlines"},
	{id: "near-edge-rectangle", dpi: 300, product: rect, w: 3, h: 2, bg: whiteBG, word: "PIZZA", fill: red, nearEdge: true, customer: "Luca Bianchi",
		want: ready, why: "designer moves the bar inside the safe margin"},
	{id: "empty-file", dpi: 300, product: die, w: 3, h: 3, empty: true, customer: "Emma Wilson",
		want: ask, why: "no artwork"},
	{id: "wrong-file-attached", dpi: 300, product: die, w: 3, h: 3, word: "OLD LOGO", fill: green, customer: "Noah Davis",
		comment: "Oops, I attached the wrong file. I'll email you the right logo tonight.",
		want:    ask, why: "the right file is not attached"},
	{id: "size-change", dpi: 300, product: die, w: 3, h: 3, word: "GYM", fill: red, customer: "Mia Johnson",
		comment: "Can you make these 5 inches instead of 3?",
		want:    ask, why: "size differs from the order"},
	{id: "text-edit", dpi: 300, product: rect, w: 4, h: 2, bg: whiteBG, word: "OPEN", fill: green, customer: "Liam Garcia",
		comment: "Please change the text to say OPEN LATE.",
		want:    ready, why: "designer edits text the customer wrote out"},
	{id: "remove-white-bg", dpi: 300, product: die, w: 3, h: 3, bg: whiteBG, word: "FOX", fill: orange, customer: "Chloe Martin",
		comment: "Remove the white background please.",
		want:    ready, why: "designer removes the background"},
	{id: "missing-blue-version", dpi: 300, product: die, w: 3, h: 3, word: "SKATE", fill: red, customer: "Ethan Clark",
		comment: "Use the blue version of the logo, not the red one.",
		want:    ask, why: "only the red version is attached"},
	{id: "conflicting-background", dpi: 300, product: die, w: 3, h: 3, bg: whiteBG, word: "DONUT", fill: orange, customer: "Grace Kim",
		comment: "Make the background transparent but keep the white box around the logo.",
		want:    ask, why: "two instructions contradict each other"},
	{id: "lowres-sending-hires", dpi: 110, product: die, w: 3, h: 3, word: "BAND", fill: navy, customer: "Jack Turner",
		comment: "This one is a bit blurry, I'll upload the high-res file tomorrow.",
		want:    ask, why: "the customer is sending a better file; wait for it"},
	{id: "tiny-tagline", dpi: 300, product: die, w: 3, h: 3, word: "CAFE", fill: green, tinyText: "est. 2019 - small batch roasters", customer: "Hannah Scott",
		comment: "Make sure the small text is readable.",
		want:    ready, why: "designer enlarges or thickens the tagline"},
	{id: "rush-request", dpi: 300, product: die, w: 2, h: 2, word: "YAY", fill: red, customer: "Owen Hughes",
		comment: "Love you guys! Rush please, need them for Saturday.",
		want:    ready, why: "turnaround request does not block the proof"},
	{id: "add-phone-number", dpi: 300, product: rect, w: 4, h: 2, bg: whiteBG, word: "PLUMBING", fill: navy, customer: "Daniel Young",
		comment: "Can you add my phone number 555-0100 under the logo?",
		want:    ready, why: "designer adds text the customer wrote out"},
	{id: "lowres-please-vectorize", dpi: 140, product: die, w: 3, h: 3, word: "WOLF", fill: navy, customer: "Ava Walker",
		comment: "Please recreate this as a vector if needed.",
		want:    recreate, why: "140 dpi and the customer asks for it"},
	{id: "fullbleed-photo", dpi: 300, product: rect, w: 3, h: 2, bleed: true, customer: "Lucas Allen",
		want: ready, why: "full-bleed rectangle at 300 dpi"},
	{id: "medium-thin-whitebox", dpi: 160, product: die, w: 3, h: 3, bg: whiteBG, word: "ZEN", fill: green, hairline: true, customer: "Ella King",
		want: ready, why: "every finding is a designer fix"},
	{id: "holographic", dpi: 300, product: die, w: 3, h: 3, word: "STAR", fill: navy, customer: "Leo Wright",
		comment: "Please print these on holographic vinyl.",
		want:    ask, why: "material differs from the order"},
	{id: "same-size-confirmation", dpi: 300, product: die, w: 2, h: 2, word: "BEE", fill: orange, customer: "Zoe Hill",
		comment: "Please print it at 2x2 inches.",
		want:    ready, why: "the size matches the order"},
	{id: "spanish-remove-bg", dpi: 300, product: die, w: 3, h: 3, bg: whiteBG, word: "SOL", fill: orange, customer: "Diego Ramirez",
		comment: "Por favor quiten el fondo blanco.",
		want:    ready, why: "asks, in Spanish, to remove the white background"},
}

func main() {
	out := flag.String("out", "testdata/cases", "output directory")
	flag.Parse()
	face := mustFace()
	var cases []eval.Case
	for _, r := range append(recipes, generated(80)...) {
		file := r.id + ".png"
		if err := writePNG(filepath.Join(*out, file), draw(r, face)); err != nil {
			log.Fatal(err)
		}
		cases = append(cases, eval.Case{
			ID: r.id, File: file, Want: r.want, Why: r.why,
			Order: artwork.Order{Customer: r.customer, Product: r.product, WidthIn: r.w, HeightIn: r.h, Comment: r.comment},
		})
	}
	raw, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "cases.json"), append(raw, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %d cases to %s\n", len(cases), *out)
}

func mustFace() *opentype.Font {
	f, err := opentype.Parse(gobold.TTF)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func draw(r recipe, f *opentype.Font) *image.NRGBA {
	w, h := int(math.Round(r.w*r.dpi)), int(math.Round(r.h*r.dpi))
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	switch r.bg {
	case whiteBG:
		fillRect(img, img.Bounds(), white)
	case creamBG:
		fillRect(img, img.Bounds(), color.NRGBA{250, 240, 215, 255})
	}
	if r.empty {
		return img
	}
	if r.bleed {
		for y := range h {
			for x := range w {
				img.Set(x, y, color.NRGBA{uint8(40 + 180*x/w), uint8(90 + 120*y/h), 160, 255})
			}
		}
		return img
	}

	cx, cy := w/2, h*2/5
	rad := min(w, h) / 4
	fillCircle(img, cx, cy, rad, r.fill)
	// Main words stay well above 1/32 in strokes; only the tagline and hairlines are thin.
	size := math.Min(float64(h)*0.18, 1.3*float64(w)/float64(len(r.word)))
	text(img, f, r.word, size, cx, cy+rad+int(size*0.95), r.fill)
	if r.tinyText != "" {
		text(img, f, r.tinyText, r.dpi/20, cx, h*9/10, navy)
	}
	if r.hairline {
		stroke := max(1, int(r.dpi/64))
		for i := -3; i <= 3; i++ {
			x := cx + i*rad/4
			fillRect(img, image.Rect(x, cy-rad-int(r.dpi/4), x+stroke, cy-rad-int(r.dpi/20)), r.fill)
		}
	}
	if r.nearEdge {
		fillRect(img, image.Rect(2, h-int(r.dpi/4), w-2, h-2), r.fill)
	}
	return img
}

func fillRect(img *image.NRGBA, b image.Rectangle, c color.NRGBA) {
	b = b.Intersect(img.Bounds())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func fillCircle(img *image.NRGBA, cx, cy, r int, c color.NRGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

// text draws a centred line with its baseline at y.
func text(img *image.NRGBA, f *opentype.Font, s string, size float64, cx, y int, c color.NRGBA) {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		log.Fatal(err)
	}
	defer face.Close()
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face}
	width := d.MeasureString(s).Round()
	d.Dot = fixed.P(cx-width/2, y)
	d.DrawString(s)
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
