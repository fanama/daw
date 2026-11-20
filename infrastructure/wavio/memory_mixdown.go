package wavio

import (
	"encoding/binary"
	"errors"
	"math"

	"gorecord/domain/session"
)

// MemoryMixdowns renders exports into memory, for clients that hand the bytes
// to a download instead of writing a file.
type MemoryMixdowns struct {
	channels   int
	sampleRate int
	samples    []float32
}

// NewMemoryMixdowns returns an empty in-memory mixdown destination.
func NewMemoryMixdowns() *MemoryMixdowns { return &MemoryMixdowns{} }

// Create starts a new mixdown, discarding any previous one.
func (m *MemoryMixdowns) Create(_ string, channels, sampleRate int) (session.MixdownFile, error) {
	if channels < 1 || channels > 64 {
		return nil, errors.New("wavio: channel count must be between 1 and 64")
	}
	if sampleRate < 8000 || sampleRate > 384000 {
		return nil, errors.New("wavio: unsupported sample rate")
	}
	m.channels = channels
	m.sampleRate = sampleRate
	m.samples = nil
	return &memoryMixdown{target: m}, nil
}

// WAV encodes the last mixdown as a 32-bit float WAV file.
func (m *MemoryMixdowns) WAV() ([]byte, error) {
	if m.channels < 1 || len(m.samples) == 0 {
		return nil, errors.New("wavio: no mixdown has been rendered")
	}
	header := buildHeader(m.channels, m.sampleRate, len(m.samples)/m.channels)
	output := make([]byte, len(header)+len(m.samples)*bytesPerSample)
	copy(output, header)
	for index, sample := range m.samples {
		offset := len(header) + index*bytesPerSample
		binary.LittleEndian.PutUint32(output[offset:], math.Float32bits(sample))
	}
	return output, nil
}

type memoryMixdown struct {
	target *MemoryMixdowns
}

// Write appends interleaved samples and returns the frames consumed.
func (w *memoryMixdown) Write(samples []float32) (int, error) {
	if len(samples)%w.target.channels != 0 {
		return 0, errors.New("wavio: mixdown samples are not frame aligned")
	}
	w.target.samples = append(w.target.samples, samples...)
	return len(samples) / w.target.channels, nil
}

// Finish completes the mixdown; the bytes are read with WAV.
func (w *memoryMixdown) Finish() error { return nil }
