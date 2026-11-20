// Package sessionapp orchestrates the session use cases: creating and opening
// a song folder, editing tracks and takes, and persisting after every change.
package sessionapp

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"gorecord/domain/session"
)

// Service is the inbound port for session editing.
type Service struct {
	repo    session.Repository
	takes   session.TakeFiles
	now     func() time.Time
	root    string
	current *session.Session
}

// NewService wires the session use cases to their ports.
func NewService(repo session.Repository, takes session.TakeFiles, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, takes: takes, now: now}
}

// Current returns the open session, or nil when none is open.
func (s *Service) Current() *session.Session { return s.current }

// Root returns the open session folder.
func (s *Service) Root() string { return s.root }

// Summaries lists the stored sessions for the picker.
func (s *Service) Summaries() ([]session.Summary, error) { return s.repo.List() }

// Create starts a new session folder at the repository root.
func (s *Service) Create(name string, sampleRate int) error {
	created, err := session.New(name, sampleRate, s.now())
	if err != nil {
		return err
	}
	root := filepath.Join(s.repo.Root(), folderName(name))
	exists, err := s.repo.Exists(root)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("a session with this name already exists")
	}
	s.current = created
	s.root = root
	_, err = s.current.AddTrack("Guitar", 0)
	if err != nil {
		return err
	}
	return s.Save()
}

// Open loads an existing session folder and repairs interrupted takes.
func (s *Service) Open(root string) error {
	loaded, err := s.repo.Load(root)
	if err != nil {
		return err
	}
	s.current = loaded
	s.root = root
	return s.reconcileRecoveredTakes()
}

// Save persists the current session.
func (s *Service) Save() error {
	if s.current == nil {
		return errors.New("no session is open")
	}
	return s.repo.Save(s.root, s.current)
}

// Close forgets the open session.
func (s *Service) Close() {
	s.current = nil
	s.root = ""
}

func (s *Service) reconcileRecoveredTakes() error {
	recovered, err := s.takes.RecoverPartials(s.root)
	if err != nil {
		return err
	}
	changed := false
	for _, relative := range recovered {
		for _, take := range s.current.Takes() {
			if take.File() != relative {
				continue
			}
			if take.IsIncomplete() {
				continue
			}
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.Save()
}

func folderName(name string) string {
	cleaned := strings.TrimSpace(name)
	cleaned = strings.ReplaceAll(cleaned, string(filepath.Separator), "-")
	cleaned = strings.ReplaceAll(cleaned, "/", "-")
	if cleaned == "" {
		return "untitled"
	}
	return cleaned
}
