package recording

import (
	"errors"
	"sync"
	"time"

	"gorecord/domain/session"
	"gorecord/domain/studio"
)

const (
	sinkBlockCount   = 64
	checkpointEvery  = 2 * time.Second
	peakBucketFrames = 256
	systemChannels   = 2
)

// StartCommand describes one recording pass.
type StartCommand struct {
	Root        string
	TakeNumber  int
	Slots       []session.ArmSlot
	StartFrame  int
	SampleRate  int
	BlockFrames int
	SystemAudio bool
	Backing     studio.PlaybackMix
}

// StopResult reports the finished take.
type StopResult struct {
	File       string
	Frames     int
	Channels   int
	StartFrame int
	Incomplete bool
	Stalls     int64
}

type activeRecording struct {
	writer   session.TakeWriter
	sink     *frameSink
	peaks    *peakCollector
	file     string
	channels int
	start    int
	finished chan struct{}
}

// Recorder is the inbound port for recording a take.
type Recorder struct {
	engine studio.AudioEngine
	takes  session.TakeFiles
	mu     sync.Mutex
	active *activeRecording
}

// NewRecorder wires the record use case to its ports.
func NewRecorder(engine studio.AudioEngine, takes session.TakeFiles) *Recorder {
	return &Recorder{engine: engine, takes: takes}
}

// Start opens the take file and rolls the engine.
func (r *Recorder) Start(command StartCommand) error {
	if len(command.Slots) == 0 {
		return errors.New("arm at least one track before recording")
	}
	channels := len(command.Slots)
	if command.SystemAudio {
		channels = systemChannels
	}
	writer, file, err := r.takes.CreateTake(command.Root, command.TakeNumber, channels, command.SampleRate)
	if err != nil {
		return err
	}
	active := &activeRecording{
		writer:   writer,
		sink:     newFrameSink(command.BlockFrames*channels, sinkBlockCount),
		peaks:    newPeakCollector(peakBucketFrames),
		file:     file,
		channels: channels,
		start:    command.StartFrame,
		finished: make(chan struct{}),
	}
	go r.drain(active)

	if !command.SystemAudio {
		r.engine.SetChains(chainsFromSlots(command.Slots))
	}
	if err := r.engine.StartRecording(recordSpec(command, active)); err != nil {
		close(active.sink.blocks)
		<-active.finished
		_ = writer.Finish(false)
		return err
	}
	r.mu.Lock()
	r.active = active
	r.mu.Unlock()
	return nil
}

// Stop finalizes the take and reports what reached disk.
func (r *Recorder) Stop() (StopResult, error) {
	r.mu.Lock()
	active := r.active
	r.active = nil
	r.mu.Unlock()
	if active == nil {
		return StopResult{}, errors.New("no recording is running")
	}
	result, engineErr := r.engine.StopRecording()
	close(active.sink.blocks)
	<-active.finished
	incomplete := engineErr != nil || result.Incomplete
	if err := active.writer.Finish(!incomplete); err != nil && engineErr == nil {
		engineErr = err
	}
	stop := StopResult{
		File:       active.file,
		Frames:     active.writer.Frames(),
		Channels:   active.channels,
		StartFrame: active.start,
		Incomplete: incomplete,
		Stalls:     active.sink.stalls.Load(),
	}
	return stop, engineErr
}

// Waveform returns the live peak envelope of the running take.
func (r *Recorder) Waveform() []studio.PeakBucket {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil {
		return nil
	}
	return r.active.peaks.snapshot()
}

// IsRecording reports whether a take is rolling.
func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active != nil
}

func (r *Recorder) drain(active *activeRecording) {
	defer close(active.finished)
	ticker := time.NewTicker(checkpointEvery)
	defer ticker.Stop()
	for {
		select {
		case block, open := <-active.sink.blocks:
			if !open {
				return
			}
			samples := block.data[:block.frames]
			if _, err := active.writer.Write(samples); err != nil {
				active.sink.stalls.Add(1)
			}
			active.peaks.add(samples, active.channels)
			active.sink.pool <- block.data
		case <-ticker.C:
			_ = active.writer.Checkpoint()
		}
	}
}

func recordSpec(command StartCommand, active *activeRecording) studio.RecordSpec {
	spec := studio.RecordSpec{
		StartFrame: command.StartFrame,
		Source:     studio.RecordSourceInput,
		Sink:       active.sink,
		Backing:    command.Backing,
	}
	if command.SystemAudio {
		spec.Source = studio.RecordSourceSystemAudio
		return spec
	}
	spec.Channels = channelsFromSlots(command.Slots)
	spec.PrintEffects = printFlagsFromSlots(command.Slots)
	return spec
}

func channelsFromSlots(slots []session.ArmSlot) []int {
	channels := make([]int, 0, len(slots))
	for _, slot := range slots {
		channels = append(channels, slot.Source.Channel)
	}
	return channels
}

func printFlagsFromSlots(slots []session.ArmSlot) []bool {
	flags := make([]bool, 0, len(slots))
	for _, slot := range slots {
		flags = append(flags, slot.PrintEffects)
	}
	return flags
}

func chainsFromSlots(slots []session.ArmSlot) []studio.ChainSettings {
	chains := make([]studio.ChainSettings, 0, len(slots))
	for _, slot := range slots {
		if slot.Source.IsSystem() {
			continue
		}
		chains = append(chains, studio.ChainSettings{
			Channel:      slot.Source.Channel,
			Effects:      slot.Effects,
			PrintEffects: slot.PrintEffects,
		})
	}
	return chains
}
