package jsonstore

import (
	"encoding/json"

	"gorecord/domain/session"
)

// EncodeState renders a session snapshot as versioned project JSON. It is the
// same document the desktop build writes to project.json.
func EncodeState(state session.State) ([]byte, error) {
	return json.MarshalIndent(projectFromState(state), "", "  ")
}

// DecodeState parses project JSON back into a transport-neutral snapshot.
func DecodeState(data []byte) (session.State, error) {
	var project projectJSON
	if err := json.Unmarshal(data, &project); err != nil {
		return session.State{}, err
	}
	return stateFromProject(project)
}

// RestoreSession decodes project JSON and rebuilds the aggregate, re-checking
// every invariant. Clients without a file system (the browser build) use it to
// open a project they loaded themselves.
func RestoreSession(data []byte) (*session.Session, error) {
	state, err := DecodeState(data)
	if err != nil {
		return nil, err
	}
	return session.Restore(state)
}
