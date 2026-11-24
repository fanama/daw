//go:build js && wasm

package main

import (
	"errors"
	"sync"
	"syscall/js"

	"fanamix/domain/session"
	"fanamix/infrastructure/jsonstore"
)

// browserRepo keeps the open project in memory and hands every save to the
// page, which persists it in IndexedDB.
type browserRepo struct {
	mu       sync.Mutex
	projects map[string][]byte
}

func newBrowserRepo() *browserRepo {
	return &browserRepo{projects: map[string][]byte{}}
}

// Root names the virtual sessions folder used in session roots.
func (r *browserRepo) Root() string { return "fanamix" }

// stage registers project JSON the page loaded before Open runs.
func (r *browserRepo) stage(root, project string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.projects[root] = []byte(project)
}

// Save encodes the session and lets the page write it to IndexedDB.
func (r *browserRepo) Save(root string, current *session.Session) error {
	data, err := jsonstore.EncodeState(current.State())
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.projects[root] = data
	r.mu.Unlock()
	host := js.Global().Get("fanamixHost")
	if host.Truthy() {
		host.Call("save", root, string(data), current.Name())
	}
	return nil
}

// Exists reports whether this page already holds a project at root.
func (r *browserRepo) Exists(root string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.projects[root]
	return ok, nil
}

// Load restores a project previously staged by the page.
func (r *browserRepo) Load(root string) (*session.Session, error) {
	r.mu.Lock()
	data, ok := r.projects[root]
	r.mu.Unlock()
	if !ok {
		return nil, errors.New("this session is not loaded")
	}
	return jsonstore.RestoreSession(data)
}

// List describes the projects staged in this page; the picker uses IndexedDB
// directly, so this is only a completeness implementation.
func (r *browserRepo) List() ([]session.Summary, error) {
	r.mu.Lock()
	roots := make([]string, 0, len(r.projects))
	for root := range r.projects {
		roots = append(roots, root)
	}
	r.mu.Unlock()
	summaries := make([]session.Summary, 0, len(roots))
	for _, root := range roots {
		loaded, err := r.Load(root)
		if err != nil {
			continue
		}
		summaries = append(summaries, session.Summary{Name: loaded.Name(), Path: root, CreatedAt: loaded.CreatedAt()})
	}
	return summaries, nil
}
