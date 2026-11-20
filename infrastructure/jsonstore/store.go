// Package jsonstore persists sessions as human-readable project.json files
// inside one folder per song idea.
package jsonstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"gorecord/domain/session"
)

const projectFileName = "project.json"

// Store is the file-system backed session repository.
type Store struct {
	root string
}

// NewStore creates a repository rooted at the given sessions folder.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// Root returns the sessions folder.
func (s *Store) Root() string { return s.root }

// Save writes the project atomically, keeping one backup of the previous file.
func (s *Store) Save(root string, sess *session.Session) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(projectFromState(sess.State()), "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(root, projectFileName)
	if _, statErr := os.Stat(target); statErr == nil {
		_ = copyFile(target, target+".bak")
	}
	temporary := target + ".tmp"
	if err := os.WriteFile(temporary, encoded, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, target)
}

// Exists reports whether a project already lives at root.
func (s *Store) Exists(root string) (bool, error) {
	_, err := os.Stat(filepath.Join(root, projectFileName))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// Load reads and restores one session folder.
func (s *Store) Load(root string) (*session.Session, error) {
	raw, err := os.ReadFile(filepath.Join(root, projectFileName))
	if err != nil {
		return nil, err
	}
	var project projectJSON
	if err := json.Unmarshal(raw, &project); err != nil {
		return nil, err
	}
	state, err := stateFromProject(project)
	if err != nil {
		return nil, err
	}
	return session.Restore(state)
}

// List returns every session folder under the repository root.
func (s *Store) List() ([]session.Summary, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	summaries := make([]session.Summary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folder := filepath.Join(s.root, entry.Name())
		if _, statErr := os.Stat(filepath.Join(folder, projectFileName)); statErr != nil {
			continue
		}
		loaded, loadErr := s.Load(folder)
		if loadErr != nil {
			continue
		}
		summaries = append(summaries, session.Summary{
			Name:      loaded.Name(),
			Path:      folder,
			CreatedAt: loaded.CreatedAt(),
		})
	}
	return summaries, nil
}

func copyFile(source, destination string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, raw, 0o644)
}
