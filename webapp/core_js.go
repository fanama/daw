//go:build js && wasm

package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"syscall/js"
	"time"

	"fanamix/application/recording"
	sessionapp "fanamix/application/session"
	"fanamix/domain/mix"
	"fanamix/domain/session"
	"fanamix/infrastructure/dsp"
	"fanamix/infrastructure/wavio"
)

// core adapts the session use cases to synchronous JSON calls from the page.
type core struct {
	repo     *browserRepo
	takes    *wavio.MemoryTakes
	mixdowns *wavio.MemoryMixdowns
	service  *sessionapp.Service
	exporter *recording.MixdownExporter
	history  *undoHistory
}

func newCore() *core {
	repo := newBrowserRepo()
	takes := wavio.NewMemoryTakes()
	mixdowns := wavio.NewMemoryMixdowns()
	service := sessionapp.NewService(repo, takes, time.Now)
	return &core{
		repo:     repo,
		takes:    takes,
		mixdowns: mixdowns,
		service:  service,
		exporter: recording.NewMixdownExporterWithProcessor(takes, mixdowns, dsp.Factory{}),
		history:  newUndoHistory(),
	}
}

func (c *core) dispatch(req request) response {
	result := response{OK: true}
	before, track := c.beforeEdit(req.Op)
	var err error
	switch req.Op {
	case "create":
		err = c.service.Create(req.Name, req.SampleRate)
	case "open":
		c.repo.stage(req.Root, req.Project)
		err = c.service.Open(req.Root)
	case "close":
		c.service.Close()
	case "state":
	case "state.undo":
		err = c.undo()
	case "take.begin":
		result.Takes, result.Slots, err = c.beginTakes(req.StartFrame)
	case "export":
		result.Frames, err = c.exportMix()
	case "":
		err = errors.New("missing command")
	default:
		err = c.applyCommand(req)
	}
	if err != nil {
		return response{OK: false, Error: err.Error()}
	}
	if resetsUndo(req.Op) {
		c.history.clear()
	} else if track {
		c.history.record(before, c.currentState())
	}
	if current := c.service.Current(); current != nil {
		result.State = newWebState(c.service.Root(), current)
	}
	return result
}

func (c *core) applyCommand(req request) error {
	switch {
	case strings.HasPrefix(req.Op, "track."):
		return c.applyTrack(req)
	case strings.HasPrefix(req.Op, "note."):
		return c.applyNote(req)
	case strings.HasPrefix(req.Op, "take."):
		return c.applyTake(req)
	case strings.HasPrefix(req.Op, "marker."):
		return c.applyMarker(req)
	case req.Op == "click.set":
		return c.service.SetClick(newClick(req))
	case req.Op == "latency.set":
		return c.service.SetLatencyOffset(req.Latency)
	default:
		return fmt.Errorf("unknown command %q", req.Op)
	}
}

func (c *core) applyTrack(req request) error {
	trackID := session.TrackID(req.TrackID)
	switch req.Op {
	case "track.add":
		_, err := c.service.AddTrackWithSource(req.Name, trackSource(req))
		return err
	case "track.transcribe":
		_, err := c.service.AddTrackWithNotes(req.Name, session.Instrument(req.Instrument), noteSpecs(req.Notes))
		return err
	case "track.rename":
		return c.service.RenameTrack(trackID, req.Name)
	case "track.remove":
		return c.service.RemoveTrack(trackID)
	case "track.arm":
		return c.service.SetArm(trackID, req.Armed)
	case "track.mute":
		return c.service.SetMute(trackID, req.Muted)
	case "track.solo":
		return c.service.SetSolo(trackID, req.Soloed)
	case "track.volume":
		return c.service.SetTrackVolume(trackID, req.VolumeDB)
	case "track.pan":
		return c.service.SetTrackPan(trackID, req.Pan)
	case "track.source":
		return c.service.SetSource(trackID, trackSource(req))
	case "track.instrument":
		return c.service.SetInstrument(trackID, session.Instrument(req.Instrument))
	case "track.repeat":
		return c.service.SetRepeat(trackID, req.Repeat)
	case "track.pattern":
		return c.service.SetPatternFrames(trackID, req.Frames)
	case "track.print":
		return c.service.SetPrintEffects(trackID, req.Print)
	case "track.effects", "track.effect.add", "track.effect.remove", "track.effect.move":
		return c.applyTrackEffect(trackID, req)
	default:
		return fmt.Errorf("unknown track command %q", req.Op)
	}
}

func (c *core) applyTake(req request) error {
	takeID := session.TakeID(req.TakeID)
	switch req.Op {
	case "take.finish":
		return c.service.FinishTake(takeID, req.Frames, req.Incomplete)
	case "take.abort":
		return c.service.AbortTake(takeID)
	case "take.active":
		return c.service.SetActiveTake(session.TrackID(req.TrackID), takeID)
	case "take.mute":
		return c.service.SetTakeMute(takeID, req.Muted)
	case "take.gain":
		return c.service.SetTakeGain(takeID, req.GainDB)
	case "take.trim":
		return c.service.SetTakeTrim(takeID, session.Trim{StartFrame: req.StartFrame, EndFrame: req.EndFrame})
	case "take.move":
		return c.service.MoveTake(takeID, req.StartFrame)
	case "take.split":
		_, err := c.service.SplitTake(takeID, req.Frame)
		return err
	case "take.duplicate":
		_, err := c.service.DuplicateTake(takeID, session.TrackID(req.TrackID), req.StartFrame)
		return err
	case "take.normalize":
		_, err := c.service.NormalizeTake(takeID)
		return err
	case "take.delete":
		return c.service.DeleteTake(takeID)
	default:
		return fmt.Errorf("unknown take command %q", req.Op)
	}
}

func (c *core) applyMarker(req request) error {
	switch req.Op {
	case "marker.add":
		_, err := c.service.AddMarker(req.Frame, req.Name)
		return err
	case "marker.remove":
		return c.service.RemoveMarker(session.MarkerID(req.MarkerID))
	default:
		return fmt.Errorf("unknown marker command %q", req.Op)
	}
}

func (c *core) setEffects(trackID session.TrackID, effects *mix.Effects) error {
	if effects == nil {
		return errors.New("effects are required")
	}
	return c.service.SetEffects(trackID, *effects)
}

func (c *core) beginTakes(startFrame int) ([]webStart, []webSlot, error) {
	takes, plan, err := c.service.BeginTrackTakes(startFrame)
	if err != nil {
		return nil, nil, err
	}
	return newWebStarts(takes, plan), newWebSlots(plan), nil
}

func (c *core) exportMix() (int, error) {
	current := c.service.Current()
	if current == nil {
		return 0, errors.New("no session is open")
	}
	result, err := c.exporter.Export(c.service.Root(), "mixdown.wav", c.service.AudibleTakes(), current.SampleRate())
	if err != nil {
		return 0, err
	}
	return result.Frames, nil
}

func trackSource(req request) session.Source {
	switch req.Source {
	case "system":
		return session.SystemSource()
	case "instrument":
		return session.InstrumentSource(session.Instrument(req.Instrument))
	default:
		return session.InputSource(req.Channel)
	}
}

func newClick(req request) mix.Click {
	return mix.Click{
		Enabled:     req.Enabled,
		BPM:         req.BPM,
		BeatsPerBar: req.BeatsPerBar,
		CountInBars: req.CountInBars,
	}
}

func (c *core) invoke(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return encodeResponse(response{OK: false, Error: "missing request"})
	}
	var req request
	if err := json.Unmarshal([]byte(args[0].String()), &req); err != nil {
		return encodeResponse(response{OK: false, Error: err.Error()})
	}
	return encodeResponse(c.dispatch(req))
}

func encodeResponse(result response) string {
	encoded, err := json.Marshal(result)
	if err != nil {
		return `{"ok":false,"error":"cannot encode response"}`
	}
	return string(encoded)
}

func (c *core) setPCM(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return false
	}
	data := make([]byte, args[1].Get("byteLength").Int())
	js.CopyBytesToGo(data, args[1])
	if len(data)%4 != 0 {
		consoleError(errors.New("take audio is not float32 aligned"))
		return false
	}
	samples := make([]float32, len(data)/4)
	for index := range samples {
		samples[index] = math.Float32frombits(binary.LittleEndian.Uint32(data[index*4:]))
	}
	if err := c.takes.Register(args[0].String(), args[2].Int(), args[3].Int(), samples); err != nil {
		consoleError(err)
		return false
	}
	return true
}

func (c *core) clearPCM(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return false
	}
	c.takes.Forget(args[0].String())
	return true
}

func (c *core) exportWAV(js.Value, []js.Value) any {
	data, err := c.mixdowns.WAV()
	if err != nil {
		consoleError(err)
		return js.Null()
	}
	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	return array
}

func consoleError(err error) {
	js.Global().Get("console").Call("error", "fanamix: "+err.Error())
}
