// Package studio is the bounded context for the audio hardware: device
// discovery, the real-time engine port and its telemetry. It knows nothing
// about sessions, persistence or any audio library.
package studio

// DeviceInfo describes one capture or playback endpoint.
type DeviceInfo struct {
	ID          string
	Name        string
	MaxChannels int
	IsDefault   bool
}

// DeviceCatalog is the outbound port for discovering audio hardware.
type DeviceCatalog interface {
	CaptureDevices() ([]DeviceInfo, error)
	PlaybackDevices() ([]DeviceInfo, error)
}

// DeviceConfig is the negotiated stream configuration.
type DeviceConfig struct {
	CaptureDeviceID     string
	PlaybackDeviceID    string
	SystemAudioDeviceID string
	SampleRate          int
	PeriodFrames        int
	Periods             int
	Channels            int
}

// LatencyMs reports the software buffer latency implied by the configuration.
func (c DeviceConfig) LatencyMs() float64 {
	if c.SampleRate <= 0 {
		return 0
	}
	periods := c.Periods
	if periods < 1 {
		periods = 1
	}
	return float64(periods*c.PeriodFrames) / float64(c.SampleRate) * 1000
}

// Valid reports whether the configuration can be opened.
func (c DeviceConfig) Valid() bool {
	return c.SampleRate > 0 && c.PeriodFrames > 0 && c.Channels > 0
}

// HasSystemAudio reports whether a PC audio source was configured.
func (c DeviceConfig) HasSystemAudio() bool { return c.SystemAudioDeviceID != "" }
