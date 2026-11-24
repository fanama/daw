package jsonstore

import "fanamix/domain/session"

type markerJSON struct {
	ID    string `json:"id"`
	Frame int    `json:"frame"`
	Name  string `json:"name"`
}

func markersFromState(markers []session.MarkerState) []markerJSON {
	stored := make([]markerJSON, 0, len(markers))
	for _, marker := range markers {
		stored = append(stored, markerJSON{
			ID:    marker.ID.String(),
			Frame: marker.Frame,
			Name:  marker.Name,
		})
	}
	return stored
}

func markersToState(markers []markerJSON) []session.MarkerState {
	restored := make([]session.MarkerState, 0, len(markers))
	for _, marker := range markers {
		restored = append(restored, session.MarkerState{
			ID:    session.MarkerID(marker.ID),
			Frame: marker.Frame,
			Name:  marker.Name,
		})
	}
	return restored
}
