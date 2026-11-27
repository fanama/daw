//go:build js && wasm

package main

import (
	"fmt"

	"fanamix/domain/session"
)

func (c *core) applyNote(req request) error {
	noteID := session.NoteID(req.NoteID)
	noteIDs := make([]session.NoteID, 0, len(req.NoteIDs))
	for _, id := range req.NoteIDs {
		noteIDs = append(noteIDs, session.NoteID(id))
	}
	switch req.Op {
	case "note.add":
		_, err := c.service.AddNote(session.TrackID(req.TrackID), req.Pitch, req.StartFrame, req.LengthFrames, req.Velocity)
		return err
	case "note.addMany":
		return c.service.AddNotes(session.TrackID(req.TrackID), noteSpecs(req.Notes))
	case "note.remove":
		return c.service.RemoveNote(noteID)
	case "note.removeMany":
		return c.service.RemoveNotes(noteIDs)
	case "note.shift":
		return c.service.ShiftNotes(noteIDs, req.PitchDelta, req.FrameDelta)
	case "note.move":
		return c.service.MoveNote(noteID, req.Pitch, req.StartFrame)
	case "note.resize":
		return c.service.ResizeNote(noteID, req.LengthFrames)
	case "note.clear":
		return c.service.ClearNotes(session.TrackID(req.TrackID))
	default:
		return fmt.Errorf("unknown note command %q", req.Op)
	}
}

// noteSpecs converts the page's pasted notes into domain note specifications.
func noteSpecs(notes []webNote) []session.NoteSpec {
	specs := make([]session.NoteSpec, 0, len(notes))
	for _, note := range notes {
		specs = append(specs, session.NoteSpec{
			Pitch:        note.Pitch,
			StartFrame:   note.StartFrame,
			LengthFrames: note.LengthFrames,
			Velocity:     note.Velocity,
		})
	}
	return specs
}
