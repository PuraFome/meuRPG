package images

import (
	"errors"
	"image"
	"testing"
)

// The order CropFit checks in, one refusal at a time: the output's size, the file's size,
// the type, the header, the answer's side, the memory budget, the pixels, the crop. A request
// that breaks two rules is refused for the first.
func TestCropFitRefusalsAreInTheirOrder(t *testing.T) {
	t.Parallel()
	whole := func(w, h int) image.Rectangle { return image.Rect(0, 0, w, h) }
	good := encodePNG(t, twoColors(40, 30))
	pngSignature := "\x89PNG\r\n\x1a\n"
	tooBig := make([]byte, MaxBytes+1)
	copy(tooBig, pngSignature)
	notAPicture := []byte("GIF89a......")
	wide := encodePNG(t, image.NewGray(image.Rect(0, 0, maxFitAnswerSide+1, 8)))
	for name, tc := range map[string]struct {
		data       []byte
		crop       func(w, h int) image.Rectangle
		outW, outH int
		want       error
	}{
		"the output's size comes before the file":    {tooBig, whole, 0, 10, ErrDimensions},
		"a file over the cap":                        {tooBig, whole, 10, 10, ErrTooLarge},
		"the file's size comes before its type":      {append(make([]byte, MaxBytes), 'x'), whole, 10, 10, ErrTooLarge},
		"a type that is not accepted":                {notAPicture, whole, 10, 10, ErrUnsupportedType},
		"a header that cannot be read":               {[]byte(pngSignature + "not a header at all"), whole, 10, 10, ErrCorrupt},
		"a side over what the model returns":         {wide, whole, 10, 10, ErrDimensions},
		"a body that cannot be decoded":              {good[:len(good)-40], whole, 10, 10, ErrCorrupt},
		"the side comes before the crop":             {wide, func(int, int) image.Rectangle { return image.Rectangle{} }, 10, 10, ErrDimensions},
		"a crop with no area":                        {good, func(int, int) image.Rectangle { return image.Rect(3, 3, 3, 3) }, 10, 10, ErrDimensions},
		"a crop beside the picture":                  {good, func(w, h int) image.Rectangle { return image.Rect(w, h, 2*w, 2*h) }, 10, 10, ErrDimensions},
		"a crop of the whole picture is accepted":    {good, whole, 10, 10, nil},
		"a crop bigger than the picture is clipped":  {good, func(w, h int) image.Rectangle { return image.Rect(-5, -5, 2*w, 2*h) }, 10, 10, nil},
		"an output at the pixel cap is accepted too": {good, whole, 4000, 4000, nil},
	} {
		res, err := CropFit(tc.data, tc.crop, tc.outW, tc.outH)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: error = %v, want %v", name, err, tc.want)
		}
		if tc.want == nil && (err != nil || res.Width != tc.outW || res.Height != tc.outH || res.ContentType != JPEG) {
			t.Errorf("%s: CropFit() = %v, %v; want a JPEG of the output's size", name, res, err)
		}
	}
}

// A 16-bit answer costs more to decode than an 8-bit one, and with a 16-megapixel output it
// passes the slot's budget: it is refused before it is decoded, and the same answer into a
// small output is let through.
func TestCropFitMemoryBudgetCountsTheOutput(t *testing.T) {
	t.Parallel()
	whole := func(w, h int) image.Rectangle { return image.Rect(0, 0, w, h) }
	big := encodePNG(t, image.NewNRGBA64(image.Rect(0, 0, 3300, 3300)))
	if _, err := CropFit(big, whole, 4000, 4000); !errors.Is(err, ErrDimensions) {
		t.Errorf("a 16-bit 3300 x 3300 answer into 16 megapixels: error = %v, want ErrDimensions", err)
	}
	if res, err := CropFit(big, whole, 400, 400); err != nil || res.Width != 400 {
		t.Errorf("the same answer into 400 x 400: %v, %v; want it let through", res, err)
	}
}
