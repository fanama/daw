package malgoengine

import (
	"encoding/binary"
	"math"
)

// monitorFadeSeconds is the ramp applied when monitoring starts or stops, so
// a toggle or a take boundary never produces a click.
const monitorFadeSeconds = 0.005

// processBlock is the real-time callback. It must never allocate, lock,
// log or send on a channel.
func (e *Engine) processBlock(output, input []byte, frames int) {
	if len(e.dry) == 0 || frames <= 0 {
		silence(output)
		return
	}
	if frames > scratchFrames {
		frames = scratchFrames
	}
	if len(output) < frames*playbackChannels*4 {
		silence(output)
		return
	}
	inputChannels := len(input) / (frames * 4)
	if inputChannels > len(e.dry) {
		inputChannels = len(e.dry)
	}
	e.refreshClick()
	record := e.recording.Load()
	monitor := e.monitor.Load()
	start := int(e.playhead.Load())

	e.deinterleaveAndProcess(input, frames, inputChannels, monitor, record)
	if record != nil {
		e.captureTake(record, frames)
		e.recordedFrame.Add(int64(frames))
	}
	e.renderOutput(output, start, frames, monitor)
	e.playhead.Add(int64(frames))
	e.writeTuner(frames)
}

func (e *Engine) refreshClick() {
	stored := e.clickSettings.Load()
	if stored == nil || *stored == e.lastClick {
		return
	}
	e.metronome.Configure(*stored, e.config.SampleRate)
	e.lastClick = *stored
}

// deinterleaveAndProcess reads only the channels the backend actually
// delivered. A shorter buffer keeps monitoring alive instead of muting the
// whole block, which is what produced intermittent dropouts.
func (e *Engine) deinterleaveAndProcess(input []byte, frames, inputChannels int, monitor bool, record *recordState) {
	for channel := 0; channel < len(e.dry); channel++ {
		dry := e.dry[channel][:frames]
		if channel < inputChannels {
			e.readChannel(input, dry, channel, inputChannels)
		} else {
			for frame := range dry {
				dry[frame] = 0
			}
		}
		e.publishMeter(channel, dry)
		needsProcessing := monitor || (record != nil && record.printsChannel(channel))
		if !needsProcessing {
			e.channelOut[channel] = dry
			continue
		}
		wet := e.wet[channel][:frames]
		copy(wet, dry)
		e.chains[channel].ProcessMono(wet)
		e.channelOut[channel] = wet
	}
}

func (e *Engine) readChannel(input []byte, destination []float32, channel, inputChannels int) {
	for frame := range destination {
		offset := (frame*inputChannels + channel) * 4
		destination[frame] = math.Float32frombits(binary.LittleEndian.Uint32(input[offset:]))
	}
}

func (e *Engine) captureTake(record *recordState, frames int) {
	if record.system {
		e.captureSystemTake(record, frames)
		return
	}
	channels := len(record.channels)
	if len(e.recordBuf) < frames*channels {
		return
	}
	buffer := e.recordBuf[:frames*channels]
	for frame := 0; frame < frames; frame++ {
		for slot, channel := range record.channels {
			if channel < 0 || channel >= len(e.dry) {
				buffer[frame*channels+slot] = 0
				continue
			}
			if record.printEffects[slot] {
				buffer[frame*channels+slot] = e.channelOut[channel][frame]
				continue
			}
			buffer[frame*channels+slot] = e.dry[channel][frame]
		}
	}
	record.sink.Offer(buffer)
}

func (e *Engine) renderOutput(output []byte, start, frames int, monitor bool) {
	mixBuffer := e.mixBuf[:frames*playbackChannels]
	for index := range mixBuffer {
		mixBuffer[index] = 0
	}
	if playing := e.playback.Load(); playing != nil {
		mixPlayback(mixBuffer, playing.mix.Takes, start, frames)
	}
	if monitor || e.monitorGain > 0 {
		e.renderMonitor(mixBuffer, frames, monitor)
	}
	clickBuffer := e.clickBuf[:frames]
	for index := range clickBuffer {
		clickBuffer[index] = 0
	}
	e.metronome.Render(clickBuffer, start)
	for frame := 0; frame < frames; frame++ {
		for channel := 0; channel < playbackChannels; channel++ {
			value := mixBuffer[frame*playbackChannels+channel] + clickBuffer[frame]
			value = float32(math.Tanh(float64(value)))
			offset := (frame*playbackChannels + channel) * 4
			binary.LittleEndian.PutUint32(output[offset:], math.Float32bits(value))
		}
	}
}

// renderMonitor mixes the processed inputs into both outputs with a short
// gain ramp, so enabling or disabling monitoring never cuts the signal.
func (e *Engine) renderMonitor(mixBuffer []float32, frames int, monitor bool) {
	target := float32(0)
	if monitor {
		target = 1
	}
	step := monitorFadeStep(e.config.SampleRate)
	for frame := 0; frame < frames; frame++ {
		e.monitorGain = approachGain(e.monitorGain, target, step)
		for channel := range e.channelOut {
			value := e.channelOut[channel][frame] * 0.5 * e.monitorGain
			mixBuffer[frame*playbackChannels] += value
			mixBuffer[frame*playbackChannels+1] += value
		}
	}
}

func approachGain(gain, target, step float32) float32 {
	if gain < target {
		gain += step
		if gain > target {
			return target
		}
		return gain
	}
	if gain > target {
		gain -= step
		if gain < target {
			return target
		}
		return gain
	}
	return gain
}

func monitorFadeStep(sampleRate int) float32 {
	if sampleRate <= 0 {
		return 1
	}
	return float32(1 / (monitorFadeSeconds * float64(sampleRate)))
}

func (e *Engine) publishMeter(channel int, samples []float32) {
	peak := float32(0)
	sum := 0.0
	for _, sample := range samples {
		magnitude := sample
		if magnitude < 0 {
			magnitude = -magnitude
		}
		if magnitude > peak {
			peak = magnitude
		}
		sum += float64(sample) * float64(sample)
	}
	e.peaks[channel].Store(math.Float32bits(peak))
	mean := 0.0
	if len(samples) > 0 {
		mean = math.Sqrt(sum / float64(len(samples)))
	}
	e.rms[channel].Store(math.Float32bits(float32(mean)))
}

func (r *recordState) printsChannel(channel int) bool {
	for slot, candidate := range r.channels {
		if candidate == channel {
			return r.printEffects[slot]
		}
	}
	return false
}

func silence(output []byte) {
	for index := range output {
		output[index] = 0
	}
}
