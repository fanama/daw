package main

import (
	"time"

	"fanamix/domain/mix"
	"fanamix/domain/session"
)

// webState is the browser view of a session. It is flat and camelCase so the
// JavaScript layer can render it without translating field names.
type webState struct {
	Root          string      `json:"root"`
	Name          string      `json:"name"`
	SampleRate    int         `json:"sampleRate"`
	CreatedAt     string      `json:"createdAt"`
	LatencyOffset int         `json:"latencyOffset"`
	AnySolo       bool        `json:"anySolo"`
	Click         webClick    `json:"click"`
	Tracks        []webTrack  `json:"tracks"`
	Takes         []webTake   `json:"takes"`
	Notes         []webNote   `json:"notes"`
	Markers       []webMarker `json:"markers"`
}

// webClick is the metronome settings in browser shape.
type webClick struct {
	Enabled     bool    `json:"enabled"`
	BPM         float64 `json:"bpm"`
	BeatsPerBar int     `json:"beatsPerBar"`
	CountInBars int     `json:"countInBars"`
}

// webMarker is one named timeline position in browser shape.
type webMarker struct {
	ID    string `json:"id"`
	Frame int    `json:"frame"`
	Name  string `json:"name"`
}

// webNote is one timed MIDI note in browser shape.
type webNote struct {
	ID           string `json:"id"`
	TrackID      string `json:"trackId"`
	Pitch        int    `json:"pitch"`
	StartFrame   int    `json:"startFrame"`
	LengthFrames int    `json:"lengthFrames"`
	Velocity     int    `json:"velocity"`
}

// webTrack is one track in browser shape.
type webTrack struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Source        string      `json:"source"`
	Channel       int         `json:"channel"`
	Instrument    string      `json:"instrument,omitempty"`
	Armed         bool        `json:"armed"`
	Muted         bool        `json:"muted"`
	Soloed        bool        `json:"soloed"`
	VolumeDB      float64     `json:"volumeDb"`
	Pan           float64     `json:"pan"`
	PrintEffects  bool        `json:"printEffects"`
	ActiveTakeID  string      `json:"activeTakeId"`
	Repeat        int         `json:"repeat"`
	PatternFrames int         `json:"patternFrames"`
	Effects       mix.Effects `json:"effects"`
}

// webTake is one take in browser shape.
type webTake struct {
	ID           string  `json:"id"`
	TrackID      string  `json:"trackId"`
	File         string  `json:"file"`
	StartFrame   int     `json:"startFrame"`
	LengthFrames int     `json:"lengthFrames"`
	Channels     int     `json:"channels"`
	SampleRate   int     `json:"sampleRate"`
	Printed      bool    `json:"printed"`
	Incomplete   bool    `json:"incomplete"`
	Muted        bool    `json:"muted"`
	Layered      bool    `json:"layered"`
	Trim         webTrim `json:"trim"`
	GainDB       float64 `json:"gainDb"`
}

// webTrim is the non-destructive in/out point pair.
type webTrim struct {
	StartFrame int `json:"startFrame"`
	EndFrame   int `json:"endFrame"`
}

// webSlot is one channel of the take the browser is about to capture.
type webSlot struct {
	TrackID  string `json:"trackId"`
	Kind     string `json:"kind"`
	Channel  int    `json:"channel"`
	Channels int    `json:"channels"`
}

// webStart is one per-track take the browser is about to fill.
type webStart struct {
	ID       string `json:"id"`
	File     string `json:"file"`
	TrackID  string `json:"trackId"`
	Channel  int    `json:"channel"`
	Channels int    `json:"channels"`
}

// request is one command from the page. A flat shape keeps the bridge small;
// each command reads only the fields it documents.
type request struct {
	Op           string       `json:"op"`
	Name         string       `json:"name"`
	SampleRate   int          `json:"sampleRate"`
	Root         string       `json:"root"`
	Project      string       `json:"project"`
	TrackID      string       `json:"trackId"`
	TakeID       string       `json:"takeId"`
	NoteID       string       `json:"noteId"`
	MarkerID     string       `json:"markerId"`
	Frame        int          `json:"frame"`
	Source       string       `json:"source"`
	Channel      int          `json:"channel"`
	Instrument   string       `json:"instrument"`
	Pitch        int          `json:"pitch"`
	Velocity     int          `json:"velocity"`
	LengthFrames int          `json:"lengthFrames"`
	Armed        bool         `json:"armed"`
	Muted        bool         `json:"muted"`
	Soloed       bool         `json:"soloed"`
	Print        bool         `json:"print"`
	Enabled      bool         `json:"enabled"`
	BPM          float64      `json:"bpm"`
	BeatsPerBar  int          `json:"beatsPerBar"`
	CountInBars  int          `json:"countInBars"`
	Latency      int          `json:"latency"`
	GainDB       float64      `json:"gainDb"`
	VolumeDB     float64      `json:"volumeDb"`
	Pan          float64      `json:"pan"`
	StartFrame   int          `json:"startFrame"`
	EndFrame     int          `json:"endFrame"`
	Frames       int          `json:"frames"`
	Incomplete   bool         `json:"incomplete"`
	Kind         string       `json:"kind"`
	Index        int          `json:"index"`
	From         int          `json:"from"`
	To           int          `json:"to"`
	Repeat       int          `json:"repeat"`
	Effects      *mix.Effects `json:"effects"`
}

// response is the command result and the resulting session view.
type response struct {
	OK     bool       `json:"ok"`
	Error  string     `json:"error,omitempty"`
	State  *webState  `json:"state,omitempty"`
	Takes  []webStart `json:"takes,omitempty"`
	Slots  []webSlot  `json:"slots,omitempty"`
	Frames int        `json:"frames,omitempty"`
}

func newWebState(root string, current *session.Session) *webState {
	state := current.State()
	view := &webState{
		Root:          root,
		Name:          state.Name,
		SampleRate:    state.SampleRate,
		CreatedAt:     state.CreatedAt.Format(time.RFC3339),
		LatencyOffset: state.LatencyOffset,
		AnySolo:       current.HasSolo(),
		Click:         newWebClick(state.Click),
		Tracks:        make([]webTrack, 0, len(state.Tracks)),
		Takes:         make([]webTake, 0, len(state.Takes)),
		Notes:         make([]webNote, 0, len(state.Notes)),
		Markers:       make([]webMarker, 0, len(state.Markers)),
	}
	for _, track := range state.Tracks {
		view.Tracks = append(view.Tracks, newWebTrack(track))
	}
	for _, take := range state.Takes {
		view.Takes = append(view.Takes, newWebTake(take))
	}
	for _, note := range state.Notes {
		view.Notes = append(view.Notes, newWebNote(note))
	}
	for _, marker := range state.Markers {
		view.Markers = append(view.Markers, webMarker{
			ID:    marker.ID.String(),
			Frame: marker.Frame,
			Name:  marker.Name,
		})
	}
	return view
}

func newWebClick(click mix.Click) webClick {
	return webClick{
		Enabled:     click.Enabled,
		BPM:         click.BPM,
		BeatsPerBar: click.BeatsPerBar,
		CountInBars: click.CountInBars,
	}
}

func newWebTrack(track session.TrackState) webTrack {
	source := "input"
	if track.Source.IsSystem() {
		source = "system"
	}
	if track.Source.IsInstrument() {
		source = "instrument"
	}
	return webTrack{
		ID:            track.ID.String(),
		Name:          track.Name,
		Source:        source,
		Channel:       track.InputChannel,
		Instrument:    string(track.Source.Instrument),
		Armed:         track.Armed,
		Muted:         track.Muted,
		Soloed:        track.Soloed,
		VolumeDB:      track.VolumeDB,
		Pan:           track.Pan,
		PrintEffects:  track.PrintEffects,
		ActiveTakeID:  track.ActiveTakeID.String(),
		Repeat:        track.Repeat,
		PatternFrames: track.PatternFrames,
		Effects:       track.Effects,
	}
}

func newWebNote(note session.NoteState) webNote {
	return webNote{
		ID:           note.ID.String(),
		TrackID:      note.TrackID.String(),
		Pitch:        note.Pitch,
		StartFrame:   note.StartFrame,
		LengthFrames: note.LengthFrames,
		Velocity:     note.Velocity,
	}
}

func newWebTake(take session.TakeState) webTake {
	return webTake{
		ID:           take.ID.String(),
		TrackID:      take.TrackID.String(),
		File:         take.File,
		StartFrame:   take.StartFrame,
		LengthFrames: take.LengthFrames,
		Channels:     take.Channels,
		SampleRate:   take.SampleRate,
		Printed:      take.Printed,
		Incomplete:   take.Incomplete,
		Muted:        take.Muted,
		Layered:      take.Layered,
		Trim:         webTrim{StartFrame: take.Trim.StartFrame, EndFrame: take.Trim.EndFrame},
		GainDB:       take.GainDB,
	}
}

func newWebStarts(takes []session.Take, plan []session.ArmSlot) []webStart {
	starts := make([]webStart, 0, len(takes))
	for index, take := range takes {
		channel := -1
		if index < len(plan) && !plan[index].Source.IsSystem() {
			channel = plan[index].Source.Channel
		}
		starts = append(starts, webStart{
			ID:       take.ID().String(),
			File:     take.File(),
			TrackID:  take.TrackID().String(),
			Channel:  channel,
			Channels: take.Channels(),
		})
	}
	return starts
}

func newWebSlots(plan []session.ArmSlot) []webSlot {
	slots := make([]webSlot, 0, len(plan))
	for _, slot := range plan {
		if slot.Source.IsSystem() {
			slots = append(slots, webSlot{TrackID: slot.TrackID.String(), Kind: "system", Channel: -1, Channels: 2})
			continue
		}
		slots = append(slots, webSlot{TrackID: slot.TrackID.String(), Kind: "input", Channel: slot.Source.Channel, Channels: 1})
	}
	return slots
}
