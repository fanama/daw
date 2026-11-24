package wavio

import "fanamix/domain/session"

// Mixdowns creates stereo float WAV files for mixdown export.
type Mixdowns struct{}

// NewMixdowns returns the mixdown-file adapter.
func NewMixdowns() *Mixdowns { return &Mixdowns{} }

// Create opens the export destination with a normal, non-crash-safe WAV file.
func (m *Mixdowns) Create(path string, channels, sampleRate int) (session.MixdownFile, error) {
	return NewWriter(path, channels, sampleRate)
}
