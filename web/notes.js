// Inline piano-roll editor for instrument tracks. Notes are stored in session
// frames; the grid follows the click tempo so a bar lines up with the transport.

import { detectKey } from "./scale.js";

const KEY_WIDTH = 36;
const ROW_HEIGHT = 13;
const ROWS = 24;
const RULER_HEIGHT = 18;
const VISIBLE_BEATS = 8;
const SNAP_BEATS = 0.25;
const MAX_PITCH = 127;
// The keyboard shows two octaves and the wheel walks it: 30 pixels of wheel per
// key, so a notch moves a few keys and a trackpad swipe moves one at a time.
const OCTAVE = 12;
const MIN_KEY = 12;
const MAX_KEY = 96;
const PITCH_WHEEL = 30;
const NOTE_NAMES = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
const BLACK_KEYS = new Set([1, 3, 6, 8, 10]);
const INSTRUMENT_OPTIONS = [["piano", "Piano"], ["bass", "Bass"], ["guitar", "Guitar"], ["drums", "Drums"]];
const LENGTH_OPTIONS = [["0.25", "1/4 beat"], ["0.5", "1/2 beat"], ["1", "1 beat"], ["2", "2 beats"]];

function element(tag, className = "", text = "") {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text) node.textContent = text;
  return node;
}

function pitchName(pitch) {
  return `${NOTE_NAMES[((pitch % 12) + 12) % 12]}${Math.floor(pitch / 12) - 1}`;
}

function pitchClass(pitch) {
  return ((pitch % 12) + 12) % 12;
}

function clampPitch(pitch) {
  return Math.max(0, Math.min(MAX_PITCH, Math.round(pitch)));
}

export function buildNoteEditor({ track, notes, sampleRate, bpm, beatsPerBar, endBeat, view, selection, onSelect, onAdd, onRemove, onShift, onResize, onAudition, onView, onInstrument }) {
  const perBar = Math.max(1, Math.round(Number(beatsPerBar) || 4));
  const contentEnd = Math.max(0, Number(endBeat) || 0);
  const maxStart = Math.max(VISIBLE_BEATS, Math.ceil(contentEnd + perBar / 2));
  const section = element("div", "notes");
  const head = element("div", "notes-head");
  head.append(element("h3", "", "Piano roll"));
  head.append(element("span", "notes-count", `${notes.length} note${notes.length === 1 ? "" : "s"}`));
  // The part's key is read from its notes, so an extracted melody tells which
  // scale it is in and which notes carry it: tonic, third and dominant.
  const key = detectKey(notes);
  const keyLine = element("p", "notes-key");
  if (key) {
    keyLine.append(
      element("b", "", key.name),
      document.createTextNode(` · scale ${key.scale.join(" ")} · tonic ${key.tonicName} · 3rd ${key.thirdName} · 5th ${key.fifthName}`),
    );
  }
  const toolbar = element("div", "notes-toolbar");
  const instrument = document.createElement("select");
  instrument.title = "Instrument sound";
  for (const [value, label] of INSTRUMENT_OPTIONS) instrument.append(new Option(label, value));
  instrument.value = track.instrument || "piano";
  instrument.addEventListener("change", () => onInstrument(instrument.value));
  const length = document.createElement("select");
  length.title = "Length of new notes, which follows the last note you created or resized";
  for (const [value, label] of LENGTH_OPTIONS) length.append(new Option(label, value));
  const state = {
    startBeat: Math.max(0, Math.min(maxStart, Number(view.startBeat) || 0)),
    lowPitch: Math.max(MIN_KEY, Math.min(MAX_KEY, Math.round(Number(view.lowPitch) || 48))),
    lengthBeats: Math.max(0.25, Math.min(64, Number(view.lengthBeats) || 1)),
  };
  // rememberLength keeps the length used for new notes in step with the last
  // note created or resized, so repeated edits stay the same size. A length off
  // the preset list gets its own option so the toolbar never misreports it.
  const rememberLength = (beats) => {
    const value = Math.max(0.25, Math.min(64, Math.round(beats * 4) / 4));
    state.lengthBeats = value;
    for (const option of [...length.options]) {
      if (!LENGTH_OPTIONS.some(([preset]) => preset === option.value)) option.remove();
    }
    if (![...length.options].some((option) => option.value === String(value))) {
      length.append(new Option(`${value} beat${value === 1 ? "" : "s"}`, String(value)));
    }
    length.value = String(value);
    onView({ lengthBeats: value });
  };
  rememberLength(state.lengthBeats);
  length.addEventListener("change", () => rememberLength(Number(length.value)));
  const octaveDown = element("button", "", "8ve −");
  octaveDown.type = "button";
  octaveDown.title = "Show the octave below";
  const octaveUp = element("button", "", "8ve +");
  octaveUp.type = "button";
  octaveUp.title = "Show the octave above";
  const scrollWrap = element("label", "notes-scroll");
  scrollWrap.title = "Scroll the piano roll along the session";
  const scroll = document.createElement("input");
  scroll.type = "range";
  scroll.min = "0";
  scroll.max = String(maxStart);
  scroll.step = String(SNAP_BEATS);
  scroll.value = String(state.startBeat);
  const scrollValue = element("span", "notes-scroll-value");
  scrollWrap.append(element("span", "unit", "scroll"), scroll, scrollValue);
  const positionLabel = (beat) => {
    const bar = Math.floor(beat / perBar) + 1;
    const inBar = beat - (bar - 1) * perBar;
    return inBar < 0.01 ? `bar ${bar}` : `bar ${bar} + ${Number(inBar.toFixed(2))}`;
  };
  const applyView = (next) => {
    Object.assign(state, next);
    state.lowPitch = Math.max(MIN_KEY, Math.min(MAX_KEY, state.lowPitch));
    state.startBeat = Math.max(0, Math.min(maxStart, state.startBeat));
    scroll.value = String(state.startBeat);
    scrollValue.textContent = positionLabel(state.startBeat);
    onView({ ...state });
    paint();
  };
  const shiftOctave = (delta) => {
    applyView({ lowPitch: state.lowPitch + delta });
    wheelPitch = state.lowPitch;
  };
  octaveDown.addEventListener("click", () => shiftOctave(-OCTAVE));
  octaveUp.addEventListener("click", () => shiftOctave(OCTAVE));
  scroll.addEventListener("input", () => applyView({ startBeat: Number(scroll.value) }));
  scrollValue.textContent = positionLabel(state.startBeat);
  toolbar.append(element("span", "unit", "sound"), instrument, length, octaveDown, octaveUp, scrollWrap);

  const canvas = document.createElement("canvas");
  canvas.className = "piano-roll";
  canvas.setAttribute("aria-label", `Piano roll for ${track.name}. Click to add a note, drag to move it, drag its right edge to resize it, drag on empty space to select several notes, Shift-click to add one to the selection, right-click to delete one.`);
  canvas.title = "Wheel scrolls the keyboard · Shift+wheel scrolls the timeline · drag on empty space to select notes";
  section.append(head, key ? keyLine : document.createDocumentFragment(), toolbar, canvas, element("p", "notes-hint", "Click to add · drag on empty space to select several · Shift+click adds one to the selection · drag a selected note to move them together · drag the right edge to resize · Ctrl/Cmd+C copies them and Ctrl/Cmd+V pastes at the playhead · wheel scrolls the keyboard up and down · Shift+wheel scrolls the timeline · right-click to delete · click a key to hear it."));

  const framesPerBeat = () => (sampleRate * 60) / Math.max(20, Number(bpm) || 120);
  const selected = new Set((selection || []).filter((id) => notes.some((note) => note.id === id)));
  let drag = null;
  let marquee = null;
  // wheelPitch keeps the wheel's sub-semitone remainder, so a slow trackpad
  // swipe still walks the keyboard one key at a time.
  let wheelPitch = state.lowPitch;

  function reportSelection() {
    onSelect?.([...selected]);
  }

  function selectedNotes() {
    return notes.filter((note) => selected.has(note.id));
  }

  // limitDrag keeps the whole group inside the MIDI range and the session, so
  // a drag never lands out of bounds and gets refused at release time.
  function limitDrag() {
    const pitches = drag.group.map((note) => note.pitch);
    const firstStart = Math.min(...drag.group.map((note) => note.startFrame));
    drag.pitchDelta = Math.max(-Math.min(...pitches), Math.min(MAX_PITCH - Math.max(...pitches), drag.pitchDelta));
    drag.frameDelta = Math.max(-firstStart, drag.frameDelta);
  }

  function geometry() {
    const width = canvas.clientWidth || 300;
    return { width, height: RULER_HEIGHT + ROWS * ROW_HEIGHT, pxPerBeat: Math.max(6, (width - KEY_WIDTH) / VISIBLE_BEATS) };
  }

  function pitchAtY(y) {
    const row = Math.floor((y - RULER_HEIGHT) / ROW_HEIGHT);
    return clampPitch(state.lowPitch + (ROWS - 1 - row));
  }

  function beatAtX(x, pxPerBeat) {
    return state.startBeat + (x - KEY_WIDTH) / pxPerBeat;
  }

  function snappedBeat(x, pxPerBeat) {
    return Math.max(0, Math.round(beatAtX(x, pxPerBeat) / SNAP_BEATS) * SNAP_BEATS);
  }

  function noteHit(x, y, geo) {
    const pitch = pitchAtY(y);
    const beat = beatAtX(x, geo.pxPerBeat);
    const tolerance = 4 / geo.pxPerBeat;
    const beatFrames = framesPerBeat();
    for (let index = notes.length - 1; index >= 0; index--) {
      const note = notes[index];
      if (note.pitch !== pitch) continue;
      const start = note.startFrame / beatFrames;
      const end = start + note.lengthFrames / beatFrames;
      if (beat >= start - tolerance && beat <= end + tolerance) return note;
    }
    return null;
  }

  // The last few pixels of a note are its resize handle, like a clip edge in a
  // DAW; short notes keep enough body to be grabbed and moved.
  function noteEndX(note, geo) {
    const endBeat = (note.startFrame + note.lengthFrames) / framesPerBeat();
    return KEY_WIDTH + (endBeat - state.startBeat) * geo.pxPerBeat;
  }

  function onResizeEdge(x, note, geo) {
    const width = Math.max(4, (note.lengthFrames / framesPerBeat()) * geo.pxPerBeat);
    return Math.abs(x - noteEndX(note, geo)) <= Math.min(6, Math.max(2, width / 2));
  }

  // noteBounds is the pixel box a note occupies, used by the selection
  // rectangle. It matches the row band rather than the drawn note, so a sweep
  // across a row is enough to catch its notes.
  function noteBounds(note, geo) {
    const beatFrames = framesPerBeat();
    const left = KEY_WIDTH + (note.startFrame / beatFrames - state.startBeat) * geo.pxPerBeat;
    const width = Math.max(4, (note.lengthFrames / beatFrames) * geo.pxPerBeat);
    const row = ROWS - 1 - (note.pitch - state.lowPitch);
    return {
      left,
      right: left + width,
      top: RULER_HEIGHT + row * ROW_HEIGHT,
      bottom: RULER_HEIGHT + (row + 1) * ROW_HEIGHT,
    };
  }

  function marqueeBox(rect) {
    return {
      left: Math.min(rect.originX, rect.x),
      right: Math.max(rect.originX, rect.x),
      top: Math.min(rect.originY, rect.y),
      bottom: Math.max(rect.originY, rect.y),
    };
  }

  function inMarquee(note, geo, rect) {
    const box = noteBounds(note, geo);
    const area = marqueeBox(rect);
    return box.right >= area.left && box.left <= area.right && box.bottom >= area.top && box.top <= area.bottom;
  }

  // highlightedNotes previews the sweep while it is being drawn, so notes light
  // up as the rectangle passes over them.
  function highlightedNotes(geo) {
    if (!marquee?.moved) return selected;
    const marked = new Set(marquee.base);
    for (const note of notes) if (inMarquee(note, geo, marquee)) marked.add(note.id);
    return marked;
  }

  // previewNote applies the drag in progress to one note, so the selection
  // moves as a block while the session itself only changes on release.
  function previewNote(note) {
    if (!drag) return note;
    if (drag.mode === "resize") {
      return note.id === drag.anchor.id ? { ...note, lengthFrames: drag.lengthFrames } : note;
    }
    if (!selected.has(note.id)) return note;
    return {
      ...note,
      pitch: clampPitch(note.pitch + drag.pitchDelta),
      startFrame: Math.max(0, note.startFrame + drag.frameDelta),
    };
  }

  function drawRows(context, geo) {
    context.fillStyle = "#0f141c";
    context.fillRect(0, 0, geo.width, geo.height);
    for (let row = 0; row < ROWS; row++) {
      const pitch = state.lowPitch + (ROWS - 1 - row);
      const y = RULER_HEIGHT + row * ROW_HEIGHT;
      const octave = Math.floor(pitch / 12);
      const black = BLACK_KEYS.has(pitchClass(pitch));
      context.fillStyle = black ? "#151c26" : octave % 2 === 0 ? "#1e2837" : "#1a2230";
      context.fillRect(KEY_WIDTH, y, geo.width - KEY_WIDTH, ROW_HEIGHT - 1);
      if (pitchClass(pitch) === 0) {
        context.fillStyle = "#2b3648";
        context.fillRect(KEY_WIDTH, y, geo.width - KEY_WIDTH, 1);
      }
    }
  }

  function drawGrid(context, geo) {
    const top = RULER_HEIGHT;
    const bottom = geo.height;
    const left = KEY_WIDTH;
    const xOf = (beat) => Math.round(left + (beat - state.startBeat) * geo.pxPerBeat) + 0.5;
    if (geo.pxPerBeat * SNAP_BEATS >= 5) {
      context.strokeStyle = "#161e29";
      context.beginPath();
      const first = Math.ceil(state.startBeat / SNAP_BEATS) * SNAP_BEATS;
      for (let beat = first; beat <= state.startBeat + VISIBLE_BEATS; beat += SNAP_BEATS) {
        if (Math.abs(beat - Math.round(beat)) < 0.001) continue;
        context.moveTo(xOf(beat), top);
        context.lineTo(xOf(beat), bottom);
      }
      context.stroke();
    }
    const lastBeat = Math.floor(state.startBeat + VISIBLE_BEATS);
    for (let beat = Math.ceil(state.startBeat); beat <= lastBeat; beat++) {
      const isBar = ((beat % perBar) + perBar) % perBar === 0;
      context.strokeStyle = isBar ? "#35435a" : "#222d3b";
      context.beginPath();
      context.moveTo(xOf(beat), top);
      context.lineTo(xOf(beat), bottom);
      context.stroke();
    }
  }

  function drawKeys(context) {
    context.fillStyle = "#0b0f15";
    context.fillRect(0, RULER_HEIGHT, KEY_WIDTH, ROWS * ROW_HEIGHT);
    for (let row = 0; row < ROWS; row++) {
      const pitch = state.lowPitch + (ROWS - 1 - row);
      const y = RULER_HEIGHT + row * ROW_HEIGHT;
      const black = BLACK_KEYS.has(pitchClass(pitch));
      context.fillStyle = black ? "#c7d0dd" : "#e4eaf2";
      context.fillRect(0, y + 1, KEY_WIDTH - 2, ROW_HEIGHT - 2);
      if (black) {
        context.fillStyle = "#101720";
        context.fillRect(0, y + 1, Math.round((KEY_WIDTH - 2) * 0.58), ROW_HEIGHT - 2);
      }
      if (pitchClass(pitch) === 0) {
        const label = pitchName(pitch);
        context.fillStyle = "#5c6a7e";
        context.font = "9px sans-serif";
        context.textBaseline = "middle";
        context.fillText(label, KEY_WIDTH - 6 - context.measureText(label).width, y + ROW_HEIGHT / 2);
      }
    }
  }

  function drawRuler(context, geo) {
    context.fillStyle = "#0b1017";
    context.fillRect(0, 0, geo.width, RULER_HEIGHT);
    context.fillStyle = "#161d28";
    context.fillRect(0, 0, KEY_WIDTH, RULER_HEIGHT);
    context.strokeStyle = "#232b38";
    context.beginPath();
    context.moveTo(0, RULER_HEIGHT - 0.5);
    context.lineTo(geo.width, RULER_HEIGHT - 0.5);
    context.stroke();
    context.font = "9px sans-serif";
    context.textBaseline = "middle";
    const lastBeat = Math.floor(state.startBeat + VISIBLE_BEATS);
    for (let beat = Math.ceil(state.startBeat); beat <= lastBeat; beat++) {
      const x = Math.round(KEY_WIDTH + (beat - state.startBeat) * geo.pxPerBeat) + 0.5;
      const isBar = ((beat % perBar) + perBar) % perBar === 0;
      context.fillStyle = isBar ? "#93a1b5" : "#46536a";
      context.fillRect(x, isBar ? 3 : RULER_HEIGHT - 6, 1, isBar ? RULER_HEIGHT - 4 : 4);
      if (!isBar) continue;
      const label = `bar ${Math.floor(beat / perBar) + 1}`;
      if (x + 6 + context.measureText(label).width > geo.width) continue;
      context.fillStyle = "#93a1b5";
      context.fillText(label, x + 5, RULER_HEIGHT / 2 + 1);
    }
  }

  function drawNotes(context, geo) {
    const beatFrames = framesPerBeat();
    const radius = 3;
    const marked = highlightedNotes(geo);
    for (const note of notes.map(previewNote)) {
      const beat = note.startFrame / beatFrames;
      const x = KEY_WIDTH + (beat - state.startBeat) * geo.pxPerBeat;
      const width = Math.max(4, (note.lengthFrames / beatFrames) * geo.pxPerBeat);
      if (x + width < KEY_WIDTH || x > geo.width) continue;
      const row = ROWS - 1 - (note.pitch - state.lowPitch);
      if (row < 0 || row >= ROWS) continue;
      const left = Math.max(KEY_WIDTH, x);
      const top = RULER_HEIGHT + row * ROW_HEIGHT + 2;
      const noteWidth = Math.max(2, x + width - left);
      const noteHeight = ROW_HEIGHT - 4;
      const isSelected = marked.has(note.id);
      const moving = Boolean(drag) && isSelected;
      context.fillStyle = moving ? "#8be8c5" : isSelected ? "#7aa7ff" : "#3f9a7c";
      context.beginPath();
      context.roundRect(left, top, noteWidth, noteHeight, radius);
      context.fill();
      if (isSelected) {
        context.strokeStyle = "#e6ecf5";
        context.lineWidth = 1;
        context.stroke();
      }
      if (noteWidth > 8) {
        context.fillStyle = moving ? "#c8f6e6" : isSelected ? "#b9cfff" : "#66e0b6";
        context.fillRect(left + radius, top, noteWidth - radius * 2, 1);
      }
    }
  }

  // drawMarquee paints the selection rectangle being dragged on empty grid.
  function drawMarquee(context) {
    if (!marquee?.moved) return;
    const area = marqueeBox(marquee);
    const width = area.right - area.left;
    const height = area.bottom - area.top;
    context.fillStyle = "rgba(122, 167, 255, 0.16)";
    context.strokeStyle = "#7aa7ff";
    context.lineWidth = 1;
    context.fillRect(area.left, area.top, width, height);
    context.strokeRect(area.left + 0.5, area.top + 0.5, width, height);
  }

  function paint() {
    const geo = geometry();
    const ratio = window.devicePixelRatio || 1;
    const pixelWidth = Math.round(geo.width * ratio);
    const pixelHeight = Math.round(geo.height * ratio);
    if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) {
      canvas.width = pixelWidth;
      canvas.height = pixelHeight;
    }
    canvas.style.height = `${geo.height + 2}px`;
    const context = canvas.getContext("2d");
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.clearRect(0, 0, geo.width, geo.height);
    drawRows(context, geo);
    drawGrid(context, geo);
    drawKeys(context);
    drawRuler(context, geo);
    drawNotes(context, geo);
    drawMarquee(context);
  }

  function pointerPosition(event) {
    const box = canvas.getBoundingClientRect();
    return { x: event.clientX - box.left, y: event.clientY - box.top };
  }

  canvas.addEventListener("contextmenu", (event) => event.preventDefault());
  canvas.addEventListener("pointerdown", (event) => {
    if (!canvas.clientWidth) return;
    const geo = geometry();
    const point = pointerPosition(event);
    if (point.y < RULER_HEIGHT) return;
    if (point.x < KEY_WIDTH) {
      onAudition(pitchAtY(point.y));
      return;
    }
    if (event.button === 2) {
      const note = noteHit(point.x, point.y, geo);
      if (!note) return;
      if (selected.delete(note.id)) reportSelection();
      onRemove(note.id);
      return;
    }
    if (event.button !== 0) return;
    const hit = noteHit(point.x, point.y, geo);
    if (!hit) {
      // A press on the empty grid starts a selection sweep; a plain click
      // without movement still adds a note, as before.
      marquee = {
        originX: Math.max(KEY_WIDTH, point.x),
        originY: Math.max(RULER_HEIGHT, point.y),
        x: Math.max(KEY_WIDTH, point.x),
        y: Math.max(RULER_HEIGHT, point.y),
        base: event.shiftKey ? new Set(selected) : new Set(),
        pointerId: event.pointerId,
        moved: false,
      };
      canvas.setPointerCapture(event.pointerId);
      canvas.style.cursor = "crosshair";
      return;
    }
    if (event.shiftKey) {
      if (!selected.delete(hit.id)) selected.add(hit.id);
      reportSelection();
      paint();
      return;
    }
    if (!selected.has(hit.id)) {
      selected.clear();
      selected.add(hit.id);
      reportSelection();
    }
    const group = selectedNotes();
    const resizing = group.length === 1 && onResizeEdge(point.x, hit, geo);
    drag = {
      anchor: hit,
      group,
      mode: resizing ? "resize" : "move",
      lengthFrames: hit.lengthFrames,
      pitchDelta: 0,
      frameDelta: 0,
      originX: point.x,
      originY: point.y,
      pointerId: event.pointerId,
      moved: false,
    };
    canvas.setPointerCapture(event.pointerId);
    canvas.style.cursor = resizing ? "ew-resize" : "grabbing";
    paint();
  });
  canvas.addEventListener("pointermove", (event) => {
    if (marquee && marquee.pointerId === event.pointerId) {
      const point = pointerPosition(event);
      marquee.x = Math.max(KEY_WIDTH, point.x);
      marquee.y = Math.max(RULER_HEIGHT, point.y);
      marquee.moved = marquee.moved || Math.abs(point.x - marquee.originX) > 2 || Math.abs(point.y - marquee.originY) > 2;
      paint();
      return;
    }
    if (!drag || drag.pointerId !== event.pointerId) {
      const point = pointerPosition(event);
      if (point.y < RULER_HEIGHT) {
        canvas.style.cursor = "default";
        return;
      }
      if (point.x < KEY_WIDTH) {
        canvas.style.cursor = "pointer";
        return;
      }
      const geo = geometry();
      const hover = noteHit(point.x, point.y, geo);
      canvas.style.cursor = !hover ? "crosshair" : onResizeEdge(point.x, hover, geo) ? "ew-resize" : "grab";
      return;
    }
    const geo = geometry();
    const point = pointerPosition(event);
    if (drag.mode === "resize") {
      const endFrame = Math.round(snappedBeat(point.x, geo.pxPerBeat) * framesPerBeat());
      const shortest = Math.max(1, Math.round(SNAP_BEATS * framesPerBeat()));
      drag.lengthFrames = Math.max(shortest, endFrame - drag.anchor.startFrame);
    } else {
      const beat = Math.round(beatAtX(point.x, geo.pxPerBeat) / SNAP_BEATS) * SNAP_BEATS;
      drag.frameDelta = Math.round((beat - drag.anchor.startFrame / framesPerBeat()) * framesPerBeat());
      drag.pitchDelta = pitchAtY(point.y) - drag.anchor.pitch;
      limitDrag();
    }
    drag.moved = drag.moved || Math.abs(point.x - drag.originX) > 2 || Math.abs(point.y - drag.originY) > 2;
    paint();
  });
  // finishMarquee commits the sweep: a real drag selects every note it crossed,
  // a click on the empty grid adds one note and clears the selection.
  const finishMarquee = (event, cancelled) => {
    if (!marquee || marquee.pointerId !== event.pointerId) return;
    const current = marquee;
    marquee = null;
    if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
    canvas.style.cursor = "crosshair";
    if (cancelled) return paint();
    if (!current.moved) {
      const pitch = pitchAtY(current.originY);
      const startFrame = Math.round(snappedBeat(current.originX, geometry().pxPerBeat) * framesPerBeat());
      selected.clear();
      reportSelection();
      onAdd(pitch, Math.max(0, startFrame), Math.max(1, Math.round(state.lengthBeats * framesPerBeat())));
      onAudition(pitch);
      return paint();
    }
    const geo = geometry();
    const next = current.base;
    for (const note of notes) if (inMarquee(note, geo, current)) next.add(note.id);
    selected.clear();
    for (const id of next) selected.add(id);
    reportSelection();
    paint();
  };
  const finishDrag = (event, cancelled) => {
    if (!drag || drag.pointerId !== event.pointerId) return;
    const current = drag;
    drag = null;
    canvas.style.cursor = "crosshair";
    if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
    paint();
    if (cancelled) return;
    const changed = current.mode === "resize"
      ? current.lengthFrames !== current.anchor.lengthFrames
      : current.pitchDelta !== 0 || current.frameDelta !== 0;
    if (current.moved && changed && current.mode === "resize") {
      rememberLength(current.lengthFrames / framesPerBeat());
      onResize(current.anchor.id, current.lengthFrames);
    } else if (current.moved && changed) {
      onShift(current.group.map((note) => note.id), current.pitchDelta, current.frameDelta);
    }
    else if (!current.moved) onAudition(current.anchor.pitch);
  };
  // The roll shows two octaves at a time, so the wheel walks the keyboard
  // vertically the way a piano does; Shift and a sideways gesture pan it in time
  // instead of scrolling the page.
  const scrollPitch = (delta) => {
    wheelPitch = Math.max(MIN_KEY, Math.min(MAX_KEY, wheelPitch - delta / PITCH_WHEEL));
    const lowPitch = Math.round(wheelPitch);
    if (lowPitch !== state.lowPitch) applyView({ lowPitch });
  };
  canvas.addEventListener("wheel", (event) => {
    if (drag || marquee) return;
    event.preventDefault();
    const sideways = event.shiftKey || Math.abs(event.deltaX) > Math.abs(event.deltaY);
    if (sideways) {
      const delta = event.shiftKey ? event.deltaY : event.deltaX;
      if (!delta) return;
      applyView({ startBeat: state.startBeat + delta / Math.max(6, geometry().pxPerBeat) });
      return;
    }
    if (event.deltaY) scrollPitch(event.deltaY);
  }, { passive: false });
  canvas.addEventListener("pointerup", (event) => {
    finishMarquee(event, false);
    finishDrag(event, false);
  });
  canvas.addEventListener("pointercancel", (event) => {
    finishMarquee(event, true);
    finishDrag(event, true);
  });
  window.requestAnimationFrame(paint);
  return section;
}
