package wavio

import (
	"encoding/binary"
	"errors"
	"math"
	"os"

	"fanamix/domain/session"
)

// ErrCapacityReached is returned when a take hits the RIFF size ceiling.
var ErrCapacityReached = errors.New("take reached the 4 GB WAV ceiling")

// Writer appends interleaved float32 frames to a WAV file, rewriting the
// header at checkpoints so an abnormal exit still leaves usable audio.
type Writer struct {
	file       *os.File
	channels   int
	sampleRate int
	frames     int
}

// NewWriter creates the file and reserves a zero-length data chunk.
func NewWriter(path string, channels, sampleRate int) (*Writer, error) {
	if channels < 1 || channels > 64 {
		return nil, errors.New("wavio: channel count must be between 1 and 64")
	}
	if sampleRate < 8000 || sampleRate > 384000 {
		return nil, errors.New("wavio: unsupported sample rate")
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	writer := &Writer{file: file, channels: channels, sampleRate: sampleRate}
	if err := writer.writeHeader(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return writer, nil
}

// Channels returns the interleave width.
func (w *Writer) Channels() int { return w.channels }

// SampleRate returns the file frame rate.
func (w *Writer) SampleRate() int { return w.sampleRate }

// Frames returns how many frames have been written.
func (w *Writer) Frames() int { return w.frames }

// Write appends interleaved samples and returns the frames consumed.
func (w *Writer) Write(samples []float32) (int, error) {
	frames := len(samples) / w.channels
	if frames == 0 {
		return 0, nil
	}
	limit := session.TakeFrameLimit(w.channels)
	if remaining := limit - w.frames; frames > remaining {
		frames = remaining
	}
	if frames <= 0 {
		return 0, ErrCapacityReached
	}
	buffer := make([]byte, frames*w.channels*bytesPerSample)
	for index := 0; index < frames*w.channels; index++ {
		binary.LittleEndian.PutUint32(buffer[index*4:], math.Float32bits(samples[index]))
	}
	if _, err := w.file.Write(buffer); err != nil {
		return 0, err
	}
	w.frames += frames
	return frames, nil
}

// Checkpoint rewrites the header with the current length and flushes to disk.
func (w *Writer) Checkpoint() error {
	if err := w.writeHeader(); err != nil {
		return err
	}
	return w.file.Sync()
}

// Finish writes the final header and closes the file.
func (w *Writer) Finish() error {
	if err := w.writeHeader(); err != nil {
		_ = w.file.Close()
		return err
	}
	if err := w.file.Sync(); err != nil {
		_ = w.file.Close()
		return err
	}
	return w.file.Close()
}

func (w *Writer) writeHeader() error {
	header := buildHeader(w.channels, w.sampleRate, w.frames)
	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}
	if _, err := w.file.Write(header); err != nil {
		return err
	}
	_, err := w.file.Seek(0, 2)
	return err
}
