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
- Per-track monitor chain for input tracks: drive + tone stack → cabinet
  filter → 3-band EQ → compressor → reverb. The equalizer has five bands at
  20 Hz, 100 Hz, 1 kHz, 6 kHz and 20 kHz. The chain is heard on live monitoring
  and on the playback of recorded takes, but it is never printed into a take.
  PC audio is monitored and played back dry.
- Per-track monitor volume fader and pan knob: they change what monitoring
  and take playback sound like, but they are not printed into takes or the
exported WAV. Drag the knob up or down to pan, double-click it to centre.
- Master output routing: pick one or more playback devices. The first selected
  output stays on the low-latency Web Audio destination; extra outputs play the
  same mix in parallel. The choice is stored locally.
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
- Records PC audio and input tracks in the same pass. Each source family is
  captured on its own clock, then re-aligned with the session clock before the
  take is stored.
- Listen to any combination of PC audio and input tracks at once with the
  per-track **L** button, without arming them.
- Live scrolling waveform and per-track meters while recording.
- Chromatic tuner and an optional click, off by default and never recorded.
- Non-destructive take gain, mute, trim and one-click normalize to a -1 dBFS
  peak. The recorded audio is never rewritten.
- Exports the audible mix as one stereo 32-bit float WAV at the session rate.
- Projects and takes live in the app window's IndexedDB; no files are written
  outside the app profile.

## Requirements

- Go 1.25+.
- Node.js 20.19+ to build the static site for Freebuff Sites. The desktop app
  itself does not need Node.
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
make site       # static browser build in dist-site/ for Freebuff Sites
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

## Deploy on Freebuff Sites

The browser build is static. `cmd/gorecord` is only the local launcher, so a
Freebuff deployment contains the Go core compiled to WebAssembly plus the files
in `web/`; the Go server is not deployed.

```bash
make site   # writes dist-site/
```

`package.json` exposes `npm run build`, which runs `scripts/build-site.mjs` and
produces `dist-site/`; `wrangler.jsonc` publishes that folder as static assets.
The publishing workspace needs Go 1.25+ and Node.js 20.19+. If the publish log
reports `site build: cannot run Go`, install Go 1.25+ in that workspace and
publish again. To check the built site locally:

```bash
python3 -m http.server 8080 -d dist-site
```

Push the repository to GitHub, connect it to a Freebuff project, then use
Publish for Preview and Production. No database, secrets or server-side storage
are needed.
Freebuff serves the site over HTTPS, which the microphone and display-capture
APIs require. Open the published page in a normal browser tab rather than an
embedded preview.

Projects and takes stay in the browser's IndexedDB for that site's origin. They
are separate from the desktop launcher's profile, and clearing site data deletes
them. The first load downloads the WASM core; no audio leaves the browser.

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

Press **L** on a track to listen to it without arming it. You can listen to a
microphone and PC audio at the same time, which is useful for playing or
recording over a backing track.

**PC audio** asks you to share a tab or window with its audio — choose the one
playing the music. It can be armed together with input tracks: the engine
captures the PC share on its own clock and the interface on another, then
corrects the drift before saving each take. Keep the share alive until the
take stops.

Use the **out** menu in the toolbar to choose one or more master outputs. The
first selected output is the primary; additional outputs play the same mix in
parallel. The choice is remembered locally.

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
- PC audio and inputs can be recorded together, but clock-drift correction is
  best-effort. It uses a Chromium clock probe; if that API is unavailable, each
  source is resampled to the session rate without drift correction. Very long
  takes, dropouts, or a paused shared tab can still leave a small offset.
- Master output selection relies on the browser's `setSinkId`. Chromium
  supports it; other browsers may only offer the system default output.
- The primary master output uses the Web Audio destination directly; extra
  outputs go through a MediaStream and can lag it slightly. Separate channels
  of one interface are only selectable if the OS exposes them as outputs.
- Effects shape live monitoring and recorded take playback for input tracks;
  they are not printed into takes or the exported WAV. The Web Audio chain
  approximates the desktop DSP. PC audio stays dry.
- Playback loads the active takes into memory.
- Export is a single stereo float32 WAV at the session rate: no format,
  bit-depth or resampling choice, and no normalization of the mix itself.
- PC audio still comes from a separate clock. The clock probe corrects the
  measured rate, but browser dropouts or a paused shared tab can still create a
  discontinuity that no resampling can reconstruct.
- No comping, MIDI, plugin hosting or time-stretch.
- Reverb and amp are musically simple by design (no IR cabinet, no neural
  models).
- The Freebuff deployment is a static site: no server-side project storage,
  accounts, collaboration or background processing. It needs Go in the build
  workspace because the WASM core is compiled at publish time.

## License

MIT — see `LICENSE`.
