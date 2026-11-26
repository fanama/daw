package jsonstore

import "fanamix/domain/session"

type noteJSON struct {
	ID           string `json:"id"`
	TrackID      string `json:"trackId"`
	Pitch        int    `json:"pitch"`
	StartFrame   int    `json:"startFrame"`
	LengthFrames int    `json:"lengthFrames"`
	Velocity     int    `json:"velocity"`
}

func notesFromState(notes []session.NoteState) []noteJSON {
	stored := make([]noteJSON, 0, len(notes))
	for _, note := range notes {
		stored = append(stored, noteJSON{
			ID:           note.ID.String(),
			TrackID:      note.TrackID.String(),
			Pitch:        note.Pitch,
			StartFrame:   note.StartFrame,
			LengthFrames: note.LengthFrames,
			Velocity:     note.Velocity,
		})
	}
	return stored
}

func notesToState(notes []noteJSON) []session.NoteState {
	restored := make([]session.NoteState, 0, len(notes))
	for _, note := range notes {
		restored = append(restored, session.NoteState{
			ID:           session.NoteID(note.ID),
			TrackID:      session.TrackID(note.TrackID),
			Pitch:        note.Pitch,
			StartFrame:   note.StartFrame,
			LengthFrames: note.LengthFrames,
			Velocity:     note.Velocity,
		})
	}
	return restored
}
