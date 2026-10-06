package refimg

import (
	"image"
	"testing"
)

// The crop keeps the map's proportions and sits in the middle: scaling it never stretches.
func TestCenterCrop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		w, h, rw, rh int
		want         image.Rectangle
	}{
		{"the same ratio: whole", 60, 40, 300, 200, image.Rect(0, 0, 300, 200)},
		{"a wider picture: the middle of its width", 60, 40, 300, 100, image.Rect(75, 0, 225, 100)},
		{"a taller picture: the middle of its height", 60, 40, 150, 150, image.Rect(0, 25, 150, 125)},
		{"a square map from a wide picture", 40, 40, 160, 90, image.Rect(35, 0, 125, 90)},
		{"an empty map: whole", 0, 40, 160, 90, image.Rect(0, 0, 160, 90)},
	} {
		got := CenterCrop(tc.w, tc.h, tc.rw, tc.rh)
		if got != tc.want {
			t.Errorf("%s: CenterCrop(%d,%d,%d,%d) = %v, want %v", tc.name, tc.w, tc.h, tc.rw, tc.rh, got, tc.want)
		}
	}
	// To a pixel, whatever the sizes: the crop's ratio is the map's.
	for _, size := range [][4]int{{199, 399, 1024, 576}, {31, 21, 1024, 1024}, {200, 100, 333, 777}} {
		r := CenterCrop(size[0], size[1], size[2], size[3])
		a, b := float64(r.Dx())/float64(r.Dy()), float64(size[0])/float64(size[1])
		if d := a/b - 1; d > 0.02 || d < -0.02 {
			t.Errorf("CenterCrop%v has ratio %.3f, want %.3f", size, a, b)
		}
	}
}
