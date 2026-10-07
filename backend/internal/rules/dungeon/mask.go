package dungeon

// buildMask marks the squares outside the silhouette as blocked (spec 3.2).
// All the arithmetic is integer: a mask never depends on floating point.
func (g *gen) buildMask() {
	w, h := g.w, g.h
	for y := range h {
		for x := range w {
			b := !g.insideMask(x, y)
			g.blocked[g.idx(x, y)] = b
			g.anyBlocked = g.anyBlocked || b
		}
	}
}

func (g *gen) insideMask(x, y int) bool {
	w, h := g.w, g.h
	switch g.o.Mask {
	case MaskDonut:
		hw, hh := w*g.o.MaskHole/100, h*g.o.MaskHole/100
		x0, y0 := (w-hw)/2, (h-hh)/2
		return x < x0 || x >= x0+hw || y < y0 || y >= y0+hh
	case MaskPlus:
		cw, ch := w/4, h/4
		return x >= cw && x < w-cw || y >= ch && y < h-ch
	case MaskLShape:
		return x < w-w*45/100 || y >= h*45/100
	case MaskEllipse:
		// The center of the square against the inscribed ellipse, scaled by 2.
		a, b := int64(2*x+1-w), int64(2*y+1-h)
		return a*a*int64(h)*int64(h)+b*b*int64(w)*int64(w) <= int64(w)*int64(w)*int64(h)*int64(h)
	case MaskDiamond:
		a, b := int64(2*x+1-w), int64(2*y+1-h)
		if a < 0 {
			a = -a
		}
		if b < 0 {
			b = -b
		}
		return a*int64(h)+b*int64(w) <= int64(w)*int64(h)
	case MaskCustom:
		m := g.o.CustomMask
		cx := (2*x + 1) * m.Width / (2 * w)
		cy := (2*y + 1) * m.Height / (2 * h)
		return m.On[cy*m.Width+cx]
	}
	return true
}
