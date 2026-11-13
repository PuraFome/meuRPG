package grid

// Surface is what a square of the map is made of for a creature that crosses it
// (RN-21): plain floor, water (swum) or a surface climbed. The values are what
// the two bits of a SurfaceLayer hold.
type Surface uint8

// The surfaces. SurfaceFloor is the square nobody painted.
const (
	SurfaceFloor Surface = iota
	SurfaceWater
	SurfaceClimb
)

// SurfaceLayer is the surfaces the master painted on a grid, two bits a square, in the same layout as
// a CoverLayer (see layer.go); 3 is never stored. A nil *SurfaceLayer reads as "nothing painted".
type SurfaceLayer struct{ c crumbs }

// SurfaceLayerSize is how many bytes a SurfaceLayer of the grid takes when encoded.
func SurfaceLayerSize(g Grid) int { return crumbsSize(g) }

// NewSurfaceLayer is a SurfaceLayer of the grid with nothing painted.
func NewSurfaceLayer(g Grid) *SurfaceLayer { return &SurfaceLayer{c: newCrumbs(g)} }

// DecodeSurfaceLayer reads the bytes Encode wrote for the same grid. It returns ErrLayerSize for
// the wrong length, a nonzero unused bit or a square holding 3.
func DecodeSurfaceLayer(g Grid, b []byte) (*SurfaceLayer, error) {
	c, err := decodeCrumbs(g, b, byte(SurfaceClimb))
	if err != nil {
		return nil, err
	}
	return &SurfaceLayer{c: c}, nil
}

// Encode is the layer's bytes, in the layout above. The caller may keep them.
func (l *SurfaceLayer) Encode() []byte {
	if l == nil {
		return nil
	}
	return append([]byte(nil), l.c.bits...)
}

// Grid is the grid the layer is sized for.
func (l *SurfaceLayer) Grid() Grid {
	if l == nil {
		return Grid{}
	}
	return l.c.g
}

// Get is the surface painted on a square: SurfaceFloor for one never painted or outside the grid.
func (l *SurfaceLayer) Get(col, row int) Surface {
	if l == nil {
		return SurfaceFloor
	}
	return Surface(l.c.get(col, row))
}

// Set paints a square (SurfaceFloor clears it). It returns false, and changes nothing, for a square
// outside the grid or a value that is not a Surface.
func (l *SurfaceLayer) Set(col, row int, v Surface) bool {
	if l == nil || v > SurfaceClimb {
		return false
	}
	return l.c.set(col, row, byte(v))
}

// Scaled is the layer on the grid k times bigger: each painted square becomes a block of k x k
// squares of the same surface.
func (l *SurfaceLayer) Scaled(k int) *SurfaceLayer {
	k = max(k, 1)
	if l == nil {
		return nil
	}
	out := NewSurfaceLayer(l.c.g.scaled(k))
	for row := range l.c.g.Rows {
		for col := range l.c.g.Columns {
			if v := l.Get(col, row); v != SurfaceFloor {
				fill(col, row, k, func(c, r int) { out.Set(c, r, v) })
			}
		}
	}
	return out
}
