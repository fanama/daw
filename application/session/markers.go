package sessionapp

import (
	"errors"

	"gorecord/domain/session"
)

// AddMarker stores a named marker in the open session.
func (s *Service) AddMarker(frame int, name string) (session.Marker, error) {
	if s.current == nil {
		return session.Marker{}, errors.New("no session is open")
	}
	marker, err := s.current.AddMarker(frame, name)
	if err != nil {
		return session.Marker{}, err
	}
	return marker, s.Save()
}

// RemoveMarker deletes a marker from the open session.
func (s *Service) RemoveMarker(id session.MarkerID) error {
	if s.current == nil {
		return errors.New("no session is open")
	}
	if _, err := s.current.RemoveMarker(id); err != nil {
		return err
	}
	return s.Save()
}
