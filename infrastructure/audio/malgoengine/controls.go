package malgoengine

import (
	"math"

	"gorecord/domain/mix"
	"gorecord/domain/studio"
)

// SetMonitor enables or disables input monitoring.
func (e *Engine) SetMonitor(enabled bool) { e.monitor.Store(enabled) }

// SetChains publishes effect settings to each channel processor.
func (e *Engine) SetChains(chains []studio.ChainSettings) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, settings := range chains {
		if settings.Channel < 0 || settings.Channel >= len(e.chains) {
			continue
		}
		e.chains[settings.Channel].SetEffects(settings.Effects)
	}
}

// SetClick stores the click configuration.
func (e *Engine) SetClick(click mix.Click) {
	normalized := click.Normalize()
	e.clickSettings.Store(&normalized)
}

// Snapshot copies the current telemetry for the delivery layer.
func (e *Engine) Snapshot() studio.Snapshot {
	e.mu.Lock()
	config := e.config
	e.mu.Unlock()
	channels := len(e.peaks)
	snapshot := studio.Snapshot{
		State:           studio.State(e.state.Load()),
		PlayheadFrame:   int(e.playhead.Load()),
		RecordedFrames:  int(e.recordedFrame.Load()),
		LatencyMs:       config.LatencyMs(),
		SampleRate:      config.SampleRate,
		Channels:        channels,
		PeriodFrames:    config.PeriodFrames,
		DeviceStopped:   e.deviceStopped.Load(),
		PitchHz:         math.Float64frombits(e.pitchHz.Load()),
		PitchConfident:  e.pitchHas.Load(),
		SystemReady:     e.systemReady.Load(),
		SystemPeak:      math.Float32frombits(e.systemPeak.Load()),
		SystemUnderruns: int(e.systemUnderruns.Load()),
		SystemError:     e.systemErrorMessage(),
	}
	snapshot.Peaks = make([]float32, channels)
	snapshot.RMS = make([]float32, channels)
	snapshot.Clipped = make([]bool, channels)
	for channel := 0; channel < channels; channel++ {
		snapshot.Peaks[channel] = math.Float32frombits(e.peaks[channel].Load())
		snapshot.RMS[channel] = math.Float32frombits(e.rms[channel].Load())
		snapshot.Clipped[channel] = snapshot.Peaks[channel] >= 0.999
	}
	return snapshot
}
