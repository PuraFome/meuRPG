package images

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"golang.org/x/image/webp"
)

// The test images are built here, in code, so the tests say exactly what
// each file holds: metadata that must go away, or a header that lies.

var (
	red         = color.NRGBA{R: 220, G: 30, B: 30, A: 255}
	blue        = color.NRGBA{R: 30, G: 30, B: 220, A: 255}
	transparent = color.NRGBA{}
)

// twoColors is a w x h image, its left half red and its right half blue.
func twoColors(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x < w/2 {
				img.SetNRGBA(x, y, red)
			} else {
				img.SetNRGBA(x, y, blue)
			}
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

// secret is text hidden in the test images' metadata, next to a GPS
// position: none of it may survive Process.
const secret = "secret place near the GPS fix"

// exifTIFF builds the TIFF part of an EXIF block, little-endian: an image
// description (secret), an orientation, and a GPS directory with a
// latitude and a longitude (Sao Paulo).
func exifTIFF(orientation uint16) []byte {
	le := binary.LittleEndian
	const desc = secret + "\x00"
	// Where each part starts: the header (8 bytes), IFD0 (a count, 12 bytes
	// per entry, the next IFD's offset), the description, the GPS
	// directory, then its two values of three rationals (24 bytes each).
	const (
		ifd0        = 8
		ifd0Entries = 3
		descAt      = ifd0 + 2 + 12*ifd0Entries + 4
		gpsAt       = descAt + len(desc)
		gpsEntries  = 4
		latAt       = gpsAt + 2 + 12*gpsEntries + 4
		lonAt       = latAt + 24
	)
	b := make([]byte, lonAt+24)

	copy(b, "II")
	le.PutUint16(b[2:], 42)
	le.PutUint32(b[4:], ifd0)
	entry := func(at int, tag, typ uint16, count, value uint32) {
		le.PutUint16(b[at:], tag)
		le.PutUint16(b[at+2:], typ)
		le.PutUint32(b[at+4:], count)
		le.PutUint32(b[at+8:], value)
	}
	const ascii, short, long, rational = 2, 3, 4, 5
	le.PutUint16(b[ifd0:], ifd0Entries)
	entry(ifd0+2, 0x010e, ascii, uint32(len(desc)), uint32(descAt)) // ImageDescription
	entry(ifd0+14, 0x0112, short, 1, uint32(orientation))           // Orientation
	entry(ifd0+26, 0x8825, long, 1, uint32(gpsAt))                  // GPSInfo: where the GPS directory is
	copy(b[descAt:], desc)

	le.PutUint16(b[gpsAt:], gpsEntries)
	entry(gpsAt+2, 0x0001, ascii, 2, 'S')               // GPSLatitudeRef
	entry(gpsAt+14, 0x0002, rational, 3, uint32(latAt)) // GPSLatitude
	entry(gpsAt+26, 0x0003, ascii, 2, 'W')              // GPSLongitudeRef
	entry(gpsAt+38, 0x0004, rational, 3, uint32(lonAt)) // GPSLongitude
	degrees := func(at int, d, m, s uint32) {
		for i, v := range []uint32{d, m, s} {
			le.PutUint32(b[at+8*i:], v)
			le.PutUint32(b[at+8*i+4:], 1)
		}
	}
	degrees(latAt, 23, 33, 2)
	degrees(lonAt, 46, 38, 1)
	return b
}

// withEXIF puts an APP1 EXIF segment right after a JPEG's start marker.
func withEXIF(jpg, tiff []byte) []byte {
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := binary.BigEndian.AppendUint16([]byte{0xff, 0xe1}, uint16(len(payload)+2)) //nolint:gosec // G115: a test EXIF block of a few hundred bytes
	out := append([]byte{}, jpg[:2]...)
	out = append(out, segment...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

// pngChunk is one PNG chunk: length, type, data, CRC.
func pngChunk(typ string, data []byte) []byte {
	c := binary.BigEndian.AppendUint32(nil, uint32(len(data))) //nolint:gosec // G115: test chunks are small
	c = append(c, typ...)
	c = append(c, data...)
	return binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(c[4:]))
}

// withPNGChunks puts chunks right after a PNG's IHDR (the 8-byte signature
// and the 25-byte IHDR chunk come first).
func withPNGChunks(p []byte, chunks ...[]byte) []byte {
	out := append([]byte{}, p[:33]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, p[33:]...)
}

// pngHeader is a PNG with only a signature, an IHDR and an IEND: enough for
// DecodeConfig, which is all a lying header needs to be refused.
func pngHeader(w, h uint32, bitDepth, colorType byte) []byte {
	return pngHeaderInterlaced(w, h, bitDepth, colorType, 0)
}

// pngHeaderInterlaced is pngHeader with an interlace method (1 is Adam7).
func pngHeaderInterlaced(w, h uint32, bitDepth, colorType, interlace byte) []byte {
	ihdr := binary.BigEndian.AppendUint32(nil, w)
	ihdr = binary.BigEndian.AppendUint32(ihdr, h)
	ihdr = append(ihdr, bitDepth, colorType, 0, 0, interlace)
	out := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", ihdr)...)
	return append(out, pngChunk("IEND", nil)...)
}

// jpegFrameOnly is a JPEG with only a frame header (SOF0 for baseline, SOF2
// for progressive) of w x h pixels in three components without
// subsampling, and the start of a scan.
func jpegFrameOnly(sof byte, w, h uint16) []byte {
	out := []byte{0xff, 0xd8, 0xff, sof, 0, 17, 8}
	out = binary.BigEndian.AppendUint16(out, h)
	out = binary.BigEndian.AppendUint16(out, w)
	out = append(out, 3, 1, 0x11, 0, 2, 0x11, 0, 3, 0x11, 0)
	out = append(out, 0xff, 0xda, 0, 12, 3, 1, 0, 2, 0x11, 3, 0x11, 0, 63, 0)
	return append(out, 0xff, 0xd9)
}

// bitWriter packs bits least significant first, as WebP lossless (VP8L)
// reads them.
type bitWriter struct {
	buf  []byte
	acc  uint64
	bits uint
}

func (w *bitWriter) write(v uint64, n uint) {
	w.acc |= v << w.bits
	w.bits += n
	for w.bits >= 8 {
		w.buf = append(w.buf, byte(w.acc)) //nolint:gosec // G115: the low byte, on purpose
		w.acc >>= 8
		w.bits -= 8
	}
}

func (w *bitWriter) bytes() []byte {
	if w.bits > 0 {
		w.buf = append(w.buf, byte(w.acc)) //nolint:gosec // G115: the last bits, on purpose
	}
	return w.buf
}

// vp8lSolid is a WebP lossless bitstream for a w x h image of one color:
// no transforms and no color cache, and each of the five prefix codes
// (green, red, blue, alpha, distance) a "simple" code with a single
// symbol, which takes no bits per pixel. See the WebP lossless spec,
// https://developers.google.com/speed/webp/docs/webp_lossless_bitstream_specification.
func vp8lSolid(w, h uint64, c color.NRGBA) []byte {
	var bw bitWriter
	bw.write(0x2f, 8) // signature
	bw.write(w-1, 14)
	bw.write(h-1, 14)
	if c.A != 255 {
		bw.write(1, 1) // alpha_is_used
	} else {
		bw.write(0, 1)
	}
	bw.write(0, 3) // version
	bw.write(0, 1) // no transform
	bw.write(0, 1) // no color cache
	bw.write(0, 1) // no meta prefix codes
	for _, symbol := range []uint8{c.G, c.R, c.B, c.A} {
		bw.write(1, 1) // simple code
		bw.write(0, 1) // one symbol
		bw.write(1, 1) // an 8-bit symbol
		bw.write(uint64(symbol), 8)
	}
	bw.write(1, 1) // distance: simple code
	bw.write(0, 1) // one symbol
	bw.write(0, 1) // a 1-bit symbol
	bw.write(0, 1) // symbol 0
	return bw.bytes()
}

func riffChunk(id string, data []byte) []byte {
	c := binary.LittleEndian.AppendUint32([]byte(id), uint32(len(data))) //nolint:gosec // G115: test chunks are small
	c = append(c, data...)
	if len(data)%2 == 1 {
		c = append(c, 0) // chunks are padded to an even size
	}
	return c
}

// webpWithEXIF is an extended WebP (VP8X) holding a lossless w x h image of
// one color and an EXIF chunk.
func webpWithEXIF(w, h uint32, c color.NRGBA) []byte {
	// Flags, 3 reserved bytes, then the width and height minus one, in 24
	// bits each (little-endian: the 4th byte of each PutUint32 is
	// overwritten or left as the padding at the end).
	vp8x := make([]byte, 11)
	vp8x[0] = 0x08 // has EXIF
	if c.A != 255 {
		vp8x[0] |= 0x10 // has alpha
	}
	binary.LittleEndian.PutUint32(vp8x[4:], w-1)
	binary.LittleEndian.PutUint32(vp8x[7:], h-1)
	vp8x = vp8x[:10]
	body := []byte("WEBP")
	body = append(body, riffChunk("VP8X", vp8x)...)
	body = append(body, riffChunk("VP8L", vp8lSolid(uint64(w), uint64(h), c))...)
	body = append(body, riffChunk("EXIF", exifTIFF(1))...)
	return append(binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(len(body))), body...) //nolint:gosec // G115: a test file is small
}

// metadataMarkers are byte strings that only metadata would put in a file:
// they must be in the test input, and gone from the output.
var metadataMarkers = []string{"Exif", "EXIF", "eXIf", "tEXt", "II*\x00", secret}

func assertNoMetadata(t *testing.T, name string, in, out []byte) {
	t.Helper()
	for _, m := range metadataMarkers {
		if bytes.Contains(out, []byte(m)) {
			t.Errorf("%s: the stored image still has %q", name, m)
		}
	}
	if !bytes.Contains(in, []byte(secret)) {
		t.Fatalf("%s: the test input has no metadata to remove", name)
	}
}

func decodeSize(t *testing.T, data []byte) (image.Image, string) {
	t.Helper()
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the stored image does not decode: %v", err)
	}
	return img, format
}

func TestProcessRemovesJPEGMetadata(t *testing.T) {
	t.Parallel()
	in := withEXIF(encodeJPEG(t, twoColors(64, 48)), exifTIFF(1))
	// Anything appended after the image (a "polyglot" file) goes too.
	in = append(in, "<?php echo 'hidden payload'; ?>"...)

	res, err := Process(in)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	assertNoMetadata(t, "JPEG", in, res.Data)
	assertNoMetadata(t, "JPEG thumbnail", in, res.Thumbnail)
	if bytes.Contains(res.Data, []byte("hidden payload")) {
		t.Error("the stored image still has the appended payload")
	}
	img, format := decodeSize(t, res.Data)
	if format != "jpeg" || res.ContentType != JPEG || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 48 || res.Width != 64 || res.Height != 48 {
		t.Errorf("stored %s %v (%s %dx%d), want a 64x48 JPEG", format, img.Bounds(), res.ContentType, res.Width, res.Height)
	}
}

func TestProcessRemovesPNGMetadata(t *testing.T) {
	t.Parallel()
	src := twoColors(40, 30)
	src.SetNRGBA(0, 0, transparent) // one transparent pixel, which must survive
	in := withPNGChunks(encodePNG(t, src),
		pngChunk("tEXt", []byte("Comment\x00"+secret)),
		pngChunk("eXIf", exifTIFF(1)),
	)

	res, err := Process(in)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	assertNoMetadata(t, "PNG", in, res.Data)
	img, format := decodeSize(t, res.Data)
	if format != "png" || res.ContentType != PNG || img.Bounds() != src.Bounds() {
		t.Errorf("stored %s %v (%s), want a PNG of %v", format, img.Bounds(), res.ContentType, src.Bounds())
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("the transparent pixel has alpha %d, want 0: PNG keeps transparency", a)
	}
}

func TestProcessRemovesWebPMetadata(t *testing.T) {
	t.Parallel()
	in := webpWithEXIF(30, 20, blue)
	if _, err := webp.Decode(bytes.NewReader(in)); err != nil {
		t.Fatalf("the test WebP does not decode: %v", err)
	}

	res, err := Process(in)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	assertNoMetadata(t, "WebP", in, res.Data)
	// An opaque WebP becomes a JPEG.
	img, format := decodeSize(t, res.Data)
	if format != "jpeg" || res.ContentType != JPEG || img.Bounds().Dx() != 30 || img.Bounds().Dy() != 20 {
		t.Errorf("stored %s %v (%s), want a 30x20 JPEG", format, img.Bounds(), res.ContentType)
	}
}

func TestProcessKeepsAWebPWithTransparencyAsPNG(t *testing.T) {
	t.Parallel()
	in := webpWithEXIF(16, 16, color.NRGBA{R: 10, G: 200, B: 10, A: 128})
	res, err := Process(in)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	assertNoMetadata(t, "WebP with alpha", in, res.Data)
	img, format := decodeSize(t, res.Data)
	if format != "png" || res.ContentType != PNG {
		t.Fatalf("stored %s (%s), want PNG: a JPEG would lose the transparency", format, res.ContentType)
	}
	if _, _, _, a := img.At(3, 3).RGBA(); a == 0xffff {
		t.Error("the half-transparent pixel became opaque")
	}
}

// TestTransparentPNGKeepsAlpha is Q62: an NPC's portrait is a cut-out PNG
// with a transparent background, and the stage draws it with nothing behind
// it. The stored image and its thumbnail must keep a transparent pixel
// transparent and an opaque one opaque, for a big image (the thumbnail is
// scaled) and a small one (it is its own thumbnail).
func TestTransparentPNGKeepsAlpha(t *testing.T) {
	t.Parallel()
	// The left half is transparent, the right half is solid red.
	cutout := func(w, h int) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := w / 2; x < w; x++ {
				img.SetNRGBA(x, y, red)
			}
		}
		return img
	}
	for _, size := range []struct {
		name string
		w, h int
	}{
		{"big, so the thumbnail is scaled", 960, 1200},
		{"small, so the thumbnail is the image", 100, 120},
	} {
		t.Run(size.name, func(t *testing.T) {
			t.Parallel()
			res, err := Process(encodePNG(t, cutout(size.w, size.h)))
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if res.ContentType != PNG {
				t.Fatalf("stored %s, want PNG", res.ContentType)
			}
			for name, data := range map[string][]byte{"image": res.Data, "thumbnail": res.Thumbnail} {
				img, format := decodeSize(t, data)
				if format != "png" {
					t.Fatalf("%s is a %s, want PNG", name, format)
				}
				b := img.Bounds()
				if _, _, _, a := img.At(b.Dx()/4, b.Dy()/2).RGBA(); a != 0 {
					t.Errorf("%s: the transparent pixel has alpha %#x, want 0", name, a)
				}
				if _, _, _, a := img.At(b.Dx()*3/4, b.Dy()/2).RGBA(); a != 0xffff {
					t.Errorf("%s: the opaque pixel has alpha %#x, want 0xffff", name, a)
				}
			}
		})
	}
}

func TestProcessRefuses(t *testing.T) {
	t.Parallel()
	var gifData bytes.Buffer
	if err := gif.Encode(&gifData, twoColors(8, 8), nil); err != nil {
		t.Fatal(err)
	}
	jpg := encodeJPEG(t, twoColors(64, 64))

	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"empty", nil, ErrUnsupportedType},
		{"SVG", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), ErrUnsupportedType},
		{"SVG with an XML declaration", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), ErrUnsupportedType},
		{"GIF", gifData.Bytes(), ErrUnsupportedType},
		{"random bytes", bytes.Repeat([]byte{0x13, 0x37, 0xca, 0xfe}, 256), ErrUnsupportedType},
		{"a PNG header claiming 50000 x 50000", pngHeader(50000, 50000, 8, 6), ErrDimensions},
		{"a PNG over 8192 pixels on one side", pngHeader(9000, 10, 8, 6), ErrDimensions},
		{"a PNG over 40 megapixels", pngHeader(8000, 5100, 8, 0), ErrDimensions},
		// 16-bit RGBA takes 8 bytes a pixel: 36 megapixels would need 288 MiB.
		{"a 16-bit PNG too big to decode", pngHeader(6000, 6000, 16, 6), ErrDimensions},
		// 8-bit RGBA at 36 megapixels fits; interlaced, it takes twice as
		// much, and does not. (The first has no image data, so it fails
		// later, as corrupt.)
		{"an 8-bit PNG header with no data", pngHeader(6000, 6000, 8, 6), ErrCorrupt},
		{"an interlaced PNG too big to decode", pngHeaderInterlaced(6000, 6000, 8, 6, 1), ErrDimensions},
		// A progressive JPEG keeps its coefficients in memory: 24 megapixels
		// without subsampling would need about 360 MiB. The same header as
		// baseline passes that check, and fails later: it has no image data.
		{"a progressive JPEG too big to decode", jpegFrameOnly(0xc2, 6000, 4000), ErrDimensions},
		{"a baseline JPEG header with no data", jpegFrameOnly(0xc0, 6000, 4000), ErrCorrupt},
		{"a truncated JPEG", jpg[:len(jpg)/2], ErrCorrupt},
		{"a PNG with a broken body", append(pngHeader(10, 10, 8, 6)[:33], pngChunk("IDAT", []byte("not zlib"))...), ErrCorrupt},
		{"a WebP with a broken body", append([]byte("RIFF\x10\x00\x00\x00WEBP"), riffChunk("VP8L", []byte{0x2f, 1, 2})...), ErrCorrupt},
		{"over 10 MiB", append([]byte("\xff\xd8\xff"), make([]byte, MaxBytes)...), ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := Process(tt.in)
			if !errors.Is(err, tt.want) || res != nil {
				t.Errorf("Process() = %v, %v; want %v", res, err, tt.want)
			}
		})
	}
}

// TestProcessTurnsJPEGsUpright: the EXIF orientation goes away with the
// rest of the EXIF, so the pixels must be turned first, or a phone photo
// would show up sideways.
func TestProcessTurnsJPEGsUpright(t *testing.T) {
	t.Parallel()
	// The source is 64 x 32, red on the left and blue on the right.
	tests := []struct {
		orientation   uint16
		width, height int
		redAt, blueAt image.Point
	}{
		{1, 64, 32, image.Pt(8, 16), image.Pt(56, 16)},
		{2, 64, 32, image.Pt(56, 16), image.Pt(8, 16)}, // mirrored
		{3, 64, 32, image.Pt(56, 16), image.Pt(8, 16)}, // upside down
		{6, 32, 64, image.Pt(16, 8), image.Pt(16, 56)}, // turned clockwise: the left side goes on top
		{8, 32, 64, image.Pt(16, 56), image.Pt(16, 8)}, // turned counterclockwise: the left side goes to the bottom
	}
	for _, tt := range tests {
		in := withEXIF(encodeJPEG(t, twoColors(64, 32)), exifTIFF(tt.orientation))
		res, err := Process(in)
		if err != nil {
			t.Fatalf("orientation %d: Process() error = %v", tt.orientation, err)
		}
		img, _ := decodeSize(t, res.Data)
		if img.Bounds().Dx() != tt.width || img.Bounds().Dy() != tt.height || res.Width != tt.width || res.Height != tt.height {
			t.Errorf("orientation %d: size %v (%dx%d), want %dx%d", tt.orientation, img.Bounds(), res.Width, res.Height, tt.width, tt.height)
			continue
		}
		if !isReddish(img.At(tt.redAt.X, tt.redAt.Y)) || isReddish(img.At(tt.blueAt.X, tt.blueAt.Y)) {
			t.Errorf("orientation %d: red is not at %v and blue at %v", tt.orientation, tt.redAt, tt.blueAt)
		}
	}
}

func isReddish(c color.Color) bool {
	r, _, b, _ := c.RGBA()
	return r > 2*b
}

func TestProcessMakesAThumbnail(t *testing.T) {
	t.Parallel()
	res, err := Process(encodeJPEG(t, twoColors(1200, 600)))
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	thumb, format := decodeSize(t, res.Thumbnail)
	if format != "jpeg" || thumb.Bounds().Dx() != ThumbnailSide || thumb.Bounds().Dy() != 240 {
		t.Errorf("thumbnail is a %s of %v, want a 480x240 JPEG", format, thumb.Bounds())
	}
	if !isReddish(thumb.At(60, 120)) || isReddish(thumb.At(420, 120)) {
		t.Error("the thumbnail does not look like the image")
	}

	tall, err := Process(encodePNG(t, twoColors(300, 900)))
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	thumb, format = decodeSize(t, tall.Thumbnail)
	if format != "png" || thumb.Bounds().Dx() != 160 || thumb.Bounds().Dy() != ThumbnailSide {
		t.Errorf("tall thumbnail is a %s of %v, want a 160x480 PNG", format, thumb.Bounds())
	}

	// A small image is its own thumbnail.
	small, err := Process(encodePNG(t, twoColors(100, 50)))
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if !bytes.Equal(small.Thumbnail, small.Data) {
		t.Error("a 100x50 image got a different thumbnail")
	}
}

func TestReadJPEGHeader(t *testing.T) {
	t.Parallel()
	// Go's encoder subsamples color 4:2:0 (1.5 samples a pixel) and writes
	// gray as one component.
	color420 := readJPEGHeader(encodeJPEG(t, twoColors(16, 16)))
	gray := readJPEGHeader(encodeJPEG(t, image.NewGray(image.Rect(0, 0, 16, 16))))
	rotated := readJPEGHeader(withEXIF(encodeJPEG(t, twoColors(16, 16)), exifTIFF(6)))
	progressive := readJPEGHeader(jpegFrameOnly(0xc2, 10, 10))
	for _, tt := range []struct {
		name string
		got  jpegHeader
		want jpegHeader
	}{
		{"color", color420, jpegHeader{samples: 1.5, orientation: 1}},
		{"gray", gray, jpegHeader{samples: 1, orientation: 1}},
		{"with EXIF orientation 6", rotated, jpegHeader{samples: 1.5, orientation: 6}},
		{"progressive, 4:4:4", progressive, jpegHeader{progressive: true, samples: 3, orientation: 1}},
		{"garbage", readJPEGHeader([]byte("\xff\xd8\xff\xe1\xff\xff")), jpegHeader{samples: 4, orientation: 1}},
	} {
		if tt.got != tt.want {
			t.Errorf("%s: readJPEGHeader() = %+v, want %+v", tt.name, tt.got, tt.want)
		}
	}
}

func TestExifOrientationIgnoresBrokenBlocks(t *testing.T) {
	t.Parallel()
	good := append([]byte("Exif\x00\x00"), exifTIFF(6)...)
	if o, ok := exifOrientation(good); !ok || o != 6 {
		t.Fatalf("exifOrientation(good) = %d, %v; want 6", o, ok)
	}
	big := append([]byte("Exif\x00\x00MM\x00\x2a"), 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 3, 0, 0)
	if o, ok := exifOrientation(big); !ok || o != 3 {
		t.Errorf("exifOrientation(big-endian) = %d, %v; want 3", o, ok)
	}
	for name, block := range map[string][]byte{
		"not EXIF":            []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta/>"),
		"cut short":           good[:20],
		"offset past the end": append([]byte("Exif\x00\x00II\x2a\x00"), 0xff, 0xff, 0xff, 0x7f),
		"bad byte order":      append([]byte("Exif\x00\x00XX"), good[8:]...),
		"orientation 9":       append([]byte("Exif\x00\x00"), exifTIFF(9)...),
	} {
		if o, ok := exifOrientation(block); ok {
			t.Errorf("%s: exifOrientation() = %d, true; want false", name, o)
		}
	}
}

// A 16-bit PNG is stored with 8 bits a channel, so what decodes it later (the
// fog's tiles) never needs 8 bytes a pixel.
func TestProcessStoresPNGsAs8Bit(t *testing.T) {
	t.Parallel()
	src := image.NewRGBA64(image.Rect(0, 0, 40, 30))
	for y := range 30 {
		for x := range 40 {
			src.SetRGBA64(x, y, color.RGBA64{R: uint16(x * 1500), G: uint16(y * 2000), B: 0x8000, A: uint16(0x8000 + x*100)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	res, err := Process(buf.Bytes())
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	out, err := png.Decode(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	switch out.(type) {
	case *image.NRGBA, *image.RGBA:
	default:
		t.Errorf("the stored PNG decodes to %T, want 8 bits a channel", out)
	}
}

// A palette image (a generated dungeon's map) gets its thumbnail by averaging, and
// Encode checks the size like Process.
func TestEncodeAPaletteImage(t *testing.T) {
	t.Parallel()
	pal := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	img := image.NewPaletted(image.Rect(0, 0, 960, 480), pal)
	for y := range 480 {
		for x := 480; x < 960; x++ {
			img.SetColorIndex(x, y, 1) // the right half is blue
		}
	}
	res, err := Encode(img)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if res.ContentType != PNG || res.Width != 960 || res.Height != 480 {
		t.Errorf("result = %s %d x %d", res.ContentType, res.Width, res.Height)
	}
	thumb, err := png.Decode(bytes.NewReader(res.Thumbnail))
	if err != nil {
		t.Fatalf("decode the thumbnail: %v", err)
	}
	if b := thumb.Bounds(); b.Dx() != ThumbnailSide || b.Dy() != 240 {
		t.Errorf("thumbnail = %v, want 480 x 240", b)
	}
	if r, _, _, _ := thumb.At(10, 10).RGBA(); r>>8 != 255 {
		t.Errorf("the left of the thumbnail is not red")
	}
	if _, _, b, _ := thumb.At(470, 10).RGBA(); b>>8 != 255 {
		t.Errorf("the right of the thumbnail is not blue")
	}
	if _, err := Encode(image.NewPaletted(image.Rect(0, 0, MaxSide+1, 1), pal)); !errors.Is(err, ErrDimensions) {
		t.Errorf("Encode(too wide) error = %v, want ErrDimensions", err)
	}
}
