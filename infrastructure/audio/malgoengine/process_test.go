package malgoengine

import (
	"encoding/binary"
	"math"
	"testing"

	"gorecord/domain/mix"
	"gorecord/domain/studio"
)

const testFrames = 256

func newTestEngine(t *testing.T, channels int) *Engine {
	t.Helper()
	engine := NewEngine(nil)
	engine.allocateScratch(studio.DeviceConfig{
		SampleRate:   48000,
		Channels:     channels,
		PeriodFrames: testFrames,
		Periods:      2,
	})
	engine.config = studio.DeviceConfig{SampleRate: 48000, Channels: channels}
	for channel := range engine.chains {
		engine.chains[channel].SetEffects(mix.Effects{})
	}
	return engine
}

func filledInput(frames, channels int, value float32) []byte {
	input := make([]byte, frames*channels*4)
	for index := 0; index < frames*channels; index++ {
		binary.LittleEndian.PutUint32(input[index*4:], math.Float32bits(value))
	}
	return input
}

func peak(output []byte) float32 {
	highest := float32(0)
	for offset := 0; offset+4 <= len(output); offset += 4 {
		value := math.Float32frombits(binary.LittleEndian.Uint32(output[offset:]))
		if value < 0 {
			value = -value
		}
		if value > highest {
			highest = value
		}
	}
	return highest
}

func TestMonitorPassesFullLengthBlock(t *testing.T) {
	engine := newTestEngine(t, 2)
	engine.monitor.Store(true)
	output := make([]byte, testFrames*playbackChannels*4)
	engine.processBlock(output, filledInput(testFrames, 2, 0.5), testFrames)
	if peak(output) < 0.1 {
		t.Fatalf("monitoring stayed silent, peak %f", peak(output))
	}
}

func TestShortInputBufferStillMonitors(t *testing.T) {
	engine := newTestEngine(t, 2)
	engine.monitor.Store(true)
	output := make([]byte, testFrames*playbackChannels*4)
	engine.processBlock(output, filledInput(testFrames, 1, 0.5), testFrames)
	if peak(output) < 0.1 {
		t.Fatalf("a short input buffer muted the monitor bus, peak %f", peak(output))
	}
}

func TestMonitorFadesOutInsteadOfCutting(t *testing.T) {
	engine := newTestEngine(t, 2)
	engine.monitor.Store(true)
	output := make([]byte, testFrames*playbackChannels*4)
	engine.processBlock(output, filledInput(testFrames, 2, 0.5), testFrames)
	engine.monitor.Store(false)
	lastPeak := float32(0)
	for block := 0; block < 8; block++ {
		engine.processBlock(output, filledInput(testFrames, 2, 0.5), testFrames)
		lastPeak = peak(output)
	}
	if lastPeak > 0.01 {
		t.Fatalf("monitor did not fade out, last peak %f", lastPeak)
	}
}

func TestSystemTakeKeepsMonitoringInputs(t *testing.T) {
	engine := newTestEngine(t, 2)
	engine.monitor.Store(true)
	sink := &countingSink{}
	engine.recording.Store(&recordState{channels: []int{0, 1}, printEffects: []bool{false, false}, sink: sink, system: true})
	output := make([]byte, testFrames*playbackChannels*4)
	engine.processBlock(output, filledInput(testFrames, 2, 0.5), testFrames)
	if peak(output) < 0.1 {
		t.Fatalf("a PC take silenced live monitoring, peak %f", peak(output))
	}
	if sink.frames != testFrames*systemAudioChannels {
		t.Fatalf("PC take captured %d samples", sink.frames)
	}
}

type countingSink struct{ frames int }

func (s *countingSink) Offer(frames []float32) { s.frames += len(frames) }
