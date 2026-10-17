package conv

import (
	"math"
	"testing"
)

func TestOrZero(t *testing.T) {
	t.Parallel()
	s, n, b := "x", int32(7), true
	if OrZero[string](nil) != "" || OrZero(&s) != "x" {
		t.Error("OrZero(string) wrong")
	}
	if OrZero[int32](nil) != 0 || OrZero(&n) != 7 {
		t.Error("OrZero(int32) wrong")
	}
	if OrZero[bool](nil) || !OrZero(&b) {
		t.Error("OrZero(bool) wrong")
	}
}

func TestInt32(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   int64
		want int32
	}{
		{"zero", 0, 0},
		{"small", 42, 42},
		{"negative", -5, -5},
		{"max", math.MaxInt32, math.MaxInt32},
		{"over", math.MaxInt32 + 1, math.MaxInt32},
		{"under", math.MinInt32 - 1, math.MinInt32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Int32(tt.in); got != tt.want {
				t.Errorf("Int32(%d) = %d, want %d", tt.in, got, tt.want)
			}
			if got := Int32(int(tt.in)); got != tt.want {
				t.Errorf("Int32(int(%d)) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
