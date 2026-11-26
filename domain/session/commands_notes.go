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

// ClearNotes deletes a whole motif: every note of an instrument track together
// with its repetitions and its widened length, so the region leaves the
// timeline in one edit that undo restores at once.
func (s *Session) ClearNotes(trackID TrackID) error {
	index, err := s.trackIndex(trackID)
	if err != nil {
		return err
	}
	if !s.tracks[index].source.IsInstrument() {
		return invalid("track %q is not an instrument track", s.tracks[index].name)
	}
	if len(s.TrackNotes(trackID)) == 0 {
		return notFound("track %q has no motif", s.tracks[index].name)
	}
	kept := make([]Note, 0, len(s.notes))
	for _, note := range s.notes {
		if note.trackID == trackID {
			continue
		}
		kept = append(kept, note)
	}
	s.notes = kept
	s.tracks[index].repeat = 0
	s.tracks[index].patternFrames = 0
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

// noteSpanFrames is the length from a track's first note to its last end.
func (s *Session) noteSpanFrames(trackID TrackID) int {
	start, end := -1, 0
	for _, note := range s.notes {
		if note.trackID != trackID {
			continue
		}
		if start < 0 || note.startFrame < start {
			start = note.startFrame
		}
		if note.EndFrame() > end {
			end = note.EndFrame()
		}
	}
	if start < 0 {
		return 0
	}
	return end - start
}

func (s *Session) noteIndex(id NoteID) (int, error) {
	for index := range s.notes {
		if s.notes[index].id == id {
			return index, nil
		}
	}
	return -1, notFound("note %s does not exist", id)
}
