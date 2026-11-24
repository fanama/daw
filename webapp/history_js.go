//go:build js && wasm

package main

import (
	"errors"

	"fanamix/domain/session"
)

func (c *core) beforeEdit(op string) (session.State, bool) {
	if !tracksUndo(op) {
		return session.State{}, false
	}
	current := c.service.Current()
	if current == nil {
		return session.State{}, false
	}
	return current.State(), true
}

func (c *core) currentState() session.State {
	current := c.service.Current()
	if current == nil {
		return session.State{}
	}
	return current.State()
}

func (c *core) undo() error {
	state, ok := c.history.pop()
	if !ok {
		return errors.New("nothing to undo")
	}
	return c.service.Restore(state)
}
