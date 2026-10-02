package dice

import (
	"errors"
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want Expr
		err  error
	}{
		{"d20", Expr{1, 20, 0}, nil},
		{"1d20+6", Expr{1, 20, 6}, nil},
		{"2d6 + 2", Expr{2, 6, 2}, nil},
		{"3d4+3", Expr{3, 4, 3}, nil},
		{"1d4 + 1", Expr{1, 4, 1}, nil},
		{"1d8-1", Expr{1, 8, -1}, nil},
		{"  1D20 - 2 ", Expr{1, 20, -2}, nil},
		{"100d100+100", Expr{100, 100, 100}, nil},
		{"1d20+0", Expr{1, 20, 0}, nil},
		{"", Expr{}, ErrSyntax},
		{"20", Expr{}, ErrSyntax},
		{"d", Expr{}, ErrSyntax},
		{"1d20+", Expr{}, ErrSyntax},
		{"1d20+6+2", Expr{}, ErrSyntax},
		{"1d20*2", Expr{}, ErrSyntax},
		{"-1d20", Expr{}, ErrSyntax},
		{"1.5d6", Expr{}, ErrSyntax},
		{"1d20; DROP", Expr{}, ErrSyntax},
		{"0d20", Expr{}, ErrCount},
		{"101d6", Expr{}, ErrCount},
		{"1d7", Expr{}, ErrSides},
		{"1d0", Expr{}, ErrSides},
		{"1d20+101", Expr{}, ErrModifier},
		{"1d20-101", Expr{}, ErrModifier},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.in)
			if !errors.Is(err, tt.err) {
				t.Fatalf("Parse(%q) error = %v, want %v", tt.in, err, tt.err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestExprString(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"d20": "1d20", "2d6 + 2": "2d6+2", "1d8 - 1": "1d8-1"} {
		e, err := Parse(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := e.String(); got != want {
			t.Errorf("Parse(%q).String() = %q, want %q", in, got, want)
		}
	}
}

func TestRollFixed(t *testing.T) {
	t.Parallel()
	e, _ := Parse("2d6+2")
	res, err := Roll(&Fixed{Faces: []int{5, 4}}, e)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Faces, []int{5, 4}) || res.Modifier != 2 || res.Total != 11 || res.Physical {
		t.Errorf("Roll() = %+v, want faces 5,4 + 2 = 11", res)
	}

	if _, err := Roll(&Fixed{Faces: []int{5}}, e); err == nil {
		t.Error("Roll() with too few faces worked, want an error")
	}
	if _, err := Roll(&Fixed{Faces: []int{7, 1}}, e); err == nil {
		t.Error("Roll() with a face that does not fit the die worked, want an error")
	}
}

func TestPhysical(t *testing.T) {
	t.Parallel()
	d20, _ := Parse("1d20+5")
	twoD6, _ := Parse("2d6-1")
	tests := []struct {
		name  string
		e     Expr
		sum   int
		total int
		err   error
	}{
		{"d20 plus modifier", d20, 16, 21, nil},
		{"lowest", d20, 1, 6, nil},
		{"highest", d20, 20, 25, nil},
		{"zero", d20, 0, 0, ErrSum},
		{"too high", d20, 21, 0, ErrSum},
		{"two dice, negative modifier", twoD6, 7, 6, nil},
		{"two dice, below the minimum", twoD6, 1, 0, ErrSum},
		{"two dice, above the maximum", twoD6, 13, 0, ErrSum},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := Physical(tt.e, tt.sum)
			if !errors.Is(err, tt.err) {
				t.Fatalf("Physical() error = %v, want %v", err, tt.err)
			}
			if err != nil {
				return
			}
			if res.Total != tt.total || !res.Physical || len(res.Faces) != 0 || res.Modifier != tt.e.Modifier {
				t.Errorf("Physical() = %+v, want total %d, physical, no faces", res, tt.total)
			}
		})
	}
}

// TestCryptoIsFair rolls a d20 60 000 times: each face should show up about
// 3 000 times (standard deviation about 54). The bounds are about 9
// standard deviations wide, so a fair roller never fails and a broken one
// (a face missing, or off by one) always does.
func TestCryptoIsFair(t *testing.T) {
	t.Parallel()
	const rolls = 60_000
	counts := make([]int, 21)
	var c Crypto
	for range rolls {
		face, err := c.Roll(20)
		if err != nil {
			t.Fatal(err)
		}
		if face < 1 || face > 20 {
			t.Fatalf("Roll(20) = %d, want 1 to 20", face)
		}
		counts[face]++
	}
	for face := 1; face <= 20; face++ {
		if counts[face] < 2500 || counts[face] > 3500 {
			t.Errorf("face %d came up %d times in %d rolls, want about 3000", face, counts[face], rolls)
		}
	}
}
