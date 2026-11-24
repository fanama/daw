package main

import (
	"fmt"
	"testing"

	"fanamix/domain/session"
)

func TestUndoHistoryRecordsOnlyChanges(t *testing.T) {
	history := newUndoHistory()
	history.record(session.State{Name: "same"}, session.State{Name: "same"})
	if _, ok := history.pop(); ok {
		t.Fatal("an unchanged state was recorded")
	}
	history.record(session.State{Name: "first"}, session.State{Name: "second"})
	history.record(session.State{Name: "second"}, session.State{Name: "third"})
	if state, ok := history.pop(); !ok || state.Name != "second" {
		t.Fatalf("first undo = %q, %v; want second", state.Name, ok)
	}
	if state, ok := history.pop(); !ok || state.Name != "first" {
		t.Fatalf("second undo = %q, %v; want first", state.Name, ok)
	}
	if _, ok := history.pop(); ok {
		t.Fatal("history returned more states than recorded")
	}
}

func TestUndoHistoryDropsOldestPastLimit(t *testing.T) {
	history := newUndoHistory()
	for index := 0; index < undoLimit+5; index++ {
		history.record(session.State{Name: fmt.Sprintf("state-%d", index)}, session.State{Name: "after"})
	}
	count := 0
	oldest := ""
	for {
		state, ok := history.pop()
		if !ok {
			break
		}
		oldest = state.Name
		count++
	}
	if count != undoLimit {
		t.Fatalf("history kept %d states, want %d", count, undoLimit)
	}
	if oldest != "state-5" {
		t.Fatalf("oldest kept state = %q, want state-5", oldest)
	}
}

func TestUndoJournalBoundaries(t *testing.T) {
	for _, op := range []string{"take.move", "take.delete", "track.remove", "marker.add", "click.set"} {
		if !tracksUndo(op) {
			t.Fatalf("%s should be undoable", op)
		}
	}
	for _, op := range []string{"take.begin", "take.finish", "take.abort", "state.undo", "open", "create", "close"} {
		if tracksUndo(op) {
			t.Fatalf("%s should not be journalled", op)
		}
	}
	if !resetsUndo("take.finish") || !resetsUndo("open") || resetsUndo("take.move") {
		t.Fatal("recording and session commands should reset the journal")
	}
}
