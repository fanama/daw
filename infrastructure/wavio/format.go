package wavio

import "encoding/binary"

const (
	riffChunkSize     = 12
	plainFmtSize      = 16
	extensibleFmtSize = 40
	floatFormatTag    = 3
	extensibleTag     = 0xFFFE
	bitsPerSample     = 32
	bytesPerSample    = 4
)

// floatSubFormat is the IEEE float GUID used by WAVE_FORMAT_EXTENSIBLE.
var floatSubFormat = [16]byte{
	0x03, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0xAA,
	0x00, 0x38, 0x9B, 0x71,
}

func isExtensible(channels int) bool { return channels > 2 }

func formatTagFor(channels int) uint16 {
	if isExtensible(channels) {
		return extensibleTag
	}
	return floatFormatTag
}

// HeaderSize returns the number of bytes before the first audio frame.
func HeaderSize(channels int) int {
	if isExtensible(channels) {
		return riffChunkSize + 8 + extensibleFmtSize + 8
	}
	return riffChunkSize + 8 + plainFmtSize + 8
}

func channelMaskFor(channels int) uint32 {
	switch channels {
	case 1:
		return 0x4
	case 2:
		return 0x3
	case 3:
		return 0x7
	case 4:
		return 0x33
	case 5:
		return 0x37
	case 6:
		return 0x3F
	case 7:
		return 0x13F
	case 8:
		return 0x63F
	default:
		return 0
	}
}

func blockAlignFor(channels int) uint16 {
	return uint16(channels * bytesPerSample)
}

func buildHeader(channels, sampleRate, dataFrames int) []byte {
	header := make([]byte, HeaderSize(channels))
	dataBytes := dataFrames * channels * bytesPerSample
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)-8+dataBytes))
	copy(header[8:12], "WAVE")
	offset := writeFormatChunk(header, riffChunkSize, channels, sampleRate)
	writeDataChunk(header, offset, dataBytes)
	return header
}

func writeFormatChunk(header []byte, offset, channels, sampleRate int) int {
	copy(header[offset:offset+4], "fmt ")
	offset += 4
	fmtSize := plainFmtSize
	if isExtensible(channels) {
		fmtSize = extensibleFmtSize
	}
	binary.LittleEndian.PutUint32(header[offset:offset+4], uint32(fmtSize))
	offset += 4
	binary.LittleEndian.PutUint16(header[offset:offset+2], formatTagFor(channels))
	offset += 2
	binary.LittleEndian.PutUint16(header[offset:offset+2], uint16(channels))
	offset += 2
	binary.LittleEndian.PutUint32(header[offset:offset+4], uint32(sampleRate))
	offset += 4
	binary.LittleEndian.PutUint32(header[offset:offset+4], uint32(sampleRate*channels*bytesPerSample))
	offset += 4
	binary.LittleEndian.PutUint16(header[offset:offset+2], blockAlignFor(channels))
	offset += 2
	binary.LittleEndian.PutUint16(header[offset:offset+2], bitsPerSample)
	offset += 2
	if !isExtensible(channels) {
		return offset
	}
	return writeExtensibleFields(header, offset, channels)
}

func writeExtensibleFields(header []byte, offset, channels int) int {
	binary.LittleEndian.PutUint16(header[offset:offset+2], 22)
	offset += 2
	binary.LittleEndian.PutUint16(header[offset:offset+2], bitsPerSample)
	offset += 2
	binary.LittleEndian.PutUint32(header[offset:offset+4], channelMaskFor(channels))
	offset += 4
	copy(header[offset:offset+16], floatSubFormat[:])
	return offset + 16
}

func writeDataChunk(header []byte, offset, dataBytes int) int {
	copy(header[offset:offset+4], "data")
	offset += 4
	binary.LittleEndian.PutUint32(header[offset:offset+4], uint32(dataBytes))
	return offset + 4
}
