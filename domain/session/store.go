package session

import "time"

// Summary is the picker-facing description of a stored session.
type Summary struct {
	Name      string
	Path      string
	CreatedAt time.Time
}

// Repository is the outbound port for session persistence. The root is an
// opaque locator; the domain never learns how it maps to disk.
type Repository interface {
	Save(root string, session *Session) error
	Load(root string) (*Session, error)
	Exists(root string) (bool, error)
	List() ([]Summary, error)
	Root() string
}

// TakeWriter appends interleaved float32 frames to a take file.
type TakeWriter interface {
	Write(frames []float32) (int, error)
	Frames() int
	Checkpoint() error
	Finish(complete bool) error
}

// TakeAudio is the read side of a recorded take.
type TakeAudio interface {
	Channels() int
	SampleRate() int
	Frames() int
	ReadFrames(atFrame int, dst []float32) (int, error)
	Close() error
}

// MixdownFile is the write side of an exported session mix.
type MixdownFile interface {
	Write(frames []float32) (int, error)
	Finish() error
}

// Mixdowns is the outbound port for exported mix files.
type Mixdowns interface {
	Create(path string, channels, sampleRate int) (MixdownFile, error)
}

// TakeFiles is the outbound port for take audio on disk.
type TakeFiles interface {
	TakePath(root string, number int) string
	CreateTake(root string, number, channels, sampleRate int) (TakeWriter, string, error)
	OpenTake(root, relativePath string) (TakeAudio, error)
	MoveToTrash(root, relativePath string) error
	RecoverPartials(root string) ([]string, error)
}
