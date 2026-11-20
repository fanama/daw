package studio

import "gorecord/domain/mix"

// State is the transport state of the engine.
type State int

const (
	// StateIdle means the transport is stopped.
	StateIdle State = iota
	// StatePlaying means takes are playing.
	StatePlaying
	// StateRecording means input is being written to a take.
	StateRecording
)

// RecordSource selects what a take captures.
type RecordSource int

const (
	// RecordSourceInput captures the armed interface channels.
	RecordSourceInput RecordSource = iota
	// RecordSourceSystemAudio captures the computer's own output in stereo.
	RecordSourceSystemAudio
)

// FrameSink accepts interleaved float32 frames from the audio callback. An
// implementation must never block or allocate on the calling thread.
type FrameSink interface {
	Offer(frames []float32)
}

// RecordSpec describes one recording pass.
type RecordSpec struct {
	StartFrame   int
	Source       RecordSource
	Channels     []int
	PrintEffects []bool
	Sink         FrameSink
	Backing      PlaybackMix
}

// RecordResult reports what actually reached disk.
type RecordResult struct {
	StartFrame int
	Frames     int
	Channels   int
	Incomplete bool
}

// PlaybackTake is one preloaded take on the monitor timeline.
type PlaybackTake struct {
	Samples    []float32
	Channels   int
	StartFrame int
	Length     int
	GainDB     float64
}

// PlaybackMix is the set of takes the monitor bus mixes.
type PlaybackMix struct {
	Takes []PlaybackTake
}

// ChainSettings is the DSP configuration for one input channel.
type ChainSettings struct {
	Channel      int
	Effects      mix.Effects
	PrintEffects bool
}

// PeakBucket is one min/max envelope bucket for waveform drawing.
type PeakBucket struct {
	Min float32
	Max float32
}

// Snapshot is a copy of the engine telemetry for the delivery layer.
type Snapshot struct {
	State           State
	PlayheadFrame   int
	RecordedFrames  int
	XRuns           int
	DiskStalls      int
	LatencyMs       float64
	SampleRate      int
	Channels        int
	PeriodFrames    int
	Peaks           []float32
	RMS             []float32
	Clipped         []bool
	PitchHz         float64
	PitchConfident  bool
	DeviceStopped   bool
	SystemReady     bool
	SystemPeak      float32
	SystemUnderruns int
	SystemError     string
}

// Peak returns the peak level of one input channel.
func (s Snapshot) Peak(channel int) float32 {
	if channel < 0 || channel >= len(s.Peaks) {
		return 0
	}
	return s.Peaks[channel]
}

// AudioEngine is the outbound port for the real-time audio device.
type AudioEngine interface {
	Open(config DeviceConfig) error
	Close() error
	StartRecording(spec RecordSpec) error
	StopRecording() (RecordResult, error)
	StartPlayback(mix PlaybackMix, fromFrame int) error
	Stop() error
	SetMonitor(enabled bool)
	SetChains(chains []ChainSettings)
	SetClick(click mix.Click)
	SetPlayhead(frame int)
	Snapshot() Snapshot
}
