package wavio_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fanamix/application/recording"
	"fanamix/domain/session"
	"fanamix/infrastructure/wavio"
)

func TestMemoryTakesReadsRegisteredFrames(t *testing.T) {
	takes := wavio.NewMemoryTakes()
	if err := takes.Register("takes/take-001.wav", 1, 48000, []float32{0.1, 0.2, 0.3, 0.4}); err != nil {
		t.Fatalf("register: %v", err)
	}
	audio, err := takes.OpenTake("", "takes/take-001.wav")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if audio.Channels() != 1 || audio.Frames() != 4 || audio.SampleRate() != 48000 {
		t.Fatalf("unexpected audio: %d ch · %d frames · %d Hz", audio.Channels(), audio.Frames(), audio.SampleRate())
	}
	buffer := make([]float32, 2)
	read, err := audio.ReadFrames(1, buffer)
	if err != nil || read != 2 {
		t.Fatalf("read: %d frames, %v", read, err)
	}
	if buffer[0] != 0.2 || buffer[1] != 0.3 {
		t.Fatalf("unexpected samples: %v", buffer)
	}
	takes.Forget("takes/take-001.wav")
	if takes.Has("takes/take-001.wav") {
		t.Fatal("forget must drop the take")
	}
	if _, err := takes.OpenTake("", "takes/take-001.wav"); err == nil {
		t.Fatal("opening a dropped take must fail")
	}
}

func TestMemoryTakesRejectsUnalignedSamples(t *testing.T) {
	takes := wavio.NewMemoryTakes()
	if err := takes.Register("takes/take-001.wav", 2, 48000, []float32{0.1, 0.2, 0.3}); err == nil {
		t.Fatal("register must reject a partial frame")
	}
}

func TestMemoryMixdownsExportFloatWav(t *testing.T) {
	takes := wavio.NewMemoryTakes()
	if err := takes.Register("takes/take-001.wav", 1, 48000, []float32{0.25, -0.5, 0.75}); err != nil {
		t.Fatalf("register: %v", err)
	}
	current, err := session.New("Mix idea", 48000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	track, err := current.AddTrack("Guitar", 0)
	if err != nil {
		t.Fatalf("add track: %v", err)
	}
	take, err := current.AddTake(track.ID(), "takes/take-001.wav", 0, 1, 48000, false, false, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("add take: %v", err)
	}
	if err := current.CompleteTake(take.ID(), 3); err != nil {
		t.Fatalf("complete take: %v", err)
	}
	files := wavio.NewMemoryMixdowns()
	exporter := recording.NewMixdownExporter(takes, files)
	result, err := exporter.Export("", "mixdown.wav", current.AudibleTakes(), current.SampleRate())
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.Frames != 3 || result.Channels != 2 {
		t.Fatalf("unexpected export: %+v", result)
	}
	data, err := files.WAV()
	if err != nil {
		t.Fatalf("encode wav: %v", err)
	}
	path := filepath.Join(t.TempDir(), "mixdown.wav")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	reader, err := wavio.OpenReader(path)
	if err != nil {
		t.Fatalf("open wav: %v", err)
	}
	defer func() { _ = reader.Close() }()
	if reader.Channels() != 2 || reader.Frames() != 3 || reader.SampleRate() != 48000 {
		t.Fatalf("unexpected header: %d ch · %d frames", reader.Channels(), reader.Frames())
	}
	read := make([]float32, 2)
	if _, err := reader.ReadFrames(0, read); err != nil {
		t.Fatalf("read wav: %v", err)
	}
	want := float32(math.Tanh(0.25))
	if math.Abs(float64(read[0]-want)) > 1e-6 || math.Abs(float64(read[1]-want)) > 1e-6 {
		t.Fatalf("expected centered %f, got %f/%f", want, read[0], read[1])
	}
}
