// Package refimg draws the reference images that go to the image model with a
// request made from a map (MR-039, RN-28, ADR-0019), and does the arithmetic of
// fitting a map to the model's ratios.
//
// A reference is a flat drawing, a pure function of its Plan, like the
// generated dungeon's image (package dungeonimg, whose look it keeps): paper for
// the floor, dark hatched walls and rock, an ink outline where the floor meets a
// wall. It has no door (an overlay), no number and no text. Two things are added
// for the players' view:
//
//   - a square nobody sees is solid black, and the plan is cropped to the squares
//     that are seen (the rest of the map is not in the picture at all);
//   - a creature the players see is a colored disc on its square (the party's
//     blue, an NPC's red), never a name.
//
// The package knows nothing about campaigns, people or the database: what may
// go to the model is what a Plan holds. Which squares are seen, which creatures
// stand where and which squares are walls (an unrevealed secret door is one) is
// decided by package maps before it calls Render (RN-10).
//
// Pad and Crop are the second half: the model returns one of ten ratios, so a map
// of another proportion is drawn on a canvas of the closest ratio, filled with
// rock around it, and the result is cropped back to the map's rectangle.
package refimg

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/maps/dungeonimg"
)

// Side is the longer side of a reference, in pixels: the same size a gallery image
// travels at to the model (images.ReferenceSide).
const Side = 1024

// MaxPixelsPerSquare is the most pixels a square has on a side: a larger one adds
// weight, not readability (as dungeonimg).
const MaxPixelsPerSquare = 24

// The colors added to dungeonimg's palette, after its four.
const (
	idxBlack = 4 + iota
	idxParty
	idxNPC
	idxRing
)

// Marker is a creature the players see, on a square.
type Marker struct {
	Col, Row int
	// Party says it is a player's character (blue); false is an NPC (red).
	Party bool
}

// Plan is what a reference shows. Solid is row-major, Cols * Rows long: the
// squares drawn as wall or rock. Seen, when set, is as long and says which squares
// are shown: every other square is solid black (and the plan is cropped to the
// seen ones, see Crop). A nil Seen shows every square.
type Plan struct {
	Cols, Rows int
	Solid      []bool
	Seen       []bool
	Markers    []Marker
}

// ErrPlan is returned for a plan whose slices do not fit its size.
var ErrPlan = errors.New("refimg: the plan does not fit its size")

func (p Plan) valid() bool {
	return p.Cols >= 1 && p.Rows >= 1 && len(p.Solid) == p.Cols*p.Rows && (p.Seen == nil || len(p.Seen) == len(p.Solid))
}

// Crop returns the plan cut to the bounding box of its seen squares and its
// markers, with one square of margin on each side (kept inside the plan), and says
// how many squares are seen. A plan with no Seen is returned as it is. A plan with
// nothing seen is returned as it is too, with a count of 0.
func (p Plan) Crop() (Plan, int) {
	if !p.valid() {
		return p, 0
	}
	if p.Seen == nil {
		return p, len(p.Solid)
	}
	c0, r0, c1, r1 := p.Cols, p.Rows, -1, -1
	seen := 0
	for i, s := range p.Seen {
		if !s {
			continue
		}
		seen++
		c, r := i%p.Cols, i/p.Cols
		c0, c1, r0, r1 = min(c0, c), max(c1, c), min(r0, r), max(r1, r)
	}
	if seen == 0 {
		return p, 0
	}
	c0, r0 = max(c0-1, 0), max(r0-1, 0)
	c1, r1 = min(c1+1, p.Cols-1), min(r1+1, p.Rows-1)
	w, h := c1-c0+1, r1-r0+1
	out := Plan{Cols: w, Rows: h, Solid: make([]bool, w*h), Seen: make([]bool, w*h)}
	for r := range h {
		copy(out.Solid[r*w:(r+1)*w], p.Solid[(r+r0)*p.Cols+c0:])
		copy(out.Seen[r*w:(r+1)*w], p.Seen[(r+r0)*p.Cols+c0:])
	}
	for _, m := range p.Markers {
		if m.Col >= c0 && m.Col <= c1 && m.Row >= r0 && m.Row <= r1 {
			out.Markers = append(out.Markers, Marker{Col: m.Col - c0, Row: m.Row - r0, Party: m.Party})
		}
	}
	return out, seen
}

// SizeFor is the pixel size of a plan of cols x rows squares whose longer side
// stays within side: each square MaxPixelsPerSquare pixels at most, and at least 1.
func SizeFor(cols, rows, side int) (w, h int) {
	longer := max(cols, rows, 1)
	p := min(MaxPixelsPerSquare, max(1, side/longer))
	return cols * p, rows * p
}

// Render draws the plan as a w x h image. Every square must have at least a pixel
// (w >= Cols, h >= Rows), as dungeonimg says.
func Render(p Plan, w, h int) (*image.Paletted, error) {
	if !p.valid() {
		return nil, ErrPlan
	}
	solid := p.Solid
	if p.Seen != nil {
		// What is not seen is rock to the outline (so a seen floor beside it is not
		// outlined as if it opened onto something), and black below.
		solid = make([]bool, len(p.Solid))
		for i := range solid {
			solid[i] = p.Solid[i] || !p.Seen[i]
		}
	}
	img, err := dungeonimg.Render(dungeonimg.Floorplan{Cols: p.Cols, Rows: p.Rows, Solid: solid}, w, h)
	if err != nil {
		return nil, err
	}
	img.Palette = extendPalette(img.Palette)
	if p.Seen != nil {
		for y := range h {
			sr := y * p.Rows / h
			row := img.Pix[y*img.Stride : y*img.Stride+w]
			for x := range w {
				if !p.Seen[sr*p.Cols+x*p.Cols/w] {
					row[x] = idxBlack
				}
			}
		}
	}
	drawMarkers(img, p, w, h)
	return img, nil
}

func extendPalette(base color.Palette) color.Palette {
	return append(base[:4:4],
		color.RGBA{A: 0xFF},                            // idxBlack: not seen
		color.RGBA{R: 0x2F, G: 0x6F, B: 0xDE, A: 0xFF}, // idxParty
		color.RGBA{R: 0xC2, G: 0x33, B: 0x1F, A: 0xFF}, // idxNPC
		color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // idxRing
	)
}

// drawMarkers puts a disc in the middle of each marker's square: a ring and a fill.
func drawMarkers(img *image.Paletted, p Plan, w, h int) {
	for _, m := range p.Markers {
		if m.Col < 0 || m.Row < 0 || m.Col >= p.Cols || m.Row >= p.Rows {
			continue
		}
		x0, x1 := m.Col*w/p.Cols, (m.Col+1)*w/p.Cols
		y0, y1 := m.Row*h/p.Rows, (m.Row+1)*h/p.Rows
		cx, cy := float64(x0+x1)/2, float64(y0+y1)/2
		r := math.Max(1.5, float64(min(x1-x0, y1-y0))*0.38)
		fill := uint8(idxNPC)
		if m.Party {
			fill = idxParty
		}
		for y := max(y0, int(cy-r)-1); y <= min(y1-1, int(cy+r)+1); y++ {
			for x := max(x0, int(cx-r)-1); x <= min(x1-1, int(cx+r)+1); x++ {
				d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
				switch {
				case d <= r-1.2:
					img.Pix[y*img.Stride+x] = fill
				case d <= r:
					img.Pix[y*img.Stride+x] = idxRing
				}
			}
		}
	}
}

// PNG encodes the image; a palette image of flat colors is small.
func PNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---- fitting a map to the model's ratios ----

// Ratios are the aspect ratios the model returns (the same list as package gen).
var Ratios = []string{"1:1", "3:2", "2:3", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"}

func ratioValue(name string) float64 {
	a, b, _ := strings.Cut(name, ":")
	x, _ := strconv.Atoi(a)
	y, _ := strconv.Atoi(b)
	if x <= 0 || y <= 0 {
		return 0
	}
	return float64(x) / float64(y)
}

// Closest is the ratio of the list nearest to w:h, by the ratio of the two (a map
// twice as wide as a ratio and a map half as wide are equally far from it). A tie
// goes to the one earlier in the list.
func Closest(w, h int) string {
	if w < 1 || h < 1 {
		return Ratios[0]
	}
	a := float64(w) / float64(h)
	best, bestD := Ratios[0], math.Inf(1)
	for _, name := range Ratios {
		if d := math.Abs(math.Log(ratioValue(name) / a)); d < bestD-1e-12 {
			best, bestD = name, d
		}
	}
	return best
}

// Pad is a w x h map set in the middle of a canvas of the model's ratio, filled
// with rock beyond the map: the drawing the model gets, and where the map is in
// the picture it returns.
type Pad struct {
	// Ratio is the model's ratio of the canvas, such as "16:9".
	Ratio string
	// W and H are the map's size; CanvasW and CanvasH the canvas's, in the same
	// unit; OffX and OffY where the map's top left corner is on it.
	W, H, CanvasW, CanvasH, OffX, OffY int
}

// PadWith sets a w x h map on a canvas of the named ratio (one of Ratios): the
// canvas is as tall (or as wide) as the map and as wide (or as tall) as the ratio
// asks, the map in the middle. An unknown ratio, or a size under 1, gives the map
// itself as the canvas.
func PadWith(w, h int, ratio string) Pad {
	p := Pad{Ratio: ratio, W: w, H: h, CanvasW: w, CanvasH: h}
	r := ratioValue(ratio)
	if w < 1 || h < 1 || r == 0 {
		return p
	}
	if a := float64(w) / float64(h); r >= a {
		p.CanvasW = max(w, int(math.Round(float64(h)*r)))
	} else {
		p.CanvasH = max(h, int(math.Round(float64(w)/r)))
	}
	p.OffX, p.OffY = (p.CanvasW-w)/2, (p.CanvasH-h)/2
	return p
}

// PadTo is PadWith with the closest ratio.
func PadTo(w, h int) Pad { return PadWith(w, h, Closest(w, h)) }

// Crop is the map's rectangle in a picture of the canvas's proportions, rw x rh
// pixels (the model's answer, whatever its exact size): the canvas's fractions,
// rounded to the pixel.
func (p Pad) Crop(rw, rh int) image.Rectangle {
	if rw < 1 || rh < 1 || p.CanvasW < 1 || p.CanvasH < 1 {
		return image.Rect(0, 0, max(rw, 0), max(rh, 0))
	}
	scale := func(v, canvas, r int) int { return int(math.Round(float64(v) * float64(r) / float64(canvas))) }
	x0, x1 := scale(p.OffX, p.CanvasW, rw), scale(p.OffX+p.W, p.CanvasW, rw)
	y0, y1 := scale(p.OffY, p.CanvasH, rh), scale(p.OffY+p.H, p.CanvasH, rh)
	return image.Rect(x0, y0, max(x1, x0+1), max(y1, y0+1)).Intersect(image.Rect(0, 0, rw, rh))
}

// Fractions is the map's rectangle on the canvas as fractions of it (0 to 1): left,
// top, right, bottom. The drawing is made at a size of its own, whose rounding is
// not the map image's, so the answer is cropped with what was really drawn.
type Fractions [4]float64

// Fractions says where the map is on the canvas.
func (p Pad) Fractions() Fractions {
	if p.CanvasW < 1 || p.CanvasH < 1 {
		return Fractions{0, 0, 1, 1}
	}
	cw, ch := float64(p.CanvasW), float64(p.CanvasH)
	return Fractions{float64(p.OffX) / cw, float64(p.OffY) / ch, float64(p.OffX+p.W) / cw, float64(p.OffY+p.H) / ch}
}

// CropBy is the map's rectangle in a picture rw x rh pixels, from fractions of
// the canvas. Bad fractions (outside 0 to 1, or empty) give the whole picture.
func CropBy(f Fractions, rw, rh int) image.Rectangle {
	whole := image.Rect(0, 0, max(rw, 0), max(rh, 0))
	if rw < 1 || rh < 1 || f[0] < 0 || f[1] < 0 || f[2] > 1 || f[3] > 1 || f[2] <= f[0] || f[3] <= f[1] {
		return whole
	}
	x0, x1 := int(math.Round(f[0]*float64(rw))), int(math.Round(f[2]*float64(rw)))
	y0, y1 := int(math.Round(f[1]*float64(rh))), int(math.Round(f[3]*float64(rh)))
	return image.Rect(x0, y0, max(x1, x0+1), max(y1, y0+1)).Intersect(whole)
}

// CenterCrop is the rectangle of an rw x rh picture that has the proportions of a w x h map and sits in
// the middle of it: the most of the picture that fits the map's ratio. It never stretches: scaling the
// rectangle to the map's size keeps every proportion (a square stays a square). A picture already of
// the map's ratio (to a pixel) is returned whole.
func CenterCrop(w, h, rw, rh int) image.Rectangle {
	whole := image.Rect(0, 0, max(rw, 0), max(rh, 0))
	if w < 1 || h < 1 || rw < 1 || rh < 1 {
		return whole
	}
	// Compare w/h with rw/rh without dividing: w*rh against h*rw.
	switch a, b := int64(w)*int64(rh), int64(h)*int64(rw); {
	case a == b:
		return whole
	case a > b:
		// The map is wider than the picture: keep the full width and the middle of the height.
		ch := int(math.Round(float64(rw) * float64(h) / float64(w)))
		ch = min(max(ch, 1), rh)
		y0 := (rh - ch) / 2
		return image.Rect(0, y0, rw, y0+ch)
	default:
		cw := int(math.Round(float64(rh) * float64(w) / float64(h)))
		cw = min(max(cw, 1), rw)
		x0 := (rw - cw) / 2
		return image.Rect(x0, 0, x0+cw, rh)
	}
}

// RenderPadded draws a whole map (the plan has no Seen and no markers: this is the
// reference of the textured map) on its padded canvas: the map at the size
// SizeFor gives for the padded canvas, and rock around it. It returns the drawing
// and the Pad it used, in the drawing's pixels.
func RenderPadded(p Plan, imgW, imgH int, side int) (*image.Paletted, Pad, error) {
	if !p.valid() || imgW < 1 || imgH < 1 {
		return nil, Pad{}, ErrPlan
	}
	ratio := Closest(imgW, imgH)
	// The scale: the canvas's longer side fits side, and no square passes
	// MaxPixelsPerSquare pixels; the map keeps the image's proportions.
	big := PadWith(imgW, imgH, ratio)
	f := math.Min(float64(side)/float64(max(big.CanvasW, big.CanvasH)), float64(MaxPixelsPerSquare*p.Cols)/float64(imgW))
	mw, mh := max(p.Cols, int(math.Round(float64(imgW)*f))), max(p.Rows, int(math.Round(float64(imgH)*f)))
	pad := PadWith(mw, mh, ratio)

	plan, err := Render(p, mw, mh)
	if err != nil {
		return nil, Pad{}, err
	}
	canvas := image.NewPaletted(image.Rect(0, 0, pad.CanvasW, pad.CanvasH), plan.Palette)
	period := min(8, max(4, min(mw/p.Cols, mh/p.Rows)/3))
	thick := max(1, period/4)
	for y := range pad.CanvasH {
		row := canvas.Pix[y*canvas.Stride : y*canvas.Stride+pad.CanvasW]
		for x := range pad.CanvasW {
			if (x+y)%period < thick {
				row[x] = dungeonimg.IndexHatch
			} else {
				row[x] = dungeonimg.IndexWall
			}
		}
	}
	for y := range mh {
		copy(canvas.Pix[(y+pad.OffY)*canvas.Stride+pad.OffX:], plan.Pix[y*plan.Stride:y*plan.Stride+mw])
	}
	return canvas, pad, nil
}
