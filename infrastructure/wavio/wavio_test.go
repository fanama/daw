package wavio_test

import (
	"os"
	"path/filepath"
	"testing"

	"gorecord/infrastructure/wavio"
)

func TestWriterReaderRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		channels int
		rate     int
	}{
		{"mono 48k", 1, 48000},
		{"stereo 44k1", 2, 44100},
		{"quad 48k", 4, 48000},
		{"octo 96k", 8, 96000},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "take.wav")
			writer, err := wavio.NewWriter(path, testCase.channels, testCase.rate)
			if err != nil {
				t.Fatalf("new writer: %v", err)
			}
			frames := 512
			samples := make([]float32, frames*testCase.channels)
			for index := range samples {
				samples[index] = float32(index%97) / 97
			}
			if written, writeErr := writer.Write(samples); writeErr != nil || written != frames {
				t.Fatalf("write: %d frames, err %v", written, writeErr)
			}
			if err := writer.Finish(); err != nil {
				t.Fatalf("finish: %v", err)
			}

			reader, err := wavio.OpenReader(path)
			if err != nil {
				t.Fatalf("open reader: %v", err)
			}
			defer func() { _ = reader.Close() }()
			if reader.Channels() != testCase.channels || reader.SampleRate() != testCase.rate {
				t.Fatalf("header mismatch: %d ch %d Hz", reader.Channels(), reader.SampleRate())
			}
			if reader.Frames() != frames {
				t.Fatalf("expected %d frames, got %d", frames, reader.Frames())
			}
			read := make([]float32, frames*testCase.channels)
			if _, readErr := reader.ReadFrames(0, read); readErr != nil {
				t.Fatalf("read frames: %v", readErr)
			}
			for index := range read {
				if read[index] != samples[index] {
					t.Fatalf("sample %d: expected %v got %v", index, samples[index], read[index])
				}
			}
		})
	}
}

func TestMultichannelHeaderUsesExtensibleFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quad.wav")
	writer, err := wavio.NewWriter(path, 4, 48000)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	if err := writer.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(raw[8:12]) != "WAVE" {
		t.Fatal("missing WAVE signature")
	}
	if got := string(raw[12:16]); got != "fmt " {
		t.Fatalf("expected fmt chunk, got %q", got)
	}
	if raw[20] != 0xFE || raw[21] != 0xFF {
		t.Fatalf("expected WAVE_FORMAT_EXTENSIBLE, got %x%x", raw[21], raw[20])
	}
}

func TestWriterIgnoresEmptyBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.wav")
	writer, err := wavio.NewWriter(path, 2, 48000)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	written, err := writer.Write(nil)
	if err != nil || written != 0 {
		t.Fatalf("expected a no-op write, got %d and %v", written, err)
	}
	if err := writer.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
}

func TestFinalizePartialRepairsTruncatedData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash.wav")
	writer, err := wavio.NewWriter(path, 2, 48000)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	if _, err := writer.Write(make([]float32, 1000)); err != nil {
		t.Fatalf("write: %v", err)
	}
	frames, err := wavio.FinalizePartial(path)
	if err != nil {
		t.Fatalf("finalize partial: %v", err)
	}
	if frames != 500 {
		t.Fatalf("expected 500 frames, got %d", frames)
	}
	reader, err := wavio.OpenReader(path)
	if err != nil {
		t.Fatalf("open repaired: %v", err)
	}
	defer func() { _ = reader.Close() }()
	if reader.Frames() != 500 {
		t.Fatalf("expected repaired header to report 500 frames, got %d", reader.Frames())
	}
}
