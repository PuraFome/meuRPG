package dungeonimg

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

// plan of 5 x 5 squares: a 3 x 3 room in the middle (floor), walls around it. The
// north wall's middle square, (2, 0), is a secret door, so it is solid like the rest.
func smallPlan() Floorplan {
	f := Floorplan{Cols: 5, Rows: 5, Solid: make([]bool, 25)}
	for i := range f.Solid {
		col, row := i%5, i/5
		f.Solid[i] = col == 0 || col == 4 || row == 0 || row == 4
	}
	return f
}

func TestPixelsPerSquare(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ cols, rows, want int }{
		{15, 15, 24}, {121, 121, 24}, {199, 399, 20}, {399, 199, 20}, {199, 199, 24}, {1, 1, 24},
	} {
		if got := PixelsPerSquare(c.cols, c.rows); got != c.want {
			t.Errorf("PixelsPerSquare(%d, %d) = %d, want %d", c.cols, c.rows, got, c.want)
		}
		if w, h := Size(c.cols, c.rows); w > MaxSide || h > MaxSide {
			t.Errorf("Size(%d, %d) = %d x %d, over %d pixels a side", c.cols, c.rows, w, h, MaxSide)
		}
	}
}

// What the image shows: paper on the floor, the ink outline where the floor meets a
// wall, the hatch and the dark on every wall, and a secret door exactly like a
// wall. Nothing else: no door, no number.
func TestRenderDrawsFloorsAndWalls(t *testing.T) {
	t.Parallel()
	f := smallPlan()
	const side = 24
	img, err := Render(f, 5*side, 5*side)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if b := img.Bounds(); b.Dx() != 120 || b.Dy() != 120 {
		t.Fatalf("size = %v, want 120 x 120", b)
	}
	at := func(x, y int) uint8 { return img.ColorIndexAt(x, y) }

	// The middle of the room is paper; the pixel on its edge next to the wall is ink.
	if got := at(2*side+12, 2*side+12); got != idxFloor {
		t.Errorf("the room's middle = %d, want paper", got)
	}
	if got := at(1*side, 2*side+12); got != idxInk {
		t.Errorf("the room's west edge = %d, want the ink outline", got)
	}
	// An inner corner of the room (a floor pixel whose diagonal is wall) is ink too.
	if got := at(1*side, 1*side); got != idxInk {
		t.Errorf("the room's north-west corner = %d, want ink", got)
	}
	// Every pixel of every solid square is the flat wall color, the secret door's
	// included: the image has no hatch (the app draws the wall mark over the walls layer,
	// so the map has one wall treatment).
	for row := range 5 {
		for col := range 5 {
			if !f.Solid[row*5+col] {
				continue
			}
			for y := row * side; y < (row+1)*side; y++ {
				for x := col * side; x < (col+1)*side; x++ {
					if c := at(x, y); c != idxWall {
						t.Fatalf("pixel (%d, %d) of the wall square (%d, %d) is index %d, want the flat wall", x, y, col, row, c)
					}
				}
			}
		}
	}
}

// The image is a pure function of the plan, and a size that is not a multiple of
// the grid still fills every pixel with the square it falls in.
func TestRenderIsAPureFunctionOfThePlan(t *testing.T) {
	t.Parallel()
	a, err := Render(smallPlan(), 120, 120)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Render(smallPlan(), 120, 120)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("two renders of the same plan differ")
	}
	open := smallPlan()
	open.Solid[2] = false // the secret door revealed: a door is floor
	c, _ := Render(open, 120, 120)
	if bytes.Equal(a.Pix, c.Pix) {
		t.Error("revealing the door does not change the image")
	}
	if c.ColorIndexAt(2*24+12, 12) != idxFloor {
		t.Error("an opened square is not floor")
	}
	if _, err := Render(smallPlan(), 53, 61); err != nil {
		t.Errorf("Render at 53 x 61 error = %v", err)
	}
}

func TestRenderRefusesAPlanThatDoesNotFit(t *testing.T) {
	t.Parallel()
	for name, f := range map[string]Floorplan{
		"no squares":  {Cols: 0, Rows: 0},
		"short solid": {Cols: 5, Rows: 5, Solid: make([]bool, 24)},
	} {
		if _, err := Render(f, 120, 120); !errors.Is(err, ErrPlan) {
			t.Errorf("%s: error = %v, want ErrPlan", name, err)
		}
	}
	if _, err := Render(smallPlan(), 4, 4); !errors.Is(err, ErrPlan) {
		t.Errorf("an image smaller than the grid: error = %v, want ErrPlan", err)
	}
}

func BenchmarkRender(b *testing.B) {
	for _, c := range []struct{ cols, rows int }{{121, 121}, {199, 399}} {
		f := Floorplan{Cols: c.cols, Rows: c.rows, Solid: make([]bool, c.cols*c.rows)}
		for i := range f.Solid {
			col, row := i%c.cols, i/c.cols
			f.Solid[i] = (col/7+row/5)%2 == 0 || col == 0 || row == 0
		}
		w, h := Size(c.cols, c.rows)
		b.Run(fmt.Sprintf("%dx%d", c.cols, c.rows), func(b *testing.B) {
			for range b.N {
				if _, err := Render(f, w, h); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
