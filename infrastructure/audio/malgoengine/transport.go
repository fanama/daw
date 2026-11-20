package malgoengine

import (
	"errors"

	"gorecord/domain/studio"
)

// StartRecording arms the capture path and starts the backing transport.
func (e *Engine) StartRecording(spec studio.RecordSpec) error {
	if e.device == nil {
		return errors.New("audio engine: open an audio device first")
	}
	system := spec.Source == studio.RecordSourceSystemAudio
	if system && (!e.systemReady.Load() || e.systemDevice == nil) {
		message := e.systemErrorMessage()
		if message == "" {
			message = "choose a PC audio source in the toolbar and reopen the device"
		}
		return errors.New("audio engine: " + message)
	}
	if spec.Sink == nil || (!system && len(spec.Channels) == 0) {
		return errors.New("audio engine: recording needs armed channels")
	}
	if e.recording.Load() != nil {
		return errors.New("audio engine: a take is already recording")
	}
	flags := spec.PrintEffects
	if len(flags) != len(spec.Channels) {
		flags = make([]bool, len(spec.Channels))
	}
	if system {
		e.systemRing.reset()
	}
	e.playback.Store(&playbackState{mix: spec.Backing, startedAtFrame: spec.StartFrame})
	e.playhead.Store(int64(spec.StartFrame))
	e.recordedFrame.Store(0)
	e.recording.Store(&recordState{
		channels:     spec.Channels,
		printEffects: flags,
		sink:         spec.Sink,
		startFrame:   spec.StartFrame,
		system:       system,
	})
	e.state.Store(int32(studio.StateRecording))
	return nil
}

// StopRecording ends the take and returns what the engine captured.
func (e *Engine) StopRecording() (studio.RecordResult, error) {
	record := e.recording.Load()
	if record == nil {
		return studio.RecordResult{}, errors.New("audio engine: no take is recording")
	}
	e.recording.Store(nil)
	e.playback.Store(nil)
	e.state.Store(int32(studio.StateIdle))
	channels := len(record.channels)
	if record.system {
		channels = systemAudioChannels
	}
	return studio.RecordResult{
		StartFrame: record.startFrame,
		Frames:     int(e.recordedFrame.Load()),
		Channels:   channels,
		Incomplete: e.deviceStopped.Load(),
	}, nil
}

// StartPlayback mixes takes from the given session position.
func (e *Engine) StartPlayback(mix studio.PlaybackMix, fromFrame int) error {
	if e.device == nil {
		return errors.New("audio engine: open an audio device first")
	}
	if e.recording.Load() != nil {
		return errors.New("audio engine: stop the take before playing back")
	}
	e.playback.Store(&playbackState{mix: mix, startedAtFrame: fromFrame})
	e.playhead.Store(int64(fromFrame))
	e.state.Store(int32(studio.StatePlaying))
	return nil
}

// Stop halts the transport without discarding an in-flight take.
func (e *Engine) Stop() error {
	e.recording.Store(nil)
	e.playback.Store(nil)
	e.state.Store(int32(studio.StateIdle))
	return nil
}

// SetPlayhead repositions the transport cursor.
func (e *Engine) SetPlayhead(frame int) {
	if frame < 0 {
		frame = 0
	}
	e.playhead.Store(int64(frame))
}
