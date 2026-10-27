package artwork

import (
	"image"
	"slices"
	"testing"
	"time"
)

// sameArt draws one design at any resolution: a white box with a red block and
// hairlines 1/64 in wide, so the background and thin-line checks both fire.
func sameArt(dpi int) *image.NRGBA {
	in := func(v float64) int { return int(v * float64(dpi)) }
	img := canvas(in(3), in(3), white)
	fill(img, image.Rect(in(0.6), in(0.6), in(2.4), in(2)), red)
	for i := range 10 {
		x := 0.5 + float64(i)*0.2
		fill(img, image.Rect(in(x), in(2.3), in(x+1.0/64), in(2.8)), black)
	}
	return img
}

// raceEnabled is set under -race, which slows every memory access by an order
// of magnitude; the time budget is checked by a plain test run in CI instead.
var raceEnabled bool

func TestLargeUploadMatchesTheSameArtAt600DPI(t *testing.T) {
	if raceEnabled {
		t.Skip("time budget is measured without the race detector")
	}
	want := Preflight(sameArt(600), dieCut(3))

	start := time.Now()
	got := Preflight(sameArt(2000), dieCut(3)) // 6000x6000 px
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("preflight took %s, want under 2s", took)
	}
	if !slices.Equal(got.Flags(), want.Flags()) {
		t.Errorf("flags = %v, want %v (as at 600 dpi)", got.Flags(), want.Flags())
	}
	if !slices.Contains(want.Flags(), ThinLines) || !slices.Contains(want.Flags(), SolidBackground) {
		t.Fatalf("fixture flags = %v, want solid_background and thin_lines", want.Flags())
	}
	if got.EffectiveDPI != 2000 || got.WidthPx != 6000 || got.HeightPx != 6000 {
		t.Errorf("report = %d x %d px at %v dpi, want the original 6000 x 6000 at 2000", got.WidthPx, got.HeightPx, got.EffectiveDPI)
	}
}
