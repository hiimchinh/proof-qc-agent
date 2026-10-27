package artwork

import (
	"image"
	"image/draw"
	"math"
)

// maxAnalysisDPI is the finest detail the mask checks need: the thinnest stroke
// they look for is 1/32 in, which is still about 19 px at this resolution.
const maxAnalysisDPI = 600.0

// forAnalysis returns the image the mask checks run on and its dpi. Files above
// maxAnalysisDPI are scaled down first: the checks cost per pixel, and a 6000 px
// upload would otherwise take minutes for no extra precision.
func forAnalysis(img image.Image, dpi float64) (image.Image, float64) {
	if dpi <= maxAnalysisDPI {
		return img, dpi
	}
	b := img.Bounds()
	scale := maxAnalysisDPI / dpi
	w := max(int(math.Round(float64(b.Dx())*scale)), 1)
	h := max(int(math.Round(float64(b.Dy())*scale)), 1)
	return boxScale(img, w, h), dpi * float64(w) / float64(b.Dx())
}

// boxScale shrinks img to w x h by averaging every source pixel under each
// output pixel, so a thin stroke fades instead of vanishing between samples.
// Source rows are converted one band at a time, which keeps image/draw on its
// fast paths without holding a second full-size copy of a large upload.
func boxScale(img image.Image, w, h int) *image.RGBA {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	x0 := make([]int, w+1) // source column where output column i starts
	for i := range x0 {
		x0[i] = i * sw / w
	}
	sum := make([]uint64, 4*w)
	var band *image.RGBA
	for y := range h {
		y0, y1 := y*sh/h, (y+1)*sh/h
		if band == nil || band.Rect.Dy() < y1-y0 {
			band = image.NewRGBA(image.Rect(0, 0, sw, y1-y0))
		}
		rows := image.Rect(0, 0, sw, y1-y0)
		draw.Draw(band, rows, img, image.Pt(b.Min.X, b.Min.Y+y0), draw.Src)
		for i := range sum {
			sum[i] = 0
		}
		for ry := range y1 - y0 {
			row := band.Pix[ry*band.Stride : ry*band.Stride+4*sw]
			for i := range w {
				s, p := sum[4*i:4*i+4], row[4*x0[i]:4*x0[i+1]]
				for k := 0; k < len(p); k += 4 {
					s[0] += uint64(p[k])
					s[1] += uint64(p[k+1])
					s[2] += uint64(p[k+2])
					s[3] += uint64(p[k+3])
				}
			}
		}
		out := dst.Pix[y*dst.Stride:]
		for i := range w {
			n := uint64((x0[i+1] - x0[i]) * (y1 - y0))
			for c := range 4 {
				out[4*i+c] = uint8((sum[4*i+c] + n/2) / n)
			}
		}
	}
	return dst
}
