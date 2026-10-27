package artwork

import (
	"image"
	"image/color"
	"slices"
	"testing"
)

func canvas(w, h int, bg color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, bg)
		}
	}
	return img
}

func fill(img *image.NRGBA, r image.Rectangle, c color.Color) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Set(x, y, c)
		}
	}
}

var (
	clear = color.NRGBA{}
	white = color.NRGBA{255, 255, 255, 255}
	black = color.NRGBA{0, 0, 0, 255}
	red   = color.NRGBA{200, 30, 40, 255}
)

func dieCut(in float64) Order {
	return Order{Customer: "Brian", Product: DieCut, WidthIn: in, HeightIn: in}
}

func TestEffectiveDPIUsesTheSmallerAxis(t *testing.T) {
	img := canvas(900, 600, clear)
	fill(img, image.Rect(100, 100, 800, 500), red)
	// 900 px over 3 in = 300 dpi, 600 px over 3 in = 200 dpi.
	r := Preflight(img, Order{Product: DieCut, WidthIn: 3, HeightIn: 3})
	if r.EffectiveDPI != 200 {
		t.Fatalf("EffectiveDPI = %v, want 200", r.EffectiveDPI)
	}
	if !r.Has(MediumResolution) || r.Has(LowResolution) {
		t.Fatalf("flags = %v, want medium_resolution only", r.Flags())
	}
}

func TestResolutionThresholds(t *testing.T) {
	cases := []struct {
		px   int
		want []Flag
	}{
		{900, nil},                      // 300 dpi at 3 in
		{600, []Flag{MediumResolution}}, // 200 dpi
		{300, []Flag{LowResolution}},    // 100 dpi
		{447, []Flag{LowResolution}},    // 149 dpi
		{450, []Flag{MediumResolution}}, // 150 dpi
	}
	for _, c := range cases {
		img := canvas(c.px, c.px, clear)
		m := c.px / 6
		fill(img, image.Rect(m, m, c.px-m, c.px-m), red)
		got := Preflight(img, dieCut(3)).Flags()
		if !slices.Equal(got, c.want) {
			t.Errorf("%d px: flags = %v, want %v", c.px, got, c.want)
		}
	}
}

func TestSolidBackgroundOnDieCut(t *testing.T) {
	img := canvas(900, 900, white)
	fill(img, image.Rect(200, 200, 700, 700), red)
	if r := Preflight(img, dieCut(3)); !r.Has(SolidBackground) {
		t.Fatalf("flags = %v, want solid_background", r.Flags())
	}
}

func TestSolidBackgroundIsFineOnRectangle(t *testing.T) {
	img := canvas(900, 900, white)
	fill(img, image.Rect(200, 200, 700, 700), red)
	o := dieCut(3)
	o.Product = Rectangle
	if r := Preflight(img, o); r.Has(SolidBackground) {
		t.Fatalf("flags = %v, rectangle stickers keep their background", r.Flags())
	}
}

func TestContentNearEdgeOnRectangle(t *testing.T) {
	// 300 dpi; the safe margin of 1/16 in is about 19 px.
	img := canvas(900, 900, white)
	fill(img, image.Rect(5, 300, 600, 600), red)
	o := dieCut(3)
	o.Product = Rectangle
	if r := Preflight(img, o); !r.Has(ContentNearEdge) {
		t.Fatalf("flags = %v, want content_near_edge", r.Flags())
	}
}

func TestThinLines(t *testing.T) {
	// 300 dpi; a 1/32 in line is about 9 px. Hairlines of 3 px are too thin to print and cut.
	img := canvas(900, 900, clear)
	fill(img, image.Rect(200, 200, 700, 700), red)
	for i := range 20 {
		x := 100 + i*30
		fill(img, image.Rect(x, 720, x+3, 880), black)
	}
	if r := Preflight(img, dieCut(3)); !r.Has(ThinLines) {
		t.Fatalf("flags = %v, want thin_lines", r.Flags())
	}
}

func TestBoldArtworkHasNoThinLines(t *testing.T) {
	img := canvas(900, 900, clear)
	fill(img, image.Rect(200, 200, 700, 700), red)
	if r := Preflight(img, dieCut(3)); r.Has(ThinLines) {
		t.Fatalf("flags = %v, want none", r.Flags())
	}
}

func TestRoundedBoldShapesAreNotThin(t *testing.T) {
	// A filled disc loses its rim pixels to a square opening; that rim is not a thin line.
	img := canvas(900, 900, clear)
	for y := range 900 {
		for x := range 900 {
			if (x-450)*(x-450)+(y-450)*(y-450) <= 200*200 {
				img.Set(x, y, red)
			}
		}
	}
	if r := Preflight(img, dieCut(3)); r.Has(ThinLines) {
		t.Fatalf("flags = %v, want none", r.Flags())
	}
}

func TestBoldDiagonalStrokeIsNotThin(t *testing.T) {
	// A 45 degree stroke 12 px wide (about 1/25 in at 300 dpi) is thick enough to print.
	img := canvas(900, 900, clear)
	for y := range 900 {
		for x := range 900 {
			if d := float64(x-y) / 1.41421356; d >= -6 && d < 6 && x > 100 && x < 800 {
				img.Set(x, y, red)
			}
		}
	}
	if r := Preflight(img, dieCut(3)); r.Has(ThinLines) {
		t.Fatalf("flags = %v, want none", r.Flags())
	}
}

func TestOneShortHairlineIsEnough(t *testing.T) {
	// A single 1 in hairline next to a large solid shape still fails at print size.
	img := canvas(900, 900, clear)
	fill(img, image.Rect(100, 100, 800, 600), red)
	fill(img, image.Rect(100, 700, 400, 703), black)
	if r := Preflight(img, dieCut(3)); !r.Has(ThinLines) {
		t.Fatalf("flags = %v, want thin_lines", r.Flags())
	}
}

func TestWhiteOnTransparent(t *testing.T) {
	img := canvas(900, 900, clear)
	fill(img, image.Rect(200, 200, 700, 700), white)
	if r := Preflight(img, dieCut(3)); !r.Has(WhiteOnTransparent) {
		t.Fatalf("flags = %v, want white_on_transparent", r.Flags())
	}
}

func TestEmptyArtwork(t *testing.T) {
	img := canvas(900, 900, clear)
	if r := Preflight(img, dieCut(3)); !r.Has(EmptyArtwork) {
		t.Fatalf("flags = %v, want empty_artwork", r.Flags())
	}
}
