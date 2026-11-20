package malgoengine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync/atomic"

	"github.com/gen2brain/malgo"
	"gorecord/domain/studio"
)

const (
	systemAudioChannels = 2
	systemRingSeconds   = 4
)

// sampleRing is a single-producer, single-consumer float ring. The capture
// callback writes, the duplex callback reads, and neither blocks.
type sampleRing struct {
	samples []float32
	write   atomic.Int64
	read    atomic.Int64
}

func newSampleRing(capacity int) *sampleRing {
	if capacity < systemAudioChannels {
		capacity = systemAudioChannels
	}
	return &sampleRing{samples: make([]float32, capacity)}
}

// reset discards buffered audio so a take starts at the current moment.
func (r *sampleRing) reset() {
	r.read.Store(r.write.Load())
}

func (r *sampleRing) writeInterleaved(input []byte, samples int) {
	capacity := int64(len(r.samples))
	if samples <= 0 || capacity == 0 {
		return
	}
	if int64(samples) > capacity {
		input = input[(int64(samples)-capacity)*4:]
		samples = int(capacity)
	}
	write := r.write.Load()
	read := r.read.Load()
	if write-read+int64(samples) > capacity {
		r.read.Add(write - read + int64(samples) - capacity)
	}
	for index := 0; index < samples; index++ {
		value := math.Float32frombits(binary.LittleEndian.Uint32(input[index*4:]))
		r.samples[(write+int64(index))%capacity] = value
	}
	r.write.Store(write + int64(samples))
}

func (r *sampleRing) readInto(destination []float32) int {
	capacity := int64(len(r.samples))
	write := r.write.Load()
	read := r.read.Load()
	available := int(write - read)
	if available > len(destination) {
		available = len(destination)
	}
	if available < 0 || capacity == 0 {
		return 0
	}
	for index := 0; index < available; index++ {
		destination[index] = r.samples[(read+int64(index))%capacity]
	}
	r.read.Store(read + int64(available))
	return available
}

// openSystemDevice opens the PC audio source: a WASAPI loopback stream on
// Windows, or the virtual device / monitor source chosen on other platforms.
func (e *Engine) openSystemDevice(config studio.DeviceConfig) error {
	if !config.HasSystemAudio() {
		return nil
	}
	if runtime.GOOS != "windows" && config.SystemAudioDeviceID == config.CaptureDeviceID {
		return errors.New("PC audio needs a virtual device (BlackHole, Loopback, monitor source)")
	}
	deviceID, err := deviceIDFor(e.context, config.SystemAudioDeviceID)
	if err != nil {
		return err
	}
	deviceType := malgo.Capture
	if runtime.GOOS == "windows" {
		deviceType = malgo.Loopback
	}
	deviceConfig := malgo.DefaultDeviceConfig(deviceType)
	deviceConfig.SampleRate = uint32(config.SampleRate)
	deviceConfig.PeriodSizeInFrames = uint32(config.PeriodFrames)
	deviceConfig.Periods = uint32(config.Periods)
	deviceConfig.PerformanceProfile = malgo.LowLatency
	deviceConfig.Capture.Format = malgo.FormatF32
	deviceConfig.Capture.Channels = systemAudioChannels
	deviceConfig.Capture.DeviceID = deviceID.Pointer()
	device, err := malgo.InitDevice(e.context.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: e.systemCallback,
		Stop: func() { e.systemReady.Store(false) },
	})
	if err != nil {
		return fmt.Errorf("PC audio device: %w", err)
	}
	if err := device.Start(); err != nil {
		device.Uninit()
		return fmt.Errorf("PC audio device: %w", err)
	}
	e.systemDevice = device
	e.systemReady.Store(true)
	return nil
}

// systemCallback copies the captured PC audio into the ring and publishes a
// peak for the meter. It runs on the device thread and never allocates.
func (e *Engine) systemCallback(_, input []byte, frames uint32) {
	samples := int(frames) * systemAudioChannels
	if len(input) < samples*4 {
		return
	}
	e.systemRing.writeInterleaved(input, samples)
	peak := float32(0)
	for index := 0; index < samples; index++ {
		value := math.Float32frombits(binary.LittleEndian.Uint32(input[index*4:]))
		if value < 0 {
			value = -value
		}
		if value > peak {
			peak = value
		}
	}
	e.systemPeak.Store(math.Float32bits(peak))
}

func (e *Engine) captureSystemTake(record *recordState, frames int) {
	samples := frames * systemAudioChannels
	if len(e.systemBuf) < samples {
		return
	}
	buffer := e.systemBuf[:samples]
	read := e.systemRing.readInto(buffer)
	for index := read; index < samples; index++ {
		buffer[index] = 0
	}
	if read < samples {
		e.systemUnderruns.Add(1)
	}
	record.sink.Offer(buffer)
}

func (e *Engine) recordSystemError(err error) {
	if err == nil {
		e.systemErr.Store(nil)
		return
	}
	message := err.Error()
	e.systemErr.Store(&message)
}

func (e *Engine) systemErrorMessage() string {
	message := e.systemErr.Load()
	if message == nil {
		return ""
	}
	return *message
}
