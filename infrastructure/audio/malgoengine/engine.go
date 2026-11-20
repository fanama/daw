package malgoengine

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/gen2brain/malgo"
	"gorecord/domain/mix"
	"gorecord/domain/studio"
	"gorecord/infrastructure/dsp"
)

const (
	defaultPeriodFrames = 256
	scratchFrames       = 8192
	playbackChannels    = 2
	tunerBufferSamples  = 8192
)

type playbackState struct {
	mix            studio.PlaybackMix
	startedAtFrame int
}

type recordState struct {
	channels     []int
	printEffects []bool
	sink         studio.FrameSink
	startFrame   int
	system       bool
}

// Engine implements the studio.AudioEngine port on top of miniaudio.
type Engine struct {
	factory mix.ProcessorFactory

	mu           sync.Mutex
	context      *malgo.AllocatedContext
	device       *malgo.Device
	systemDevice *malgo.Device
	config       studio.DeviceConfig
	chains       []mix.Processor

	monitor       atomic.Bool
	monitorGain   float32
	playhead      atomic.Int64
	recordedFrame atomic.Int64
	state         atomic.Int32
	deviceStopped atomic.Bool
	clickSettings atomic.Pointer[mix.Click]
	playback      atomic.Pointer[playbackState]
	recording     atomic.Pointer[recordState]

	peaks    []atomic.Uint32
	rms      []atomic.Uint32
	pitchHz  atomic.Uint64
	pitchHas atomic.Bool

	systemReady     atomic.Bool
	systemPeak      atomic.Uint32
	systemUnderruns atomic.Int64
	systemErr       atomic.Pointer[string]
	systemRing      *sampleRing
	systemBuf       []float32

	dry        [][]float32
	wet        [][]float32
	channelOut [][]float32
	recordBuf  []float32
	mixBuf     []float32
	clickBuf   []float32

	metronome *dsp.Metronome
	lastClick mix.Click

	tunerSamples []atomic.Uint32
	tunerWrite   atomic.Int64
	tunerDone    chan struct{}
}

// NewEngine creates the engine with the DSP factory it should use.
func NewEngine(factory mix.ProcessorFactory) *Engine {
	if factory == nil {
		factory = dsp.Factory{}
	}
	return &Engine{
		factory:      factory,
		metronome:    dsp.NewMetronome(),
		lastClick:    mix.Click{BPM: -1},
		tunerSamples: make([]atomic.Uint32, tunerBufferSamples),
	}
}

var _ studio.AudioEngine = (*Engine)(nil)

// Open negotiates and starts a duplex device.
func (e *Engine) Open(config studio.DeviceConfig) error {
	if !config.Valid() {
		return errors.New("audio engine: incomplete device configuration")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closeLocked()
	return e.openLocked(normalizeConfig(config))
}

// Close stops the device and releases the backend.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closeLocked()
	return nil
}

func (e *Engine) openLocked(config studio.DeviceConfig) error {
	context, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return err
	}
	captureID, playbackID, err := resolveDeviceIDs(context, config)
	if err != nil {
		uninitContext(context)
		return err
	}
	e.allocateScratch(config)
	e.config = config
	device, err := malgo.InitDevice(context.Context, duplexConfig(config, captureID, playbackID), e.callbacks())
	if err != nil {
		uninitContext(context)
		return err
	}
	if err := device.Start(); err != nil {
		device.Uninit()
		uninitContext(context)
		return err
	}
	e.context = context
	e.device = device
	e.deviceStopped.Store(false)
	e.state.Store(int32(studio.StateIdle))
	if err := e.openSystemDevice(config); err != nil {
		e.recordSystemError(err)
	}
	e.startTuner()
	return nil
}

func (e *Engine) closeLocked() {
	if e.tunerDone != nil {
		close(e.tunerDone)
		e.tunerDone = nil
	}
	if e.device != nil {
		_ = e.device.Stop()
		e.device.Uninit()
		e.device = nil
	}
	if e.systemDevice != nil {
		_ = e.systemDevice.Stop()
		e.systemDevice.Uninit()
		e.systemDevice = nil
	}
	e.systemReady.Store(false)
	e.systemPeak.Store(0)
	if e.context != nil {
		_ = e.context.Uninit()
		e.context.Free()
		e.context = nil
	}
	e.recording.Store(nil)
	e.playback.Store(nil)
	e.state.Store(int32(studio.StateIdle))
}
