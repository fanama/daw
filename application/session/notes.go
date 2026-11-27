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

// AddNotes appends several notes in one edit, so pasting a copied selection is
// a single undo step.
func (s *Service) AddNotes(trackID session.TrackID, specs []session.NoteSpec) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if _, err := s.current.AddNotes(trackID, specs); err != nil {
		return err
	}
	return s.Save()
}

// RemoveNotes deletes several notes in one edit.
func (s *Service) RemoveNotes(ids []session.NoteID) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.RemoveNotes(ids); err != nil {
		return err
	}
	return s.Save()
}

// ShiftNotes moves several notes together by the same offset.
func (s *Service) ShiftNotes(ids []session.NoteID, pitchDelta, frameDelta int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.ShiftNotes(ids, pitchDelta, frameDelta); err != nil {
		return err
	}
	return s.Save()
}

// AddTrackWithNotes creates an instrument track and fills it in one edit, so an
// extracted MIDI part is a single undo step.
func (s *Service) AddTrackWithNotes(name string, instrument session.Instrument, specs []session.NoteSpec) (session.Track, error) {
	if err := s.requireSession(); err != nil {
		return session.Track{}, err
	}
	track, err := s.current.AddTrackWithSource(name, session.InstrumentSource(instrument))
	if err != nil {
		return session.Track{}, err
	}
	if _, err := s.current.AddNotes(track.ID(), specs); err != nil {
		// The notes are refused as a whole, so drop the empty track rather than
		// leaving a half-applied edit behind.
		_, _ = s.current.RemoveTrack(track.ID())
		return session.Track{}, err
	}
	return track, s.Save()
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

// ClearNotes deletes a whole motif, including its repetitions and widening.
func (s *Service) ClearNotes(trackID session.TrackID) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.ClearNotes(trackID); err != nil {
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
