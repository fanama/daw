package session_test

import (
	"testing"

	"fanamix/domain/session"
)

func TestInstrumentTrackHoldsNotesAndCannotBeArmed(t *testing.T) {
	s := newSession(t)
	track, err := s.AddTrackWithSource("Keys", session.InstrumentSource(session.InstrumentPiano))
	if err != nil {
		t.Fatalf("add instrument track: %v", err)
	}
	if !track.IsInstrument() || track.Instrument() != session.InstrumentPiano {
		t.Fatalf("unexpected instrument track: %+v", track)
	}
	if err := s.SetArm(track.ID(), true); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("instrument track must not be armed, got %v", err)
	}

	note, err := s.AddNote(track.ID(), 60, 4800, 2400, 0)
	if err != nil {
		t.Fatalf("add note: %v", err)
	}
	if note.Velocity() != session.DefaultNoteVelocity || note.EndFrame() != 7200 {
		t.Fatalf("unexpected note: %+v", note)
	}
	if len(s.TrackNotes(track.ID())) != 1 {
		t.Fatalf("expected one note, got %d", len(s.TrackNotes(track.ID())))
	}

	if err := s.SetInstrument(track.ID(), session.InstrumentBass); err != nil {
		t.Fatalf("change instrument: %v", err)
	}
	updated, ok := s.FindTrack(track.ID())
	if !ok || updated.Instrument() != session.InstrumentBass {
		t.Fatalf("instrument change was not kept: %+v", updated)
	}
	if err := s.SetInstrument(track.ID(), session.InstrumentDrums); err != nil {
		t.Fatalf("drums are a valid instrument: %v", err)
	}
	if err := s.SetInstrument(track.ID(), "sitar"); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("unknown instrument must be refused, got %v", err)
	}

	if err := s.MoveNote(note.ID(), 64, 9600); err != nil {
		t.Fatalf("move note: %v", err)
	}
	moved := s.TrackNotes(track.ID())[0]
	if moved.Pitch() != 64 || moved.StartFrame() != 9600 {
		t.Fatalf("note did not move: %+v", moved)
	}

	if err := s.ResizeNote(note.ID(), 12000); err != nil {
		t.Fatalf("resize note: %v", err)
	}
	resized := s.TrackNotes(track.ID())[0]
	if resized.LengthFrames() != 12000 || resized.EndFrame() != 21600 {
		t.Fatalf("note did not resize: %+v", resized)
	}
	if err := s.ResizeNote(note.ID(), 0); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("an empty note must be refused, got %v", err)
	}

	if _, err := s.RemoveTrack(track.ID()); err != nil {
		t.Fatalf("remove track: %v", err)
	}
	if len(s.Notes()) != 0 {
		t.Fatalf("removing a track must drop its notes, got %d", len(s.Notes()))
	}
}

func TestNoteCommandsRejectAVoiceTrackAndBadPitch(t *testing.T) {
	s := newSession(t)
	audio := addTrack(t, s, "Mic", 0)
	if _, err := s.AddNote(audio.ID(), 60, 0, 120, 100); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("notes belong to instrument tracks only, got %v", err)
	}

	keys, err := s.AddTrackWithSource("Keys", session.InstrumentSource(session.InstrumentGuitar))
	if err != nil {
		t.Fatalf("add instrument track: %v", err)
	}
	if _, err := s.AddNote(keys.ID(), 200, 0, 120, 100); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("pitch 200 must be refused, got %v", err)
	}
	note, err := s.AddNote(keys.ID(), 40, 0, 120, 400)
	if err != nil {
		t.Fatalf("add note: %v", err)
	}
	if note.Velocity() != session.MaxNoteVelocity {
		t.Fatalf("velocity was not clamped: %d", note.Velocity())
	}
	if err := s.RemoveNote("note_404"); !session.IsCode(err, session.CodeNotFound) {
		t.Fatalf("unknown note must not be found, got %v", err)
	}
}
