package mix

// Processor is an outbound port for the real-time effect chain of one input
// channel. Implementations must be allocation-free and lock-free once
// Prepare has been called, because they run inside the audio callback.
type Processor interface {
	// Prepare allocates every buffer the processor needs.
	Prepare(sampleRate int)
	// SetEffects publishes a new configuration; it is safe to call from the
	// control goroutine and must never block the audio thread.
	SetEffects(effects Effects)
	// ProcessMono filters a mono block in place.
	ProcessMono(samples []float32)
	// GainReductionDB reports the current compressor gain reduction.
	GainReductionDB() float32
	// Reset clears filter and delay state.
	Reset()
}

// ProcessorFactory is the port the audio adapter uses to create processors
// without knowing which DSP implementation is wired.
type ProcessorFactory interface {
	NewProcessor() Processor
}
