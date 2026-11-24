package wavio

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"fanamix/domain/session"
)

// TakeFiles stores take audio inside a session folder.
type TakeFiles struct{}

// NewTakeFiles returns the take-file adapter.
func NewTakeFiles() *TakeFiles { return &TakeFiles{} }

// TakePath reports the session-relative path a take number will use.
func (f *TakeFiles) TakePath(_ string, number int) string {
	return path.Join("takes", fmt.Sprintf("take-%03d.wav", number))
}

// CreateTake opens a new take writer and reports its session-relative path.
func (f *TakeFiles) CreateTake(root string, number, channels, sampleRate int) (session.TakeWriter, string, error) {
	takesDir := filepath.Join(root, "takes")
	if err := os.MkdirAll(takesDir, 0o755); err != nil {
		return nil, "", err
	}
	relative := f.TakePath(root, number)
	finalPath := filepath.Join(root, filepath.FromSlash(relative))
	writer, err := NewWriter(finalPath+".part", channels, sampleRate)
	if err != nil {
		return nil, "", err
	}
	return &takeWriter{writer: writer, partPath: finalPath + ".part", finalPath: finalPath}, relative, nil
}

// OpenTake opens a recorded take for playback.
func (f *TakeFiles) OpenTake(root, relativePath string) (session.TakeAudio, error) {
	reader, err := OpenReader(filepath.Join(root, filepath.FromSlash(relativePath)))
	if err != nil {
		return nil, err
	}
	return reader, nil
}

// MoveToTrash moves a take file into the session trash folder.
func (f *TakeFiles) MoveToTrash(root, relativePath string) error {
	trashDir := filepath.Join(root, "trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return err
	}
	source := filepath.Join(root, filepath.FromSlash(relativePath))
	destination := filepath.Join(trashDir, filepath.Base(source))
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Rename(source, destination)
}

// RecoverPartials finalizes every interrupted take in the session and returns
// the recovered session-relative paths.
func (f *TakeFiles) RecoverPartials(root string) ([]string, error) {
	pattern := filepath.Join(root, "takes", "*.wav.part")
	partials, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	recovered := make([]string, 0, len(partials))
	for _, partial := range partials {
		if _, err := FinalizePartial(partial); err != nil {
			continue
		}
		finalPath := partial[:len(partial)-len(".part")]
		if err := os.Rename(partial, finalPath); err != nil {
			continue
		}
		relative, err := filepath.Rel(root, finalPath)
		if err != nil {
			continue
		}
		recovered = append(recovered, filepath.ToSlash(relative))
	}
	return recovered, nil
}

type takeWriter struct {
	writer    *Writer
	partPath  string
	finalPath string
}

func (t *takeWriter) Write(frames []float32) (int, error) {
	written, err := t.writer.Write(frames)
	if err != nil {
		return written, err
	}
	if written > 0 && written%480 == 0 {
		_ = t.writer.Checkpoint()
	}
	return written, nil
}

func (t *takeWriter) Frames() int { return t.writer.Frames() }

func (t *takeWriter) Checkpoint() error { return t.writer.Checkpoint() }

func (t *takeWriter) Finish(complete bool) error {
	if err := t.writer.Finish(); err != nil {
		return err
	}
	return os.Rename(t.partPath, t.finalPath)
}
