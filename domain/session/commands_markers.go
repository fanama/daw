package session

import "fmt"

// AddMarker places a named marker at a frame, naming it automatically when the
// label is empty.
func (s *Session) AddMarker(frame int, name string) (Marker, error) {
	marker := newMarker(
		MarkerID(fmt.Sprintf("mrk_%d", s.nextMarker)),
		frame,
		cleanMarkerName(name, fmt.Sprintf("Marker %d", s.nextMarker)),
	)
	s.nextMarker++
	s.markers = append(s.markers, marker)
	return marker, nil
}

// RemoveMarker deletes a marker and returns it.
func (s *Session) RemoveMarker(id MarkerID) (Marker, error) {
	for index, marker := range s.markers {
		if marker.id != id {
			continue
		}
		s.markers = append(s.markers[:index], s.markers[index+1:]...)
		return marker, nil
	}
	return Marker{}, notFound("marker %s does not exist", id)
}
