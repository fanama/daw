# gorecord

A desktop recorder for song ideas, built on a browser UI: plug a mic and a
guitar into your audio interface, record several inputs at once, layer takes
over what you just played, and keep the whole session in the app. The Go core
compiles to WebAssembly, the UI is plain HTML/CSS/JS, Web Audio replaces the
native engine, and IndexedDB replaces the session folder.

`gorecord` is a local launcher. It serves the browser build on `127.0.0.1` and
opens it in a dedicated, chromeless app window of an installed Chromium browser
(Chrome, Edge, Chromium or Brave). The window uses its own profile, separate
from your everyday browsing. If no Chromium browser is found, it falls back to
your default browser. There is no native GUI toolkit: Fyne has been removed.

On macOS, `make bundle` packages the launcher and the web build into
`dist/gorecord.app`, which can be double-clicked from Finder. The bundle still
uses an installed Chromium for the window; it does not embed a webview.

The domain and application layers are pure Go. The original native audio
adapters and their tests remain in the tree, but they are not part of the
shipping app.

```
domain/            session aggregate, effects vocabulary, ports (no I/O, no UI)
application/       use cases: session editing, recording, mixdown export
infrastructure/    float32 WAV, JSON projects, DSP, native audio adapters
webapp/            WebAssembly composition root (same domain and use cases)
web/               browser UI, Web Audio engine, IndexedDB storage
cmd/gorecord/      local server and desktop window launcher
packaging/macos/   Info.plist for the double-clickable app bundle
```

## Features

- Opens as a desktop app window with its own profile and project storage.
- Arm one or more input tracks and record once: every armed channel of the
  selected device is captured together, and each track receives its own take.
- Layer takes on a sample-accurate timeline; previous takes play back while you
  record, and each take stores its start frame.
- Per-track monitor chain: drive + tone stack → cabinet filter → 3-band EQ →
  compressor → reverb. The equalizer has five bands at 20 Hz, 100 Hz, 1 kHz,
  6 kHz and 20 kHz. The chain is heard on live monitoring and on the playback
  of recorded takes, but it is never printed into a take.
- Per-track monitor volume fader and pan knob: they change what monitoring
  and take playback sound like, but they are not printed into takes or the
exported WAV. Drag the knob up or down to pan, double-click it to centre.
- Live spectrum analyzer behind the Equalizer curve, fed by the monitored
  signal and updated while you play.
- Live voice trace on the compressor transfer curve: the recent input level
  rides the curve with a crosshair and an input/output readout, fed by the dry
  monitored signal.
- Named markers: add one at the playhead with the Marker button or `M`, click
  a flag to jump to it, right-click it to delete it. Markers are saved with the
  project and never affect the audio.
- Optional one-bar count-in: a bar of clicks leads into recording, whether or
  not the metronome stays on during the take. The count-in is never recorded.
- Records the computer's own output onto a dedicated stereo track by sharing a
  tab or window with its audio.
- Live scrolling waveform and per-track meters while recording.
- Chromatic tuner and an optional click, off by default and never recorded.
- Non-destructive take gain, mute, trim and one-click normalize to a -1 dBFS
  peak. The recorded audio is never rewritten.
- Exports the audible mix as one stereo 32-bit float WAV at the session rate.
- Projects and takes live in the app window's IndexedDB; no files are written
  outside the app profile.

## Requirements

- Go 1.25+.
- macOS 12+ to build the `.app` bundle. The bundle still needs Chromium.
- Chrome, Edge, Chromium or Brave for the desktop app window. Without one, the
  app opens in your default browser instead.
- Web Audio and microphone capture need a secure context; `localhost` counts.
- Running the full `go test ./...` also builds the retained native audio
  packages, which need a C toolchain (Xcode command line tools on macOS, gcc on
  Linux, MinGW-w64 on Windows). Building the app itself does not.

## Build and run

```bash
make build      # web build + bin/gorecord
make bundle     # macOS bundle in dist/gorecord.app
make run        # builds, serves on 127.0.0.1:8080, opens the app window
make serve      # builds and serves without opening a window
make test
make vet
make caps       # 300-line file / 40-line function check
```

`make build` runs the WebAssembly build first and puts the launcher in
`bin/gorecord`. The launcher looks for `web/` in the current directory; inside
the app bundle it uses `Contents/Resources/web` instead. Override the location
with `-dir`:

```bash
bin/gorecord -dir /path/to/web -addr 127.0.0.1:8080
```

`make run` and `make serve` use `127.0.0.1:8080`. Running `bin/gorecord` or the
bundle without `-addr` picks a free local port, so it does not collide with an
existing server.

Window modes:

```bash
bin/gorecord                    # dedicated app window, browser fallback
bin/gorecord -window=app -addr 127.0.0.1:8080
bin/gorecord -window=browser    # default browser tab or window
bin/gorecord -window=none       # server only, no window
bin/gorecord -browser "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
```

`-browser` is only needed when automatic Chromium detection picks the wrong
binary. The launcher stops about two seconds after its window or tab closes,
even if Chromium keeps running in the background. A reload sends a fresh page
signal and keeps the server alive. Ctrl+C still works from a terminal.

## macOS app bundle

```bash
make bundle
open dist/gorecord.app
```

The bundle contains `Contents/MacOS/gorecord` and the complete web build in
`Contents/Resources/web`. Finder launches it with `/` as the working directory,
so the launcher resolves its resources relative to the executable. The bundle
is unsigned and not notarized: on another Mac, Gatekeeper may require
right-click → Open, or you can sign and notarize it for distribution.

The app profile lives in:

- macOS: `~/Library/Application Support/gorecord/browser`
- Linux: `~/.config/gorecord/browser`
- Windows: `%AppData%\gorecord\browser`

It is deliberately separate from your normal browser profile, so projects
recorded in a browser tab are not listed in the app window, and clearing the
app profile deletes its projects.

## Using the app

Create a session, arm the guitar track and press record. The app asks for
microphone access once. Add a second input track and arm both to capture two
interface channels in one pass.

**PC audio** asks you to share a tab or window with its audio — choose the one
playing the music. Record it on its own pass: it cannot be armed together with
live inputs, because the two sources have different clocks.

Takes are written to IndexedDB after you stop. **Export WAV** downloads the
audible mix; mute, solo, active takes, take gain and trim are already applied.

## Limitations

- Desktop mode is a window of an installed Chromium browser. The macOS `.app`
  bundles the launcher and web build, but still requires Chromium; there is no
  embedded browser engine. Windows and Linux have no bundle target and still
  run `bin/gorecord` with `web/` beside it.
- The macOS bundle is unsigned and unnotarized, so Gatekeeper may block it on
  another machine. Right-click → Open bypasses that for local use.
- Projects are local to the app profile. Clearing site data deletes them, and
  there is no project-folder import/export.
- Projects do not sync between app-window mode and normal browser tabs.
- Takes are written after stop rather than incrementally; a closed window loses
  the take in progress.
- Input tracks share the selected device (Input 1 / Input 2). Choosing a
  separate physical device per track is not available yet.
- Effects shape live monitoring and recorded take playback; they are not
  printed into takes or the exported WAV. The Web Audio chain approximates the
  desktop DSP.
- Playback loads the active takes into memory.
- Export is a single stereo float32 WAV at the session rate: no format,
  bit-depth or resampling choice, and no normalization of the mix itself.
- PC audio captures through a second clock; dropouts are padded with silence and
  a long take can drift by a few milliseconds against the input timeline.
- No comping, MIDI, plugin hosting or time-stretch.
- Reverb and amp are musically simple by design (no IR cabinet, no neural
  models).

## License

MIT — see `LICENSE`.
