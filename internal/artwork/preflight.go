package artwork

import (
	"fmt"
	"image"
	"math"
)

// Flag names one preflight finding.
type Flag string

const (
	EmptyArtwork       Flag = "empty_artwork"
	LowResolution      Flag = "low_resolution"    // under 150 dpi at print size: recreate
	MediumResolution   Flag = "medium_resolution" // 150-299 dpi: prints, but soft
	SolidBackground    Flag = "solid_background"  // die cut would follow the box, not the art
	ContentNearEdge    Flag = "content_near_edge" // inside the 1/16 in safe margin
	ThinLines          Flag = "thin_lines"        // strokes under 1/32 in
	WhiteOnTransparent Flag = "white_on_transparent"
	PrintsSmaller      Flag = "prints_smaller" // die cut follows the art, so wide or tall art leaves the box mostly empty
)

const (
	minDPI        = 150.0
	targetDPI     = 300.0
	safeMarginIn  = 1.0 / 16
	minStrokeIn   = 1.0 / 32
	colorTolerant = 24.0
	// minBoxShare is the smallest share of the ordered box a die-cut sticker may
	// fill before the customer is likely to find it smaller than expected.
	minBoxShare = 0.5
)

// Finding is one flag with the measurement behind it.
type Finding struct {
	Flag    Flag   `json:"flag"`
	Message string `json:"message"`
}

// Report is the preflight result for one artwork file.
type Report struct {
	WidthPx      int       `json:"width_px"`
	HeightPx     int       `json:"height_px"`
	EffectiveDPI float64   `json:"effective_dpi"`
	Findings     []Finding `json:"findings"`
}

// Has reports whether the flag was raised.
func (r Report) Has(f Flag) bool {
	for _, x := range r.Findings {
		if x.Flag == f {
			return true
		}
	}
	return false
}

// Flags lists the raised flags in check order.
func (r Report) Flags() []Flag {
	var out []Flag
	for _, x := range r.Findings {
		out = append(out, x.Flag)
	}
	return out
}

func (r *Report) add(f Flag, format string, args ...any) {
	r.Findings = append(r.Findings, Finding{Flag: f, Message: fmt.Sprintf(format, args...)})
}

// Preflight measures the artwork against the order. It never decides what to do
// about a finding; that is the agent's job.
func Preflight(img image.Image, o Order) Report {
	b := img.Bounds()
	r := Report{WidthPx: b.Dx(), HeightPx: b.Dy()}
	printDPI := math.Min(float64(b.Dx())/o.WidthIn, float64(b.Dy())/o.HeightIn)
	r.EffectiveDPI = math.Round(printDPI*10) / 10

	// Resolution is judged on the file as uploaded; every other check runs on a
	// copy of at most maxAnalysisDPI, with dpi describing that copy.
	img, dpi := forAnalysis(img, printDPI)
	px := newPixels(img)

	bg, hasBG := px.solidBorder()
	mask := px.contentMask(bg, hasBG)
	content := mask.count()
	if content == 0 {
		r.add(EmptyArtwork, "no printable pixels")
		return r
	}

	switch {
	case printDPI < minDPI:
		r.add(LowResolution, "%.0f dpi at %.4gx%.4g in; needs %.0f", printDPI, o.WidthIn, o.HeightIn, targetDPI)
	case printDPI < targetDPI:
		r.add(MediumResolution, "%.0f dpi at %.4gx%.4g in; target %.0f", printDPI, o.WidthIn, o.HeightIn, targetDPI)
	}

	if o.Product == DieCut && hasBG {
		r.add(SolidBackground, "border is solid rgb(%d,%d,%d); a die cut would follow the rectangle", bg.r, bg.g, bg.b)
	}

	if o.Product == DieCut {
		if bw, bh := mask.bounds(); bw > 0 && bh > 0 {
			// The cut follows the art, so the sticker is the content box scaled to fit the ordered box.
			scale := math.Min(o.WidthIn/float64(bw), o.HeightIn/float64(bh))
			pw, ph := float64(bw)*scale, float64(bh)*scale
			if share := pw * ph / (o.WidthIn * o.HeightIn); share < minBoxShare {
				r.add(PrintsSmaller, "art fills %.3gx%.3g in of the %.4gx%.4g in box (%.0f%% of the area)",
					pw, ph, o.WidthIn, o.HeightIn, share*100)
			}
		}
	}

	if o.Product == Rectangle {
		margin := int(math.Ceil(dpi * safeMarginIn))
		if n := mask.inMargin(margin); n > 0 && mask.borderShare() < 0.5 {
			r.add(ContentNearEdge, "%d px of artwork inside the %d px safe margin", n, margin)
		}
	}

	if thin := thinPixels(mask, dpi); float64(thin) >= dpi*dpi/200 {
		r.add(ThinLines, "%.2f sq in of the artwork is thinner than 1/32 in", float64(thin)/(dpi*dpi))
	}

	if !hasBG && px.transparentBorder() {
		if share := px.whiteShare(mask); share >= 0.5 {
			r.add(WhiteOnTransparent, "%.0f%% of the artwork is white on a transparent background", share*100)
		}
	}
	return r
}

// thinPixels counts artwork that an opening at 1/32 in removes and that lies away
// from any thick part. The second condition ignores the rims of round or diagonal
// shapes, which an opening also shaves off.
func thinPixels(m mask, dpi float64) int {
	radius := int(dpi * minStrokeIn / 2)
	if radius < 1 {
		return 0
	}
	near := m.open(radius).filter(2*radius+1, true)
	n := 0
	for i, on := range m.on {
		if on && !near.on[i] {
			n++
		}
	}
	return n
}
