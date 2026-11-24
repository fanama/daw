package sessionapp_test

import (
	"math"
	"testing"

	"fanamix/domain/session"
	"fanamix/infrastructure/wavio"
)

func TestNormalizeTakeBoostsToTargetPeak(t *testing.T) {
	service := newService(t)
	if err := service.Create("Normalize idea", 48000); err != nil {
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
	writePeakTake(t, service.Root(), 1000, 0.25)
	if err := service.FinishTake(take.ID(), 1000, false); err != nil {
		t.Fatalf("finish take: %v", err)
	}

	gain, err := service.NormalizeTake(take.ID())
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	want := session.NormalizeTargetDB - 20*math.Log10(0.25)
	if math.Abs(gain-want) > 1e-6 {
		t.Fatalf("expected %f dB, got %f", want, gain)
	}
	stored, ok := service.Current().FindTake(take.ID())
	if !ok || math.Abs(stored.GainDB()-want) > 1e-6 {
		t.Fatalf("normalized gain was not stored: %+v", stored)
	}
}

func TestNormalizeTakeRefusesSilence(t *testing.T) {
	service := newService(t)
	if err := service.Create("Silent idea", 48000); err != nil {
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
	writePeakTake(t, service.Root(), 480, 0)
	if err := service.FinishTake(take.ID(), 480, false); err != nil {
		t.Fatalf("finish take: %v", err)
	}
	if _, err := service.NormalizeTake(take.ID()); err == nil {
		t.Fatal("a silent take must not normalize")
	}
}

func writePeakTake(t *testing.T, root string, frames int, value float32) {
	t.Helper()
	writer, _, err := wavio.NewTakeFiles().CreateTake(root, 1, 1, 48000)
	if err != nil {
		t.Fatalf("create take: %v", err)
	}
	samples := make([]float32, frames)
	for index := range samples {
		samples[index] = value
	}
	if _, err := writer.Write(samples); err != nil {
		t.Fatalf("write take: %v", err)
	}
	if err := writer.Finish(true); err != nil {
		t.Fatalf("finish take: %v", err)
	}
}
