package maps

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
)

// An edit of a textured map comes back in another ratio than the map's: fitToMap cuts the middle that has
// the map's proportions and scales it to the map's size. A square in the answer stays a square.
func TestFitToMapNeverStretches(t *testing.T) {
	t.Parallel()
	// 300 x 100 answer, white, with a black 100 x 100 square in the middle.
	src := image.NewRGBA(image.Rect(0, 0, 300, 100))
	for y := range 100 {
		for x := range 300 {
			c := color.RGBA{255, 255, 255, 255}
			if x >= 100 && x < 200 {
				c = color.RGBA{0, 0, 0, 255}
			}
			src.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	w, h := int32(60), int32(40) // the map's image: 3:2
	s := &Service{processing: make(chan struct{}, 1)}
	res, err := s.fitToMap(t.Context(), buf.Bytes(), mapsdb.ImageRequest{MapWidth: &w, MapHeight: &h})
	if err != nil {
		t.Fatalf("fitToMap() error = %v", err)
	}
	if res.Width != 60 || res.Height != 40 {
		t.Fatalf("the result is %dx%d, want the map's 60x40", res.Width, res.Height)
	}
	out, err := jpeg.Decode(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("decode the result: %v", err)
	}
	// The dark region is the square: 100 of the 150 kept columns, the whole height, so 40 x 40 here.
	minX, maxX := 1<<30, -1
	for x := range 60 {
		if r, _, _, _ := out.At(x, 20).RGBA(); r < 0x4000 {
			minX, maxX = min(minX, x), max(maxX, x)
		}
	}
	if width := maxX - minX + 1; width < 38 || width > 42 {
		t.Errorf("the square is %d px wide in a 40 px tall picture: it was stretched", width)
	}
}
