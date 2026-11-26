//go:build js && wasm

package main

import (
	"fmt"

	"fanamix/domain/session"
)

func (c *core) applyNote(req request) error {
	noteID := session.NoteID(req.NoteID)
	switch req.Op {
	case "note.add":
		_, err := c.service.AddNote(session.TrackID(req.TrackID), req.Pitch, req.StartFrame, req.LengthFrames, req.Velocity)
		return err
	case "note.remove":
		return c.service.RemoveNote(noteID)
	case "note.move":
		return c.service.MoveNote(noteID, req.Pitch, req.StartFrame)
	case "note.resize":
		return c.service.ResizeNote(noteID, req.LengthFrames)
	default:
		return fmt.Errorf("unknown note command %q", req.Op)
	}
}
