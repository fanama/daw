package session

import (
	"strings"
)

const maxMarkerName = 60

// Marker is a named position on the session timeline, like a memory location
// in a DAW. Markers are saved with the project and never affect the audio.
type Marker struct {
	id    MarkerID
	frame int
	name  string
}

// ID returns the marker identifier.
func (m Marker) ID() MarkerID { return m.id }

// Frame returns the marker position in session frames.
func (m Marker) Frame() int { return m.frame }

// Name returns the marker label.
func (m Marker) Name() string { return m.name }

func (m Marker) state() MarkerState {
	return MarkerState{ID: m.id, Frame: m.frame, Name: m.name}
}

func newMarker(id MarkerID, frame int, name string) Marker {
	return Marker{id: id, frame: max(0, frame), name: name}
}

func cleanMarkerName(name, fallback string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		clean = fallback
	}
	runes := []rune(clean)
	if len(runes) > maxMarkerName {
		clean = string(runes[:maxMarkerName])
	}
	return clean
}

// markersFromState rebuilds markers while rejecting corrupt stored entries.
func markersFromState(states []MarkerState) ([]Marker, error) {
	markers := make([]Marker, 0, len(states))
	seen := map[MarkerID]bool{}
	for _, state := range states {
		name := strings.TrimSpace(state.Name)
		if state.ID.IsZero() || seen[state.ID] {
			return nil, conflict("marker %s appears twice or has no id", state.ID)
		}
		if state.Frame < 0 {
			return nil, invalid("marker %s has a negative frame", state.ID)
		}
		if name == "" {
			return nil, invalid("marker %s has no name", state.ID)
		}
		seen[state.ID] = true
		markers = append(markers, newMarker(state.ID, state.Frame, name))
	}
	return markers, nil
}
