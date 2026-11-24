package main

import (
	"reflect"
	"strings"

	"fanamix/domain/session"
)

// undoLimit bounds the journal so a long editing session stays light.
const undoLimit = 50

// undoHistory keeps the states to restore, oldest first.
type undoHistory struct {
	states []session.State
}

func newUndoHistory() *undoHistory { return &undoHistory{} }

// record stores a state only when the edit actually changed the session.
func (h *undoHistory) record(before, after session.State) {
	if reflect.DeepEqual(before, after) {
		return
	}
	h.states = append(h.states, before)
	if len(h.states) > undoLimit {
		h.states = h.states[len(h.states)-undoLimit:]
	}
}

func (h *undoHistory) pop() (session.State, bool) {
	if len(h.states) == 0 {
		return session.State{}, false
	}
	last := len(h.states) - 1
	state := h.states[last]
	h.states = h.states[:last]
	return state, true
}

func (h *undoHistory) clear() { h.states = nil }

// tracksUndo reports whether a command belongs in the journal. Recording is a
// boundary: rewinding past a saved take would reuse its identifier.
func tracksUndo(op string) bool {
	switch op {
	case "", "state", "export", "state.undo", "create", "open", "close":
		return false
	}
	return !strings.HasPrefix(op, "take.begin") &&
		!strings.HasPrefix(op, "take.finish") &&
		!strings.HasPrefix(op, "take.abort")
}

func resetsUndo(op string) bool {
	switch op {
	case "create", "open", "close", "take.begin", "take.finish", "take.abort":
		return true
	}
	return false
}
