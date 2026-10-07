package refimg

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/maps/dungeonimg"
)

// cave is a 6 x 4 plan: a wall ring around a 4 x 2 floor.
func cave() Plan {
	p := Plan{Cols: 6, Rows: 4, Solid: make([]bool, 24)}
	for i := range p.Solid {
		c, r := i%6, i/6
		p.Solid[i] = c == 0 || c == 5 || r == 0 || r == 3
	}
	return p
}

// pixelAt reads the palette index of the middle of a square.
func pixelAt(img *image.Paletted, p Plan, col, row int) uint8 {
	b := img.Bounds()
	x := (2*col + 1) * b.Dx() / (2 * p.Cols)
	y := (2*row + 1) * b.Dy() / (2 * p.Rows)
	return img.Pix[y*img.Stride+x]
}

func TestRenderUnseenSquaresAreBlack(t *testing.T) {
	t.Parallel()
	p := cave()
	p.Seen = make([]bool, 24)
	for _, i := range []int{7, 8, 13, 14} { // the floor squares (1,1) (2,1) (1,2) (2,2) and no more
		p.Seen[i] = true
	}
	w, h := SizeFor(p.Cols, p.Rows, Side)
	img, err := Render(p, w, h)
	if err != nil {
		t.Fatal(err)
	}
	for row := range 4 {
		for col := range 6 {
			got := pixelAt(img, p, col, row)
			if p.Seen[row*6+col] {
				if got == idxBlack {
					t.Errorf("a seen square (%d,%d) is black", col, row)
				}
			} else if got != idxBlack {
				t.Errorf("an unseen square (%d,%d) is index %d, want black", col, row, got)
			}
		}
	}
	if got := pixelAt(img, p, 1, 1); got != dungeonimg.IndexFloor && got != dungeonimg.IndexInk {
		t.Errorf("a seen floor square is index %d", got)
	}
}

func TestRenderMarkers(t *testing.T) {
	t.Parallel()
	p := cave()
	p.Markers = []Marker{{Col: 1, Row: 1, Party: true}, {Col: 3, Row: 2}, {Col: 99, Row: 99}}
	w, h := SizeFor(p.Cols, p.Rows, Side)
	img, err := Render(p, w, h)
	if err != nil {
		t.Fatal(err)
	}
	if got := pixelAt(img, p, 1, 1); got != idxParty {
		t.Errorf("the party's marker is index %d, want %d", got, idxParty)
	}
	if got := pixelAt(img, p, 3, 2); got != idxNPC {
		t.Errorf("the NPC's marker is index %d, want %d", got, idxNPC)
	}
	if got := pixelAt(img, p, 2, 1); got == idxParty || got == idxNPC {
		t.Errorf("a square with no marker has a disc")
	}
}

func TestRenderRefusesAPlanThatDoesNotFit(t *testing.T) {
	t.Parallel()
	if _, err := Render(Plan{Cols: 3, Rows: 3, Solid: make([]bool, 8)}, 30, 30); err == nil {
		t.Error("a short Solid was drawn")
	}
	if _, err := Render(Plan{Cols: 3, Rows: 3, Solid: make([]bool, 9), Seen: make([]bool, 4)}, 30, 30); err == nil {
		t.Error("a short Seen was drawn")
	}
}

// What the players do not see leaves the picture: the plan is cut to the seen
// squares and one square around them.
func TestCropKeepsTheSeenSquaresAndTheirMargin(t *testing.T) {
	t.Parallel()
	p := Plan{Cols: 10, Rows: 8, Solid: make([]bool, 80), Seen: make([]bool, 80)}
	p.Seen[3*10+4] = true // (4,3)
	p.Seen[4*10+6] = true // (6,4)
	p.Markers = []Marker{{Col: 4, Row: 3, Party: true}, {Col: 9, Row: 7}}
	p.Solid[3*10+4] = true
	got, seen := p.Crop()
	if seen != 2 {
		t.Errorf("seen = %d, want 2", seen)
	}
	// Columns 3..7 and rows 2..5.
	if got.Cols != 5 || got.Rows != 4 {
		t.Fatalf("cropped to %d x %d, want 5 x 4", got.Cols, got.Rows)
	}
	if !got.Solid[1*5+1] || !got.Seen[1*5+1] {
		t.Errorf("the cropped plan lost the square (4,3): %v %v", got.Solid, got.Seen)
	}
	if len(got.Markers) != 1 || got.Markers[0].Col != 1 || got.Markers[0].Row != 1 || !got.Markers[0].Party {
		t.Errorf("markers = %+v, want the one inside, moved to (1,1)", got.Markers)
	}
}

func TestCropAtTheEdgesAndWithNothingSeen(t *testing.T) {
	t.Parallel()
	p := Plan{Cols: 4, Rows: 4, Solid: make([]bool, 16), Seen: make([]bool, 16)}
	p.Seen[0] = true
	if got, seen := p.Crop(); got.Cols != 2 || got.Rows != 2 || seen != 1 {
		t.Errorf("a corner: %d x %d, seen %d; want 2 x 2, 1", got.Cols, got.Rows, seen)
	}
	p.Seen[0] = false
	if got, seen := p.Crop(); got.Cols != 4 || got.Rows != 4 || seen != 0 {
		t.Errorf("nothing seen: %d x %d, seen %d; want the plan as it is and 0", got.Cols, got.Rows, seen)
	}
	whole := Plan{Cols: 4, Rows: 4, Solid: make([]bool, 16)}
	if got, seen := whole.Crop(); got.Cols != 4 || seen != 16 {
		t.Errorf("no Seen: %d, seen %d; want the whole map", got.Cols, seen)
	}
}

func TestClosest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		w, h int
		want string
	}{
		{1000, 1000, "1:1"},
		{1200, 800, "3:2"},
		{800, 1200, "2:3"},
		{1920, 1080, "16:9"},
		{1080, 1920, "9:16"},
		{2400, 1000, "21:9"}, // 2.4 is nearest to 2.33
		{1000, 2400, "9:16"}, // very tall: the tallest the model has
		{5000, 500, "21:9"},  // very wide: the widest
		{1024, 768, "4:3"},
		{2048, 1536, "4:3"},
		{900, 1100, "4:5"},
		{1100, 900, "5:4"},
		{0, 100, "1:1"},
		{100, 0, "1:1"},
	}
	for _, tt := range tests {
		if got := Closest(tt.w, tt.h); got != tt.want {
			t.Errorf("Closest(%d, %d) = %s, want %s", tt.w, tt.h, got, tt.want)
		}
	}
}

func TestPadSetsTheMapInTheMiddleOfTheCanvas(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		w, h             int
		ratio            string
		canvasW, canvasH int
		offX, offY       int
	}{
		{"wider canvas", 1000, 1000, "3:2", 1500, 1000, 250, 0},
		{"taller canvas", 1000, 1000, "2:3", 1000, 1500, 0, 250},
		{"already that ratio", 1920, 1080, "16:9", 1920, 1080, 0, 0},
		{"much wider", 1200, 400, "21:9", 1200, 514, 0, 57},
		{"the map is wider than the ratio", 1200, 400, "16:9", 1200, 675, 0, 137},
		{"the map is taller than the ratio", 400, 1200, "4:3", 1600, 1200, 600, 0},
	}
	for _, tt := range tests {
		p := PadWith(tt.w, tt.h, tt.ratio)
		if p.CanvasW != tt.canvasW || p.CanvasH != tt.canvasH || p.OffX != tt.offX || p.OffY != tt.offY {
			t.Errorf("%s: PadWith(%d, %d, %s) = canvas %dx%d at (%d,%d), want %dx%d at (%d,%d)",
				tt.name, tt.w, tt.h, tt.ratio, p.CanvasW, p.CanvasH, p.OffX, p.OffY, tt.canvasW, tt.canvasH, tt.offX, tt.offY)
		}
		if p.OffX < 0 || p.OffY < 0 || p.OffX+p.W > p.CanvasW || p.OffY+p.H > p.CanvasH {
			t.Errorf("%s: the map is not inside the canvas: %+v", tt.name, p)
		}
	}
}

// Cropping a result of the canvas's own size gives the map's rectangle back, and
// a result of any other size, at the same proportions, gives the same fractions.
func TestCropBackGivesTheMapsRectangle(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{1000, 1000}, {1200, 800}, {400, 1200}, {2048, 1536}, {1920, 1080}, {3000, 700}, {123, 457}} {
		pad := PadTo(size[0], size[1])
		if got := pad.Crop(pad.CanvasW, pad.CanvasH); got != image.Rect(pad.OffX, pad.OffY, pad.OffX+pad.W, pad.OffY+pad.H) {
			t.Errorf("%v: Crop of the canvas itself = %v, want the map's rectangle", size, got)
		}
		// The model answers a 1K-ish picture of the ratio: the crop keeps the map's
		// proportions to within a pixel of rounding.
		for _, res := range [][2]int{{1024, 1024}, {1376, 768}, {768, 1376}, {2048, 1152}} {
			// Only a result of the canvas's ratio is meaningful; make one.
			rw := res[0]
			rh := int(float64(rw) * float64(pad.CanvasH) / float64(pad.CanvasW))
			got := pad.Crop(rw, rh)
			wantAspect := float64(pad.W) / float64(pad.H)
			gotAspect := float64(got.Dx()) / float64(got.Dy())
			if d := gotAspect/wantAspect - 1; d > 0.02 || d < -0.02 {
				t.Errorf("%v at %dx%d: crop %v has aspect %.3f, want %.3f", size, rw, rh, got, gotAspect, wantAspect)
			}
			if !got.In(image.Rect(0, 0, rw, rh)) {
				t.Errorf("%v at %dx%d: crop %v leaves the picture", size, rw, rh, got)
			}
		}
	}
}

func TestCropIsAlwaysInsideAndNotEmpty(t *testing.T) {
	t.Parallel()
	pad := PadTo(10, 4000)
	for _, r := range [][2]int{{1, 1}, {5, 9}, {768, 1376}} {
		got := pad.Crop(r[0], r[1])
		if got.Empty() || !got.In(image.Rect(0, 0, r[0], r[1])) {
			t.Errorf("Crop(%d, %d) = %v", r[0], r[1], got)
		}
	}
}

func TestRenderPaddedDrawsRockAroundTheMap(t *testing.T) {
	t.Parallel()
	p := Plan{Cols: 10, Rows: 10, Solid: make([]bool, 100)} // all floor
	img, pad, err := RenderPadded(p, 1000, 1000, Side)
	if err != nil {
		t.Fatal(err)
	}
	if pad.Ratio != "1:1" {
		t.Fatalf("a square map is padded to %s", pad.Ratio)
	}
	// A square map on a 1:1 canvas has no padding.
	if b := img.Bounds(); b.Dx() != b.Dy() || pad.OffX != 0 || pad.OffY != 0 {
		t.Errorf("1:1: canvas %v, offset (%d,%d)", b, pad.OffX, pad.OffY)
	}

	wide := Plan{Cols: 20, Rows: 5, Solid: make([]bool, 100)}
	img, pad, err = RenderPadded(wide, 2000, 500, Side)
	if err != nil {
		t.Fatal(err)
	}
	if pad.Ratio != "21:9" {
		t.Fatalf("a 4:1 map is padded to %s, want 21:9", pad.Ratio)
	}
	b := img.Bounds()
	if b.Dx() != pad.CanvasW || b.Dy() != pad.CanvasH || b.Dx() > Side+1 {
		t.Errorf("canvas %v, pad %+v", b, pad)
	}
	// Above the map: rock (wall or hatch); in the middle of the map: floor.
	if px := img.Pix[0]; px != dungeonimg.IndexWall && px != dungeonimg.IndexHatch {
		t.Errorf("the top left corner is index %d, want rock", px)
	}
	mid := (pad.OffY+pad.H/2)*img.Stride + pad.OffX + pad.W/2
	if px := img.Pix[mid]; px != dungeonimg.IndexFloor {
		t.Errorf("the middle of the map is index %d, want floor", px)
	}
	// The result's crop is the drawing's map region.
	if got, want := pad.Crop(b.Dx(), b.Dy()), image.Rect(pad.OffX, pad.OffY, pad.OffX+pad.W, pad.OffY+pad.H); got != want {
		t.Errorf("Crop on the drawing = %v, want %v", got, want)
	}
}

func TestPNGRoundTrips(t *testing.T) {
	t.Parallel()
	img, err := Render(cave(), 60, 40)
	if err != nil {
		t.Fatal(err)
	}
	data, err := PNG(img)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 60 || cfg.Height != 40 {
		t.Errorf("PNG() = %dx%d, %v", cfg.Width, cfg.Height, err)
	}
}

// bigPlan is the largest map the rules allow, 200 x 400, as a cave: walls on the
// edge and pillars.
func bigPlan() Plan {
	p := Plan{Cols: 200, Rows: 400, Solid: make([]bool, 200*400), Seen: make([]bool, 200*400)}
	for i := range p.Solid {
		c, r := i%200, i/200
		p.Solid[i] = c == 0 || r == 0 || c == 199 || r == 399 || (c%7 == 3 && r%5 == 2)
		p.Seen[i] = true
	}
	p.Markers = []Marker{{Col: 10, Row: 10, Party: true}, {Col: 100, Row: 200}}
	return p
}

func BenchmarkRenderBiggestMap(b *testing.B) {
	p := bigPlan()
	w, h := SizeFor(p.Cols, p.Rows, Side)
	b.ReportAllocs()
	for b.Loop() {
		img, err := Render(p, w, h)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := PNG(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderPaddedBiggestMap(b *testing.B) {
	p := bigPlan()
	p.Seen = nil
	b.ReportAllocs()
	for b.Loop() {
		img, _, err := RenderPadded(p, 1200, 2400, Side)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := PNG(img); err != nil {
			b.Fatal(err)
		}
	}
}

// MEURPG_MEASURE=1 go test -run TestMeasureTheBiggestMap -v ./internal/maps/refimg
// prints what drawing the biggest map's references costs (docs/operations.md).
func TestMeasureTheBiggestMap(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	measure := func(name string, run func()) {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		run()
		took := time.Since(start)
		runtime.ReadMemStats(&after)
		t.Logf("%s: %v, %.1f MB allocated, %d allocations", name, took.Round(time.Millisecond),
			float64(after.TotalAlloc-before.TotalAlloc)/1e6, after.Mallocs-before.Mallocs)
	}
	p := bigPlan()
	measure("players' view, 200 x 400, crop, render, PNG", func() {
		c, _ := p.Crop()
		w, h := SizeFor(c.Cols, c.Rows, Side)
		img, err := Render(c, w, h)
		if err != nil {
			t.Fatal(err)
		}
		data, err := PNG(img)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("  drawing %dx%d, PNG %d kB", w, h, len(data)/1000)
	})
	whole := p
	whole.Seen, whole.Markers = nil, nil
	measure("textured map, 200 x 400 padded, render, PNG", func() {
		img, pad, err := RenderPadded(whole, 1200, 2400, Side)
		if err != nil {
			t.Fatal(err)
		}
		data, err := PNG(img)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("  canvas %dx%d (%s), PNG %d kB", pad.CanvasW, pad.CanvasH, pad.Ratio, len(data)/1000)
	})
}

// The answer is cropped with the fractions of what was drawn, not with a pad worked out
// again from the image's size: on the biggest dungeon (199 x 399 squares, an image of
// 24 pixels a square) the map's rectangle on the answer is its squares to within 0.05 of
// a square, whatever the answer's size.
func TestCropByFractionsOfTheDrawing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ cols, rows, ppsq int }{{199, 399, 20}, {121, 121, 24}, {41, 31, 24}, {24, 16, 10}, {200, 400, 6}} {
		imgW, imgH := tc.cols*tc.ppsq, tc.rows*tc.ppsq
		p := Plan{Cols: tc.cols, Rows: tc.rows, Solid: make([]bool, tc.cols*tc.rows)}
		canvas, pad, err := RenderPadded(p, imgW, imgH, Side)
		if err != nil {
			t.Fatal(err)
		}
		f := pad.Fractions()
		// The drawing's own pixels: the crop is exactly the region the map was drawn in.
		b := canvas.Bounds()
		if got, want := CropBy(f, b.Dx(), b.Dy()), image.Rect(pad.OffX, pad.OffY, pad.OffX+pad.W, pad.OffY+pad.H); got != want {
			t.Errorf("%dx%d: crop of the drawing = %v, want %v", tc.cols, tc.rows, got, want)
		}
		// An answer at the model's own size (1K of the ratio, and 2K): the crop is
		// cols x rows squares to within 0.05 of a square on each side.
		for _, long := range []int{1024, 2048} {
			rw, rh := long, long
			if b.Dx() >= b.Dy() {
				rh = int(math.Round(float64(long) * float64(b.Dy()) / float64(b.Dx())))
			} else {
				rw = int(math.Round(float64(long) * float64(b.Dx()) / float64(b.Dy())))
			}
			r := CropBy(f, rw, rh)
			sqW, sqH := float64(r.Dx())/float64(tc.cols), float64(r.Dy())/float64(tc.rows) // pixels a square
			// The squares of the cropped picture must be where the drawing's are: the
			// drift of the edge in squares is |edge - exact| / pixels a square.
			exactX0 := f[0] * float64(rw)
			exactY0 := f[1] * float64(rh)
			exactX1 := f[2] * float64(rw)
			exactY1 := f[3] * float64(rh)
			for name, d := range map[string]float64{
				"left": math.Abs(float64(r.Min.X)-exactX0) / sqW, "right": math.Abs(float64(r.Max.X)-exactX1) / sqW,
				"top": math.Abs(float64(r.Min.Y)-exactY0) / sqH, "bottom": math.Abs(float64(r.Max.Y)-exactY1) / sqH,
			} {
				if d >= 0.05 && sqW >= 1 && sqH >= 1 {
					t.Errorf("%dx%d at %d: the %s edge drifts %.3f of a square", tc.cols, tc.rows, long, name, d)
				}
			}
		}
		// And the drawing's squares are the grid's to within 0.05 of a square: its map
		// region is cols x rows squares of a whole number of pixels, give or take rounding.
		sw, sh := float64(pad.W)/float64(tc.cols), float64(pad.H)/float64(tc.rows)
		if math.Abs(sw/(float64(pad.H)/float64(tc.rows))-float64(imgH)/float64(tc.rows)/(float64(imgW)/float64(tc.cols))) > 0.05 {
			t.Errorf("%dx%d: the drawing's squares are %.3f x %.3f px, not the image's proportions", tc.cols, tc.rows, sw, sh)
		}
	}
}

func TestCropByBadFractionsGiveTheWholePicture(t *testing.T) {
	t.Parallel()
	for _, f := range []Fractions{{}, {0, 0, 0, 0}, {-0.1, 0, 1, 1}, {0, 0, 1.1, 1}, {0.5, 0, 0.5, 1}} {
		if got := CropBy(f, 100, 50); got != image.Rect(0, 0, 100, 50) {
			t.Errorf("CropBy(%v) = %v, want the whole picture", f, got)
		}
	}
}
