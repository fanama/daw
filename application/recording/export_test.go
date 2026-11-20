package recording_test

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"gorecord/application/recording"
	sessionapp "gorecord/application/session"
	"gorecord/domain/session"
	"gorecord/infrastructure/jsonstore"
	"gorecord/infrastructure/wavio"
)

func newExportSession(t *testing.T, frames int) (*sessionapp.Service, *session.Session, session.Take) {
	t.Helper()
	root := t.TempDir()
	takeFiles := wavio.NewTakeFiles()
	service := sessionapp.NewService(jsonstore.NewStore(root), takeFiles, func() time.Time { return time.Unix(0, 0) })
	if err := service.Create("Export idea", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	track := service.Current().Tracks()[0]
	if err := service.SetArm(track.ID(), true); err != nil {
		t.Fatalf("arm: %v", err)
	}
	take, _, err := service.BeginTake(0)
	if err != nil {
		t.Fatalf("begin take: %v", err)
	}
	writeTake(t, service.Root(), "takes/take-001.wav", frames)
	if err := service.FinishTake(take.ID(), frames, false); err != nil {
		t.Fatalf("finish take: %v", err)
	}
	return service, service.Current(), take
}

func TestExportMixdownWritesStereoFromAudibleTakes(t *testing.T) {
	service, current, _ := newExportSession(t, 1000)
	path := filepath.Join(t.TempDir(), "mix.wav")
	exporter := recording.NewMixdownExporter(wavio.NewTakeFiles(), wavio.NewMixdowns())
	result, err := exporter.Export(service.Root(), path, current.AudibleTakes(), current.SampleRate())
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.Frames != 1000 || result.Channels != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	reader, err := wavio.OpenReader(path)
	if err != nil {
		t.Fatalf("open export: %v", err)
	}
	defer func() { _ = reader.Close() }()
	if reader.Channels() != 2 || reader.Frames() != 1000 || reader.SampleRate() != 48000 {
		t.Fatalf("unexpected export header: %d ch · %d frames · %d Hz", reader.Channels(), reader.Frames(), reader.SampleRate())
	}
	read := make([]float32, 2)
	if _, err := reader.ReadFrames(100, read); err != nil {
		t.Fatalf("read export: %v", err)
	}
	want := float32(math.Tanh(float64(sampleValue(100))))
	if math.Abs(float64(read[0]-want)) > 1e-6 || math.Abs(float64(read[1]-want)) > 1e-6 {
		t.Fatalf("expected centered mono %f, got %f/%f", want, read[0], read[1])
	}
}

func TestExportMixdownAppliesTrim(t *testing.T) {
	service, current, take := newExportSession(t, 2000)
	if err := service.SetTakeTrim(take.ID(), session.Trim{StartFrame: 200, EndFrame: 600}); err != nil {
		t.Fatalf("trim: %v", err)
	}
	path := filepath.Join(t.TempDir(), "trimmed.wav")
	exporter := recording.NewMixdownExporter(wavio.NewTakeFiles(), wavio.NewMixdowns())
	if _, err := exporter.Export(service.Root(), path, current.AudibleTakes(), current.SampleRate()); err != nil {
		t.Fatalf("export: %v", err)
	}
	reader, err := wavio.OpenReader(path)
	if err != nil {
		t.Fatalf("open export: %v", err)
	}
	defer func() { _ = reader.Close() }()
	if reader.Frames() != 400 {
		t.Fatalf("expected 400 trimmed frames, got %d", reader.Frames())
	}
	read := make([]float32, 2)
	if _, err := reader.ReadFrames(0, read); err != nil {
		t.Fatalf("read export: %v", err)
	}
	want := float32(math.Tanh(float64(sampleValue(200))))
	if math.Abs(float64(read[0]-want)) > 1e-6 {
		t.Fatalf("expected the trimmed start %f, got %f", want, read[0])
	}
}

func TestExportMixdownRefusesEmptySelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.wav")
	exporter := recording.NewMixdownExporter(wavio.NewTakeFiles(), wavio.NewMixdowns())
	if _, err := exporter.Export(t.TempDir(), path, nil, 48000); err == nil {
		t.Fatal("export must refuse an empty selection")
	}
}
