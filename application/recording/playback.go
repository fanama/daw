package recording

import (
	"errors"

	"gorecord/domain/session"
	"gorecord/domain/studio"
)

// PlaybackBudgetBytes caps how much take audio is preloaded for monitoring.
// Longer sessions are refused with an instruction instead of exhausting RAM.
const PlaybackBudgetBytes = 256 << 20

const preloadChunkFrames = 8192

// Loader builds the monitor mix from the session's audible takes.
type Loader struct {
	takes session.TakeFiles
}

// NewLoader wires the loader to the take-file port.
func NewLoader(takes session.TakeFiles) *Loader { return &Loader{takes: takes} }

// Load reads the audible takes into memory and maps them onto the timeline.
func (l *Loader) Load(root string, audible []session.AudibleTake) (studio.PlaybackMix, error) {
	var mix studio.PlaybackMix
	totalBytes := 0
	for _, entry := range audible {
		take := entry.Take
		audio, err := l.takes.OpenTake(root, take.File())
		if err != nil {
			return studio.PlaybackMix{}, err
		}
		loaded, bytes, readErr := readTake(audio, take)
		_ = audio.Close()
		if readErr != nil {
			return studio.PlaybackMix{}, readErr
		}
		totalBytes += bytes
		if totalBytes > PlaybackBudgetBytes {
			return studio.PlaybackMix{}, errors.New("this session is too large to play back from memory; delete old takes or split the session")
		}
		mix.Takes = append(mix.Takes, loaded)
	}
	return mix, nil
}

func readTake(audio session.TakeAudio, take session.Take) (studio.PlaybackTake, int, error) {
	channels := audio.Channels()
	length := take.AudibleLength()
	samples := make([]float32, length*channels)
	cursor := 0
	offset := take.Trim().AudioOffset()
	for cursor < length {
		wanted := length - cursor
		if wanted > preloadChunkFrames {
			wanted = preloadChunkFrames
		}
		buffer := samples[cursor*channels : (cursor+wanted)*channels]
		read, err := audio.ReadFrames(offset+cursor, buffer)
		if err != nil {
			return studio.PlaybackTake{}, 0, err
		}
		if read == 0 {
			break
		}
		cursor += read
	}
	return studio.PlaybackTake{
		Samples:    samples[:cursor*channels],
		Channels:   channels,
		StartFrame: take.StartFrame(),
		Length:     cursor,
		GainDB:     take.GainDB(),
	}, len(samples) * 4, nil
}
