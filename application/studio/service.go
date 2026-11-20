// Package studioapp exposes device discovery and transport control to the
// delivery layer without leaking the audio library.
package studioapp

import (
	"errors"

	"gorecord/domain/mix"
	"gorecord/domain/studio"
)

// Service is the inbound port for the audio hardware.
type Service struct {
	engine  studio.AudioEngine
	catalog studio.DeviceCatalog
	config  studio.DeviceConfig
	isOpen  bool
}

// NewService wires the studio use cases to their ports.
func NewService(engine studio.AudioEngine, catalog studio.DeviceCatalog) *Service {
	return &Service{engine: engine, catalog: catalog}
}

// CaptureDevices lists recording endpoints.
func (s *Service) CaptureDevices() ([]studio.DeviceInfo, error) {
	return s.catalog.CaptureDevices()
}

// PlaybackDevices lists monitoring endpoints.
func (s *Service) PlaybackDevices() ([]studio.DeviceInfo, error) {
	return s.catalog.PlaybackDevices()
}

// Open starts the duplex device with the given configuration.
func (s *Service) Open(config studio.DeviceConfig) error {
	if !config.Valid() {
		return errors.New("select a device, sample rate and buffer size first")
	}
	if err := s.engine.Open(config); err != nil {
		return err
	}
	s.config = config
	s.isOpen = true
	return nil
}

// Close stops the device.
func (s *Service) Close() error {
	s.isOpen = false
	return s.engine.Close()
}

// IsOpen reports whether the device is running.
func (s *Service) IsOpen() bool { return s.isOpen }

// Config returns the negotiated configuration.
func (s *Service) Config() studio.DeviceConfig { return s.config }

// Snapshot returns the latest engine telemetry.
func (s *Service) Snapshot() studio.Snapshot { return s.engine.Snapshot() }

// SetMonitor enables or disables the monitor bus.
func (s *Service) SetMonitor(enabled bool) { s.engine.SetMonitor(enabled) }

// SetClick forwards the click configuration.
func (s *Service) SetClick(click mix.Click) { s.engine.SetClick(click) }

// SetChains publishes the per-channel chains.
func (s *Service) SetChains(chains []studio.ChainSettings) { s.engine.SetChains(chains) }

// Play starts the monitor transport at a position.
func (s *Service) Play(mix studio.PlaybackMix, fromFrame int) error {
	return s.engine.StartPlayback(mix, fromFrame)
}

// Stop halts the transport, keeping any recording intact.
func (s *Service) Stop() error { return s.engine.Stop() }
