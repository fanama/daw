// fanamix browser page. The Go/wasm core owns the session model; this module
// renders it, runs the Web Audio transport, and persists audio in IndexedDB.

import * as storage from "./storage.js";
import { AudioEngine, encodeWav, renderMixdown } from "./audio.js";
import { clipAt, computePeaks, drawTimeline, frameAtX, markerAtX } from "./waveform.js";
import { audibleTakes, formatDuration, takeLength } from "./model.js";
import { effectGraph } from "./fxgraph.js";
import { effectsFromPreset, effectsToPreset } from "./effects-preset.js";
import { FACTORY_PRESETS } from "./effects-factory-presets.js";

const App = {
  state: null,
  root: null,
  pcm: new Map(),
  peaks: new Map(),
  audioTrash: new Map(),
  engine: null,
  monitored: new Set(),
  selectedTrackId: null,
  selectedTakeId: null,
  frame: 0,
  zoom: 1,
  viewStart: 0,
  playing: false,
  recording: false,
  countingIn: false,
  pendingTake: null,
  clipboard: null,
  drag: null,
  rafHandle: 0,
  sessions: [],
  saving: Promise.resolve(),
  eqGraph: null,
  eqTrackId: null,
  compGraph: null,
  compTrackId: null,
  openEffects: new Set(),
  openTakes: new Set(),
  outputs: [],
  outputIds: [""],
  outputPreference: [""],
  outputRefreshToken: 0,
};

const VOLUME_KEY = "fanamix.masterVolume";
const OUTPUT_KEY = "fanamix.masterOutputs";

// Keep a connection open for the launcher. Closing the window closes it and
// lets the Go server stop; reloads reconnect within the shutdown grace period.
// Static builds opened without this marker have no live endpoint to poll.
const appLive = new URLSearchParams(window.location.search).has("desktop")
  ? new EventSource("/__fanamix/events")
  : null;
if (appLive) appLive.onerror = () => {};

// One entry per effect kind the chain can hold. "state" is the settings block
// name the core sends back, so a chain can hold several instances of a kind.
const EFFECT_KINDS = [
  {
    kind: "amp",
    state: "Amp",
    title: "Amp",
    controls: [
      ["Drive", "Drive", 0, 1, 0.01],
      ["Level", "LevelDB", -24, 6, 0.5],
      ["Bass", "BassDB", -12, 12, 0.5],
      ["Mid", "MidDB", -12, 12, 0.5],
      ["Treble", "TrebleDB", -12, 12, 0.5],
    ],
  },
  { kind: "cab", state: "Cab", title: "Cabinet", controls: [["Character", "Character", 0, 1, 0.01]] },
  {
    kind: "eq",
    state: "EQ",
    title: "Equalizer",
    controls: [
      ["20", "Bands.0.GainDB", -24, 24, 0.5],
      ["100", "Bands.1.GainDB", -24, 24, 0.5],
      ["1k", "Bands.2.GainDB", -24, 24, 0.5],
      ["6k", "Bands.3.GainDB", -24, 24, 0.5],
      ["20k", "Bands.4.GainDB", -24, 24, 0.5],
    ],
  },
  {
    kind: "comp",
    state: "Comp",
    title: "Compressor",
    controls: [
      ["Threshold", "ThresholdDB", -60, 0, 1],
      ["Ratio", "Ratio", 1, 20, 0.5],
      ["Makeup", "MakeupDB", 0, 24, 0.5],
    ],
  },
  {
    kind: "reverb",
    state: "Reverb",
    title: "Reverb",
    controls: [
      ["Size", "Size", 0, 1, 0.01],
      ["Damp", "Damp", 0, 1, 0.01],
      ["Mix", "Mix", 0, 1, 0.01],
    ],
  },
];

function element(tag, className = "", text = "") {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text) node.textContent = text;
  return node;
}

function coreRequest(op, params = {}) {
  const raw = window.fanamixCore(JSON.stringify({ op, ...params }));
  const result = JSON.parse(raw);
  if (!result.ok) throw new Error(result.error || "The command failed.");
  return result;
}

function registerPCM(file, audio) {
  window.fanamixSetPCM(
    file,
    new Uint8Array(audio.samples.buffer, audio.samples.byteOffset, audio.samples.byteLength),
    audio.channels,
    audio.sampleRate,
  );
}

function nextPaint() {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()));
}

function setStatus(text, kind = "") {
  document.getElementById("status").textContent = text;
  const bar = document.querySelector(".statusbar");
  bar.classList.toggle("error", kind === "error");
  bar.classList.toggle("busy", kind === "busy");
}

async function loadCore() {
  const go = new Go();
  let instance;
  try {
    instance = await WebAssembly.instantiateStreaming(fetch("fanamix.wasm"), go.importObject);
  } catch (error) {
    const bytes = await (await fetch("fanamix.wasm")).arrayBuffer();
    instance = await WebAssembly.instantiate(bytes, go.importObject);
  }
  go.run(instance.instance);
  for (let attempt = 0; attempt < 150 && !window.fanamixReady; attempt++) {
    await new Promise((resolve) => setTimeout(resolve, 20));
  }
  if (!window.fanamixReady) throw new Error("The session core did not start.");
}

async function boot() {
  if (!window.WebAssembly || !window.indexedDB) {
    document.getElementById("picker").innerHTML = "<div class=\"card\"><h1>fanamix</h1><p class=\"lead\">This browser needs WebAssembly and IndexedDB to record.</p></div>";
    return;
  }
  window.fanamixHost = {
    save(root, project, name) {
      App.saving = App.saving.then(() => storage.saveProject(root, project, name)).catch((error) => console.error(error));
    },
  };
  try {
    await loadCore();
  } catch (error) {
    setStatus(error.message, "error");
    document.getElementById("picker").innerHTML = `<div class="card"><h1>fanamix</h1><p class="lead">${error.message}</p></div>`;
    return;
  }
  bindControls();
  await refreshSessions();
  window.setInterval(() => {
    if (!App.state) return;
    updateMeters();
    updateTuner();
  }, 120);
  window.setInterval(updateSpectrum, 50);
}

function bindControls() {
  document.getElementById("btn-record").addEventListener("click", toggleRecord);
  document.getElementById("btn-play").addEventListener("click", togglePlayback);
  document.getElementById("btn-stop").addEventListener("click", () => (App.recording ? stopRecording() : stopTransport()));
  document.getElementById("btn-marker").addEventListener("click", addMarker);
  document.getElementById("btn-export").addEventListener("click", exportMix);
  document.getElementById("btn-sessions").addEventListener("click", showPicker);
  document.getElementById("btn-add-track").addEventListener("click", addInputTrack);
  document.getElementById("btn-add-pc").addEventListener("click", addSystemTrack);
  document.getElementById("monitor-enabled").addEventListener("change", (event) => {
    App.engine?.setMonitor(event.target.checked);
    if (event.target.checked) openMonitoring();
  });
  const volume = document.getElementById("master-volume");
  volume.value = String(Math.round(readVolume() * 100));
  updateVolumeLabel();
  volume.addEventListener("input", () => {
    const level = Math.min(1, Math.max(0, Number(volume.value) / 100));
    window.localStorage.setItem(VOLUME_KEY, String(level));
    updateVolumeLabel();
    App.engine?.setMasterVolume(level);
  });
  App.outputPreference = readOutputPreference();
  refreshOutputs();
  document.getElementById("btn-output-rescan").addEventListener("click", () => refreshOutputs());
  const outputMenu = document.getElementById("master-output");
  const shortcutMenu = document.getElementById("shortcut-menu");
  document.addEventListener("pointerdown", (event) => {
    if (outputMenu.open && !outputMenu.contains(event.target)) outputMenu.open = false;
    if (shortcutMenu.open && !shortcutMenu.contains(event.target)) shortcutMenu.open = false;
  });
  const shortcutActions = {
    record: toggleRecord,
    play: togglePlayback,
    stop: () => (App.recording ? stopRecording() : stopTransport()),
    marker: addMarker,
    split: splitTakeAtPlayhead,
    copy: copyTakeAtPlayhead,
    paste: pasteTakeAtPlayhead,
    deleteTake: deleteSelectedTake,
    undo: undoLastEdit,
    export: exportMix,
    zoomIn: () => zoomTo(App.zoom * 1.5),
    zoomOut: () => zoomTo(App.zoom / 1.5),
    resetZoom,
  };
  shortcutMenu.querySelectorAll("[data-action]").forEach((button) => {
    button.addEventListener("click", () => {
      shortcutMenu.open = false;
      button.blur();
      shortcutActions[button.dataset.action]?.();
    });
  });
  if (navigator.mediaDevices?.addEventListener) {
    navigator.mediaDevices.addEventListener("devicechange", () => refreshOutputs());
  }
  document.getElementById("click-enabled").addEventListener("change", pushClick);
  document.getElementById("click-bpm").addEventListener("change", pushClick);
  document.getElementById("count-in").addEventListener("change", pushClick);
  const timeline = document.getElementById("timeline");
  timeline.addEventListener("pointerdown", startTimelinePointer);
  timeline.addEventListener("pointermove", moveTimelinePointer);
  timeline.addEventListener("pointerup", (event) => finishTimelinePointer(event, false));
  timeline.addEventListener("pointercancel", (event) => finishTimelinePointer(event, true));
  timeline.addEventListener("contextmenu", deleteMarkerAt);
  timeline.addEventListener("wheel", zoomTimeline, { passive: false });
  document.getElementById("track-list").addEventListener("scroll", drawTimelineNow, { passive: true });
  document.getElementById("new-session").addEventListener("submit", (event) => {
    event.preventDefault();
    const name = document.getElementById("session-name-input").value.trim();
    createSession(name, Number(document.getElementById("session-rate").value));
  });
  window.addEventListener("resize", drawTimelineNow);
  window.addEventListener("keydown", (event) => {
    if (event.target.closest("input, select, textarea, button, summary")) return;
    const key = event.key.toLowerCase();
    if ((event.metaKey || event.ctrlKey) && key === "z") {
      event.preventDefault();
      undoLastEdit();
      return;
    }
    if ((event.metaKey || event.ctrlKey) && key === "c") {
      event.preventDefault();
      copyTakeAtPlayhead();
      return;
    }
    if ((event.metaKey || event.ctrlKey) && key === "v") {
      event.preventDefault();
      pasteTakeAtPlayhead();
      return;
    }
    if (event.code === "Space") {
      event.preventDefault();
      togglePlayback();
    }
    if (key === "r") toggleRecord();
    if (key === "p") togglePlayback();
    if (key === "m") addMarker();
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (key === "delete" || key === "backspace") {
      event.preventDefault();
      deleteSelectedTake();
      return;
    }
    if (key === "s") splitTakeAtPlayhead();
    if (key === "+" || key === "=") zoomTo(App.zoom * 1.5);
    if (key === "-" || key === "_") zoomTo(App.zoom / 1.5);
    if (key === "0") resetZoom();
  });
  if (navigator.storage?.persist) navigator.storage.persist().catch(() => {});
}

function applyState(next) {
  App.state = next;
  App.root = next.root;
  if (!App.selectedTrackId || !next.tracks.some((track) => track.id === App.selectedTrackId)) {
    App.selectedTrackId = next.tracks[0]?.id || null;
  }
  if (App.selectedTakeId && !next.takes.some((take) => take.id === App.selectedTakeId)) {
    App.selectedTakeId = null;
  }
  restoreAudioTrash(next);
  pruneAudio(next);
  const trackIds = new Set(next.tracks.map((track) => track.id));
  for (const id of [...App.monitored]) if (!trackIds.has(id)) App.monitored.delete(id);
  App.engine?.syncMonitoring(next, App.monitored);
  App.engine?.syncTakeLevels(next);
  App.engine?.setClick(next.click);
  document.getElementById("session-name").textContent = next.name;
  document.getElementById("session-info").textContent = `${next.sampleRate / 1000} kHz · ${next.tracks.length} track(s) · ${next.takes.length} take(s)`;
  renderTracks();
  renderInspector();
  renderTransport();
  drawTimelineNow();
}

function pruneAudio(state) {
  const files = new Set(state.takes.map((take) => take.file));
  for (const file of [...App.pcm.keys()]) {
    if (files.has(file)) continue;
    // Keep the audio reachable so undo can bring a deleted clip back with its
    // sound; leaving the session flushes the trash from storage.
    App.audioTrash.set(file, { root: App.root, audio: App.pcm.get(file), peaks: App.peaks.get(file) });
    App.pcm.delete(file);
    App.peaks.delete(file);
    window.fanamixClearPCM(file);
    App.engine?.invalidate(file);
  }
}

function restoreAudioTrash(state) {
  for (const take of state.takes) {
    const kept = App.audioTrash.get(take.file);
    if (!kept || App.pcm.has(take.file)) continue;
    App.pcm.set(take.file, kept.audio);
    if (kept.peaks) App.peaks.set(take.file, kept.peaks);
    App.audioTrash.delete(take.file);
  }
}

function purgeAudioTrash() {
  for (const [file, kept] of App.audioTrash) {
    storage.deleteTake(kept.root, file).catch((error) => console.error(error));
  }
  App.audioTrash.clear();
}

function selectedTake() {
  return App.state?.takes.find((take) => take.id === App.selectedTakeId) || null;
}

async function deleteTake(takeId) {
  if (!App.state || App.recording) return;
  const take = App.state.takes.find((candidate) => candidate.id === takeId);
  if (!take) return;
  const position = App.state.takes.filter((candidate) => candidate.trackId === take.trackId).indexOf(take) + 1;
  if (!window.confirm(`Delete take ${position}?`)) return;
  if (App.playing) stopTransport();
  const result = await dispatch("take.delete", { takeId });
  if (result) setStatus("Take deleted — Ctrl/Cmd+Z restores it");
}

async function deleteSelectedTake() {
  const take = selectedTake();
  if (!take) return setStatus("Select a clip in the timeline to delete it.", "error");
  await deleteTake(take.id);
}

async function undoLastEdit() {
  if (!App.state || App.recording) return;
  if (App.playing) stopTransport();
  const result = await dispatch("state.undo");
  if (result) setStatus("Undone");
}

function currentTrack() {
  return App.state?.tracks.find((track) => track.id === App.selectedTrackId) || null;
}

function selectTrack(id) {
  App.selectedTrackId = id;
  renderTracks();
  renderInspector();
  drawTimelineNow();
}

async function dispatch(op, params) {
  try {
    const result = coreRequest(op, params);
    if (result.state) applyState(result.state);
    return result;
  } catch (error) {
    setStatus(error.message, "error");
    return null;
  }
}

function toggleButton(kind, label, on, title, handler) {
  const button = element("button", `${kind}${on ? " on" : ""}`, label);
  button.title = title;
  button.addEventListener("click", (event) => {
    event.stopPropagation();
    handler();
  });
  return button;
}

function renderTracks() {
  const list = document.getElementById("track-list");
  list.innerHTML = "";
  App.meterNodes = new Map();
  if (!App.state) return;
  if (!App.state.tracks.length) {
    list.append(element("p", "hint", "No tracks yet. Add one with +."));
    return;
  }
  for (const track of App.state.tracks) list.append(trackRow(track));
}

function trackRow(track) {
  const row = element("div", `track${track.id === App.selectedTrackId ? " selected" : ""}`);
  row.addEventListener("click", (event) => {
    if (event.target.closest("input, select, button, .pan-knob")) return;
    selectTrack(track.id);
  });
  const top = element("div", "track-top");
  const name = document.createElement("input");
  name.className = "track-name";
  name.value = track.name;
  name.title = "Track name";
  name.addEventListener("change", () => dispatch("track.rename", { trackId: track.id, name: name.value }));
  top.append(name, element("span", "track-kind", track.source === "system" ? "PC" : `in ${track.channel + 1}`));
  const controls = element("div", "track-controls");
  controls.append(
    toggleButton("listen", "L", App.monitored.has(track.id), App.monitored.has(track.id) ? "Stop listening to this track" : "Listen without arming", () => {
      const listening = !App.monitored.has(track.id);
      if (listening) App.monitored.add(track.id);
      else App.monitored.delete(track.id);
      renderTracks();
      App.engine?.syncMonitoring(App.state, App.monitored);
      if (listening && document.getElementById("monitor-enabled").checked) openMonitoring();
    }),
    toggleButton("arm", "●", track.armed, "Arm this track", async () => {
      const arming = !track.armed;
      const result = await dispatch("track.arm", { trackId: track.id, armed: arming });
      if (!result) return;
      if (arming) App.monitored.add(track.id);
      renderTracks();
      if (!App.recording && document.getElementById("monitor-enabled").checked) openMonitoring();
    }),
    toggleButton("mute", "M", track.muted, "Mute", () => dispatch("track.mute", { trackId: track.id, muted: !track.muted })),
    toggleButton("solo", "S", track.soloed, "Solo", () => dispatch("track.solo", { trackId: track.id, soloed: !track.soloed })),
  );
  const meter = element("span", "meter");
  const bar = document.createElement("i");
  meter.append(bar);
  controls.append(meter);
  const remove = element("button", "delete", "×");
  remove.title = "Delete this track";
  remove.addEventListener("click", (event) => {
    event.stopPropagation();
    if (window.confirm(`Delete "${track.name}" and its takes?`)) dispatch("track.remove", { trackId: track.id });
  });
  controls.append(remove);
  const source = document.createElement("select");
  source.className = "track-source";
  source.title = "Recording source";
  for (const [value, label] of [["input:0", "Input 1"], ["input:1", "Input 2"], ["system", "PC audio"]]) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    source.append(option);
  }
  source.value = track.source === "system" ? "system" : `input:${track.channel}`;
  source.addEventListener("change", async () => {
    const [kind, channel] = source.value.split(":");
    const result = await dispatch("track.source", { trackId: track.id, source: kind, channel: Number(channel || 0) });
    if (result && !App.recording && document.getElementById("monitor-enabled").checked) openMonitoring();
  });
  row.append(top, controls, source, volumeControl(track), panControl(track));
  App.meterNodes.set(track.id, bar);
  return row;
}

function volumeControl(track) {
  const wrap = element("label", "track-volume");
  wrap.title = "Monitor volume";
  const slider = document.createElement("input");
  slider.type = "range";
  slider.min = "-60";
  slider.max = "6";
  slider.step = "1";
  slider.value = String(Math.round(track.volumeDb || 0));
  const value = element("span", "volume-value", `${Math.round(Number(slider.value))} dB`);
  slider.addEventListener("input", () => {
    value.textContent = `${Math.round(Number(slider.value))} dB`;
  });
  slider.addEventListener("change", () => {
    dispatch("track.volume", { trackId: track.id, volumeDb: Number(slider.value) });
  });
  wrap.append(element("span", "unit", "vol"), slider, value);
  return wrap;
}

function panLabel(value) {
  if (value === 0) return "C";
  return `${value < 0 ? "L" : "R"} ${Math.abs(value)}`;
}

function panControl(track) {
  const wrap = element("div", "track-pan");
  wrap.title = "Monitor pan: drag up for right, down for left; double-click to centre";
  const knob = element("span", "pan-knob");
  knob.tabIndex = 0;
  knob.setAttribute("role", "slider");
  knob.setAttribute("aria-label", "Pan");
  knob.setAttribute("aria-orientation", "horizontal");
  knob.setAttribute("aria-valuemin", "-100");
  knob.setAttribute("aria-valuemax", "100");
  const pointer = element("i");
  knob.append(pointer);
  const value = element("span", "pan-value");
  let current = Math.round((Number(track.pan) || 0) * 100);
  const paint = (next) => {
    current = Math.max(-100, Math.min(100, Math.round(next)));
    knob.style.setProperty("--pan-angle", `${current * 1.35}deg`);
    knob.setAttribute("aria-valuenow", String(current));
    knob.setAttribute("aria-valuetext", current === 0 ? "Centre" : `${Math.abs(current)}% ${current < 0 ? "left" : "right"}`);
    value.textContent = panLabel(current);
  };
  paint(current);
  let startY = 0;
  let startValue = 0;
  knob.addEventListener("pointerdown", (event) => {
    if (event.button !== 0) return;
    event.preventDefault();
    knob.focus();
    knob.setPointerCapture(event.pointerId);
    knob.classList.add("dragging");
    startY = event.clientY;
    startValue = current;
  });
  knob.addEventListener("pointermove", (event) => {
    if (!knob.hasPointerCapture(event.pointerId)) return;
    paint(startValue + ((startY - event.clientY) / 150) * 200);
  });
  const commit = (event) => {
    if (!knob.hasPointerCapture(event.pointerId)) return;
    knob.releasePointerCapture(event.pointerId);
    knob.classList.remove("dragging");
    dispatch("track.pan", { trackId: track.id, pan: current / 100 });
  };
  knob.addEventListener("pointerup", commit);
  knob.addEventListener("pointercancel", commit);
  knob.addEventListener("dblclick", () => {
    paint(0);
    dispatch("track.pan", { trackId: track.id, pan: 0 });
  });
  knob.addEventListener("keydown", (event) => {
    const step = event.shiftKey ? 10 : 5;
    if (event.key === "ArrowUp" || event.key === "ArrowRight") paint(current + step);
    else if (event.key === "ArrowDown" || event.key === "ArrowLeft") paint(current - step);
    else if (event.key === "Home") paint(0);
    else return;
    event.preventDefault();
    dispatch("track.pan", { trackId: track.id, pan: current / 100 });
  });
  wrap.append(element("span", "unit", "pan"), knob, value);
  return wrap;
}

function addInputTrack() {
  const name = window.prompt("Track name", "Guitar");
  if (!name) return;
  const used = new Set(App.state.tracks.filter((track) => track.source === "input").map((track) => track.channel));
  const channel = used.has(0) ? 1 : 0;
  dispatch("track.add", { name, source: "input", channel });
}

function addSystemTrack() {
  const name = window.prompt("Track name", "PC audio");
  if (!name) return;
  dispatch("track.add", { name, source: "system", channel: 0 });
}

function renderInspector() {
  const panel = document.getElementById("inspector");
  panel.innerHTML = "";
  App.eqGraph = null;
  App.eqTrackId = null;
  App.compGraph = null;
  App.compTrackId = null;
  const track = currentTrack();
  if (!track) {
    panel.append(element("p", "empty", "Select a track to edit its takes and effects."));
    return;
  }
  panel.append(element("h2", "", track.name));
  panel.append(takesSection(track));
  panel.append(effectsSection(track));
}

function takesSection(track) {
  const details = document.createElement("details");
  details.className = "takes";
  details.open = App.openTakes.has(track.id);
  details.addEventListener("toggle", () => {
    if (details.open) App.openTakes.add(track.id);
    else App.openTakes.delete(track.id);
  });
  const takes = App.state.takes.filter((take) => take.trackId === track.id);
  const summary = document.createElement("summary");
  summary.className = "section-toggle";
  summary.append(
    element("span", "section-title", "Take stack"),
    element("span", "section-meta", `${takes.length} take${takes.length === 1 ? "" : "s"}`),
  );
  details.append(summary);
  if (!takes.length) {
    details.append(element("p", "empty", "No takes yet."));
    return details;
  }
  takes.forEach((take, index) => details.append(takeRow(track, take, index)));
  return details;
}

function takeRow(track, take, index) {
  const row = element("div", `take${take.id === track.activeTakeId ? " active" : ""}`);
  const head = element("div", "take-head");
  const active = document.createElement("input");
  active.type = "radio";
  active.name = `active-${track.id}`;
  active.checked = take.id === track.activeTakeId;
  active.title = "Make this take audible";
  active.addEventListener("change", () => dispatch("take.active", { trackId: track.id, takeId: take.id }));
  const name = element("span", "take-name", `take ${index + 1}${take.incomplete ? " · partial" : ""}`);
  const meta = element("span", "take-meta", formatDuration(takeLength(take) / take.sampleRate));
  const normalize = element("button", "", "normalize");
  normalize.title = "Set the gain so the loudest peak reaches -1 dBFS";
  normalize.addEventListener("click", () => normalizeTake(take));
  const remove = element("button", "", "×");
  remove.title = "Delete this take";
  remove.addEventListener("click", () => deleteTake(take.id));
  head.append(active, name, meta, normalize, remove);
  const controls = element("div", "take-controls");
  const muteLabel = document.createElement("label");
  const mute = document.createElement("input");
  mute.type = "checkbox";
  mute.checked = take.muted;
  mute.addEventListener("change", () => dispatch("take.mute", { takeId: take.id, muted: mute.checked }));
  muteLabel.append(mute, document.createTextNode("mute"));
  const gainLabel = document.createElement("label");
  const gain = document.createElement("input");
  gain.type = "range";
  gain.min = "-24";
  gain.max = "24";
  gain.step = "0.5";
  gain.value = take.gainDb;
  const gainValue = element("span", "gain-value", `${take.gainDb.toFixed(1)} dB`);
  gain.addEventListener("input", () => { gainValue.textContent = `${Number(gain.value).toFixed(1)} dB`; });
  gain.addEventListener("change", () => dispatch("take.gain", { takeId: take.id, gainDb: Number(gain.value) }));
  gainLabel.append(document.createTextNode("gain"), gain, gainValue);
  controls.append(muteLabel, gainLabel, trimControl(take, "in", true), trimControl(take, "out", false));
  row.append(head, controls);
  return row;
}

function trimControl(take, label, isStart) {
  const wrapper = document.createElement("label");
  const input = document.createElement("input");
  input.type = "number";
  input.min = "0";
  input.step = "0.01";
  input.value = ((isStart ? take.trim.startFrame : take.trim.endFrame || take.lengthFrames) / take.sampleRate).toFixed(2);
  input.title = `Trim ${label} in seconds`;
  input.addEventListener("change", () => {
    const startFrame = isStart ? Math.max(0, Math.round(Number(input.value) * take.sampleRate)) : take.trim.startFrame;
    let endFrame = isStart ? take.trim.endFrame : Math.round(Number(input.value) * take.sampleRate);
    if (!isStart && endFrame >= take.lengthFrames) endFrame = 0;
    dispatch("take.trim", { takeId: take.id, startFrame, endFrame });
  });
  wrapper.append(document.createTextNode(label), input, document.createTextNode("s"));
  return wrapper;
}

async function normalizeTake(take) {
  const audio = App.pcm.get(take.file);
  if (!audio) return setStatus("This take's audio is not loaded.", "error");
  setStatus("Measuring the peak…", "busy");
  await nextPaint();
  try {
    registerPCM(take.file, audio);
    applyState(coreRequest("take.normalize", { takeId: take.id }).state);
    setStatus("Normalized to -1 dBFS");
  } catch (error) {
    setStatus(error.message, "error");
  } finally {
    window.fanamixClearPCM(take.file);
  }
}

function effectsSection(track) {
  const section = element("div");
  const head = element("div", "fx-heading");
  head.append(element("h3", "", "Effects"));
  const actions = element("span", "fx-actions");
  const file = document.createElement("input");
  file.type = "file";
  file.accept = "application/json,.json";
  file.hidden = true;
  const load = element("button", "", "Load");
  load.type = "button";
  load.title = "Load an effects preset from a JSON file";
  load.addEventListener("click", () => {
    file.value = "";
    file.click();
  });
  file.addEventListener("change", () => {
    const selected = file.files?.[0];
    if (selected) loadEffectsPreset(track, selected);
  });
  const save = element("button", "", "Download");
  save.type = "button";
  save.title = "Download these effects as a JSON preset";
  save.addEventListener("click", () => downloadEffects(track));
  actions.append(load, save);
  head.append(actions, file);
  section.append(head, presetPicker(track), addEffectControl(track));
  const chain = Array.isArray(track.effects?.Chain) ? track.effects.Chain : [];
  if (!chain.length) {
    section.append(element("p", "empty", "No effects on this track. Add one above."));
    return section;
  }
  const ordinals = new Map();
  chain.forEach((effect, index) => {
    const kind = String(effect?.Kind || "");
    const ordinal = ordinals.get(kind) || 0;
    ordinals.set(kind, ordinal + 1);
    section.append(effectBlock(track, effect, index, chain.length, ordinal));
  });
  return section;
}

function addEffectControl(track) {
  const row = element("div", "fx-add");
  row.append(element("span", "", "Add"));
  const select = document.createElement("select");
  select.append(new Option("Choose an effect…", ""));
  for (const entry of EFFECT_KINDS) select.append(new Option(entry.title, entry.kind));
  const add = element("button", "", "Add");
  add.type = "button";
  add.addEventListener("click", () => {
    if (!select.value) return;
    dispatch("track.effect.add", { trackId: track.id, kind: select.value });
  });
  row.append(select, add);
  return row;
}

function downloadEffects(track) {
  try {
    const preset = effectsToPreset(track.effects);
    download(`${track.name}-effects.json`, JSON.stringify(preset, null, 2), "application/json", ".json");
    setStatus(`Downloaded ${track.name} effects`);
  } catch (error) {
    setStatus(error.message, "error");
  }
}

function presetPicker(track) {
  const row = element("label", "fx-presets");
  row.append(element("span", "", "Preset"));
  const select = document.createElement("select");
  select.append(new Option("Choose a preset…", ""));
  const groups = new Map();
  for (const entry of FACTORY_PRESETS) {
    let group = groups.get(entry.group);
    if (!group) {
      group = document.createElement("optgroup");
      group.label = entry.group;
      groups.set(entry.group, group);
      select.append(group);
    }
    group.append(new Option(entry.name, entry.name));
  }
  select.addEventListener("change", () => {
    const entry = FACTORY_PRESETS.find((candidate) => candidate.name === select.value);
    select.value = "";
    if (entry) applyEffectsPreset(track, entry.preset, `Applied ${entry.name} preset`);
  });
  row.append(select);
  return row;
}

async function applyEffectsPreset(track, preset, message) {
  try {
    const effects = effectsFromPreset(preset, track.effects);
    const result = await dispatch("track.effects", { trackId: track.id, effects });
    if (result) setStatus(message);
  } catch (error) {
    setStatus(error.message, "error");
  }
}

async function loadEffectsPreset(track, file) {
  let preset;
  try {
    preset = JSON.parse(await file.text());
  } catch (error) {
    return setStatus("That file is not valid JSON.", "error");
  }
  await applyEffectsPreset(track, preset, `Loaded ${track.name} effects from ${file.name}`);
}

function effectBlock(track, effect, index, count, ordinal) {
  const kind = String(effect?.Kind || "");
  const spec = EFFECT_KINDS.find((entry) => entry.kind === kind);
  const block = document.createElement("details");
  block.className = "fx";
  if (!spec) return block;
  const settings = effect[spec.state] || {};
  const key = `${track.id}:${kind}:${ordinal}`;
  block.open = App.openEffects.has(key);
  block.addEventListener("toggle", () => {
    if (block.open) App.openEffects.add(key);
    else App.openEffects.delete(key);
  });
  const head = document.createElement("summary");
  head.className = "fx-head section-toggle";
  const toggle = document.createElement("input");
  toggle.type = "checkbox";
  toggle.title = `Enable ${spec.title}`;
  toggle.checked = Boolean(settings.Enabled);
  // Keep the checkbox and the buttons plain controls: clicking one must not
  // fold the block.
  toggle.addEventListener("click", (event) => event.stopPropagation());
  toggle.addEventListener("change", () => {
    const effects = cloneEffects(track.effects);
    effects.Chain[index][spec.state].Enabled = toggle.checked;
    dispatch("track.effects", { trackId: track.id, effects });
  });
  const actions = element("span", "fx-item-actions");
  actions.append(
    itemButton("▲", "Move this effect earlier", index === 0, () => {
      dispatch("track.effect.move", { trackId: track.id, from: index, to: index - 1 });
    }),
    itemButton("▼", "Move this effect later", index === count - 1, () => {
      dispatch("track.effect.move", { trackId: track.id, from: index, to: index + 1 });
    }),
    itemButton("×", "Remove this effect", false, () => {
      dispatch("track.effect.remove", { trackId: track.id, index });
    }),
  );
  head.append(toggle, element("span", "section-title", spec.title), actions, element("span", "fx-chevron"));
  block.append(head);
  const graphKey = spec.kind === "eq" ? "EQ" : spec.kind === "comp" ? "Comp" : "";
  const graph = effectGraph(graphKey, settings, App.state.sampleRate);
  if (graph) block.append(graph.element);
  if (graph && spec.kind === "eq") {
    App.eqGraph = graph;
    App.eqTrackId = track.id;
  }
  if (graph && spec.kind === "comp") {
    App.compGraph = graph;
    App.compTrackId = track.id;
  }
  for (const [label, path, min, max, step] of spec.controls) {
    block.append(effectRow(track, effect, index, spec, label, path, min, max, step, graph));
  }
  return block;
}

function itemButton(text, title, disabled, handler) {
  const button = element("button", "", text);
  button.type = "button";
  button.title = title;
  button.disabled = disabled;
  button.addEventListener("click", (event) => {
    event.stopPropagation();
    if (!button.disabled) handler();
  });
  return button;
}

function effectRow(track, effect, index, spec, label, path, min, max, step, graph) {
  const row = element("div", "fx-row");
  row.append(element("span", "", effectLabel(effect, spec, path, label)));
  const slider = document.createElement("input");
  slider.type = "range";
  slider.min = min;
  slider.max = max;
  slider.step = step;
  const current = Number(readPath(effect[spec.state] || {}, path));
  slider.value = Number.isFinite(current) ? current : min;
  const value = element("span", "value", formatEffectValue(Number(slider.value), max - min));
  slider.addEventListener("input", () => {
    value.textContent = formatEffectValue(Number(slider.value), max - min);
    if (!graph) return;
    const preview = cloneEffects(track.effects);
    writePath(preview.Chain[index][spec.state], path, Number(slider.value));
    graph.update(preview.Chain[index][spec.state]);
  });
  slider.addEventListener("change", () => {
    const effects = cloneEffects(track.effects);
    writePath(effects.Chain[index][spec.state], path, Number(slider.value));
    dispatch("track.effects", { trackId: track.id, effects });
  });
  row.append(slider, value);
  return row;
}

function cloneEffects(effects) {
  return JSON.parse(JSON.stringify(effects));
}

function readPath(object, path) {
  return path.split(".").reduce((value, key) => (value == null ? undefined : value[key]), object);
}

function writePath(object, path, value) {
  const keys = path.split(".");
  let target = object;
  for (const key of keys.slice(0, -1)) target = target[key];
  target[keys[keys.length - 1]] = value;
}

function formatEffectValue(value, span) {
  return span <= 2 ? value.toFixed(2) : value.toFixed(1);
}

// Presets may move the EQ bands, so the slider label follows the instance's
// actual band frequency instead of the default 20 / 100 / 1k / 6k / 20k text.
function effectLabel(effect, spec, path, fallback) {
  if (spec.state !== "EQ") return fallback;
  const match = /^Bands\.(\d+)\./.exec(path);
  if (!match) return fallback;
  const frequency = Number(effect.EQ?.Bands?.[Number(match[1])]?.FrequencyHz);
  if (!Number.isFinite(frequency) || frequency <= 0) return fallback;
  if (frequency < 1000) return String(Math.round(frequency));
  const thousands = frequency / 1000;
  return `${Number.isInteger(thousands) ? thousands : thousands.toFixed(1)}k`;
}

function renderTransport() {
  const app = document.getElementById("app");
  app.classList.toggle("recording", App.recording);
  app.classList.toggle("playing", App.playing && !App.recording);
  document.getElementById("btn-record").title = App.recording ? "Stop recording (R)" : "Record (R)";
  updateTimecode();
}

function updateTimecode() {
  const rate = App.state?.sampleRate || 48000;
  document.getElementById("timecode").textContent = formatDuration(App.frame / rate);
}

function syncControls() {
  if (!App.state) return;
  document.getElementById("click-enabled").checked = Boolean(App.state.click.Enabled);
  document.getElementById("click-bpm").value = App.state.click.BPM;
  document.getElementById("count-in").checked = App.state.click.countInBars > 0;
  document.getElementById("track-list").scrollTop = 0;
  renderTransport();
}

function pushClick() {
  if (!App.state) return;
  const enabled = document.getElementById("click-enabled").checked;
  const bpm = Math.min(300, Math.max(20, Number(document.getElementById("click-bpm").value) || 120));
  const countInBars = document.getElementById("count-in").checked ? 1 : 0;
  dispatch("click.set", { enabled, bpm, beatsPerBar: App.state.click.beatsPerBar, countInBars });
}

function ensureEngine() {
  if (!App.engine) {
    App.engine = new AudioEngine();
    App.engine.setMonitor(document.getElementById("monitor-enabled").checked);
    App.engine.setMasterVolume(readVolume());
    App.engine.setOutputs(App.outputIds).catch((error) => setStatus(`Master output: ${error.message}`, "error"));
  }
  return App.engine;
}

function readVolume() {
  const raw = window.localStorage.getItem(VOLUME_KEY);
  if (raw === null) return 1;
  const level = Number(raw);
  return Number.isFinite(level) ? Math.min(1, Math.max(0, level)) : 1;
}

function updateVolumeLabel() {
  const level = Number(document.getElementById("master-volume").value) || 0;
  document.getElementById("master-volume-value").textContent = `${Math.round(level)}%`;
}

function readOutputPreference() {
  try {
    const stored = JSON.parse(window.localStorage.getItem(OUTPUT_KEY) || "null");
    if (!Array.isArray(stored)) return [""];
    const ids = [...new Set(stored.filter((id) => typeof id === "string"))];
    return ids.length ? ids : [""];
  } catch (error) {
    return [""];
  }
}

function outputName(id) {
  const output = App.outputs.find((item) => item.id === id);
  return output ? output.label : (id ? "Selected output" : "System default");
}

function updateOutputLabel() {
  const label = document.getElementById("output-label");
  if (!label) return;
  const names = App.outputIds.map(outputName);
  label.textContent = names.length > 1 ? `${names[0]} +${names.length - 1}` : names[0];
  const menu = document.getElementById("master-output");
  if (menu) menu.title = `Master output: ${names.join(", ")} — click to change`;
  const hint = document.getElementById("output-hint");
  if (hint) {
    hint.textContent = names.length > 1
      ? `${names.length} outputs active. The first is the primary.`
      : "Select one or more outputs. The first is the primary.";
  }
}

function renderOutputList() {
  const list = document.getElementById("output-list");
  if (!list) return;
  list.innerHTML = "";
  if (!App.outputs.length) {
    list.append(element("p", "output-empty", "No output devices found."));
    return;
  }
  for (const output of App.outputs) {
    const row = element("label", "output-option");
    const input = document.createElement("input");
    input.type = "checkbox";
    input.value = output.id;
    input.checked = App.outputIds.includes(output.id);
    input.addEventListener("change", () => {
      const checked = [...list.querySelectorAll("input[type=checkbox]:checked")].map((node) => node.value);
      chooseOutputs(checked);
    });
    row.append(input, element("span", "output-name", output.label));
    row.classList.toggle("primary", output.id === App.outputIds[0]);
    list.append(row);
  }
}

function syncOutputChecks() {
  const list = document.getElementById("output-list");
  if (!list) return;
  for (const input of list.querySelectorAll("input[type=checkbox]")) {
    input.checked = App.outputIds.includes(input.value);
    input.closest(".output-option").classList.toggle("primary", input.value === App.outputIds[0]);
  }
}

function chooseOutputs(ids) {
  const available = new Set(App.outputs.map((output) => output.id));
  let selected = [...new Set(ids.filter((id) => available.has(id)))];
  if (!selected.length) selected = [""];
  App.outputIds = selected;
  App.outputPreference = selected;
  try {
    window.localStorage.setItem(OUTPUT_KEY, JSON.stringify(selected));
  } catch (error) {
    // Private browsing can refuse local storage; keep the choice in memory.
  }
  syncOutputChecks();
  updateOutputLabel();
  if (!App.engine) return;
  App.engine.setOutputs(selected)
    .then(() => setStatus(selected.length > 1 ? `Master on ${selected.length} outputs` : `Master output: ${outputName(selected[0])}`))
    .catch((error) => setStatus(`Master output: ${error.message}`, "error"));
}

async function refreshOutputs() {
  const token = ++App.outputRefreshToken;
  if (!navigator.mediaDevices?.enumerateDevices) {
    App.outputs = [{ id: "", label: "System default" }];
    App.outputIds = [""];
    renderOutputList();
    updateOutputLabel();
    return;
  }
  let devices;
  try {
    devices = await navigator.mediaDevices.enumerateDevices();
  } catch (error) {
    setStatus("Could not list audio outputs.", "error");
    return;
  }
  if (token !== App.outputRefreshToken) return;
  const outputs = [];
  const seen = new Set();
  for (const device of devices) {
    if (device.kind !== "audiooutput") continue;
    const id = device.deviceId === "default" ? "" : device.deviceId;
    if (seen.has(id)) continue;
    seen.add(id);
    const fallback = id ? `Output ${outputs.length + 1}` : "System default";
    outputs.push({ id, label: device.label || fallback });
  }
  if (!seen.has("")) outputs.push({ id: "", label: "System default" });
  outputs.sort((left, right) => {
    if (left.id === "") return -1;
    if (right.id === "") return 1;
    return left.label.localeCompare(right.label);
  });
  App.outputs = outputs;
  const available = new Set(outputs.map((output) => output.id));
  let selected = App.outputPreference.filter((id) => available.has(id));
  selected = [...new Set(selected)];
  if (!selected.length) selected = [""];
  App.outputIds = selected;
  renderOutputList();
  updateOutputLabel();
  if (!App.engine) return;
  try {
    await App.engine.setOutputs(selected);
  } catch (error) {
    if (token === App.outputRefreshToken) setStatus(`Master output: ${error.message}`, "error");
  }
}

function disposeEngine() {
  if (App.engine) App.engine.dispose();
  App.engine = null;
}

async function openMonitoring() {
  if (!App.state || App.recording) return;
  const live = App.state.tracks.filter((track) => App.monitored.has(track.id));
  const kinds = [...new Set(live.map((track) => track.source))]
    .filter((kind) => kind === "input" || kind === "system");
  if (!kinds.length) return;
  const engine = ensureEngine();
  try {
    const failures = await engine.openSources(kinds, App.state.sampleRate);
    if (App.engine !== engine || !App.state) return;
    refreshOutputs();
    engine.syncMonitoring(App.state, App.monitored);
    if (failures.system) setStatus(sourceErrorMessage(failures.system, "system"), "error");
    else if (failures.input) setStatus(sourceErrorMessage(failures.input, "input"), "error");
    else {
      const label = kinds.length > 1 ? "PC audio and input" : (kinds[0] === "system" ? "PC audio" : "input");
      setStatus(`Monitoring ${label} — press record to capture.`);
    }
  } catch (error) {
    setStatus(inputErrorMessage(error), "error");
  }
}

async function toggleRecord() {
  if (App.countingIn) return;
  if (App.recording) return stopRecording();
  return startRecording();
}

async function startRecording() {
  if (!App.state) return;
  const armed = App.state.tracks.filter((track) => track.armed);
  if (!armed.length) return setStatus("Arm a track before recording.", "error");
  const kinds = [...new Set(armed.map((track) => track.source))];
  const engine = ensureEngine();
  let failures;
  try {
    failures = await engine.openSources(kinds, App.state.sampleRate);
  } catch (error) {
    return setStatus(sourceErrorMessage(error, kinds[0]), "error");
  }
  for (const kind of kinds) {
    if (failures[kind]) return setStatus(sourceErrorMessage(failures[kind], kind), "error");
  }
  refreshOutputs();
  engine.syncMonitoring(App.state, App.monitored);
  const countInBars = Math.min(2, Math.max(0, Number(App.state.click.countInBars) || 0));
  if (countInBars > 0) {
    App.countingIn = true;
    try {
      await engine.countIn({
        bpm: App.state.click.bpm,
        beatsPerBar: App.state.click.beatsPerBar,
        bars: countInBars,
        onBeat: (remaining) => setStatus(`Count-in · ${remaining}`),
      });
    } catch (error) {
      App.countingIn = false;
      return setStatus(error.message, "error");
    }
    App.countingIn = false;
  }
  const playhead = Math.round(App.frame);
  const takeStart = Math.max(0, playhead - engine.latencyFrames);
  const drop = Math.max(0, engine.latencyFrames - playhead + takeStart);
  let begin;
  try {
    begin = coreRequest("take.begin", { startFrame: takeStart });
  } catch (error) {
    return setStatus(error.message, "error");
  }
  App.pendingTake = {
    takes: (begin.takes || []).map((start) => ({
      takeId: start.id,
      takeFile: start.file,
      trackId: start.trackId,
      channels: start.channels,
    })),
    startFrame: takeStart,
    drop,
    playhead,
  };
  App.recording = true;
  App.playing = true;
  applyState(begin.state);
  try {
    await engine.start(App.state, App.pcm, playhead, { record: true, slots: begin.slots });
  } catch (error) {
    App.recording = false;
    App.playing = false;
    const takes = App.pendingTake?.takes || [];
    App.pendingTake = null;
    for (const take of takes) await dispatch("take.abort", { takeId: take.takeId });
    return setStatus(error.message, "error");
  }
  ensureAnimation();
  if (kinds.length > 1) setStatus("Recording PC audio and input…");
  else if (kinds[0] === "system") setStatus("Recording PC audio…");
  else if (armed.length > 1) setStatus(`Recording ${armed.length} inputs…`);
  else setStatus("Recording…");
}

async function stopRecording() {
  const pending = App.pendingTake;
  if (!pending) return;
  App.pendingTake = null;
  App.recording = false;
  App.playing = false;
  const captured = App.engine ? await App.engine.stopRecording(pending.drop) : null;
  App.frame = 0;
  renderTransport();
  if (!captured || captured.length !== pending.takes.length || captured[0].frames < 128) {
    for (const take of pending.takes) await dispatch("take.abort", { takeId: take.takeId });
    return setStatus("The take was too short to keep.", "error");
  }
  let state = null;
  try {
    pending.takes.forEach((take, index) => {
      const audio = {
        channels: captured[index].channels,
        sampleRate: captured[index].sampleRate,
        samples: captured[index].samples,
      };
      App.pcm.set(take.takeFile, audio);
      App.peaks.set(take.takeFile, computePeaks(audio.samples, audio.channels));
      state = coreRequest("take.finish", { takeId: take.takeId, frames: captured[index].frames }).state;
    });
    applyState(state);
    await Promise.all(pending.takes.map((take) => {
      const audio = App.pcm.get(take.takeFile);
      return storage.saveTake(App.root, take.takeFile, audio.channels, audio.sampleRate, audio.samples);
    }));
    const label = pending.takes.length === 1 ? "Take saved" : `${pending.takes.length} takes saved`;
    setStatus(`${label} · ${formatDuration(captured[0].frames / captured[0].sampleRate)}`);
  } catch (error) {
    setStatus(error.message, "error");
  }
  drawTimelineNow();
}

async function togglePlayback() {
  if (App.recording) return stopRecording();
  if (!App.state) return;
  if (App.playing) return stopTransport();
  if (!audibleTakes(App.state).length) return setStatus("Record a take or select an audible one first.", "error");
  const engine = ensureEngine();
  try {
    await engine.start(App.state, App.pcm, Math.round(App.frame));
  } catch (error) {
    return setStatus(error.message, "error");
  }
  App.playing = true;
  renderTransport();
  ensureAnimation();
  setStatus("Playing");
}

function stopTransport() {
  const frame = App.engine ? App.engine.position() : 0;
  App.engine?.stop();
  App.playing = false;
  App.recording = false;
  App.frame = 0;
  renderTransport();
  drawTimelineNow();
  updateMeters();
  if (frame > 0) setStatus("Stopped");
}

function addMarker() {
  if (!App.state) return;
  const name = window.prompt("Marker name", `Marker ${App.state.markers.length + 1}`);
  if (name === null) return;
  dispatch("marker.add", { frame: Math.round(App.frame), name });
}

function timelinePoint(event) {
  const box = event.currentTarget.getBoundingClientRect();
  return { x: event.clientX - box.left, y: event.clientY - box.top };
}

function startTimelinePointer(event) {
  if (event.button !== 0) return;
  const canvas = event.currentTarget;
  const point = timelinePoint(event);
  const clip = clipAt(canvas, point.x, point.y);
  if (!clip) {
    if (App.selectedTakeId) {
      App.selectedTakeId = null;
      drawTimelineNow();
    }
    return seekFromPointer(event);
  }
  event.preventDefault();
  App.selectedTakeId = clip.takeId;
  if (App.selectedTrackId !== clip.trackId) selectTrack(clip.trackId);
  else drawTimelineNow();
  App.drag = {
    pointerId: event.pointerId,
    canvas,
    takeId: clip.takeId,
    originFrame: clip.startFrame,
    startX: point.x,
    pxPerFrame: canvas.layout?.pxPerFrame || 1,
    moved: false,
  };
  canvas.setPointerCapture(event.pointerId);
  canvas.style.cursor = "grabbing";
}

function moveTimelinePointer(event) {
  const canvas = event.currentTarget;
  const drag = App.drag;
  if (!drag || drag.pointerId !== event.pointerId) {
    const point = timelinePoint(event);
    canvas.style.cursor = clipAt(canvas, point.x, point.y) ? "grab" : "text";
    return;
  }
  if (!canvas.hasPointerCapture(event.pointerId)) return;
  const point = timelinePoint(event);
  if (Math.abs(point.x - drag.startX) > 2) drag.moved = true;
  if (!drag.moved) return;
  const take = App.state?.takes.find((candidate) => candidate.id === drag.takeId);
  if (!take) return;
  const delta = Math.round((point.x - drag.startX) / drag.pxPerFrame);
  take.startFrame = Math.max(0, drag.originFrame + delta);
  drawTimelineNow();
}

function finishTimelinePointer(event, cancelled) {
  const drag = App.drag;
  if (!drag || drag.pointerId !== event.pointerId) return;
  App.drag = null;
  const canvas = drag.canvas;
  if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
  canvas.style.cursor = "text";
  const take = App.state?.takes.find((candidate) => candidate.id === drag.takeId);
  if (cancelled || !drag.moved) {
    if (take) take.startFrame = drag.originFrame;
    drawTimelineNow();
    if (!cancelled) seekFromPointer(event);
    return;
  }
  const frame = take ? take.startFrame : drag.originFrame;
  dispatch("take.move", { takeId: drag.takeId, startFrame: frame }).then((result) => {
    if (result) return setStatus(`Moved clip to ${formatDuration(frame / App.state.sampleRate)}`);
    if (take) take.startFrame = drag.originFrame;
    drawTimelineNow();
  });
}

function seekFromPointer(event) {
  if (!App.state) return;
  const canvas = event.currentTarget;
  const x = event.clientX - canvas.getBoundingClientRect().left;
  const marker = markerAtX(canvas, x);
  if (marker) {
    if (App.playing) stopTransport();
    App.frame = marker.frame;
    updateTimecode();
    drawTimelineNow();
    setStatus(`Marker: ${marker.name}`);
    return;
  }
  const frame = frameAtX(canvas, x);
  if (App.playing) stopTransport();
  App.frame = frame;
  updateTimecode();
  drawTimelineNow();
}

function deleteMarkerAt(event) {
  if (!App.state) return;
  const canvas = event.currentTarget;
  const marker = markerAtX(canvas, event.clientX - canvas.getBoundingClientRect().left);
  if (!marker) return;
  event.preventDefault();
  if (!window.confirm(`Delete marker "${marker.name}"?`)) return;
  dispatch("marker.remove", { markerId: marker.id });
}

function takeUnderPlayhead(track) {
  const frame = Math.round(App.frame);
  let fallback = null;
  for (const take of App.state.takes) {
    if (take.trackId !== track.id || take.muted) continue;
    const length = takeLength(take);
    if (length === 0 || frame < take.startFrame || frame >= take.startFrame + length) continue;
    if (take.id === track.activeTakeId || take.layered) return take;
    fallback = fallback || take;
  }
  return fallback;
}

async function splitTakeAtPlayhead() {
  const track = currentTrack();
  if (!track || App.recording) return;
  const take = takeUnderPlayhead(track);
  if (!take) return setStatus("Move the playhead over a take to split it.", "error");
  const result = await dispatch("take.split", { takeId: take.id, frame: Math.round(App.frame) });
  if (result) setStatus(`Split ${track.name} at the playhead`);
}

function copyTakeAtPlayhead() {
  const track = currentTrack();
  if (!track) return;
  const take = takeUnderPlayhead(track);
  if (!take) return setStatus("Move the playhead over a take to copy it.", "error");
  App.clipboard = { takeId: take.id };
  setStatus(`Copied ${track.name} take — paste with Ctrl/Cmd+V`);
}

async function pasteTakeAtPlayhead() {
  const track = currentTrack();
  if (!track || App.recording) return;
  const sourceId = App.clipboard?.takeId;
  if (!sourceId) return setStatus("Copy a take first (Ctrl/Cmd+C).", "error");
  if (!App.state.takes.some((take) => take.id === sourceId)) {
    App.clipboard = null;
    return setStatus("The copied take no longer exists.", "error");
  }
  const result = await dispatch("take.duplicate", {
    takeId: sourceId,
    trackId: track.id,
    startFrame: Math.round(App.frame),
  });
  if (result) setStatus(`Pasted onto ${track.name} at the playhead`);
}

async function exportMix() {
  if (!App.state) return;
  const selection = audibleTakes(App.state);
  if (!selection.length) return setStatus("Nothing to export: select an audible take first.", "error");
  for (const { take } of selection) {
    if (!App.pcm.get(take.file)) return setStatus(`Missing audio for ${take.file}.`, "error");
  }
  setStatus("Rendering the mix…", "busy");
  await nextPaint();
  try {
    const buffer = await renderMixdown(App.state, App.pcm);
    download(`${App.state.name}.wav`, encodeWav(buffer));
    setStatus(`Exported ${formatDuration(buffer.length / buffer.sampleRate)}`);
  } catch (error) {
    setStatus(error.message, "error");
  }
}

function download(name, data, type = "audio/wav", extension = ".wav") {
  const safe = name.replace(/[\\/:*?"<>|]+/g, "-");
  const url = URL.createObjectURL(new Blob([data], { type }));
  const link = document.createElement("a");
  link.href = url;
  link.download = safe.toLowerCase().endsWith(extension) ? safe : `${safe}${extension}`;
  document.body.append(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1500);
}

function ensureAnimation() {
  if (!App.rafHandle) App.rafHandle = requestAnimationFrame(animate);
}

function animate() {
  App.rafHandle = 0;
  if (!App.state) return;
  if (App.playing || App.recording) {
    App.frame = App.engine ? App.engine.position() : App.frame;
    updateTimecode();
  }
  drawTimelineNow();
  updateMeters();
  updateTuner();
  if (App.playing || App.recording) ensureAnimation();
}

function drawTimelineNow() {
  if (!App.state) return;
  const canvas = document.getElementById("timeline");
  let live = null;
  if (App.pendingTake && App.engine) {
    const peaks = App.engine.livePeaks();
    live = peaks.map((item) => ({ ...item, startFrame: App.pendingTake.startFrame }));
  }
  drawTimeline(canvas, {
    state: App.state,
    peaks: App.peaks,
    live,
    lanes: timelineLanes(canvas),
    frame: App.frame,
    selectedTakeId: App.selectedTakeId,
    zoom: App.zoom,
    startFrame: App.viewStart,
    follow: App.playing || App.recording,
    selectedTrackId: App.selectedTrackId,
  });
  if (canvas.layout) App.viewStart = canvas.layout.startFrame;
}

function zoomTo(next, anchorX) {
  const canvas = document.getElementById("timeline");
  const layout = canvas.layout;
  if (!layout) return;
  const zoom = Math.max(1, Math.min(128, next));
  if (Math.abs(zoom - App.zoom) < 0.001) return;
  const x = anchorX ?? layout.width / 2;
  const anchorFrame = layout.startFrame + x / layout.pxPerFrame;
  App.zoom = zoom;
  App.viewStart = Math.max(0, anchorFrame - (x * layout.baseFrames) / (layout.width * zoom));
  drawTimelineNow();
  setStatus(`Zoom ${Math.round(zoom * 100)}%`);
}

function resetZoom() {
  App.zoom = 1;
  App.viewStart = 0;
  drawTimelineNow();
  setStatus("Zoom reset");
}

function zoomTimeline(event) {
  if (!App.state) return;
  const canvas = event.currentTarget;
  const layout = canvas.layout;
  if (!layout) return;
  event.preventDefault();
  if (event.ctrlKey || event.metaKey) {
    const x = event.clientX - canvas.getBoundingClientRect().left;
    zoomTo(App.zoom * Math.exp(-event.deltaY * 0.002), x);
    return;
  }
  App.viewStart = Math.max(0, layout.startFrame + (event.deltaX + event.deltaY) / layout.pxPerFrame);
  drawTimelineNow();
}

// timelineLanes mirrors the track cards' real geometry so each canvas lane
// lines up with its row in the left panel, including while the list scrolls.
function timelineLanes(canvas) {
  const rows = document.querySelectorAll("#track-list .track");
  if (!rows.length) return null;
  const canvasBox = canvas.getBoundingClientRect();
  return Array.from(rows, (row) => {
    const box = row.getBoundingClientRect();
    return { top: box.top - canvasBox.top, height: box.height };
  });
}

function updateSpectrum() {
  const track = currentTrack();
  if (App.eqGraph) {
    const live = track && track.id === App.eqTrackId && App.engine;
    App.eqGraph.spectrum(live ? App.engine.spectrum(track.id) : null);
  }
  if (App.compGraph) {
    const live = track && track.id === App.compTrackId && App.engine;
    App.compGraph.voice(live ? App.engine.level(track.id) : null);
  }
}

function updateMeters() {
  if (!App.state || !App.meterNodes) return;
  for (const [id, bar] of App.meterNodes) {
    const peak = App.engine ? App.engine.meter(id) : 0;
    const decibels = 20 * Math.log10(peak || 0.000001);
    bar.style.width = `${Math.max(0, Math.min(100, ((decibels + 60) / 60) * 100))}%`;
    bar.classList.toggle("hot", decibels > -6);
    bar.classList.toggle("clip", peak >= 0.999);
  }
}

function updateTuner() {
  const display = document.getElementById("tuner");
  if (!App.state || !App.engine) {
    display.textContent = "—";
    display.classList.add("off");
    return;
  }
  const track = currentTrack();
  if (!track || track.source === "system") {
    display.textContent = "—";
    display.classList.add("off");
    return;
  }
  const reading = App.engine.tuner(track.channel);
  if (!reading) {
    display.textContent = "—";
    display.classList.add("off");
    return;
  }
  display.classList.remove("off");
  const sign = reading.cents > 0 ? "+" : "";
  display.textContent = `${reading.note} ${sign}${reading.cents}¢`;
  display.style.color = Math.abs(reading.cents) <= 5 ? "var(--accent)" : "var(--warn)";
}

function inputErrorMessage(error) {
  if (error?.name === "NotAllowedError") return "Microphone access was denied. Allow it in the site settings and try again.";
  if (error?.name === "NotFoundError") return "No audio input was found on this device.";
  if (error?.name === "NotReadableError") return "The audio input is busy in another application.";
  return error?.message || "The audio input could not be opened.";
}

function sourceErrorMessage(error, kind) {
  if (kind !== "system") return inputErrorMessage(error);
  if (error?.name === "NotAllowedError") return "PC audio sharing was cancelled or denied.";
  if (error?.name === "NotFoundError") return "No tab audio was available to monitor.";
  return error?.message || "PC audio could not be opened.";
}

function showPicker() {
  document.getElementById("picker").hidden = false;
  document.getElementById("app").hidden = true;
  refreshSessions();
}

async function refreshSessions() {
  try {
    App.sessions = await storage.listSessions();
  } catch (error) {
    App.sessions = [];
  }
  const list = document.getElementById("session-list");
  list.innerHTML = "";
  if (!App.sessions.length) {
    list.append(element("p", "hint", "No sessions yet. Create one below."));
    return;
  }
  for (const session of App.sessions) {
    const card = element("div", "session-card");
    const meta = element("div", "meta");
    meta.append(element("div", "name", session.name), element("div", "date", new Date(session.updatedAt).toLocaleString()));
    const open = element("button", "", "Open");
    open.addEventListener("click", () => openSession(session.root));
    const remove = element("button", "", "Delete");
    remove.addEventListener("click", () => deleteSession(session));
    card.append(meta, open, remove);
    list.append(card);
  }
  if (App.state) {
    const back = element("button", "ghost", "Back to current session");
    back.addEventListener("click", () => {
      document.getElementById("picker").hidden = true;
      document.getElementById("app").hidden = false;
    });
    list.append(back);
  }
}

async function openSession(root) {
  try {
    const record = await storage.loadProject(root);
    if (!record) throw new Error("That session could not be found.");
    const result = coreRequest("open", { root, project: record.project });
    const takes = await storage.loadTakes(root);
    purgeAudioTrash();
    disposeEngine();
    App.monitored = new Set(result.state.tracks.filter((track) => track.armed).map((track) => track.id));
    App.pcm = new Map(takes.map((take) => [take.file, { channels: take.channels, sampleRate: take.sampleRate, samples: take.samples }]));
    App.peaks = new Map(takes.map((take) => [take.file, computePeaks(take.samples, take.channels)]));
    App.frame = 0;
    App.playing = false;
    App.recording = false;
    App.pendingTake = null;
    App.selectedTrackId = null;
    App.selectedTakeId = null;
    App.zoom = 1;
    App.viewStart = 0;
    applyState(result.state);
    // A freshly opened project has no undo history, so any audio it does not
    // reference is an orphan from an earlier visit and can be dropped.
    purgeAudioTrash();
    syncControls();
    document.getElementById("picker").hidden = true;
    document.getElementById("app").hidden = false;
    setStatus(`Opened ${result.state.name}`);
  } catch (error) {
    setStatus(error.message, "error");
  }
}

async function createSession(name, sampleRate) {
  if (!name) return;
  if (App.sessions.some((session) => session.name.toLowerCase() === name.toLowerCase())) {
    return setStatus("A session with this name already exists.", "error");
  }
  try {
    const result = coreRequest("create", { name, sampleRate });
    purgeAudioTrash();
    disposeEngine();
    App.monitored = new Set();
    App.pcm = new Map();
    App.peaks = new Map();
    App.frame = 0;
    App.playing = false;
    App.recording = false;
    App.pendingTake = null;
    App.selectedTrackId = null;
    App.selectedTakeId = null;
    App.zoom = 1;
    App.viewStart = 0;
    applyState(result.state);
    // A freshly opened project has no undo history, so any audio it does not
    // reference is an orphan from an earlier visit and can be dropped.
    purgeAudioTrash();
    syncControls();
    document.getElementById("picker").hidden = true;
    document.getElementById("app").hidden = false;
    setStatus("Session created — arm the track and record.");
  } catch (error) {
    setStatus(error.message, "error");
  }
}

async function deleteSession(session) {
  if (!window.confirm(`Delete session "${session.name}" and all its audio?`)) return;
  await storage.deleteSession(session.root);
  for (const [file, kept] of App.audioTrash) {
    if (kept.root === session.root) App.audioTrash.delete(file);
  }
  if (App.root === session.root) {
    disposeEngine();
    App.state = null;
    App.monitored.clear();
    App.pcm.clear();
    App.peaks.clear();
    document.getElementById("app").hidden = true;
  }
  refreshSessions();
}

boot();
