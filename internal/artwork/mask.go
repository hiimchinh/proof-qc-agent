package artwork

// mask is a binary image of printable pixels.
type mask struct {
	w, h int
	on   []bool
}

func newMask(w, h int) mask { return mask{w: w, h: h, on: make([]bool, w*h)} }

func (m mask) count() int {
	n := 0
	for _, v := range m.on {
		if v {
			n++
		}
	}
	return n
}

func (m mask) inMargin(margin int) int {
	n := 0
	for y := range m.h {
		for x := range m.w {
			if m.on[y*m.w+x] && (x < margin || y < margin || x >= m.w-margin || y >= m.h-margin) {
				n++
			}
		}
	}
	return n
}

// borderShare is the share of edge pixels that are artwork; full-bleed designs score near 1.
func (m mask) borderShare() float64 {
	total, on := 0, 0
	for x := range m.w {
		for _, y := range []int{0, m.h - 1} {
			total++
			if m.on[y*m.w+x] {
				on++
			}
		}
	}
	for y := 1; y < m.h-1; y++ {
		for _, x := range []int{0, m.w - 1} {
			total++
			if m.on[y*m.w+x] {
				on++
			}
		}
	}
	return float64(on) / float64(total)
}

// open is a morphological opening with a square of side 2r+1: anything thinner disappears.
func (m mask) open(r int) mask {
	return m.filter(r, false).filter(r, true)
}

// filter applies a separable square min (erode) or max (dilate) filter.
func (m mask) filter(r int, dilate bool) mask {
	tmp, out := newMask(m.w, m.h), newMask(m.w, m.h)
	pass := func(src, dst mask, horizontal bool) {
		for y := range m.h {
			for x := range m.w {
				v := !dilate
				for d := -r; d <= r; d++ {
					xx, yy := x, y
					if horizontal {
						xx += d
					} else {
						yy += d
					}
					s := false
					if xx >= 0 && yy >= 0 && xx < m.w && yy < m.h {
						s = src.on[yy*m.w+xx]
					}
					if dilate && s {
						v = true
						break
					}
					if !dilate && !s {
						v = false
						break
					}
				}
				dst.on[y*m.w+x] = v
			}
		}
	}
	pass(m, tmp, true)
	pass(tmp, out, false)
	return out
}
