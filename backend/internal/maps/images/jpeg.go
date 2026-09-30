package images

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
)

// jpegHeader is what Process needs from a JPEG's header and image/jpeg does
// not tell: how costly the image is to decode, and its EXIF orientation.
type jpegHeader struct {
	// progressive is true for a progressive JPEG, which costs much more
	// memory to decode (see decodeCost).
	progressive bool
	// samples is the number of samples per pixel: 1 for gray, 1.5 for the
	// usual 4:2:0 color, 3 for color without subsampling, 4 for CMYK.
	samples float64
	// orientation is the EXIF orientation, 1 (upright) to 8.
	orientation int
}

// readJPEGHeader walks a JPEG's segments up to the frame header (SOF),
// reading the EXIF orientation on the way. It never fails: whatever it
// cannot read keeps the cautious default (4 samples per pixel, upright),
// and image/jpeg judges the file itself.
//
// A JPEG is a list of segments, each one 0xFF, a marker byte, a 2-byte
// big-endian length (which counts itself) and the data. The EXIF block is
// the APP1 segment (0xE1) starting with "Exif\0\0"; the frame header is
// SOF0 to SOF15, except 0xC4, 0xC8 and 0xCC, which are other things.
func readJPEGHeader(data []byte) jpegHeader {
	h := jpegHeader{samples: 4, orientation: 1}
	i := 2 // after the start-of-image marker, FF D8
	for i+4 <= len(data) && data[i] == 0xff {
		marker := data[i+1]
		if marker == 0xff { // a fill byte before the marker
			i++
			continue
		}
		length := int(binary.BigEndian.Uint16(data[i+2:]))
		if length < 2 || i+2+length > len(data) {
			break
		}
		segment := data[i+4 : i+2+length]
		switch {
		case marker == 0xe1:
			if o, ok := exifOrientation(segment); ok {
				h.orientation = o
			}
		case marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 && marker != 0xc8 && marker != 0xcc:
			// SOF2, SOF6, SOF10 and SOF14 are the progressive ones.
			h.progressive = marker&0x03 == 0x02
			if s, ok := samplesPerPixel(segment); ok {
				h.samples = s
			}
			return h
		case marker == 0xda: // start of scan: the image data begins
			return h
		}
		i += 2 + length
	}
	return h
}

// samplesPerPixel reads a frame header's components and their sampling
// factors: each component has h x v samples for every hmax x vmax pixels.
func samplesPerPixel(sof []byte) (float64, bool) {
	// Precision (1 byte), height (2), width (2), number of components (1),
	// then 3 bytes per component: its ID, its h<<4|v factors, its table.
	if len(sof) < 6 {
		return 0, false
	}
	n := int(sof[5])
	if n < 1 || len(sof) < 6+3*n {
		return 0, false
	}
	var hmax, vmax, total int
	for c := range n {
		factors := sof[6+3*c+1]
		ch, cv := int(factors>>4), int(factors&0x0f)
		if ch < 1 || cv < 1 {
			return 0, false
		}
		hmax, vmax = max(hmax, ch), max(vmax, cv)
		total += ch * cv
	}
	return float64(total) / float64(hmax*vmax), true
}

// exifOrientation reads the Orientation tag (0x0112) of an APP1 segment's
// EXIF block: "Exif\0\0", then a TIFF header ("II" little-endian or "MM"
// big-endian, the number 42, the offset of the first directory) and the
// first directory (a 2-byte count of 12-byte entries: tag, type, count,
// value).
func exifOrientation(app1 []byte) (int, bool) {
	tiff, ok := bytes.CutPrefix(app1, []byte("Exif\x00\x00"))
	if !ok || len(tiff) < 8 {
		return 0, false
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0, false
	}
	offset := order.Uint32(tiff[4:])
	if offset < 8 || uint64(offset)+2 > uint64(len(tiff)) {
		return 0, false
	}
	dir := tiff[offset:]
	count := int(order.Uint16(dir))
	for e := 0; e < count && 2+12*(e+1) <= len(dir); e++ {
		entry := dir[2+12*e:]
		const orientationTag, typeShort = 0x0112, 3
		if order.Uint16(entry) != orientationTag {
			continue
		}
		if order.Uint16(entry[2:]) != typeShort {
			return 0, false
		}
		o := int(order.Uint16(entry[8:]))
		return o, o >= 1 && o <= 8
	}
	return 0, false
}

// upright turns or mirrors img the way an EXIF orientation says to show
// it: a phone keeps the sensor's pixels and only writes down how it was
// held. Without this, a photo taken upright would show up sideways once
// the EXIF is gone. Orientation 1, or an unknown one, returns img as is.
func upright(img image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// Copy into RGBA first (image/draw has a fast path from JPEG's YCbCr),
	// so the loop below only moves 4-byte pixels around.
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)

	dw, dh := w, h
	if orientation >= 5 { // 5 to 8 turn the image by 90 degrees
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range h {
		for x := range w {
			var dx, dy int
			switch orientation {
			case 2: // mirrored left to right
				dx, dy = w-1-x, y
			case 3: // upside down
				dx, dy = w-1-x, h-1-y
			case 4: // mirrored top to bottom
				dx, dy = x, h-1-y
			case 5: // mirrored along the main diagonal
				dx, dy = y, x
			case 6: // turn 90 degrees clockwise
				dx, dy = h-1-y, x
			case 7: // mirrored along the other diagonal
				dx, dy = h-1-y, w-1-x
			case 8: // turn 90 degrees counterclockwise
				dx, dy = y, w-1-x
			}
			copy(dst.Pix[dst.PixOffset(dx, dy):][:4], src.Pix[src.PixOffset(x, y):][:4])
		}
	}
	return dst
}
