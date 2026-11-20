// Package malgoengine is the miniaudio adapter for the studio context: device
// discovery and the real-time duplex engine.
package malgoengine

import (
	"github.com/gen2brain/malgo"
	"gorecord/domain/studio"
)

// Catalog lists audio endpoints through a short-lived miniaudio context.
type Catalog struct{}

// NewCatalog returns the device-discovery adapter.
func NewCatalog() *Catalog { return &Catalog{} }

// CaptureDevices lists recording endpoints.
func (c *Catalog) CaptureDevices() ([]studio.DeviceInfo, error) {
	return c.list(malgo.Capture)
}

// PlaybackDevices lists monitoring endpoints.
func (c *Catalog) PlaybackDevices() ([]studio.DeviceInfo, error) {
	return c.list(malgo.Playback)
}

func (c *Catalog) list(kind malgo.DeviceType) ([]studio.DeviceInfo, error) {
	context, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = context.Uninit()
		context.Free()
	}()
	infos, err := context.Devices(kind)
	if err != nil {
		return nil, err
	}
	devices := make([]studio.DeviceInfo, 0, len(infos))
	for index := range infos {
		info := &infos[index]
		devices = append(devices, studio.DeviceInfo{
			ID:          info.ID.String(),
			Name:        info.Name(),
			MaxChannels: maxChannels(info),
			IsDefault:   info.IsDefault != 0,
		})
	}
	return devices, nil
}

func maxChannels(info *malgo.DeviceInfo) int {
	maximum := 0
	for _, format := range info.Formats {
		if channels := int(format.Channels); channels > maximum {
			maximum = channels
		}
	}
	return maximum
}
