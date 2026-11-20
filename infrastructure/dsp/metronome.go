package dsp

import (
	"math"

	"gorecord/domain/mix"
)

const clickDecaySeconds = 0.03

// Metronome renders the optional click into the monitor mix. It is driven by
// the absolute frame position, so blocks never drift and the downbeat is
// always exact.
type Metronome struct {
	click         mix.Click
	sampleRate    int
	framesPerBeat int
	clickFrames   int
}

// NewMetronome returns a silent metronome until Configure is called.
func NewMetronome() *Metronome { return &Metronome{} }

// Configure installs the click settings for a sample rate.
func (m *Metronome) Configure(click mix.Click, sampleRate int) {
	m.click = click.Normalize()
	m.sampleRate = sampleRate
	m.framesPerBeat = m.click.FramesPerBeat(sampleRate)
	m.clickFrames = int(clickDecaySeconds * float64(sampleRate))
}

// Render adds the click for one mono block starting at blockStartFrame.
func (m *Metronome) Render(buffer []float32, blockStartFrame int) {
	if !m.click.Enabled || m.framesPerBeat <= 0 || m.clickFrames <= 0 {
		return
	}
	for index := range buffer {
		position := blockStartFrame + index
		beatPosition := position % m.framesPerBeat
		if beatPosition >= m.clickFrames {
			continue
		}
		beatNumber := position / m.framesPerBeat
		isDownbeat := beatNumber%m.click.BeatsPerBar == 0
		frequency := 1000.0
		gain := 0.35
		if isDownbeat {
			frequency = 1600
			gain = 0.5
		}
		seconds := float64(beatPosition) / float64(m.sampleRate)
		tone := math.Sin(2 * math.Pi * frequency * seconds)
		envelope := math.Exp(-seconds / (clickDecaySeconds * 0.25))
		buffer[index] += float32(tone * envelope * gain)
	}
}
