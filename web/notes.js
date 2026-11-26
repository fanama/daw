// Inline piano-roll editor for instrument tracks. Notes are stored in session
// frames; the grid follows the click tempo so a bar lines up with the transport.

const KEY_WIDTH = 36;
const ROW_HEIGHT = 13;
const ROWS = 24;
const RULER_HEIGHT = 18;
const VISIBLE_BEATS = 8;
const SNAP_BEATS = 0.25;
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
  return Math.max(0, Math.min(127, Math.round(pitch)));
}

export function buildNoteEditor({ track, notes, sampleRate, bpm, beatsPerBar, endBeat, view, onAdd, onRemove, onMove, onResize, onAudition, onView, onInstrument }) {
  const perBar = Math.max(1, Math.round(Number(beatsPerBar) || 4));
  const contentEnd = Math.max(0, Number(endBeat) || 0);
  const maxStart = Math.max(VISIBLE_BEATS, Math.ceil(contentEnd + perBar / 2));
  const section = element("div", "notes");
  const head = element("div", "notes-head");
  head.append(element("h3", "", "Piano roll"));
  head.append(element("span", "notes-count", `${notes.length} note${notes.length === 1 ? "" : "s"}`));
  const toolbar = element("div", "notes-toolbar");
  const instrument = document.createElement("select");
  instrument.title = "Instrument sound";
  for (const [value, label] of INSTRUMENT_OPTIONS) instrument.append(new Option(label, value));
  instrument.value = track.instrument || "piano";
  instrument.addEventListener("change", () => onInstrument(instrument.value));
  const length = document.createElement("select");
  length.title = "Length of new notes";
  for (const [value, label] of LENGTH_OPTIONS) length.append(new Option(label, value));
  const state = {
    startBeat: Math.max(0, Math.min(maxStart, Number(view.startBeat) || 0)),
    lowPitch: Math.max(12, Math.min(96, Math.round(Number(view.lowPitch) || 48))),
    lengthBeats: Math.max(0.25, Math.min(8, Number(view.lengthBeats) || 1)),
  };
  length.value = String(state.lengthBeats);
  length.addEventListener("change", () => {
    state.lengthBeats = Number(length.value);
    onView({ lengthBeats: state.lengthBeats });
  });
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
    state.lowPitch = Math.max(12, Math.min(96, state.lowPitch));
    state.startBeat = Math.max(0, Math.min(maxStart, state.startBeat));
    scroll.value = String(state.startBeat);
    scrollValue.textContent = positionLabel(state.startBeat);
    onView({ ...state });
    paint();
  };
  octaveDown.addEventListener("click", () => applyView({ lowPitch: state.lowPitch - 12 }));
  octaveUp.addEventListener("click", () => applyView({ lowPitch: state.lowPitch + 12 }));
  scroll.addEventListener("input", () => applyView({ startBeat: Number(scroll.value) }));
  scrollValue.textContent = positionLabel(state.startBeat);
  toolbar.append(element("span", "unit", "sound"), instrument, length, octaveDown, octaveUp, scrollWrap);

  const canvas = document.createElement("canvas");
  canvas.className = "piano-roll";
  canvas.setAttribute("aria-label", `Piano roll for ${track.name}. Click to add a note, drag to move it, drag its right edge to resize it, right-click to delete it.`);
  canvas.title = "Wheel scrolls the timeline · Shift+wheel changes octave";
  section.append(head, toolbar, canvas, element("p", "notes-hint", "Click to add · drag to move · drag the right edge to resize · wheel to scroll · right-click to delete · click a key to hear it."));

  const framesPerBeat = () => (sampleRate * 60) / Math.max(20, Number(bpm) || 120);
  let drag = null;

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

  function visibleNotes() {
    if (!drag) return notes;
    const list = notes.filter((note) => note.id !== drag.note.id);
    list.push({
      ...drag.note,
      pitch: drag.pitch,
      startFrame: drag.startFrame,
      lengthFrames: drag.lengthFrames,
    });
    return list;
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
    for (const note of visibleNotes()) {
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
      context.fillStyle = drag ? "#8be8c5" : "#3f9a7c";
      context.beginPath();
      context.roundRect(left, top, noteWidth, noteHeight, radius);
      context.fill();
      if (noteWidth > 8) {
        context.fillStyle = drag ? "#c8f6e6" : "#66e0b6";
        context.fillRect(left + radius, top, noteWidth - radius * 2, 1);
      }
    }
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
      if (note) onRemove(note.id);
      return;
    }
    if (event.button !== 0) return;
    const hit = noteHit(point.x, point.y, geo);
    if (!hit) {
      const pitch = pitchAtY(point.y);
      const startFrame = Math.round(snappedBeat(point.x, geo.pxPerBeat) * framesPerBeat());
      onAdd(pitch, Math.max(0, startFrame), Math.max(1, Math.round(state.lengthBeats * framesPerBeat())));
      onAudition(pitch);
      return;
    }
    const resizing = onResizeEdge(point.x, hit, geo);
    drag = {
      note: hit,
      mode: resizing ? "resize" : "move",
      pitch: hit.pitch,
      startFrame: hit.startFrame,
      lengthFrames: hit.lengthFrames,
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
      drag.lengthFrames = Math.max(shortest, endFrame - drag.note.startFrame);
    } else {
      drag.startFrame = Math.max(0, Math.round(snappedBeat(point.x, geo.pxPerBeat) * framesPerBeat()));
      drag.pitch = pitchAtY(point.y);
    }
    drag.moved = drag.moved || Math.abs(point.x - drag.originX) > 2 || Math.abs(point.y - drag.originY) > 2;
    paint();
  });
  const finishDrag = (event, cancelled) => {
    if (!drag || drag.pointerId !== event.pointerId) return;
    const current = drag;
    drag = null;
    canvas.style.cursor = "crosshair";
    if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
    paint();
    if (cancelled) return;
    const changed = current.mode === "resize"
      ? current.lengthFrames !== current.note.lengthFrames
      : current.pitch !== current.note.pitch || current.startFrame !== current.note.startFrame;
    if (current.moved && changed && current.mode === "resize") onResize(current.note.id, current.lengthFrames);
    else if (current.moved && changed) onMove(current.note.id, current.pitch, current.startFrame);
    else if (!current.moved) onAudition(current.note.pitch);
  };
  // The roll is a window on the session, so the wheel pans it in time instead
  // of scrolling the page; Shift repeats the 8ve buttons for the pitch range.
  canvas.addEventListener("wheel", (event) => {
    if (drag) return;
    event.preventDefault();
    if (event.shiftKey) {
      const step = Math.sign(event.deltaY) || 1;
      applyView({ lowPitch: state.lowPitch - step });
      return;
    }
    const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY;
    if (!delta) return;
    applyView({ startBeat: state.startBeat + delta / Math.max(6, geometry().pxPerBeat) });
  }, { passive: false });
  canvas.addEventListener("pointerup", (event) => finishDrag(event, false));
  canvas.addEventListener("pointercancel", (event) => finishDrag(event, true));
  window.requestAnimationFrame(paint);
  return section;
}
