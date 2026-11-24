package recording

import (
	"errors"
	"math"

	"fanamix/domain/mix"
	"fanamix/domain/session"
	"fanamix/domain/studio"
)

const exportChunkFrames = 8192

// ExportResult reports what an export wrote.
type ExportResult struct {
	Path       string
	Frames     int
	Channels   int
	SampleRate int
}

// MixdownExporter renders the audible takes into one stereo WAV file. Like the
// monitor bus it applies mute, solo, active-take selection, take gain, track
// effects and trim, then soft-clips the sum.
type MixdownExporter struct {
	loader *Loader
	files  session.Mixdowns
}

// NewMixdownExporter wires the exporter to take audio and mixdown files without
// processing effects.
func NewMixdownExporter(takes session.TakeFiles, files session.Mixdowns) *MixdownExporter {
	return &MixdownExporter{loader: NewLoader(takes), files: files}
}

// NewMixdownExporterWithProcessor bakes each track's effect chain into the
// rendered mix so the download matches what playback sounds like.
func NewMixdownExporterWithProcessor(takes session.TakeFiles, files session.Mixdowns, factory mix.ProcessorFactory) *MixdownExporter {
	return &MixdownExporter{loader: NewLoaderWithProcessor(takes, factory), files: files}
}

// Export mixes the audible takes into path as a stereo float WAV.
func (e *MixdownExporter) Export(root, path string, audible []session.AudibleTake, sampleRate int) (ExportResult, error) {
	if len(audible) == 0 {
		return ExportResult{}, errors.New("nothing to export: record or select a take first")
	}
	if sampleRate <= 0 {
		return ExportResult{}, errors.New("export needs a valid sample rate")
	}
	mix, err := e.loader.Load(root, audible)
	if err != nil {
		return ExportResult{}, err
	}
	total := mixdownFrames(mix.Takes)
	if total == 0 {
		return ExportResult{}, errors.New("nothing to export: the selected takes are empty")
	}
	file, err := e.files.Create(path, 2, sampleRate)
	if err != nil {
		return ExportResult{}, err
	}
	if err := writeMixdown(file, mix.Takes, total); err != nil {
		_ = file.Finish()
		return ExportResult{}, err
	}
	return ExportResult{Path: path, Frames: total, Channels: 2, SampleRate: sampleRate}, nil
}

func writeMixdown(file session.MixdownFile, takes []studio.PlaybackTake, total int) error {
	buffer := make([]float32, exportChunkFrames*2)
	for frame := 0; frame < total; frame += exportChunkFrames {
		frames := total - frame
		if frames > exportChunkFrames {
			frames = exportChunkFrames
		}
		chunk := buffer[:frames*2]
		clear(chunk)
		mixTakes(chunk, takes, frame, frames)
		softClip(chunk)
		if _, err := file.Write(chunk); err != nil {
			return err
		}
	}
	return file.Finish()
}

func mixTakes(destination []float32, takes []studio.PlaybackTake, startFrame, frames int) {
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

func softClip(samples []float32) {
	for index, sample := range samples {
		samples[index] = float32(math.Tanh(float64(sample)))
	}
}

func mixdownFrames(takes []studio.PlaybackTake) int {
	end := 0
	for _, take := range takes {
		if takeEnd := take.StartFrame + take.Length; takeEnd > end {
			end = takeEnd
		}
	}
	return end
}
