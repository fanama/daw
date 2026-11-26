package jsonstore_test

import (
	"testing"
	"time"

	"fanamix/domain/session"
	"fanamix/infrastructure/jsonstore"
)

func TestNotesRoundTripWithTheInstrumentSource(t *testing.T) {
	current, err := session.New("Note idea", 48000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	track, err := current.AddTrackWithSource("Keys", session.InstrumentSource(session.InstrumentGuitar))
	if err != nil {
		t.Fatalf("add instrument track: %v", err)
	}
	if _, err := current.AddNote(track.ID(), 67, 960, 480, 88); err != nil {
		t.Fatalf("add note: %v", err)
	}
	if err := current.SetRepeat(track.ID(), 3); err != nil {
		t.Fatalf("loop the pattern: %v", err)
	}
	if err := current.SetPatternFrames(track.ID(), 1920); err != nil {
		t.Fatalf("widen the pattern: %v", err)
	}

	encoded, err := jsonstore.EncodeState(current.State())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	restored, err := jsonstore.RestoreSession(encoded)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	tracks := restored.Tracks()
	if len(tracks) != 1 || !tracks[0].IsInstrument() || tracks[0].Instrument() != session.InstrumentGuitar {
		t.Fatalf("instrument source did not round-trip: %+v", tracks)
	}
	if tracks[0].Repeat() != 3 {
		t.Fatalf("repeat count did not round-trip: %d", tracks[0].Repeat())
	}
	if tracks[0].PatternFrames() != 1920 {
		t.Fatalf("pattern length did not round-trip: %d", tracks[0].PatternFrames())
	}
	notes := restored.Notes()
	if len(notes) != 1 {
		t.Fatalf("expected one note, got %d", len(notes))
	}
	note := notes[0]
	if note.Pitch() != 67 || note.StartFrame() != 960 || note.LengthFrames() != 480 || note.Velocity() != 88 {
		t.Fatalf("note did not round-trip: %+v", note)
	}
	if next, err := restored.AddNote(track.ID(), 60, 0, 240, 100); err != nil || next.ID() == note.ID() {
		t.Fatalf("restored session must continue note numbering: %v / %s", err, next.ID())
	}
}
