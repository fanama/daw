package sessionapp_test

import (
	"testing"
)

func TestBeginTrackTakesCreatesOneTakePerArmedInput(t *testing.T) {
	service := newService(t)
	if err := service.Create("Two inputs", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	guitar := service.Current().Tracks()[0]
	voice, err := service.AddTrack("Voice", 1)
	if err != nil {
		t.Fatalf("add track: %v", err)
	}
	if err := service.SetArm(guitar.ID(), true); err != nil {
		t.Fatalf("arm guitar: %v", err)
	}
	if err := service.SetArm(voice.ID(), true); err != nil {
		t.Fatalf("arm voice: %v", err)
	}

	takes, plan, err := service.BeginTrackTakes(1200)
	if err != nil {
		t.Fatalf("begin takes: %v", err)
	}
	if len(takes) != 2 || len(plan) != 2 {
		t.Fatalf("expected two takes and slots, got %d / %d", len(takes), len(plan))
	}
	if takes[0].TrackID() != guitar.ID() || takes[1].TrackID() != voice.ID() {
		t.Fatalf("takes landed on the wrong tracks: %s / %s", takes[0].TrackID(), takes[1].TrackID())
	}
	if takes[0].File() != "takes/take-001.wav" || takes[1].File() != "takes/take-002.wav" {
		t.Fatalf("unexpected take files: %s / %s", takes[0].File(), takes[1].File())
	}
	for _, take := range takes {
		if take.Channels() != 1 {
			t.Fatalf("input take must be mono, got %d channels", take.Channels())
		}
		if err := service.FinishTake(take.ID(), 9600, false); err != nil {
			t.Fatalf("finish take: %v", err)
		}
	}

	root := service.Root()
	service.Close()
	reopened := newService(t)
	if err := reopened.Open(root); err != nil {
		t.Fatalf("open: %v", err)
	}
	loaded := reopened.Current()
	if got := len(loaded.Takes()); got != 2 {
		t.Fatalf("expected two persisted takes, got %d", got)
	}
	for _, track := range loaded.Tracks() {
		if !track.IsArmed() {
			t.Fatalf("track %s lost its arming", track.ID())
		}
		if track.ActiveTakeID().IsZero() {
			t.Fatalf("track %s has no active take", track.ID())
		}
	}
}
