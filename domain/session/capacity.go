package session

import "time"

// MaxTakeBytes is the conservative RIFF ceiling the recorder refuses to cross.
const MaxTakeBytes = 4_000_000_000

const bytesPerSample = 4

// CapacityLevel ranks how close a take is to the file-size ceiling.
type CapacityLevel int

const (
	// CapacityOK is below 75 percent of the ceiling.
	CapacityOK CapacityLevel = iota
	// CapacityWarn is at or above 75 percent.
	CapacityWarn
	// CapacityCritical is at or above 90 percent.
	CapacityCritical
	// CapacityFull means recording must stop.
	CapacityFull
)

// BytesPerFrame returns the size of one interleaved frame.
func BytesPerFrame(channels int) int {
	if channels < 1 {
		return 0
	}
	return channels * bytesPerSample
}

// TakeFrameLimit returns how many frames fit under the ceiling.
func TakeFrameLimit(channels int) int {
	bytesPerFrame := BytesPerFrame(channels)
	if bytesPerFrame == 0 {
		return 0
	}
	return MaxTakeBytes / bytesPerFrame
}

// RemainingFrames returns how many more frames can be written.
func RemainingFrames(written, channels int) int {
	remaining := TakeFrameLimit(channels) - written
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CapacityFor classifies a take that has written the given number of frames.
func CapacityFor(written, channels int) CapacityLevel {
	limit := TakeFrameLimit(channels)
	if limit == 0 {
		return CapacityFull
	}
	switch {
	case written >= limit*95/100:
		return CapacityFull
	case written >= limit*90/100:
		return CapacityCritical
	case written >= limit*75/100:
		return CapacityWarn
	default:
		return CapacityOK
	}
}

// RemainingDuration converts leftover frames into wall-clock time.
func RemainingDuration(written, channels, sampleRate int) time.Duration {
	if sampleRate <= 0 {
		return 0
	}
	frames := RemainingFrames(written, channels)
	return time.Duration(float64(frames) / float64(sampleRate) * float64(time.Second))
}
