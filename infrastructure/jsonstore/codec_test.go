package jsonstore_test

import (
	"testing"
	"time"

	"gorecord/domain/mix"
	"gorecord/domain/session"
	"gorecord/infrastructure/jsonstore"
)

func TestEncodeStateRoundTripsTheProject(t *testing.T) {
	current, err := session.New("Round trip", 48000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	track, err := current.AddTrack("Mic", 1)
	if err != nil {
		t.Fatalf("add track: %v", err)
	}
	take, err := current.AddTake(track.ID(), "takes/take-001.wav", 120, 1, 48000, false, false, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("add take: %v", err)
	}
	if err := current.SetPan(track.ID(), -0.4); err != nil {
		t.Fatalf("pan: %v", err)
	}
	current.SetClick(mix.Click{Enabled: true, BPM: 90, BeatsPerBar: 3, CountInBars: 1})
	if _, err := current.AddMarker(480, "Verse"); err != nil {
		t.Fatalf("add marker: %v", err)
	}
	if err := current.SetTakeGain(take.ID(), 3.5); err != nil {
		t.Fatalf("gain: %v", err)
	}
	if err := current.SetTakeTrim(take.ID(), session.Trim{StartFrame: 10, EndFrame: 20}); err != nil {
		t.Fatalf("trim: %v", err)
	}
	if err := current.CompleteTake(take.ID(), 2000); err != nil {
		t.Fatalf("complete take: %v", err)
	}
	encoded, err := jsonstore.EncodeState(current.State())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	restored, err := jsonstore.RestoreSession(encoded)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Name() != "Round trip" || restored.SampleRate() != 48000 {
		t.Fatalf("unexpected session: %q at %d Hz", restored.Name(), restored.SampleRate())
	}
	if restored.Click().BPM != 90 || !restored.Click().Enabled {
		t.Fatalf("unexpected click: %+v", restored.Click())
	}
	tracks := restored.Tracks()
	takes := restored.Takes()
	if len(tracks) != 1 || len(takes) != 1 {
		t.Fatalf("expected one track and one take, got %d/%d", len(tracks), len(takes))
	}
	if tracks[0].Pan() != -0.4 {
		t.Fatalf("unexpected pan: %v", tracks[0].Pan())
	}
	if takes[0].GainDB() != 3.5 || takes[0].Trim().StartFrame != 10 || takes[0].StartFrame() != 120 {
		t.Fatalf("unexpected take: %+v", takes[0])
	}
	markers := restored.Markers()
	if len(markers) != 1 || markers[0].Name() != "Verse" || markers[0].Frame() != 480 {
		t.Fatalf("unexpected markers: %+v", markers)
	}
}

func TestEncodeStateKeepsSplitSegments(t *testing.T) {
	current, err := session.New("Split segments", 48000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	track, err := current.AddTrack("Mic", 1)
	if err != nil {
		t.Fatalf("add track: %v", err)
	}
	take, err := current.AddTake(track.ID(), "takes/take-001.wav", 0, 1, 48000, false, false, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("add take: %v", err)
	}
	if err := current.CompleteTake(take.ID(), 2000); err != nil {
		t.Fatalf("complete take: %v", err)
	}
	if _, err := current.SplitTake(take.ID(), 800, time.Unix(0, 0)); err != nil {
		t.Fatalf("split: %v", err)
	}

	encoded, err := jsonstore.EncodeState(current.State())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	restored, err := jsonstore.RestoreSession(encoded)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	takes := restored.Takes()
	if len(takes) != 2 {
		t.Fatalf("expected two segments, got %d", len(takes))
	}
	if !takes[1].IsLayered() || takes[1].StartFrame() != 800 || takes[1].Trim().StartFrame != 800 {
		t.Fatalf("split segment did not round-trip: %+v", takes[1])
	}
}
