package wavio

import (
	"errors"
	"fmt"
	"path"

	"fanamix/domain/session"
)

// MemoryTakes keeps take audio in memory. The browser build records with the
// Web Audio API and registers the captured samples here, so the session use
// cases, normalization and mixdown export run unchanged.
type MemoryTakes struct {
	audio map[string]*memoryAudio
}

// NewMemoryTakes returns an empty in-memory take store.
func NewMemoryTakes() *MemoryTakes {
	return &MemoryTakes{audio: map[string]*memoryAudio{}}
}

// Register stores interleaved samples for a session-relative take path,
// replacing any previous audio for that path.
func (m *MemoryTakes) Register(file string, channels, sampleRate int, samples []float32) error {
	if file == "" {
		return errors.New("wavio: take file is required")
	}
	if channels < 1 || channels > 64 {
		return errors.New("wavio: channel count must be between 1 and 64")
	}
	if sampleRate < 8000 || sampleRate > 384000 {
		return errors.New("wavio: unsupported sample rate")
	}
	if len(samples)%channels != 0 {
		return errors.New("wavio: sample count is not frame aligned")
	}
	m.audio[file] = &memoryAudio{channels: channels, sampleRate: sampleRate, samples: samples}
	return nil
}

// Forget drops a take's samples.
func (m *MemoryTakes) Forget(file string) { delete(m.audio, file) }

// Has reports whether samples are registered for a take.
func (m *MemoryTakes) Has(file string) bool {
	_, ok := m.audio[file]
	return ok
}

// TakePath reports the session-relative path a take number will use.
func (m *MemoryTakes) TakePath(_ string, number int) string {
	return path.Join("takes", fmt.Sprintf("take-%03d.wav", number))
}

// CreateTake is not used in the browser: recordings are registered as PCM
// after the Web Audio capture stops.
func (m *MemoryTakes) CreateTake(string, int, int, int) (session.TakeWriter, string, error) {
	return nil, "", errors.New("wavio: browser recordings are registered after capture")
}

// OpenTake exposes a registered take to playback, normalization and export.
func (m *MemoryTakes) OpenTake(_, relativePath string) (session.TakeAudio, error) {
	audio, ok := m.audio[relativePath]
	if !ok {
		return nil, fmt.Errorf("wavio: take %q has no registered audio", relativePath)
	}
	return &memoryReader{audio: audio}, nil
}

// MoveToTrash drops the in-memory audio of a deleted take.
func (m *MemoryTakes) MoveToTrash(_, relativePath string) error {
	m.Forget(relativePath)
	return nil
}

// RecoverPartials reports nothing: browser takes are stored atomically.
func (m *MemoryTakes) RecoverPartials(string) ([]string, error) { return nil, nil }

type memoryAudio struct {
	channels   int
	sampleRate int
	samples    []float32
}

func (a *memoryAudio) frames() int { return len(a.samples) / a.channels }

type memoryReader struct {
	audio *memoryAudio
}

// Channels returns the interleave width.
func (r *memoryReader) Channels() int { return r.audio.channels }

// SampleRate returns the stored frame rate.
func (r *memoryReader) SampleRate() int { return r.audio.sampleRate }

// Frames returns how many frames the take holds.
func (r *memoryReader) Frames() int { return r.audio.frames() }

// ReadFrames fills dst with interleaved samples starting at atFrame.
func (r *memoryReader) ReadFrames(atFrame int, dst []float32) (int, error) {
	channels := r.audio.channels
	frames := r.audio.frames()
	if atFrame < 0 || atFrame >= frames || len(dst) < channels {
		return 0, nil
	}
	wanted := len(dst) / channels
	if remaining := frames - atFrame; wanted > remaining {
		wanted = remaining
	}
	copy(dst[:wanted*channels], r.audio.samples[atFrame*channels:(atFrame+wanted)*channels])
	return wanted, nil
}

// Close releases the reader; the samples stay available for other readers.
func (r *memoryReader) Close() error { return nil }
