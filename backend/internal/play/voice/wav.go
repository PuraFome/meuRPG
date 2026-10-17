package voice

import "encoding/binary"

// wav wraps 16-bit mono PCM samples in a WAV file.
func wav(pcm []byte, sampleRate int) []byte {
	const headerSize = 44
	out := make([]byte, headerSize, headerSize+len(pcm))
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm))) //nolint:gosec // G115: bounded by the answer limit
	copy(out[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)                   // the size of the format chunk
	binary.LittleEndian.PutUint16(out[20:], 1)                    // PCM
	binary.LittleEndian.PutUint16(out[22:], 1)                    // mono
	binary.LittleEndian.PutUint32(out[24:], uint32(sampleRate))   //nolint:gosec // G115: a small positive rate
	binary.LittleEndian.PutUint32(out[28:], uint32(sampleRate*2)) //nolint:gosec // G115: bytes a second
	binary.LittleEndian.PutUint16(out[32:], 2)                    // bytes a sample frame
	binary.LittleEndian.PutUint16(out[34:], 16)                   // bits a sample
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(len(pcm))) //nolint:gosec // G115: bounded by the answer limit
	return append(out, pcm...)
}
