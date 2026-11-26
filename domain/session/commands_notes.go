package session

import "fmt"

// AddNote appends a note to an instrument track.
func (s *Session) AddNote(trackID TrackID, pitch, startFrame, lengthFrames, velocity int) (Note, error) {
	index, err := s.trackIndex(trackID)
	if err != nil {
		return Note{}, err
	}
	if !s.tracks[index].source.IsInstrument() {
		return Note{}, invalid("track %q is not an instrument track", s.tracks[index].name)
	}
	note, err := newNote(NoteID(fmt.Sprintf("note_%d", s.nextNote)), trackID, pitch, startFrame, lengthFrames, velocity)
	if err != nil {
		return Note{}, err
	}
	s.nextNote++
	s.notes = append(s.notes, note)
	return note, nil
}

// RemoveNote deletes one note.
func (s *Session) RemoveNote(id NoteID) error {
	index, err := s.noteIndex(id)
	if err != nil {
		return err
	}
	s.notes = append(s.notes[:index], s.notes[index+1:]...)
	return nil
}

// MoveNote changes a note's pitch and start position.
func (s *Session) MoveNote(id NoteID, pitch, startFrame int) error {
	index, err := s.noteIndex(id)
	if err != nil {
		return err
	}
	if pitch < MinNotePitch || pitch > MaxNotePitch {
		return invalid("note pitch %d is outside 0..127", pitch)
	}
	if startFrame < 0 {
		return invalid("note start must not be negative")
	}
	s.notes[index].pitch = pitch
	s.notes[index].startFrame = startFrame
	return nil
}

// ResizeNote changes how long a note is held.
func (s *Session) ResizeNote(id NoteID, lengthFrames int) error {
	index, err := s.noteIndex(id)
	if err != nil {
		return err
	}
	if lengthFrames < 1 {
		return invalid("note length must be positive")
	}
	s.notes[index].lengthFrames = lengthFrames
	return nil
}

func (s *Session) noteIndex(id NoteID) (int, error) {
	for index := range s.notes {
		if s.notes[index].id == id {
			return index, nil
		}
	}
	return -1, notFound("note %s does not exist", id)
}
