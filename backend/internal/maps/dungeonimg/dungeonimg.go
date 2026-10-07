// Package dungeonimg draws the flat image of a generated dungeon's map
// (MR-010, Etapa 10): floors and walls, nothing else.
//
// The image is a pure function of a floor plan (which squares are solid), so the
// same plan always gives the same pixels, and "Redesenhar" can draw it again from
// the walls and doors a map has now. What it never draws is what the master keeps
// (RN-10): no door (a door is an overlay the app draws from the doors layer, so
// it can open and close), no room number, no stair. A secret door is a solid
// square in the plan, so it is drawn as wall like any wall.
//
// The look follows the map language of the app (the E9 "ficha de papel"): paper
// for the floor, an ink outline along every floor edge that meets a wall, and a flat
// dark fill on the walls and on the rock beyond them. The hatch is not in the image:
// the app draws the wall mark (MAP-LANGUAGE-E10: the hatch over a veil) over every
// square of the walls layer, which covers the walls and all the rock, so the map has
// one wall treatment, the editor's. The grid is not drawn either: the app draws it.
package dungeonimg

import (
	"errors"
	"image"
	"image/color"
)

// The most pixels a generated image has on a side and the most a square has.
// 8,192 is the gallery's limit per side (images.MaxSide); a square larger than
// 24 pixels adds weight, not readability.
const (
	MaxSide            = 8192
	MaxPixelsPerSquare = 24
	minPixelsPerSquare = 1
	outlineDivisor     = 12
)

// The palette: index 0 is the floor. The hatch entry is kept (the palette's other
// users index it) but this image draws walls flat.
const (
	idxFloor = iota
	idxWall
	idxHatch
	idxInk
)

// The palette indices of an image Render returns, for a caller that draws more on
// it (package refimg, MR-039).
const (
	IndexFloor = idxFloor
	IndexWall  = idxWall
	IndexHatch = idxHatch
	IndexInk   = idxInk
)

var palette = color.Palette{
	idxFloor: color.RGBA{R: 0xEC, G: 0xE3, B: 0xCC, A: 0xFF}, // paper
	idxWall:  color.RGBA{R: 0x3A, G: 0x37, B: 0x40, A: 0xFF}, // dark ink
	idxHatch: color.RGBA{R: 0x5C, G: 0x58, B: 0x66, A: 0xFF}, // the hatch lines
	idxInk:   color.RGBA{R: 0x1B, G: 0x1A, B: 0x20, A: 0xFF}, // the outline
}

// Floorplan says which squares of a cols x rows grid are solid (wall, rock or a
// secret door); every other square is floor. Solid is row-major, cols * rows
// long.
type Floorplan struct {
	Cols, Rows int
	Solid      []bool
}

// PixelsPerSquare is how many pixels a square has on a side in an image of a
// cols x rows dungeon: the most that keeps the longer side within MaxSide
// (floor(8192 / max(cols, rows))), and at most MaxPixelsPerSquare.
func PixelsPerSquare(cols, rows int) int {
	longer := max(cols, rows, 1)
	return min(MaxPixelsPerSquare, max(minPixelsPerSquare, MaxSide/longer))
}

// Size is the image's size in pixels for a cols x rows dungeon.
func Size(cols, rows int) (w, h int) {
	p := PixelsPerSquare(cols, rows)
	return cols * p, rows * p
}

// ErrPlan is returned for a plan whose Solid does not fit its size.
var ErrPlan = errors.New("dungeonimg: the floor plan does not fit its size")

// Render draws the plan as a w x h image (palette-based, 8 bits a pixel). Each
// pixel belongs to the square it falls in, x * cols / w across and y * rows / h
// down, so any size works; squares are square when w / cols == h / rows, which is
// how the maps module sizes it.
func Render(f Floorplan, w, h int) (*image.Paletted, error) {
	if f.Cols < 1 || f.Rows < 1 || len(f.Solid) != f.Cols*f.Rows || w < f.Cols || h < f.Rows {
		return nil, ErrPlan
	}
	cols, rows := f.Cols, f.Rows
	side := max(w/cols, h/rows, 1)
	edge := max(1, side/outlineDivisor)

	colSq, nearW, nearE := axis(w, cols, edge)
	rowSq, nearN, nearS := axis(h, rows, edge)
	solidAt := func(c, r int) bool {
		if c < 0 || r < 0 || c >= cols || r >= rows {
			return true // beyond the grid is rock
		}
		return f.Solid[r*cols+c]
	}

	img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
	for y := range h {
		sr := rowSq[y]
		row := img.Pix[y*img.Stride : y*img.Stride+w]
		for x := range w {
			sc := colSq[x]
			if f.Solid[sr*cols+sc] {
				row[x] = idxWall
				continue
			}
			// A floor pixel is ink where it is along an edge that meets a solid
			// square (the corner pixels also look at the diagonal square).
			ink := nearW[x] && solidAt(sc-1, sr) || nearE[x] && solidAt(sc+1, sr) ||
				nearN[y] && solidAt(sc, sr-1) || nearS[y] && solidAt(sc, sr+1) ||
				nearW[x] && nearN[y] && solidAt(sc-1, sr-1) || nearE[x] && nearN[y] && solidAt(sc+1, sr-1) ||
				nearW[x] && nearS[y] && solidAt(sc-1, sr+1) || nearE[x] && nearS[y] && solidAt(sc+1, sr+1)
			if ink {
				row[x] = idxInk
			} // else idxFloor, the zero value
		}
	}
	return img, nil
}

// axis maps each of the n pixels of an axis to its square (of count squares), and
// says which pixels are within edge pixels of the square's low side (lo) and of
// its high side (hi).
func axis(n, count, edge int) (square []int, lo, hi []bool) {
	square = make([]int, n)
	lo, hi = make([]bool, n), make([]bool, n)
	start := make([]int, count)
	end := make([]int, count) // one past the square's last pixel
	for i := range start {
		start[i] = -1
	}
	for p := range n {
		sq := p * count / n
		square[p] = sq
		if start[sq] < 0 {
			start[sq] = p
		}
		end[sq] = p + 1
	}
	for p := range n {
		sq := square[p]
		lo[p] = p-start[sq] < edge
		hi[p] = end[sq]-1-p < edge
	}
	return square, lo, hi
}
