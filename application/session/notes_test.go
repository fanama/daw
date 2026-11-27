package sessionapp_test

import (
	"testing"

	"fanamix/domain/session"
)

func TestAddTrackWithNotesFillsAnInstrumentTrack(t *testing.T) {
	service := newService(t)
	if err := service.Create("Extract", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	track, err := service.AddTrackWithNotes("Guitar chords", session.InstrumentPiano, []session.NoteSpec{
		{Pitch: 60, StartFrame: 0, LengthFrames: 4800, Velocity: 100},
		{Pitch: 64, StartFrame: 0, LengthFrames: 4800, Velocity: 90},
		{Pitch: 67, StartFrame: 4800, LengthFrames: 4800, Velocity: 90},
	})
	if err != nil {
		t.Fatalf("add track with notes: %v", err)
	}
	if !track.IsInstrument() || track.Instrument() != session.InstrumentPiano {
		t.Fatalf("expected a piano track, got %s named %s", track.Instrument(), track.Name())
	}
	notes := service.Current().State().Notes
	if len(notes) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(notes))
	}
	for _, note := range notes {
		if note.TrackID != track.ID() {
			t.Fatalf("note %d landed on track %s", note.Pitch, note.TrackID)
		}
	}
}

func TestAddTrackWithNotesRollsBackOnARefusedNote(t *testing.T) {
	service := newService(t)
	if err := service.Create("Extract", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	before := len(service.Current().Tracks())
	_, err := service.AddTrackWithNotes("Broken", session.InstrumentPiano, []session.NoteSpec{
		{Pitch: 60, StartFrame: 0, LengthFrames: 4800, Velocity: 100},
		{Pitch: 200, StartFrame: 0, LengthFrames: 4800, Velocity: 100},
	})
	if err == nil {
		t.Fatal("a note outside 0..127 must be refused")
	}
	if after := len(service.Current().Tracks()); after != before {
		t.Fatalf("expected %d tracks after the rollback, got %d", before, after)
	}
	if notes := service.Current().State().Notes; len(notes) != 0 {
		t.Fatalf("expected no notes, got %d", len(notes))
	}
}
