package malgoengine

import (
	"sync/atomic"

	"github.com/gen2brain/malgo"
	"gorecord/domain/mix"
	"gorecord/domain/studio"
)

func normalizeConfig(config studio.DeviceConfig) studio.DeviceConfig {
	if config.PeriodFrames <= 0 {
		config.PeriodFrames = defaultPeriodFrames
	}
	if config.Periods <= 0 {
		config.Periods = 2
	}
	return config
}

func resolveDeviceIDs(context *malgo.AllocatedContext, config studio.DeviceConfig) (malgo.DeviceID, malgo.DeviceID, error) {
	captureID, err := deviceIDFor(context, config.CaptureDeviceID)
	if err != nil {
		return malgo.DeviceID{}, malgo.DeviceID{}, err
	}
	playbackID := captureID
	if config.PlaybackDeviceID != "" && config.PlaybackDeviceID != config.CaptureDeviceID {
		playbackID, err = deviceIDFor(context, config.PlaybackDeviceID)
		if err != nil {
			return malgo.DeviceID{}, malgo.DeviceID{}, err
		}
	}
	return captureID, playbackID, nil
}

func duplexConfig(config studio.DeviceConfig, captureID, playbackID malgo.DeviceID) malgo.DeviceConfig {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Duplex)
	deviceConfig.SampleRate = uint32(config.SampleRate)
	deviceConfig.PeriodSizeInFrames = uint32(config.PeriodFrames)
	deviceConfig.Periods = uint32(config.Periods)
	deviceConfig.PerformanceProfile = malgo.LowLatency
	deviceConfig.Capture.Format = malgo.FormatF32
	deviceConfig.Capture.Channels = uint32(config.Channels)
	deviceConfig.Capture.DeviceID = captureID.Pointer()
	deviceConfig.Playback.Format = malgo.FormatF32
	deviceConfig.Playback.Channels = playbackChannels
	deviceConfig.Playback.DeviceID = playbackID.Pointer()
	return deviceConfig
}

func (e *Engine) callbacks() malgo.DeviceCallbacks {
	return malgo.DeviceCallbacks{
		Data: func(output, input []byte, frames uint32) {
			e.processBlock(output, input, int(frames))
		},
		Stop: func() {
			e.deviceStopped.Store(true)
			e.state.Store(int32(studio.StateIdle))
		},
	}
}

func (e *Engine) allocateScratch(config studio.DeviceConfig) {
	channels := config.Channels
	e.dry = make([][]float32, channels)
	e.wet = make([][]float32, channels)
	e.channelOut = make([][]float32, channels)
	e.chains = make([]mix.Processor, channels)
	for channel := 0; channel < channels; channel++ {
		e.dry[channel] = make([]float32, scratchFrames)
		e.wet[channel] = make([]float32, scratchFrames)
		e.chains[channel] = e.factory.NewProcessor()
		e.chains[channel].Prepare(config.SampleRate)
	}
	e.recordBuf = make([]float32, scratchFrames*channels)
	e.mixBuf = make([]float32, scratchFrames*playbackChannels)
	e.clickBuf = make([]float32, scratchFrames)
	e.systemRing = newSampleRing(config.SampleRate * systemRingSeconds * systemAudioChannels)
	e.systemBuf = make([]float32, scratchFrames*systemAudioChannels)
	e.systemReady.Store(false)
	e.metronome.Configure(mix.DefaultClick(), config.SampleRate)

	e.peaks = make([]atomic.Uint32, channels)
	e.rms = make([]atomic.Uint32, channels)
	e.tunerWrite.Store(0)
	e.monitorGain = 0
}

func deviceIDFor(context *malgo.AllocatedContext, id string) (malgo.DeviceID, error) {
	if id == "" {
		return malgo.DeviceID{}, nil
	}
	return malgo.ParseDeviceID(id)
}

func uninitContext(context *malgo.AllocatedContext) {
	_ = context.Uninit()
	context.Free()
}
