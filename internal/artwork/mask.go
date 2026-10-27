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

// filter applies a separable square min (erode) or max (dilate) filter. Each
// pass slides a count of set pixels along the line, so the cost does not grow
// with r. Pixels outside the image count as unset.
func (m mask) filter(r int, dilate bool) mask {
	tmp, out := newMask(m.w, m.h), newMask(m.w, m.h)
	// pass filters n lines of length l; at(i, j) is the index of pixel j on line i.
	pass := func(src, dst mask, n, l int, at func(i, j int) int) {
		for i := range n {
			count := 0
			for j := 0; j < r && j < l; j++ {
				if src.on[at(i, j)] {
					count++
				}
			}
			for j := range l {
				if in := j + r; in < l && src.on[at(i, in)] {
					count++
				}
				if outIdx := j - r - 1; outIdx >= 0 && src.on[at(i, outIdx)] {
					count--
				}
				if dilate {
					dst.on[at(i, j)] = count > 0
				} else {
					dst.on[at(i, j)] = count == 2*r+1
				}
			}
		}
	}
	pass(m, tmp, m.h, m.w, func(y, x int) int { return y*m.w + x })
	pass(tmp, out, m.w, m.h, func(x, y int) int { return y*m.w + x })
	return out
}
