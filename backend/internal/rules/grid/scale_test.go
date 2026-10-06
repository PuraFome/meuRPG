package grid

import (
	"errors"
	"testing"
)

func TestEngineGrid(t *testing.T) {
	tests := []struct {
		name            string
		columns, factor int
		width, height   int
		want            Grid
		wantErr         error
	}{
		{"factor 1 is the drawing", 24, 1, 2400, 1600, Grid{24, 16}, nil},
		{"factor 0 reads as 1", 24, 0, 2400, 1600, Grid{24, 16}, nil},
		{"3 m: twice the drawing", 12, 2, 1200, 800, Grid{24, 16}, nil},
		{"4,5 m", 12, 3, 1200, 800, Grid{36, 24}, nil},
		{"the rows are the drawn rows times the factor", 7, 2, 100, 100, Grid{14, 14}, nil},
		{"rows round before the factor, so a block stays whole", 5, 3, 100, 130, Grid{15, 3 * 7}, nil},
		{"200 columns are fine", 100, 2, 100, 100, Grid{200, 200}, nil},
		{"201 columns are not", 101, 2, 100, 100, Grid{}, ErrGridLimits},
		{"401 rows are not", 10, 2, 100, 2100, Grid{}, ErrGridLimits},
		{"no columns, no grid", 0, 2, 100, 100, Grid{}, nil},
	}
	for _, tc := range tests {
		got, err := EngineGrid(tc.columns, tc.factor, tc.width, tc.height)
		if !errors.Is(err, tc.wantErr) || got != tc.want {
			t.Errorf("%s: EngineGrid(%d, %d, %d, %d) = %v, %v; want %v, %v", tc.name, tc.columns, tc.factor, tc.width, tc.height, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestScaleStep(t *testing.T) {
	tests := []struct {
		from, to int
		step     int
		ok       bool
	}{
		{1, 2, 2, true},
		{1, 3, 3, true},
		{2, 4, 2, true},
		{2, 6, 3, true},
		{1, 1, 1, true},
		{2, 3, 0, false},
		{2, 1, 0, false},
		{4, 2, 0, false},
		{3, 4, 0, false},
		{0, 2, 0, false},
	}
	for _, tc := range tests {
		if step, ok := ScaleStep(tc.from, tc.to); step != tc.step || ok != tc.ok {
			t.Errorf("ScaleStep(%d, %d) = %d, %v; want %d, %v", tc.from, tc.to, step, ok, tc.step, tc.ok)
		}
	}
}

// Every layer, square by square: a painted square becomes its k x k block and
// nothing else is painted.
func TestScaledLayers(t *testing.T) {
	g := Grid{Columns: 4, Rows: 3}
	walls, terrain := NewLayer(g), NewLayer(g)
	light, cover, doors := NewLightLayer(g), NewCoverLayer(g), NewDoorLayer(g)
	walls.Set(0, 0, true)
	walls.Set(3, 2, true)
	terrain.Set(1, 1, true)
	light.Set(2, 0, Bright)
	light.Set(1, 2, Dark)
	cover.Set(3, 0, CoverHalf)
	cover.Set(0, 2, CoverThreeQuarters)
	doors.Set(1, 0, DoorLocked)
	doors.Set(2, 2, DoorSecret)
	doors.Set(0, 1, DoorBarred)

	for _, k := range []int{1, 2, 3} {
		big := Grid{Columns: 4 * k, Rows: 3 * k}
		w, tr, li, co, d := walls.Scaled(k), terrain.Scaled(k), light.Scaled(k), cover.Scaled(k), doors.Scaled(k)
		if w.Grid() != big || tr.Grid() != big || li.Grid() != big || co.Grid() != big || d.Grid() != big {
			t.Fatalf("k=%d: a scaled layer is not on %v", k, big)
		}
		for row := 0; row < big.Rows; row++ {
			for col := 0; col < big.Columns; col++ {
				oc, or := col/k, row/k
				if w.Get(col, row) != walls.Get(oc, or) || tr.Get(col, row) != terrain.Get(oc, or) ||
					li.Get(col, row) != light.Get(oc, or) || co.Get(col, row) != cover.Get(oc, or) || d.Get(col, row) != doors.Get(oc, or) {
					t.Fatalf("k=%d: square (%d, %d) differs from its origin (%d, %d)", k, col, row, oc, or)
				}
			}
		}
		if w.Count() != 2*k*k || d.Count() != 3*k*k {
			t.Errorf("k=%d: %d wall squares and %d door squares, want %d and %d", k, w.Count(), d.Count(), 2*k*k, 3*k*k)
		}
		// The bytes are the layout of the new grid: they decode on it.
		if _, err := DecodeDoorLayer(big, d.Encode()); err != nil {
			t.Errorf("k=%d: scaled doors do not decode: %v", k, err)
		}
		if _, err := DecodeLayer(big, w.Encode()); err != nil {
			t.Errorf("k=%d: scaled walls do not decode: %v", k, err)
		}
	}
}

func TestScaledNilLayersStayNil(t *testing.T) {
	if (*Layer)(nil).Scaled(2) != nil || (*LightLayer)(nil).Scaled(2) != nil || (*CoverLayer)(nil).Scaled(2) != nil || (*DoorLayer)(nil).Scaled(2) != nil {
		t.Error("a nil layer scaled is not nil")
	}
}

// Odd sizes: a grid that is not a multiple of 8 (or of 2, for the nibbles and the
// crumbs) keeps its unused bits at 0 when scaled.
func TestScaledOddGrids(t *testing.T) {
	for _, g := range []Grid{{Columns: 5, Rows: 7}, {Columns: 1, Rows: 1}, {Columns: 9, Rows: 3}} {
		walls, doors, light := NewLayer(g), NewDoorLayer(g), NewLightLayer(g)
		last := Square{Col: g.Columns - 1, Row: g.Rows - 1}
		walls.Set(last.Col, last.Row, true)
		doors.Set(last.Col, last.Row, DoorBarred)
		light.Set(last.Col, last.Row, Dim)
		for _, k := range []int{2, 3, 7} {
			big := Grid{Columns: g.Columns * k, Rows: g.Rows * k}
			w, d, l := walls.Scaled(k), doors.Scaled(k), light.Scaled(k)
			if _, err := DecodeLayer(big, w.Encode()); err != nil {
				t.Errorf("%v x%d walls: %v", g, k, err)
			}
			if _, err := DecodeDoorLayer(big, d.Encode()); err != nil {
				t.Errorf("%v x%d doors: %v", g, k, err)
			}
			if _, err := DecodeLightLayer(big, l.Encode()); err != nil {
				t.Errorf("%v x%d light: %v", g, k, err)
			}
			if w.Count() != k*k || d.Count() != k*k || d.Get(big.Columns-1, big.Rows-1) != DoorBarred || l.Get(big.Columns-1, big.Rows-1) != Dim {
				t.Errorf("%v x%d: %d walls, %d doors", g, k, w.Count(), d.Count())
			}
		}
	}
}
