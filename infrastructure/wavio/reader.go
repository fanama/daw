package wavio

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
)

// ErrNotFloatWav is returned when a file is not 32-bit float WAV audio.
var ErrNotFloatWav = errors.New("wavio: not a 32-bit float WAV file")

// Reader streams frames out of a recorded take.
type Reader struct {
	file       *os.File
	channels   int
	sampleRate int
	frames     int
	dataOffset int64
}

type headerInfo struct {
	channels   int
	sampleRate int
	dataOffset int64
	dataBytes  int64
}

// OpenReader opens and validates a take file.
func OpenReader(path string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := readHeader(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	stat, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return nil, statErr
	}
	available := stat.Size() - info.dataOffset
	if available < 0 {
		available = 0
	}
	if info.dataBytes <= 0 || info.dataBytes > available {
		info.dataBytes = available
	}
	blockAlign := int64(info.channels * bytesPerSample)
	frames := 0
	if blockAlign > 0 {
		frames = int(info.dataBytes / blockAlign)
	}
	return &Reader{
		file:       file,
		channels:   info.channels,
		sampleRate: info.sampleRate,
		frames:     frames,
		dataOffset: info.dataOffset,
	}, nil
}

// Channels returns the interleave width.
func (r *Reader) Channels() int { return r.channels }

// SampleRate returns the file frame rate.
func (r *Reader) SampleRate() int { return r.sampleRate }

// Frames returns how many frames the file holds.
func (r *Reader) Frames() int { return r.frames }

// ReadFrames fills dst with interleaved samples starting at atFrame and
// returns the frames actually read.
func (r *Reader) ReadFrames(atFrame int, dst []float32) (int, error) {
	if atFrame < 0 || atFrame >= r.frames || len(dst) == 0 {
		return 0, nil
	}
	wanted := len(dst) / r.channels
	if remaining := r.frames - atFrame; wanted > remaining {
		wanted = remaining
	}
	if wanted <= 0 {
		return 0, nil
	}
	byteCount := wanted * r.channels * bytesPerSample
	buffer := make([]byte, byteCount)
	offset := r.dataOffset + int64(atFrame*r.channels*bytesPerSample)
	if _, err := r.file.ReadAt(buffer, offset); err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	for index := 0; index < wanted*r.channels; index++ {
		dst[index] = math.Float32frombits(binary.LittleEndian.Uint32(buffer[index*4:]))
	}
	return wanted, nil
}

// Close releases the file handle.
func (r *Reader) Close() error { return r.file.Close() }

func readHeader(file io.ReadSeeker) (headerInfo, error) {
	if _, err := file.Seek(0, 0); err != nil {
		return headerInfo{}, err
	}
	magic := make([]byte, riffChunkSize)
	if _, err := io.ReadFull(file, magic); err != nil {
		return headerInfo{}, err
	}
	if string(magic[0:4]) != "RIFF" || string(magic[8:12]) != "WAVE" {
		return headerInfo{}, ErrNotFloatWav
	}
	return scanChunks(file)
}

func scanChunks(file io.ReadSeeker) (headerInfo, error) {
	info := headerInfo{}
	foundFormat := false
	for {
		chunkID, chunkSize, position, err := readChunkHeader(file)
		if err != nil {
			break
		}
		if chunkID == "fmt " {
			if parseErr := parseFormatChunk(file, chunkSize, &info); parseErr != nil {
				return headerInfo{}, parseErr
			}
			foundFormat = true
			continue
		}
		if chunkID == "data" {
			if !foundFormat {
				return headerInfo{}, ErrNotFloatWav
			}
			info.dataOffset = position
			info.dataBytes = chunkSize
			return info, nil
		}
		if _, err := file.Seek(position+chunkSize+chunkSize%2, 0); err != nil {
			return headerInfo{}, err
		}
	}
	return headerInfo{}, ErrNotFloatWav
}

func readChunkHeader(file io.ReadSeeker) (string, int64, int64, error) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		return "", 0, 0, err
	}
	chunkID := string(header[0:4])
	chunkSize := int64(binary.LittleEndian.Uint32(header[4:8]))
	position, err := file.Seek(0, 1)
	if err != nil {
		return "", 0, 0, err
	}
	return chunkID, chunkSize, position, nil
}

func parseFormatChunk(file io.ReadSeeker, chunkSize int64, info *headerInfo) error {
	payload := make([]byte, chunkSize)
	if _, err := io.ReadFull(file, payload); err != nil {
		return err
	}
	tag := binary.LittleEndian.Uint16(payload[0:2])
	if tag != floatFormatTag && tag != extensibleTag {
		return ErrNotFloatWav
	}
	info.channels = int(binary.LittleEndian.Uint16(payload[2:4]))
	info.sampleRate = int(binary.LittleEndian.Uint32(payload[4:8]))
	return nil
}
