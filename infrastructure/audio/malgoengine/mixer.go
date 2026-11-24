package malgoengine

import (
	"math"

	"fanamix/domain/studio"
)

// mixPlayback sums the preloaded takes onto the stereo monitor bus at the
// given session position.
func mixPlayback(destination []float32, takes []studio.PlaybackTake, startFrame, frames int) {
	for _, take := range takes {
		gain := float32(math.Pow(10, take.GainDB/20))
		for frame := 0; frame < frames; frame++ {
			position := startFrame + frame - take.StartFrame
			if position < 0 || position >= take.Length {
				continue
			}
			base := position * take.Channels
			if base >= len(take.Samples) {
				continue
			}
			left := take.Samples[base] * gain
			right := left
			if take.Channels > 1 && base+1 < len(take.Samples) {
				right = take.Samples[base+1] * gain
			}
			destination[frame*2] += left
			destination[frame*2+1] += right
		}
	}
}
