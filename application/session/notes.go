package sessionapp

import "fanamix/domain/session"

// AddNote appends a timed note to an instrument track.
func (s *Service) AddNote(trackID session.TrackID, pitch, startFrame, lengthFrames, velocity int) (session.Note, error) {
	if err := s.requireSession(); err != nil {
		return session.Note{}, err
	}
	note, err := s.current.AddNote(trackID, pitch, startFrame, lengthFrames, velocity)
	if err != nil {
		return session.Note{}, err
	}
	return note, s.Save()
}

// RemoveNote deletes one note.
func (s *Service) RemoveNote(id session.NoteID) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.RemoveNote(id); err != nil {
		return err
	}
	return s.Save()
}

// MoveNote changes a note's pitch and start position.
func (s *Service) MoveNote(id session.NoteID, pitch, startFrame int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.MoveNote(id, pitch, startFrame); err != nil {
		return err
	}
	return s.Save()
}

// ResizeNote changes how long a note is held.
func (s *Service) ResizeNote(id session.NoteID, lengthFrames int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.ResizeNote(id, lengthFrames); err != nil {
		return err
	}
	return s.Save()
}

// SetInstrument changes the built-in sound of an instrument track.
func (s *Service) SetInstrument(id session.TrackID, instrument session.Instrument) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetInstrument(id, instrument); err != nil {
		return err
	}
	return s.Save()
}
