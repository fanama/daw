# gorecord — Specification

**Status:** draft, pre-implementation
**Working title:** `gorecord` (rename freely; it is only used for the module path and binary name here)
**Date:** 2026-11-20
**Author:** interview-driven, based on the user's request and five rounds of clarifying questions

> **Implementation note:** the Fyne desktop shell described in this spec has been retired. The shipping app is the browser build (`webapp/` + `web/`), which `cmd/gorecord` opens in a dedicated Chromium app window (with a default-browser fallback); the sections below remain as the original design record.

> One sentence: a cross-platform Go desktop app for recording a microphone and electric guitar through an audio interface into layered, timeline-aligned takes — with a live amp/EQ/comp/reverb monitor chain, a tuner, and an optional click — organized into one session folder per song idea.

---

## 1. Problem and product thesis

Songwriting ideas arrive as fragments: a riff played once, a vocal line hummed over it, a second guitar part. Capturing them today means either opening a full DAW (heavy, slow, project-file ceremony) or using a phone voice memo (no layering, no monitoring, no alignment).

`gorecord` is the middle ground: press record and play, hear yourself processed, then immediately layer another take against what you just recorded, and keep the whole thing as one session folder you can hand to a real DAW later. It is deliberately **not** a DAW: no comping, no MIDI, no plugin hosting, and no destructive iteration on recorded audio.

The app must feel like a musical instrument: live monitoring under 10 ms, no clicks, and no lost audio ever.

---

## 2. Goals

| # | Goal |
|---|------|
| G1 | Record from an audio interface's microphone and instrument inputs, with an arbitrary channel count and per-channel arm/mute. |
| G2 | Live, processed monitoring (amp → EQ → compressor → reverb) with under ~10 ms total latency, while recording the dry signal by default. |
| G3 | Layer takes in time: each take stores a sample-accurate start position relative to a session timeline and plays back aligned with earlier takes. |
| G4 | Never lose audio: crash, unplug or disk error leaves a playable, recoverable take on disk. |
| G5 | Store everything as one self-describing session folder (project file + WAV takes) that any DAW can open. |
| G6 | Run natively on macOS, Windows and Linux from one Go codebase, installed by musicians, published as an MIT-licensed open-source project. |
| G7 | Ship a tuner and an optional click, because both are part of "playing in tune and in time" rather than editing. |

## 3. Non-goals (explicitly out of scope for v1)

- MIDI input/output, virtual instruments, drum machines.
- VST/AU/CLAP/LV2 plugin hosting — the effects chain is the app's own DSP.
- Comping, punch-in/punch-out, crossfades, automation lanes.
- Pitch correction, time-stretch or tempo mapping.
- Neural amp models (`.nam`), cabinet impulse responses, convolution reverb.
- Cloud sync, collaboration, mobile, video.
- Non-destructive editing beyond trim, gain, mute/solo (see §9.9 for what is allowed).
- More than one interface at a time (no aggregate/multi-device recording).

---

## 4. Confirmed decisions

These were chosen in the interview and are binding for v1.

| Area | Decision |
|---|---|
| Primary use | Session recorder for songwriting demos (not quick memos, not full multitrack production) |
| Inputs | Arbitrary channel count discovered at runtime, with **per-channel arm/mute toggles** |
| Overdub behavior | Layers **automatically align on a timeline** (shared transport, stored start offsets) |
| UI stack | **Fyne v2** — pure Go widgets, single toolchain, no JS |
| Monitoring | **Full duplex**: live processed monitoring *and* playback of earlier takes |
| Click | **Optional toggle, off by default** |
| File format | **WAV, 32-bit float IEEE**, one **multichannel** file per take |
| Storage | **One session folder per song**, project file inside, takes in a subfolder |
| Long takes | **Cap at 4 GB** with a clear warning as the limit approaches (no RF64, no splitting) |
| Platforms | **macOS + Windows + Linux all supported**, not best-effort |
| Distribution | **Open-source public release** with signed macOS builds and CI artifacts per OS |
| DSP | Reverb, parametric EQ, amp, compressor, tuner — all per-track |
| Amp model | **Simple drive + tone stack in pure Go** (no neural models) |
| Cabinet | **Fixed speaker-emulation filter**, no IR loading; chain designed so convolution could be added later |
| Effects printing | **Per-track toggle**: record dry, or bake the processed signal into the file |
| Latency target | **Under ~10 ms total** monitoring |
| Sample rate | **Follow the interface's native rate**; stored in the project |
| Takes | **Stack of takes per track; one audible, the rest muted** |
| Failure policy | **Never lose audio** — keep partial takes, warn loudly |
| Recording display | **Live scrolling waveform + per-channel meters** |
| License | **MIT** |

---

## 5. Primary user workflows

### W1 — Start a song idea
1. App opens with a session picker (recent sessions + "New session").
2. New session asks only for a name; sample rate and channel layout are taken from the selected interface.
3. Track 1 "Guitar" is created with input channel N armed; the tuner is one click away.
4. User hits spacebar/record; a take records; on stop it is kept and shown on the timeline.

### W2 — Layer a part
1. The finished take plays from session zero.
2. Arm a second channel (e.g. mic), choose its effects (compressor + EQ, no amp).
3. Press record; playback continues from the top; the new take's start offset is stored so it lines up sample-accurately.
4. Repeat for a third part; mute/solo earlier tracks to listen.

### W3 — Practice tone
1. Arm the guitar channel, enable monitor with amp + cab + reverb.
2. Play; adjust drive/tone/EQ while listening (parameter changes must be click-free).
3. Record only if the idea is worth keeping; the recording stays dry unless "print effects" is on for that track.

### W4 — Recover from an accident
1. USB cable is pulled mid-take; the engine stops, the UI shows a clear error.
2. The partial take is finalized on disk and marked **incomplete** with its actual duration.
3. It stays in the take stack, flagged in red, playable as-is.

---

## 6. Functional requirements

Identifiers are stable so they can be referenced from issues and tests.

### 6.1 Audio devices

- **FR-1** Enumerate capture and playback devices at startup and on demand ("Rescan"), using `gen2brain/malgo`. Show the system default device first and mark it.
- **FR-2** Show, for the selected capture device, its name, native sample rate and channel count.
- **FR-3** Selecting a device opens a duplex device on the **same** interface for capture and monitoring playback whenever possible, so capture and playback share one clock. If the user picks a separate playback device, monitoring is disabled with an explicit warning (see §8.4 clock-domain rule) but pure playback still works.
- **FR-4** Channel count is read from the device at open time. Tracks map to input channels; a track remembers its channel index, so a re-opened device with the same layout keeps working.
- **FR-5** Device loss (unplug, format change, sleep/wake) is a handled event, not a crash: stop transport, keep the partial take, surface a persistent banner with a "Reconnect" action.
- **FR-6** The app must never silently record silence because permission was denied. On macOS, the first capture attempt verifies input is actually non-zero for ~1 s and, if not, points the user at System Settings → Privacy → Microphone.

### 6.2 Channels and arming

- **FR-7** Each track has: name (editable), input channel index, **armed** toggle, mute, solo, gain (see FR-24), and an effects chain.
- **FR-8** Multiple tracks may be armed simultaneously; one take records all armed tracks' channels as a single interleaved multichannel WAV (see §7.3).
- **FR-9** Channels not armed are not written to the file. Their channel slot is absent, not silent, and the project records the channel map explicitly.
- **FR-10** Arming/disarming during a take is not supported in v1; the arm state is latched when recording starts (buttons disabled while rolling).

### 6.3 Transport and recording

- **FR-11** Transport: Record, Play, Stop, and a session-zero-relative position readout (mm:ss.mmm) plus bars/beats when a click tempo is set.
- **FR-12** Recording starts at the current transport position. The engine records the **actual** first sample frame it captured relative to session zero (not the requested position), so alignment survives scheduling jitter.
- **FR-13** Stopping finalizes the WAV header and creates a take entry with duration, peak data and "complete" status.
- **FR-14** While rolling, earlier audible takes stream from disk and are mixed into the monitor bus (see §8.5).
- **FR-15** A **record latency offset** (in samples or ms, signed) is a per-app setting applied to a take's stored start position, so a user can compensate for their interface's round-trip delay. Default 0; the UI shows the computed offset it will apply.
- **FR-16** Recording stops automatically when the file approaches the 4 GB ceiling (see §7.4) and the take is finalized, not corrupted.

### 6.4 Monitoring and effects

- **FR-17** Monitor path: input channel → (amp drive/tone → cab filter) → parametric EQ → compressor → [dry/wet] reverb → output, per track, mixable into the duplex output.
- **FR-18** The **effect chain is per track**, with the fixed order above in v1. Slots can be individually bypassed; reordering is out of scope.
- **FR-19** Parameter changes from the UI must not click or block: they are published to the audio thread atomically and applied at block boundaries, with smoothing where a discontinuity would be audible (gain, drive, mix).
- **FR-20** Per-track **"print effects"** toggle: when enabled, the take file contains the processed signal; when disabled, the dry input. The monitor always hears the processed signal regardless.
- **FR-21** Monitoring is independently switchable (global monitor on/off), because hardware/direct monitoring is often preferable and is lower latency.
- **FR-22** Reverb may be monitor-only in v1 if printing it proves click-prone; this is a documented fallback, not a silent behavior change (see §14 Q3).

### 6.5 Tuner

- **FR-23** A chromatic tuner reads the **pre-effects** signal of the selected channel at ~10–20 Hz update, showing note name, cents deviation (±1 cent), and a tuning indicator. It runs outside the audio callback on a downsampled copy of the input.
- No recording required; tuner works whenever a device is open.

### 6.6 Take and track management

- **FR-24** A take's audible state is governed by its track's mute/solo plus its own mute. Only one take per track is **active** (audible); the take stack shows the others greyed with a "make active" action.
- **FR-25** Takes can be renamed, deleted (with confirmation), and played in isolation ("preview").
- **FR-26** Tracks can be added, renamed, deleted, and reordered.
- **FR-27** Trim (start/end) and per-take gain are allowed and are **non-destructive**: stored as project parameters, applied on playback and on any exported copy; the recorded file is never rewritten.
- **FR-28** No undo stack in v1; destructive operations (delete take/track) require confirmation and move files to a `trash/` subfolder inside the session rather than deleting them immediately.

### 6.7 Click

- **FR-29** Optional click, **off by default**, generated in the audio thread, with BPM (20–300) and 4/4 default time signature, accented downbeat, and an optional 1-bar count-in.
- **FR-30** Click is never written into recordings.
- **FR-31** Click and monitoring mix independently: click audible in output only.

### 6.8 Persistence and recovery

- **FR-32** The project file is human-readable JSON, saved on every structural change and on a timer while recording; a take's audio is never inside the JSON.
- **FR-33** Audio is flushed to disk continuously (see §8.7) so an abnormal exit leaves usable data.
- **FR-34** On launch, a **repair pass** scans for unfinished takes (`.wav.part`), recomputes their data size from file length, writes a valid header, and lists them as recovered with a banner.
- **FR-35** Session data is never deleted implicitly: deleting a take or track moves files to `trash/`, and the app offers "empty trash" explicitly.
- **FR-36** A session opened with a device whose rate/layout differs from the project's shows a warning and requires either matching the device or starting a new take at a different rate (which v1 refuses — see §14 Q5).

---

## 7. Data model and file formats

### 7.1 Session folder layout

```
~/Music/gorecord/<Session Name>/
├── project.json                     # session + tracks + takes + settings
├── takes/
│   ├── take-001.wav                 # 32-bit float, N channels, session rate
│   ├── take-002.wav
│   └── take-003.wav.part            # present only after a crash; repaired on launch
├── peaks/
│   ├── take-001.pk                  # min/max envelope cache for fast waveform drawing
│   └── ...
├── trash/                           # soft-deleted takes, emptied explicitly
└── session.log                      # plain-text log: device changes, xruns, errors
```

Rules:

- Folder name is user-facing; `project.json` inside is authoritative. There must be no absolute paths in the JSON — takes are referenced relative to the session root, so a session folder can be moved or shared.
- A session is created by "New session" only; the app does not auto-create sessions on launch.

### 7.2 Project schema (`project.json`, versioned)

```jsonc
{
  "schemaVersion": 1,
  "app": { "name": "gorecord", "version": "0.1.0" },
  "session": {
    "name": "Riff idea 3",
    "createdAt": "2026-11-20T10:12:00Z",
    "sampleRate": 48000,          // device-native at creation
    "channelLayout": ["Guitar", "Mic"],   // names of recorded channels, in file order
    "transportZero": 0,           // frames; session zero is always 0 in v1
    "click": { "enabled": false, "bpm": 120, "beatsPerBar": 4, "countInBars": 0 },
    "recordLatencyOffset": 0      // samples, signed
  },
  "tracks": [
    {
      "id": "trk_1",
      "name": "Guitar",
      "inputChannel": 0,          // device channel index at record time
      "armed": true,
      "mute": false,
      "solo": false,
      "printEffects": false,
      "effects": {
        "amp":   { "enabled": true,  "drive": 0.4, "bass": 0.0, "mid": 0.0, "treble": 0.0, "level": 0.8 },
        "cab":   { "enabled": true },
        "eq":    { "enabled": true,  "bands": [ { "freq": 100, "q": 0.7, "gain": 0.0 } ] },
        "comp":  { "enabled": false, "threshold": -18, "ratio": 3.0, "attackMs": 10, "releaseMs": 120, "makeup": 0 },
        "reverb":{ "enabled": false, "size": 0.5, "damp": 0.4, "mix": 0.2, "predelayMs": 10 }
      },
      "activeTakeId": "tk_1"
    }
  ],
  "takes": [
    {
      "id": "tk_1",
      "trackId": "trk_1",
      "file": "takes/take-001.wav",
      "startFrame": 0,            // relative to session zero, session sample rate
      "lengthFrames": 1920000,
      "channels": 1,
      "sampleRate": 48000,
      "format": "f32le",
      "printed": false,           // true if effects were baked in
      "incomplete": false,
      "trim": { "startFrame": 0, "endFrame": 0 },
      "gainDb": 0.0,
      "muted": false,
      "createdAt": "2026-11-20T10:13:04Z"
    }
  ]
}
```

Schema rules:
- `schemaVersion` gates migration; opening a newer version is refused with a clear message rather than guessed at.
- Unknown fields are preserved on write where practical, so a future version does not destroy data.
- All times are frames at the session sample rate; the UI converts for display. No floating-point seconds in the model.

### 7.3 WAV take files

- **Container**: RIFF/WAVE.
- **Sample format**: IEEE float, 32-bit little-endian (`WAVE_FORMAT_IEEE_FLOAT` = 3) for 1–2 channels; **`WAVE_FORMAT_EXTENSIBLE` (0xFFFE) with sub-format IEEE float and a channel mask for >2 channels**, which is the correct way to describe 4/6/8-channel float files. This is a hard requirement of this spec, not an implementation detail: tools such as Audacity/Reaper/ffmpeg behave inconsistently with plain tag-3 multichannel files.
- **Channels**: number of armed tracks in the take, each mapped per `session.channelLayout`.
- **Sample rate**: device-native at record time (typically 48 kHz); stored in both the header and the project.
- **No resampling during recording.** If playback of an existing take must go to a device at a different rate, resampling happens in the playback path, not the record path.
- **Naming**: `takes/take-%03d.wav`, monotonically increasing per session; track names live in the project, not the filename.
- **Header maintenance**: header written with placeholder sizes, then rewritten at checkpoints (~every 2 s) and on stop, so a killed process leaves at most ~2 s of unaccounted data — recovered by the repair pass from file length.
- **Interleaving**: frame-interleaved (`L R L R …` per frame), the standard DAW layout.

### 7.4 Size ceiling math (why the cap is what it is)

32-bit float = 4 bytes/sample. Bytes per second = `rate × channels × 4`.

| Session | Channels | Bytes/s | Safe duration before 4 GB |
|---|---|---|---|
| 48 kHz | 1 | 192,000 | ~6 h 12 min |
| 48 kHz | 2 | 384,000 | ~3 h 06 min |
| 48 kHz | 4 | 768,000 | ~1 h 33 min |
| 48 kHz | 8 | 1,536,000 | ~46 min |
| 96 kHz | 4 | 1,536,000 | ~46 min |

- **FR-37** The app shows remaining safe time in the transport bar while rolling.
- **FR-38** Warnings at 75 % and 90 % of the cap; at 95 % it stops the take cleanly and finalizes.
- The cap is `4,000,000,000` bytes of data (conservative, below the 4 GiB field limit), not the exact `0xFFFFFFFF`, so header arithmetic never wraps.

### 7.5 Peak cache

- `peaks/take-NNN.pk`: for each take, a binary file of min/max float32 pairs per bucket (bucket = 256 frames at 48 kHz), plus a header with channel count, rate and bucket size.
- Written incrementally during recording, so the live waveform does not require re-reading the take.
- Rebuilt automatically if missing or shorter than the take (cheap, streaming, cancellable).

---

## 8. Architecture

### 8.1 Package layout

```
gorecord/
├── cmd/gorecord/main.go          # app bootstrap, Fyne app + window, wiring
├── cmd/gorecord/FyneApp.toml     # packaging metadata (app ID, icon, version)
├── internal/audio/               # device layer (malgo): enumerate, open, callbacks
│   ├── device.go                 # capture/duplex device lifecycle
│   ├── engine.go                 # transport state machine, timeline, mixing
│   ├── ring.go                   # lock-free SPSC ring buffers
│   └── clock.go                  # frame counters, xrun detection
├── internal/dsp/                 # real-time DSP, allocation-free
│   ├── biquad.go                 # RBJ cookbook biquads (EQ, filters)
│   ├── compressor.go
│   ├── reverb.go                 # Freeverb-style
│   ├── drive.go                  # waveshaper + tone stack
│   ├── cab.go                    # fixed speaker-emulation filter
│   ├── chain.go                  # per-track chain application
│   └── tuner/                    # pitch detection (outside RT callback)
├── internal/session/             # project model, JSON persistence, migrations
│   ├── model.go
│   ├── store.go                  # load/save, atomic write via temp+rename
│   └── repair.go                 # .part recovery on launch
├── internal/wavio/               # float32 WAV writer/reader, EXTENSIBLE, 4 GB guard
├── internal/peaks/               # peak cache writer/reader
├── internal/player/              # streaming take playback with preload ring
└── internal/ui/                  # Fyne views and custom widgets
    ├── app.go                    # window, layout, navigation
    ├── transport.go
    ├── tracklist.go
    ├── timeline.go               # lanes, live scrolling waveform
    ├── meters.go                 # custom widget
    ├── inspector.go              # effects chain panels
    └── tuner.go
```

- Domain/audio code must not import Fyne. The UI observes state and issues commands through a small interface, so the engine is testable headless.
- The engine exposes a command channel (`Start`, `Stop`, `Arm`, `SetParam`, …) consumed by a control goroutine, never called directly from the audio callback.

### 8.2 Real-time rules (non-negotiable)

The audio data callback is the highest-risk code in the project. Rules:

1. **No allocation, no locks, no channel sends, no logging, no `fmt`** in the callback.
2. All buffers, delay lines, filter states and reverb networks are allocated when the device opens, sized to the negotiated period.
3. UI → audio parameter changes use atomic scalars / small fixed parameter structs read once per block, with smoothing where needed.
4. Audio → UI data uses lock-free SPSC rings (meter values, waveform peaks, playhead frame, xrun count); the UI polls at ~30–60 fps.
5. Disk I/O stays off the callback: captured frames are pushed into a ring that a writer goroutine drains and writes.
6. If the writer ring is nearly full, the engine **prioritizes not blocking audio** and increments a `diskStall` counter; the UI must surface this loudly because it means audio may be dropped.

### 8.3 Threading model

| Goroutine | Responsibility |
|---|---|
| Audio callback (C thread via malgo) | Read input, run DSP chain, mix playback + click, write output, push frames to record ring, publish meters |
| Record writer | Drain ring, write interleaved float32, maintain header checkpoints, feed peak cache |
| Playback reader | Preload/stream active take files into preallocated buffers for the mixer (~500 ms lookahead, 250 ms ahead of playhead) |
| Control | Apply UI commands, device open/close, transport transitions |
| UI | Fyne event loop; render meters/waveform; poll state ring |

All UI updates from goroutines go through `fyne.Do` (Fyne 2.6+ requirement); direct widget mutation from other goroutines is a bug.

### 8.4 Clock-domain rule

Recording and monitoring share **one device clock**. Concretely:

- Capture and playback are opened as one **duplex** device on the same interface whenever the user selects the same device for both.
- If the user selects a different playback device from the capture device, the two clocks drift and can also have unrelated latencies. v1 then **disables live monitoring** and allows only playback of existing takes; the UI explains why. Supporting independent devices requires drift compensation (resampling the runaway side), which is explicitly deferred.
- The record path never resamples.

### 8.5 Timeline and mixing

- Session zero is frame 0. Every take has `startFrame` and `lengthFrames`.
- The mixer's playhead is a single frame counter advanced by the audio callback. For each active take whose range covers the current block, samples are read from its preloaded buffer and summed.
- Take trimming is applied as an offset into the file (`trim.startFrame`), so trimming never touches audio data.
- Gain is applied per take; mute/solo resolution happens in the control layer, producing a simple per-take gain vector once per block (not per sample).
- Summing is float32; no clipping is applied to the record path, but the monitor bus is soft-clipped or limited to avoid speaker damage from a summed overload.

### 8.6 Latency budget (target < 10 ms)

At 48 kHz:

| Period | Frames | Callback interval |
|---|---|---|
| Low | 128 | 2.67 ms |
| **Default** | **256** | **5.33 ms** |
| Safe | 512 | 10.67 ms |

- **FR-39** The device is opened with an explicit `PeriodSizeInFrames` (256 default) and `Periods = 2`, **not** miniaudio's default of 10 ms × 3 periods (which alone is 30 ms and cannot meet the target).
- Estimated software latency = `periods × frames / rate`; the status bar displays it alongside the reported backend period so the number is not fictional. Hardware/USB/DAC latency (~1–3 ms) is additional and is not guessed at.
- **FR-40** Buffer size is user-selectable (128 / 256 / 512 / 1024) in settings, with a live xrun counter. Default 256; 128 is offered with a warning that Go's GC may cause dropouts.
- **FR-41** If the xrun rate during a take exceeds a threshold, show a non-blocking notice suggesting a larger buffer.

**Honest risk:** CPU-heavy effects plus Go's garbage collector at 128-frame periods can miss deadlines. Mitigations: preallocated arena-style buffers, `debug.SetGCPercent` tuning at startup, optional `GOMAXPROCS` pinning, and defaulting to 256. This is tracked as risk R1 in §13.

### 8.7 Durability

- Record writer appends to `take-NNN.wav.part` (header + data), renaming to `.wav` on clean stop.
- `fsync` (or platform equivalent) at header checkpoints and on stop; not per block.
- On launch, `repair.go` finds `.part` files, computes valid data length from the file size, writes a correct header (including the EXTENSIBLE block when needed), and creates an `incomplete: true` take entry.
- The project JSON is written atomically (temp file + rename) and keeps one `.bak`.

---

## 9. DSP specification

All effects are per-track, run in the audio callback, allocation-free after construction, and process float32.

### 9.1 Parametric EQ
- 3–5 peaking/shelving bands (v1: 3 fully parametric + low/high shelf optional flags), RBJ cookbook biquads in transposed direct form II, per-channel state.
- Parameters: frequency (20 Hz–20 kHz, log), Q (0.3–10), gain (−24…+24 dB), band enable.
- Coefficients recomputed on parameter change in the control goroutine, double-buffered and swapped at block boundary.
- Expected cost at 48 kHz/2 ch: well under 1 % of one core for 5 bands.

### 9.2 Compressor
- Feed-forward, soft-knee, peak/RMS hybrid detector, log-domain gain computer.
- Parameters: threshold (−60…0 dB), ratio (1:1–20:1), attack (0.1–100 ms), release (10–1000 ms), makeup gain (0–24 dB), knee (0–12 dB).
- Mono-linked per track (not per channel).
- UI shows gain reduction in dB, updated at 30 fps from an atomic value.

### 9.3 Reverb
- Freeverb-style: 8 parallel comb filters + 4 series allpasses per channel, with damping and room size; stereo output from a mono input.
- Parameters: room size, damping, pre-delay (0–100 ms), wet/dry mix.
- Monitor-only or printed per track (see FR-22).

### 9.4 Amp (drive + tone stack)
- Input gain → pre-emphasis filter → asymmetric waveshaper (soft knee tanh for low drive, harder clipping curve at high drive) → 4× oversampled (to reduce aliasing) → tone stack (bass/mid/treble via three biquads approximating a Fender-style stack) → output level.
- Parameters: drive (0–100 %), bass/mid/treble (each −12…+12 dB), level (−24…+6 dB).
- Oversampling factor is configurable down to 1× if CPU-bound; documented in the UI as "quality".

### 9.5 Cabinet (fixed filter, no IR)
- Fixed speaker-emulation chain: high-pass at ~80 Hz, low-pass at ~5.5 kHz with a gentle resonance around 100–120 Hz, and a presence shelf — tuned by ear, exposed as a single "cab character" amount (0–100 %) rather than fake microphone controls.
- Explicitly designed behind an interface (`CabProcessor`) so a future partitioned-FFT IR convolver can replace it without touching the chain (see §14 Q2).

### 9.6 Tuner
- Runs on a decimated copy (~8 kHz) of the pre-effects input, outside the callback, at 10–20 Hz.
- Algorithm: YIN or normalized autocorrelation with parabolic interpolation; confidence threshold to suppress display on noise.
- Shows note (A440 reference, adjustable 415–445 Hz), cents offset, and a stable/needs-tune indicator.

### 9.7 Click
- Sample-accurate click generator driven by the frame counter and BPM: short enveloped sine or noise burst, ~1 ms attack, 30 ms decay, higher pitch on the downbeat.
- Optional count-in: click plays for N bars with the transport armed but recording not yet started; recording begins exactly on the downbeat.

### 9.8 Meters
- Per-channel: peak and RMS computed in the callback (cheap, `max`/sum of squares), published as atomics.
- Peak-hold ~1 s, clip indicator latches until clicked.
- dBFS scale from −60 to 0, with the last 6 dB visually distinct.

### 9.9 Editing allowed in v1
- Trim start/end, per-take gain, mute/solo, rename, delete-to-trash, choose active take. Nothing rewrites sample data.
- No copy/paste, no fades, no normalization in v1 (candidates for v1.1).

---

## 10. UI specification (Fyne v2)

### 10.1 Main window

```
┌───────────────────────────────────────────────────────────────────────────┐
│ ● Record   ▶ Play   ■ Stop   │ 00:01:23.456 │ 1.1.1 │ Click 120 BPM  ⏱ 5.3ms │  ← transport bar
├───────────────┬───────────────────────────────────────────┬───────────────┤
│ TRACKS        │ TIMELINE                                  │ INSPECTOR     │
│ ▸ Guitar      │  bars  1     2     3     4     5          │ Guitar        │
│   [A] [M] [S] │  ┌───────────────────────────────────────┐│ Amp   [on]    │
│   ▮▮▮▮ meter  │  │ ████████████████ take 1               ││  Drive ▓▓▓░░  │
│ ▸ Voice       │  └───────────────────────────────────────┘│  Bass  ▓▓░░░  │
│   [A] [M] [S] │  ┌───────────────────────────────────────┐│  Mid   ▓▓▓░░  │
│   ▮▮▮▮ meter  │  │ ▓▓▓▓▓▓▓▓ live recording…              ││  Treble▓▓▓▓░  │
│               │  └───────────────────────────────────────┘│ Cab   [on]    │
│ [+ Track]     │  ▲ playhead                               │ EQ    [on] 3b │
│               │                                           │ Comp  [off]   │
│               │                                           │ Reverb[on]    │
│               │                                           │ ☐ print fx    │
├───────────────┴───────────────────────────────────────────┴───────────────┤
│ Device: Scarlett 4i4 · 48000 Hz · 4 ch · buffer 256 (5.3 ms) · xruns 0     │
└───────────────────────────────────────────────────────────────────────────┘
```

- **Transport bar**: record/play/stop, timecode, bars/beats, click toggle + BPM, remaining-safe-time, latency readout, monitor on/off, tuner button.
- **Track list (left)**: track name, arm/mute/solo, live meter strip, drag-reorder, add track.
- **Timeline (center)**: horizontal lanes per track, take blocks sized by duration, playhead, zoom (scroll/ctrl-scroll), horizontal scroll, click-to-position. During recording the current lane grows in real time.
- **Inspector (right)**: selected track's chain, each effect a collapsible panel with toggles and knobs/sliders; gain reduction and clip indicators included.
- **Status bar**: device, rate, channels, buffer+latency, xrun count, disk headroom.

### 10.2 Waveform rendering (the performance-critical UI path)

- Live waveform is drawn from the peak cache, not from raw samples: the record writer appends min/max pairs; the UI consumes new buckets at 30 fps and appends them to an `image.RGBA` strip rather than rebuilding the whole canvas.
- Implementation: a custom widget holding a `canvas.Image` whose `image.RGBA` is written in place, with `Refresh()` limited to the dirty region where supported. Full redraw only on zoom/resize.
- Zoomed-in views (>~1 px per frame) read raw samples in a background goroutine, never on the UI thread.
- Explicit target: **UI must not allocate per frame** and must not exceed ~15 % of one core while recording.
- If frame rates degrade on a target machine, the fallback is meters-only during the take with the waveform drawn on stop — but that is a fallback, not the shipped default, since live scrolling waveform is a stated requirement.

### 10.3 Interaction details
- Space = record/stop (configurable to play/stop), Esc = stop without saving? **No** — never discard silently; Esc = stop and keep.
- Arm toggles disabled while rolling.
- Device change while rolling is blocked with an explanation.
- All destructive actions confirm; no action deletes audio immediately.
- Keyboard: Space, R (record), P (play), M (mute selected), S (solo selected), T (tuner), C (click).

### 10.4 Accessibility and platform polish
- Respect system light/dark theme via Fyne themes; usable at 200 % scaling (no fixed pixel widths that clip labels).
- All controls reachable by keyboard; tooltips on icon-only buttons.
- Window remembers size/position per session.

---

## 11. Platform requirements

### 11.1 macOS (primary dev machine: darwin/arm64, Go 1.25.1 present)
- Ship as a **`.app` bundle** built with `fyne package -os darwin -appID <id>`, with a real icon; a bare binary would launch through Terminal and inherit the terminal's identity, which breaks microphone attribution.
- `Info.plist` must contain `NSMicrophoneUsageDescription`. **Verify this during milestone 0**: if the Fyne bundler's generated plist lacks it (or the string is not ours), post-process `Contents/Info.plist` (`plutil -insert`) and re-sign. This must be checked on a built bundle, not assumed.
- Microphone access is keyed to the app's signing identity. For local development, use a **stable self-signed certificate** rather than ad-hoc signing, so the TCC grant survives rebuilds; document the one-time "toggle in System Settings → Privacy → Microphone" step.
- Public release: Developer ID signing + hardened runtime + `com.apple.security.device.audio-input` entitlement + notarization + stapling. Without notarization, downloaders see "damaged"/quarantine and need `xattr -dr com.apple.quarantine`; since G7 promises an installable release, notarization is in scope **if** an Apple Developer account is available (see §14 Q1).
- Support Apple Silicon and Intel (universal or two artifacts). Minimum macOS 12 unless a blocker appears.
- Handle sleep/wake and default-device changes; CoreAudio invalidates device IDs on replug.

### 11.2 Windows
- WASAPI shared mode by default (works with any interface, no exclusive-mode lockout); show exclusive-mode option if malgo exposes it cleanly, off by default.
- 64-bit only. Windows 10 1809+.
- Ship a single `.exe` (with embedded icon/manifest via `fyne package`) and a zip; no installer in v1.
- Test matrix must include at least one ASIO-less USB interface (WASAPI) — ASIO is **not** supported by malgo and is out of scope; state this clearly in the README because guitarists will ask.
- Mic permission: Windows 10+ privacy setting blocks input; detect silence-at-open (FR-6) and point to Settings → Privacy → Microphone.

### 11.3 Linux
- Default backend PulseAudio (via miniaudio), ALSA fallback. JACK support is inherited from miniaudio but untested in v1; document as experimental.
- Ship a tarball with desktop entry + icon, plus a `.deb` if `nfpm` makes it cheap. No Flatpak/Snap in v1 (Flatpak would need `--device=all` for audio device access — document it if someone packages it).
- Test on at least one PipeWire-as-PulseAudio system, since that is the modern default; note any channel-count quirks.

### 11.4 Cross-cutting
- **cgo is required** (malgo). Builds need a C toolchain: clang/Xcode CLT on macOS, MinGW-w64 on Windows (and for cross builds), gcc on Linux.
- Cross-compilation from one host: `fyne-cross` (Docker-based) for Windows/Linux artifacts; macOS artifacts built on macOS runners.
- CI runs unit tests (no audio device) plus build-only jobs per OS. Audio behavior is verified by a manual hardware test checklist (§13).

---

## 12. Build, tooling, release

| Concern | Decision |
|---|---|
| Go version | 1.25.x (installed: 1.25.1). Avoid libraries requiring 1.26 (e.g. `tphakala/go-wav`). |
| `CGO_ENABLED` | 1 on all platforms |
| Module path | `github.com/<owner>/gorecord` (owner decided at publish time) |
| Dependencies | `fyne.io/fyne/v2` (BSD-3), `github.com/gen2brain/malgo` (Unlicense). Everything else (WAV I/O, DSP, peaks, playback) is written in-repo to keep the dependency surface small and MIT-clean. |
| Why not `go-audio/wav` | Its encoder takes integer buffers and its float64/float32 decode path is still a TODO; 32-bit float multichannel output needs explicit `WAVE_FORMAT_EXTENSIBLE` support. Own writer is ~300 lines and fully testable. |
| License | MIT (`LICENSE`, SPDX headers on new files) |
| Versioning | SemVer via `FyneApp.toml`; `--version` and About dialog show commit + tag |
| Release artifacts | macOS: universal `.app` in zip/dmg, signed+notarized. Windows: `.exe` zip. Linux: `.tar.gz` |
| CI | GitHub Actions: `test` (unit, race, vet, gofmt), `build` matrix (darwin/arm64+amd64, windows/amd64, linux/amd64+arm64), `release` on tag |
| Docs | README (what it is, screenshots, per-OS setup, ASIO limitation, mic permission steps), CONTRIBUTING, a manual test checklist, `docs/architecture.md` generated later from this spec |
| Repo hygiene | `.gitignore` for build artifacts and session fixtures; no binary audio committed except tiny test fixtures |

### 12.1 Test strategy

| Layer | Tests |
|---|---|
| `wavio` | Round-trip write→read for 1/2/4/8 ch, 44.1/48/96 kHz; header correctness (tag, mask, block align, sizes); 4 GB guard refused cleanly; truncated-file repair from length. Validate outputs with `ffprobe`/`sox`/Audacity in the manual checklist. |
| `dsp` | Biquad frequency-response against analytic response; compressor gain-reduction curve; drive THD/aliasing sanity; reverb tail decay; chain runs allocation-free (assert `AllocsPerRun == 0` on the process function). |
| `session` | JSON round-trip, schema-version refusal, atomic write leaves valid file at all times, relative-path survival after moving the folder, trash instead of delete. |
| `peaks` | Bucket math, incremental append matches full rebuild, regenerated cache equals cached values. |
| `player` | Streaming offsets honor `trim`, gain, and `startFrame`; block boundaries do not crackle (statistical test of continuity). |
| `engine` (headless) | Transport state machine with a fake device: record→stop finalizes, arm latching, take start offset recording, disk-stall pathway. |
| Manual hardware | Checklist across macOS/Windows/Linux with a real interface: 30 min 4-ch take, unplug/replug, sleep/wake, tune, monitor latency feel, 128/256/512 buffer sweep with xrun counts. |

---

## 13. Risks and mitigations

| # | Risk | Impact | Mitigation |
|---|---|---|---|
| R1 | Go GC pauses exceed a 2.67 ms (128-frame) deadline → dropouts while monitoring | High — kills G2 | Default 256 frames; arena preallocation; no allocation in callback (enforced by tests); GC percent tuning; surface xruns; allow 512. |
| R2 | Fyne waveform redraw cost competes with audio | Medium | Peak-cache strip appended in place; 30 fps cap; no per-frame allocation; benchmark in milestone 4; fallback to meters-only during take if needed. |
| R3 | Fyne's macOS bundle lacks the mic usage string, or ad-hoc signing resets TCC every build | High — app appears broken on macOS | Milestone 0 spike: build a bundle, inspect plist, test permission; if needed post-process plist + stable self-signed cert; document steps. |
| R4 | macOS notarization needs a paid Apple Developer account | Medium — G7 installability | Confirm account availability; otherwise ship unsigned with clear quarantine instructions (documented gap, not hidden). |
| R5 | WASAPI-only means no ASIO/high-end interfaces on Windows | Medium | State limitation prominently; shared-mode WASAPI generally works for USB interfaces; log device list in issue template. |
| R6 | 32-bit float multichannel files read inconsistently by some tools | Low–Medium | Use `WAVE_FORMAT_EXTENSIBLE` + channel mask; verify in Audacity, Reaper, ffmpeg, Logic in the manual checklist. |
| R7 | Separate capture/playback devices drift | Medium | §8.4 rule: one clock or no live monitoring; UI explains. |
| R8 | Four supported effects + tuner is a large v1 | High — schedule | Milestones (§15) sequence EQ→comp→drive/cab→reverb→tuner; tuner can land last. |
| R9 | Linux backend variance (PipeWire/Pulse/ALSA/JACK) | Medium | PulseAudio default, ALSA fallback, JACK documented experimental, capture a device-list diagnostic in the app's Help. |
| R10 | Disk write stalls during recording | High — data loss | Large writer ring, continuous append, stall counters, 95 % cap guard, repair pass. |

---

## 14. Open questions (to settle before or during M1)

1. **Apple Developer account**: is one available for notarization, or does the release ship unsigned with documented quarantine steps? (Affects G7/R4.)
2. **Cabinet upgrade path**: no IR in v1, but should the chain interface and UI reserve a slot/"load IR" affordance now, or stay clean and add it in a later minor version?
3. **Reverb printing**: is printing reverb into a take acceptable in v1, or should reverb be monitor-only (dry recording always) to avoid a DOM-like mismatch when the dry take is played back?
4. **Default for "print effects"**: off for all new tracks (safe default), or inherit the last used value?
5. **Mixed sample rates**: v1 refuses to record into a session whose sample rate differs from the device. Is refusing right, or should it offer "start a new session at this rate"?
6. **Session naming**: free text only, or auto-suggest a name from date + first track (e.g. "2026-11-20 guitar idea")?
7. **Where sessions live**: default `~/Music/gorecord` (proposed) vs a user-chosen root on first run.
8. **Multi-channel take with uneven arming mid-session**: if take 1 recorded channels {0,1} and take 2 records {1,2}, is that acceptable (channel map per take) — proposed yes — or should the session fix a channel layout at creation?
9. **Tuner reference**: fixed A440, or user-selectable 415–445 Hz from v1?
10. **Repo name and binary name**: confirm `gorecord`, and whether the public repo should carry the same name.

---

## 15. Milestones

Each milestone must be independently runnable and demoable. No milestone ships a half-working record path.

**M0 — Feasibility spikes (timeboxed)**
- malgo capture on all three OSes from a bare binary; enumerate devices, print channel counts.
- Duplex open at 128/256/512 frames; measure callback jitter and xruns with a trivial passthrough.
- Fyne app with a meter widget at 60 fps *while* a capture callback runs; measure CPU of each.
- macOS bundle: `fyne package`, inspect `Info.plist`, prove the mic prompt appears and persists across rebuilds.
- Float32 multichannel WAV written by hand, opened in Audacity/Reaper/ffmpeg.
- **Exit criteria:** each of the above has a recorded result; R1/R3 resolved one way or the other.

**M1 — Record a dry take**
- Session create/open, track add, device/arm selection, record/stop, take file + project JSON, live meters.
- 4 GB guard, header checkpoints, crash repair pass.
- **Demo:** record 10 minutes of 4-channel audio, kill the process mid-take, relaunch, play the recovered take.

**M2 — Layer and manage takes**
- Timeline with take blocks, playback of existing takes while recording, stored start offsets, active-take switching, trim/gain, mute/solo, delete-to-trash, project reload fidelity.
- **Demo:** record a chord part, layer a lead over it, mute/solo, reopen the session and hear the same mix.

**M3 — Monitor chain and click**
- Drive/tone, cab filter, EQ, compressor, reverb, chain bypass, parameter smoothing, print-effects toggle, click + count-in, latency readout and buffer settings.
- **Demo:** play through amp+cab+reverb at 256 frames with <10 ms reported software latency and zero xruns over 10 minutes.

**M4 — Tuner, waveform polish, and release**
- Tuner, live scrolling waveform via peak cache, keyboard shortcuts, light/dark polish, error surfaces, session picker.
- CI matrix, signed/notarized macOS artifact, Windows/Linux artifacts, README + screenshots + manual test checklist.
- **Demo:** install from the release artifacts on all three OSes, complete the manual checklist.

---

## 16. Acceptance criteria (v1 definition of done)

1. Fresh install on macOS, Windows and Linux opens a window and lists the connected interface with its real channel count.
2. A 4-channel, 48 kHz, 10-minute take records without xruns at the default 256-frame buffer, and the resulting file opens correctly (right channel count, rate, float32) in Audacity and Reaper.
3. Live monitoring with amp+cab+EQ+comp reports software latency under 10 ms and feels immediate; adjusting knobs produces no clicks.
4. Recording a second take over a played-back first take yields sample-aligned layers; muting and unmuting the first take during the second take does not disturb the recording.
5. Killing the process mid-take and relaunching recovers a playable, flagged-incomplete take with an accurate duration.
6. Unplugging the interface mid-take stops cleanly, preserves the take, and shows a reconnect path.
7. A session folder survives being moved/renamed (relative paths) and reopens identically.
8. Recording stops cleanly with a clear message at 95 % of the 4 GB cap.
9. The click is off by default, and when enabled is audible without appearing in the recording.
10. The tuner reports a known-pitch test tone within ±2 cents.
11. `go vet`, `gofmt`, unit tests with `-race`, and builds on all three OSes pass in CI.
12. The public repo carries an MIT `LICENSE`, install instructions per OS, the ASIO limitation, and the macOS microphone-permission steps.

---

## 17. Appendix A — interview log (verbatim selections)

**Round 1 — purpose and shape**
- Core use case: *Session recorder for songwriting demos*
- Input channels: *More than two — 4+ inputs*
- UI stack: *Fyne — pure Go widgets, single toolchain*
- Monitoring: *Both — monitor live and also play back*

**Round 2 — workflow**
- Channel count: *Arbitrary, with per-channel arm/mute toggles*
- Overdub sync: *Yes — layered takes align on a timeline automatically*
- Click: *Optional toggle — off by default, available when I want it*
- File format: *WAV, 32-bit float*
- Storage layout: *Session folder per song, project file inside*

**Round 3 — constraints**
- Take files: *One multichannel WAV per take*
- Long takes: *Cap at 4 GB with a clear warning*
- Platforms: *macOS + Windows + Linux equally*
- Distribution: *Open source public release*
- Editing: *Reverb, equalizer, amp, compressor* (custom answer — expanded the scope from "editing" to a DSP chain)

**Round 4 — DSP**
- DSP timing: *Per-track choice — toggle baked vs dry*
- Amp sim: *Simple drive + tone stack (pure Go)*
- Latency: *As low as possible — monitor under ~10 ms total*
- Must-have effects: *Parametric EQ, Compressor, Reverb, Amp/Cabinet, Tuner* (multi-select)
- License: *(no selection)*

**Round 5 — final details**
- License: *MIT*
- Cabinet: *Simple cabinet filter — no IR files*
- Sample rate: *Follow the interface's native rate*
- Takes: *Stack of takes per track, one active, others muted*
- Failure handling: *Never lose audio — keep partial takes, warn loudly*
- Recording display: *Live scrolling waveform plus per-channel meters*

## 18. Appendix B — technical references consulted

- `gen2brain/malgo` (miniaudio bindings): cgo, no extra system libraries on Windows/macOS, `-ldl` on Linux; supports WASAPI/DirectSound/WinMM, CoreAudio, PulseAudio/ALSA/JACK; `DeviceConfig` exposes `PeriodSizeInFrames`, `Periods`, `PerformanceProfile`; device enumeration via `Context.Devices()`; one context can host several devices.
- miniaudio defaults: sample rate 48000, periods 3, low-latency period 10 ms — too slow for the stated target, so period size and period count are set explicitly.
- WAV: `WAVE_FORMAT_IEEE_FLOAT = 3`; `WAVE_FORMAT_EXTENSIBLE = 0xFFFE` with sub-format + channel mask required for reliable >2-channel files; 32-bit size fields cap a WAV near 4 GB (RF64/BW64 lift it — deliberately out of scope).
- `go-audio/wav`: integer-buffer encoder; float decode incomplete → not used for writing.
- Fyne: `fyne package` builds `.app`/`.exe`/tarball; `fyne-cross` for cross-OS builds; `fyne.Do` required for goroutine→UI updates; custom widgets via `widget.BaseWidget` + renderer, `canvas.Image` for raster waveform.
- macOS microphone: `NSMicrophoneUsageDescription` required; TCC keys grants to the bundle id + designated requirement, so ad-hoc signing loses the grant on every rebuild; hardened runtime + `com.apple.security.device.audio-input` entitlement required for notarized distribution; a bare binary inherits the launching terminal's grant (which is why the app must ship as a bundle).
