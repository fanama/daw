package session_test

import (
	"testing"
	"time"

	"gorecord/domain/session"
)

func TestAddMarkerClampsAndNames(t *testing.T) {
	current, err := session.New("Markers", 48000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	verse, err := current.AddMarker(-20, "  Verse  ")
	if err != nil {
		t.Fatalf("add marker: %v", err)
	}
	if verse.Frame() != 0 || verse.Name() != "Verse" {
		t.Fatalf("unexpected marker: %+v", verse)
	}
	auto, err := current.AddMarker(960, "   ")
	if err != nil {
		t.Fatalf("add automatic marker: %v", err)
	}
	if auto.Name() != "Marker 2" || auto.Frame() != 960 {
		t.Fatalf("unexpected automatic marker: %+v", auto)
	}
	if len(current.Markers()) != 2 {
		t.Fatalf("expected two markers, got %d", len(current.Markers()))
	}
}

func TestRemoveMarker(t *testing.T) {
	current, err := session.New("Markers", 48000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	marker, err := current.AddMarker(1200, "Chorus")
	if err != nil {
		t.Fatalf("add marker: %v", err)
	}
	removed, err := current.RemoveMarker(marker.ID())
	if err != nil || removed.Name() != "Chorus" {
		t.Fatalf("remove marker: %+v, %v", removed, err)
	}
	if len(current.Markers()) != 0 {
		t.Fatalf("expected no markers, got %d", len(current.Markers()))
	}
	if _, err := current.RemoveMarker(marker.ID()); err == nil {
		t.Fatal("removing a missing marker must fail")
	}
}
