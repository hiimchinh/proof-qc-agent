package artwork

import (
	"image"
	"image/color"
	"math"
)

type rgba struct{ r, g, b, a uint8 }

func (c rgba) dist(o rgba) float64 {
	dr, dg, db := float64(c.r)-float64(o.r), float64(c.g)-float64(o.g), float64(c.b)-float64(o.b)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

type pixels struct {
	w, h int
	p    []rgba
}

func newPixels(img image.Image) pixels {
	b := img.Bounds()
	px := pixels{w: b.Dx(), h: b.Dy(), p: make([]rgba, b.Dx()*b.Dy())}
	switch src := img.(type) {
	case *image.NRGBA:
		for y := range px.h {
			row := src.Pix[(y+b.Min.Y-src.Rect.Min.Y)*src.Stride+(b.Min.X-src.Rect.Min.X)*4:]
			for x := range px.w {
				px.p[y*px.w+x] = rgba{row[4*x], row[4*x+1], row[4*x+2], row[4*x+3]}
			}
		}
		return px
	case *image.RGBA:
		for y := range px.h {
			row := src.Pix[(y+b.Min.Y-src.Rect.Min.Y)*src.Stride+(b.Min.X-src.Rect.Min.X)*4:]
			for x := range px.w {
				px.p[y*px.w+x] = unpremultiply(row[4*x], row[4*x+1], row[4*x+2], row[4*x+3])
			}
		}
		return px
	}
	for y := range px.h {
		for x := range px.w {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			px.p[y*px.w+x] = rgba{c.R, c.G, c.B, c.A}
		}
	}
	return px
}

// unpremultiply matches color.NRGBAModel without boxing every pixel in an interface.
func unpremultiply(r, g, b, a uint8) rgba {
	switch a {
	case 0:
		return rgba{}
	case 255:
		return rgba{r, g, b, a}
	}
	un := func(c uint8) uint8 {
		c16, a16 := uint32(c)*0x101, uint32(a)*0x101
		return uint8((c16 * 0xffff / a16) >> 8)
	}
	return rgba{un(r), un(g), un(b), a}
}

func (px pixels) at(x, y int) rgba { return px.p[y*px.w+x] }

func (px pixels) border(visit func(rgba)) {
	for x := range px.w {
		visit(px.at(x, 0))
		visit(px.at(x, px.h-1))
	}
	for y := 1; y < px.h-1; y++ {
		visit(px.at(0, y))
		visit(px.at(px.w-1, y))
	}
}

// solidBorder reports a background colour when almost the whole border is one opaque colour.
func (px pixels) solidBorder() (rgba, bool) {
	ref := px.at(0, 0)
	if ref.a < 250 {
		return rgba{}, false
	}
	total, same := 0, 0
	px.border(func(c rgba) {
		total++
		if c.a >= 250 && c.dist(ref) <= colorTolerant {
			same++
		}
	})
	return ref, float64(same)/float64(total) >= 0.98
}

func (px pixels) transparentBorder() bool {
	total, clear := 0, 0
	px.border(func(c rgba) {
		total++
		if c.a < 128 {
			clear++
		}
	})
	return float64(clear)/float64(total) >= 0.9
}

func (px pixels) contentMask(bg rgba, hasBG bool) mask {
	m := newMask(px.w, px.h)
	for i, c := range px.p {
		m.on[i] = c.a >= 128 && (!hasBG || c.dist(bg) > colorTolerant)
	}
	return m
}

func (px pixels) whiteShare(m mask) float64 {
	white, total := 0, 0
	for i, c := range px.p {
		if !m.on[i] {
			continue
		}
		total++
		if c.r >= 235 && c.g >= 235 && c.b >= 235 {
			white++
		}
	}
	return float64(white) / float64(total)
}
