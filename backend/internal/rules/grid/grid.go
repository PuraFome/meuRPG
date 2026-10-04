package grid

// FeetPerSquare is the side of a map square: 5 ft, which the app shows as
// 1,5 m.
const FeetPerSquare = 5

// DFtPerSquare is FeetPerSquare in tenths of a foot, the unit of movement.
const DFtPerSquare = FeetPerSquare * 10

// The size limits of a grid: the maps module keeps 4..200 columns, and the
// rows follow the image's proportions up to 400 (the encounters_grid_valid
// CHECK).
const (
	MaxColumns = 200
	MaxRows    = 400
)

// Square is a square of the grid: Col counts from the west, Row from the
// north, both from 0.
type Square struct {
	Col, Row int
}

// Grid is the squares of a map: Columns across the image's width and Rows
// down its height.
type Grid struct {
	Columns, Rows int
}

// RowsFor is how many rows of squares a grid of columns across has on an
// image of width × height pixels: the squares are square, so the rows follow
// the image's proportions, rounded to the nearest, and kept between 1 and
// MaxRows for an extremely long image. It returns 0 for a grid with no
// columns or an image with no width.
func RowsFor(columns, width, height int) int {
	if columns <= 0 || width <= 0 {
		return 0
	}
	rows := (columns*height + width/2) / width
	return min(max(rows, 1), MaxRows)
}

// Valid says whether g is a grid a map can have.
func (g Grid) Valid() bool {
	return g.Columns >= 1 && g.Columns <= MaxColumns && g.Rows >= 1 && g.Rows <= MaxRows
}

// Squares is how many squares the grid has.
func (g Grid) Squares() int {
	if g.Columns <= 0 || g.Rows <= 0 {
		return 0
	}
	return g.Columns * g.Rows
}

// Contains says whether the square is inside the grid.
func (g Grid) Contains(s Square) bool {
	return s.Col >= 0 && s.Col < g.Columns && s.Row >= 0 && s.Row < g.Rows
}

// index is the position of an inside square in the row-major order of the
// packed layers. Callers checked Contains.
func (g Grid) index(col, row int) int { return row*g.Columns + col }

// SquareOf is the square a position belongs to. A position is in basis points
// (0 to 10000) of the image's width and height; the grid is the squares across
// the width and down the height. A position on an edge belongs to the last
// square, and one outside the image to the nearest.
func (g Grid) SquareOf(xBP, yBP int) Square {
	if g.Columns <= 0 || g.Rows <= 0 {
		return Square{}
	}
	col := min(xBP*g.Columns/10000, g.Columns-1)
	row := min(yBP*g.Rows/10000, g.Rows-1)
	return Square{Col: max(col, 0), Row: max(row, 0)}
}

// CenterOf is the position, in basis points, of the middle of a square: where
// a token goes when it is placed on the square.
func (g Grid) CenterOf(s Square) (xBP, yBP int) {
	if g.Columns <= 0 || g.Rows <= 0 {
		return 0, 0
	}
	return (2*s.Col + 1) * 10000 / (2 * g.Columns), (2*s.Row + 1) * 10000 / (2 * g.Rows)
}
