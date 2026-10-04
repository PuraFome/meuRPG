package grid

import (
	"errors"
	"fmt"
)

// The packed layers are what the maps module stores and the API sends, and
// what the web decodes, so the byte layout is part of the contract.
//
// Squares are numbered row-major: square number n = row × columns + col,
// counting from 0 at the north-west corner.
//
//   - A Layer (walls, difficult terrain) is one bit per square: square n is
//     bit n%8 of byte n/8, the least significant bit first. It takes
//     ceil(squares/8) bytes. The unused bits of the last byte are 0.
//   - A LightLayer and a CoverLayer are two bits per square: square n is bits
//     2×(n%4) and 2×(n%4)+1 of byte n/4, the least significant pair first. A
//     LightLayer holds a Light (0 unpainted, 1 dark, 2 dim, 3 bright) and a
//     CoverLayer a Cover (0 none, 1 half, 2 three-quarters; 3 is never
//     stored). They take ceil(squares/4) bytes. The unused bits of the last
//     byte are 0.
//
// A wrong length, a nonzero unused bit, or (for cover) a value of 3 is
// refused when decoding: the maps module sizes every layer by the grid, so
// any of them means the bytes belong to another grid or are damaged.

// ErrLayerSize is returned by the Decode functions for bytes that do not have
// the length of the grid's layer, that set an unused bit, or that hold a
// value the layer never stores.
var ErrLayerSize = errors.New("grid: the layer's bytes do not fit the grid")

// Layer is a set of squares of a grid, one bit each: the walls, or the
// squares of difficult terrain. The zero value and a nil *Layer are empty
// and read as "no square set".
type Layer struct {
	g    Grid
	bits []byte
}

// LayerSize is how many bytes a Layer of the grid takes when encoded.
func LayerSize(g Grid) int { return (g.Squares() + 7) / 8 }

// NewLayer is an empty Layer of the grid.
func NewLayer(g Grid) *Layer {
	return &Layer{g: g, bits: make([]byte, LayerSize(g))}
}

// DecodeLayer reads the bytes Encode wrote for the same grid. It returns
// ErrLayerSize for the wrong length or a nonzero unused bit.
func DecodeLayer(g Grid, b []byte) (*Layer, error) {
	if !g.Valid() || len(b) != LayerSize(g) {
		return nil, fmt.Errorf("%w: %d bytes for %d x %d squares", ErrLayerSize, len(b), g.Columns, g.Rows)
	}
	if used := g.Squares() % 8; used != 0 && b[len(b)-1]>>used != 0 {
		return nil, fmt.Errorf("%w: unused bits are set", ErrLayerSize)
	}
	return &Layer{g: g, bits: append([]byte(nil), b...)}, nil
}

// Encode is the layer's bytes, in the layout above. The caller may keep them.
func (l *Layer) Encode() []byte {
	if l == nil {
		return nil
	}
	return append([]byte(nil), l.bits...)
}

// Grid is the grid the layer is sized for.
func (l *Layer) Grid() Grid {
	if l == nil {
		return Grid{}
	}
	return l.g
}

// Get says whether the square is set. A square outside the grid is not.
func (l *Layer) Get(col, row int) bool {
	if l == nil || col < 0 || col >= l.g.Columns || row < 0 || row >= l.g.Rows {
		return false
	}
	n := l.g.index(col, row)
	return l.bits[n>>3]>>(n&7)&1 == 1
}

// Has is Get for a Square, so a Layer is a Set.
func (l *Layer) Has(s Square) bool { return l.Get(s.Col, s.Row) }

// Set sets or clears a square. It returns false, and changes nothing, for a
// square outside the grid.
func (l *Layer) Set(col, row int, on bool) bool {
	if l == nil || col < 0 || col >= l.g.Columns || row < 0 || row >= l.g.Rows {
		return false
	}
	n := l.g.index(col, row)
	if on {
		l.bits[n>>3] |= 1 << (n & 7)
	} else {
		l.bits[n>>3] &^= 1 << (n & 7)
	}
	return true
}

// Count is how many squares are set.
func (l *Layer) Count() int {
	if l == nil {
		return 0
	}
	n := 0
	for i := range l.bits {
		for b := l.bits[i]; b != 0; b &= b - 1 {
			n++
		}
	}
	return n
}

// crumbs is the two-bits-a-square store of LightLayer and CoverLayer.
type crumbs struct {
	g    Grid
	bits []byte
}

func crumbsSize(g Grid) int { return (g.Squares() + 3) / 4 }

func newCrumbs(g Grid) crumbs { return crumbs{g: g, bits: make([]byte, crumbsSize(g))} }

// decodeCrumbs checks the length, the unused bits and that no value is above
// highest.
func decodeCrumbs(g Grid, b []byte, highest byte) (crumbs, error) {
	if !g.Valid() || len(b) != crumbsSize(g) {
		return crumbs{}, fmt.Errorf("%w: %d bytes for %d x %d squares", ErrLayerSize, len(b), g.Columns, g.Rows)
	}
	if used := g.Squares() % 4; used != 0 && b[len(b)-1]>>(2*used) != 0 {
		return crumbs{}, fmt.Errorf("%w: unused bits are set", ErrLayerSize)
	}
	c := crumbs{g: g, bits: append([]byte(nil), b...)}
	if highest < 3 {
		for n := 0; n < g.Squares(); n++ {
			if c.at(n) > highest {
				return crumbs{}, fmt.Errorf("%w: square %d holds %d", ErrLayerSize, n, c.at(n))
			}
		}
	}
	return c, nil
}

func (c crumbs) at(n int) byte { return c.bits[n>>2] >> (2 * (n & 3)) & 3 }

func (c crumbs) get(col, row int) byte {
	if len(c.bits) == 0 || col < 0 || col >= c.g.Columns || row < 0 || row >= c.g.Rows {
		return 0
	}
	return c.at(c.g.index(col, row))
}

func (c crumbs) set(col, row int, v byte) bool {
	if len(c.bits) == 0 || col < 0 || col >= c.g.Columns || row < 0 || row >= c.g.Rows {
		return false
	}
	n := c.g.index(col, row)
	shift := 2 * (n & 3)
	c.bits[n>>2] = c.bits[n>>2]&^(3<<shift) | v<<shift
	return true
}

// Light is how lit a square is, from what the master painted (Etapa 9, D2).
// The values are in order of brightness, and the value is what the two bits
// of a LightLayer hold.
type Light uint8

// The light levels. Unpainted is a square the master left alone: it takes the
// map's base light.
const (
	Unpainted Light = iota
	Dark
	Dim
	Bright
)

// LightLayer is the painted light of a grid, two bits a square. A nil
// *LightLayer reads as "nothing painted".
type LightLayer struct{ c crumbs }

// LightLayerSize is how many bytes a LightLayer of the grid takes when
// encoded.
func LightLayerSize(g Grid) int { return crumbsSize(g) }

// NewLightLayer is a LightLayer of the grid with nothing painted.
func NewLightLayer(g Grid) *LightLayer { return &LightLayer{c: newCrumbs(g)} }

// DecodeLightLayer reads the bytes Encode wrote for the same grid. It returns
// ErrLayerSize for the wrong length or a nonzero unused bit.
func DecodeLightLayer(g Grid, b []byte) (*LightLayer, error) {
	c, err := decodeCrumbs(g, b, 3)
	if err != nil {
		return nil, err
	}
	return &LightLayer{c: c}, nil
}

// Encode is the layer's bytes, in the layout above. The caller may keep them.
func (l *LightLayer) Encode() []byte {
	if l == nil {
		return nil
	}
	return append([]byte(nil), l.c.bits...)
}

// Grid is the grid the layer is sized for.
func (l *LightLayer) Grid() Grid {
	if l == nil {
		return Grid{}
	}
	return l.c.g
}

// Get is the painted light of a square: Unpainted for one never painted or
// outside the grid.
func (l *LightLayer) Get(col, row int) Light {
	if l == nil {
		return Unpainted
	}
	return Light(l.c.get(col, row))
}

// Set paints a square (Unpainted clears it). It returns false, and changes
// nothing, for a square outside the grid or a value that is not a Light.
func (l *LightLayer) Set(col, row int, v Light) bool {
	if l == nil || v > Bright {
		return false
	}
	return l.c.set(col, row, byte(v))
}

// Cover is how much a square covers whoever is behind it (D4, the SRD's
// cover): half gives +2 to AC and Dexterity saves, three-quarters +5, and
// total makes the creature untargetable.
type Cover uint8

// The degrees of cover. CoverTotal is what a wall gives and what a line that
// squeezes between walls gives; it is never painted, so a CoverLayer never
// holds it.
const (
	CoverNone Cover = iota
	CoverHalf
	CoverThreeQuarters
	CoverTotal
)

// CoverLayer is the cover the master painted on a grid, two bits a square: a
// half-cover square (a low wall, crates) can be crossed, and a
// three-quarters one (a column, an arrow slit) blocks movement like a wall but
// not sight or light. A nil *CoverLayer reads as "no cover painted".
type CoverLayer struct{ c crumbs }

// CoverLayerSize is how many bytes a CoverLayer of the grid takes when
// encoded.
func CoverLayerSize(g Grid) int { return crumbsSize(g) }

// NewCoverLayer is a CoverLayer of the grid with nothing painted.
func NewCoverLayer(g Grid) *CoverLayer { return &CoverLayer{c: newCrumbs(g)} }

// DecodeCoverLayer reads the bytes Encode wrote for the same grid. It returns
// ErrLayerSize for the wrong length, a nonzero unused bit or a square holding
// 3.
func DecodeCoverLayer(g Grid, b []byte) (*CoverLayer, error) {
	c, err := decodeCrumbs(g, b, byte(CoverThreeQuarters))
	if err != nil {
		return nil, err
	}
	return &CoverLayer{c: c}, nil
}

// Encode is the layer's bytes, in the layout above. The caller may keep them.
func (l *CoverLayer) Encode() []byte {
	if l == nil {
		return nil
	}
	return append([]byte(nil), l.c.bits...)
}

// Grid is the grid the layer is sized for.
func (l *CoverLayer) Grid() Grid {
	if l == nil {
		return Grid{}
	}
	return l.c.g
}

// Get is the cover painted on a square: CoverNone for one never painted or
// outside the grid.
func (l *CoverLayer) Get(col, row int) Cover {
	if l == nil {
		return CoverNone
	}
	return Cover(l.c.get(col, row))
}

// Set paints a square (CoverNone clears it). It returns false, and changes
// nothing, for a square outside the grid or CoverTotal.
func (l *CoverLayer) Set(col, row int, v Cover) bool {
	if l == nil || v > CoverThreeQuarters {
		return false
	}
	return l.c.set(col, row, byte(v))
}
