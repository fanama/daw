package sessionapp

import "gorecord/domain/session"

// SplitTake cuts one take in two at a session frame.
func (s *Service) SplitTake(id session.TakeID, atFrame int) (session.Take, error) {
	if err := s.requireSession(); err != nil {
		return session.Take{}, err
	}
	second, err := s.current.SplitTake(id, atFrame, s.now())
	if err != nil {
		return session.Take{}, err
	}
	return second, s.Save()
}

// MoveTake slides one take to a new session position.
func (s *Service) MoveTake(id session.TakeID, startFrame int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.MoveTake(id, startFrame); err != nil {
		return err
	}
	return s.Save()
}

// DuplicateTake copies a take onto a track at a new position.
func (s *Service) DuplicateTake(id session.TakeID, trackID session.TrackID, startFrame int) (session.Take, error) {
	if err := s.requireSession(); err != nil {
		return session.Take{}, err
	}
	copied, err := s.current.DuplicateTake(id, trackID, startFrame, s.now())
	if err != nil {
		return session.Take{}, err
	}
	return copied, s.Save()
}
