package recording_test

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"fanamix/application/recording"
	sessionapp "fanamix/application/session"
	"fanamix/domain/mix"
	"fanamix/domain/session"
	"fanamix/infrastructure/jsonstore"
	"fanamix/infrastructure/wavio"
)

type boostFactory struct{}

type boostProcessor struct{}

func (boostFactory) NewProcessor() mix.Processor { return boostProcessor{} }

func (boostProcessor) Prepare(int) {}

func (boostProcessor) SetEffects(mix.Effects) {}

func (boostProcessor) ProcessMono(samples []float32) {
	for index := range samples {
		samples[index] += 0.25
	}
}

func (boostProcessor) GainReductionDB() float32 { return 0 }

func (boostProcessor) Reset() {}

func TestLoadPlacesTakesOnTheTimeline(t *testing.T) {
	root := t.TempDir()
	takeFiles := wavio.NewTakeFiles()
	store := jsonstore.NewStore(root)
	service := sessionapp.NewService(store, takeFiles, func() time.Time { return time.Unix(0, 0) })
	if err := service.Create("Playback idea", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	track := service.Current().Tracks()[0]
	if err := service.SetArm(track.ID(), true); err != nil {
		t.Fatalf("arm: %v", err)
	}
	take, _, err := service.BeginTake(24000)
	if err != nil {
		t.Fatalf("begin take: %v", err)
	}
	writeTake(t, service.Root(), "takes/take-001.wav", 1000)
	if err := service.FinishTake(take.ID(), 1000, false); err != nil {
		t.Fatalf("finish take: %v", err)
	}

	loader := recording.NewLoader(takeFiles)
	mix, err := loader.Load(service.Root(), service.AudibleTakes())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(mix.Takes) != 1 {
		t.Fatalf("expected one take, got %d", len(mix.Takes))
	}
	loaded := mix.Takes[0]
	if loaded.StartFrame != 24000 || loaded.Length != 1000 || loaded.Channels != 1 {
		t.Fatalf("unexpected placement: %+v", loaded)
	}
	if len(loaded.Samples) != 1000 {
		t.Fatalf("expected 1000 monophonic frames, got %d samples", len(loaded.Samples))
	}
}

func TestLoadHonoursTrimWithoutTouchingTheFile(t *testing.T) {
	root := t.TempDir()
	takeFiles := wavio.NewTakeFiles()
	store := jsonstore.NewStore(root)
	service := sessionapp.NewService(store, takeFiles, func() time.Time { return time.Unix(0, 0) })
	if err := service.Create("Trim idea", 48000); err != nil {
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
	writeTake(t, service.Root(), "takes/take-001.wav", 2000)
	if err := service.FinishTake(take.ID(), 2000, false); err != nil {
		t.Fatalf("finish take: %v", err)
	}
	if err := service.SetTakeTrim(take.ID(), session.Trim{StartFrame: 500, EndFrame: 1500}); err != nil {
		t.Fatalf("set trim: %v", err)
	}

	loader := recording.NewLoader(takeFiles)
	mix, err := loader.Load(service.Root(), service.AudibleTakes())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mix.Takes[0].Length != 1000 {
		t.Fatalf("expected 1000 audible frames after trim, got %d", mix.Takes[0].Length)
	}
	if got := mix.Takes[0].Samples[0]; got != sampleValue(500) {
		t.Fatalf("expected playback to start at the trim point, got %v", got)
	}
}

func TestLoaderAppliesTrackEffects(t *testing.T) {
	root := t.TempDir()
	takeFiles := wavio.NewTakeFiles()
	service := sessionapp.NewService(jsonstore.NewStore(root), takeFiles, func() time.Time { return time.Unix(0, 0) })
	if err := service.Create("Effects idea", 48000); err != nil {
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
	writeTake(t, service.Root(), "takes/take-001.wav", 32)
	if err := service.FinishTake(take.ID(), 32, false); err != nil {
		t.Fatalf("finish take: %v", err)
	}

	loader := recording.NewLoaderWithProcessor(takeFiles, boostFactory{})
	wet, err := loader.Load(service.Root(), service.AudibleTakes())
	if err != nil {
		t.Fatalf("load wet: %v", err)
	}
	want := sampleValue(0) + 0.25
	if got := wet.Takes[0].Samples[0]; math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("expected the effect chain to alter the take, got %v want %v", got, want)
	}

	if err := service.SetEffects(track.ID(), mix.DryEffects()); err != nil {
		t.Fatalf("dry effects: %v", err)
	}
	dry, err := loader.Load(service.Root(), service.AudibleTakes())
	if err != nil {
		t.Fatalf("load dry: %v", err)
	}
	if got := dry.Takes[0].Samples[0]; got != sampleValue(0) {
		t.Fatalf("bypassed chain changed the take: %v want %v", got, sampleValue(0))
	}
}

func writeTake(t *testing.T, root, relative string, frames int) {
	t.Helper()
	writer, _, err := wavio.NewTakeFiles().CreateTake(root, 1, 1, 48000)
	if err != nil {
		t.Fatalf("create take: %v", err)
	}
	samples := make([]float32, frames)
	for index := range samples {
		samples[index] = sampleValue(index)
	}
	if _, err := writer.Write(samples); err != nil {
		t.Fatalf("write take: %v", err)
	}
	if err := writer.Finish(true); err != nil {
		t.Fatalf("finish take: %v", err)
	}
	_ = relative
	_ = filepath.Base(relative)
}

func sampleValue(index int) float32 {
	return float32(index%1000) / 1000
}
