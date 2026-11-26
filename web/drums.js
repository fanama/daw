// FL-style step sequencer for drum tracks: one row per kit piece and 16 steps
// per bar. Each step is an ordinary note, so playback, export and persistence
// reuse the instrument-track path.

import { DRUM_KIT } from "./drum-kit.js";

const LABEL_WIDTH = 82;
const RULER_HEIGHT = 20;
const ROW_HEIGHT = 24;
const BEATS_PER_STEP = 0.25;
const INSTRUMENT_OPTIONS = [["piano", "Piano"], ["bass", "Bass"], ["guitar", "Guitar"], ["drums", "Drums"]];

function element(tag, className = "", text = "") {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text) node.textContent = text;
  return node;
}

export function buildDrumEditor({ track, notes, sampleRate, bpm, beatsPerBar, endBeat, view, onAdd, onRemove, onAudition, onView, onInstrument }) {
  const perBar = Math.max(1, Math.round(Number(beatsPerBar) || 4));
  const stepsPerBar = Math.max(4, Math.round(perBar / BEATS_PER_STEP));
  const kit = DRUM_KIT;
  const bars = Math.max(4, Math.ceil(Math.max(0, Number(endBeat) || 0) / perBar) + 1);
  let bar = Math.max(0, Math.min(bars - 1, Math.floor((Number(view.startBeat) || 0) / perBar)));
  const framesPerBeat = () => (sampleRate * 60) / Math.max(20, Number(bpm) || 120);
  const stepFrames = () => framesPerBeat() * BEATS_PER_STEP;
  let stroke = null;

  const section = element("div", "notes drum-machine");
  const head = element("div", "notes-head");
  head.append(element("h3", "", "Step sequencer"));
  head.append(element("span", "notes-count", `${notes.length} hit${notes.length === 1 ? "" : "s"}`));

  const toolbar = element("div", "notes-toolbar");
  const instrument = document.createElement("select");
  instrument.title = "Instrument sound";
  for (const [value, label] of INSTRUMENT_OPTIONS) instrument.append(new Option(label, value));
  instrument.value = track.instrument || "drums";
  instrument.addEventListener("change", () => onInstrument(instrument.value));
  const barWrap = element("label", "notes-scroll");
  barWrap.title = "Choose the bar to edit";
  const barInput = document.createElement("input");
  barInput.type = "range";
  barInput.min = "1";
  barInput.max = String(bars);
  barInput.step = "1";
  barInput.value = String(bar + 1);
  const barValue = element("span", "notes-scroll-value", `bar ${bar + 1}`);
  barWrap.append(element("span", "unit", "bar"), barInput, barValue);
  const setBar = (next) => {
    bar = Math.max(0, Math.min(bars - 1, next));
    barInput.value = String(bar + 1);
    barValue.textContent = `bar ${bar + 1}`;
    onView({ startBeat: bar * perBar });
    paint();
  };
  barInput.addEventListener("input", () => setBar(Number(barInput.value) - 1));
  toolbar.append(element("span", "unit", "sound"), instrument, barWrap);

  const canvas = document.createElement("canvas");
  canvas.className = "drum-roll";
  canvas.setAttribute("aria-label", `Step sequencer for ${track.name}. Rows are drums, columns are sixteenth notes.`);
  canvas.title = "Click or drag to add steps · right-click to erase · click a drum name to hear it";
  section.append(
    head,
    toolbar,
    canvas,
    element("p", "notes-hint", "Click or drag steps · right-click to erase · wheel to change bar · click a name to hear it."),
  );

  function geometry() {
    const width = canvas.clientWidth || 300;
    return { width, height: RULER_HEIGHT + kit.length * ROW_HEIGHT, stepWidth: (width - LABEL_WIDTH) / stepsPerBar };
  }

  function noteAt(pitch, step) {
    const target = step * stepFrames();
    const tolerance = stepFrames() / 2;
    return notes.find((note) => note.pitch === pitch && Math.abs(note.startFrame - target) < tolerance) || null;
  }

  function stepOf(note) {
    return Math.round(note.startFrame / stepFrames());
  }

  function drawBackground(context, geo) {
    context.fillStyle = "#0f141c";
    context.fillRect(0, 0, geo.width, geo.height);
    for (let row = 0; row < kit.length; row++) {
      const y = RULER_HEIGHT + row * ROW_HEIGHT;
      context.fillStyle = row % 2 === 0 ? "#1b2431" : "#172029";
      context.fillRect(LABEL_WIDTH, y, geo.width - LABEL_WIDTH, ROW_HEIGHT - 1);
      context.fillStyle = "#0b0f15";
      context.fillRect(0, y, LABEL_WIDTH, ROW_HEIGHT - 1);
      context.fillStyle = "#c9d4e3";
      context.font = "11px sans-serif";
      context.textBaseline = "middle";
      context.fillText(kit[row].label, 8, y + ROW_HEIGHT / 2);
    }
  }

  function drawGrid(context, geo) {
    const bottom = geo.height;
    for (let step = 0; step <= stepsPerBar; step++) {
      const x = Math.round(LABEL_WIDTH + step * geo.stepWidth) + 0.5;
      const beat = (step * BEATS_PER_STEP) % 1 === 0;
      context.strokeStyle = step === 0 || step === stepsPerBar ? "#35435a" : beat ? "#283445" : "#1f2836";
      context.beginPath();
      context.moveTo(x, RULER_HEIGHT);
      context.lineTo(x, bottom);
      context.stroke();
    }
    context.strokeStyle = "#2b3648";
    context.beginPath();
    context.moveTo(LABEL_WIDTH - 0.5, RULER_HEIGHT);
    context.lineTo(LABEL_WIDTH - 0.5, bottom);
    context.stroke();
  }

  function drawRuler(context, geo) {
    context.fillStyle = "#0b1017";
    context.fillRect(0, 0, geo.width, RULER_HEIGHT);
    context.fillStyle = "#161d28";
    context.fillRect(0, 0, LABEL_WIDTH, RULER_HEIGHT);
    context.fillStyle = "#93a1b5";
    context.font = "9px sans-serif";
    context.textBaseline = "middle";
    context.fillText(`bar ${bar + 1}`, 8, RULER_HEIGHT / 2 + 1);
    for (let step = 0; step < stepsPerBar; step++) {
      const x = LABEL_WIDTH + (step + 0.5) * geo.stepWidth;
      const downbeat = step % Math.round(1 / BEATS_PER_STEP) === 0;
      context.fillStyle = downbeat ? "#93a1b5" : "#46536a";
      if (geo.stepWidth >= 12) {
        const label = String(step + 1);
        context.fillText(label, x - context.measureText(label).width / 2, RULER_HEIGHT / 2 + 1);
      } else if (downbeat) {
        context.fillRect(Math.round(x), 6, 1, RULER_HEIGHT - 8);
      }
    }
    context.strokeStyle = "#232b38";
    context.beginPath();
    context.moveTo(0, RULER_HEIGHT - 0.5);
    context.lineTo(geo.width, RULER_HEIGHT - 0.5);
    context.stroke();
  }

  function paintStep(context, geo, row, stepInBar, ghost) {
    const width = Math.max(4, geo.stepWidth - 2);
    const left = LABEL_WIDTH + stepInBar * geo.stepWidth + (geo.stepWidth - width) / 2;
    const top = RULER_HEIGHT + row * ROW_HEIGHT + 4;
    context.fillStyle = ghost ? "#5fae91" : "#3f9a7c";
    context.beginPath();
    context.roundRect(left, top, width, ROW_HEIGHT - 8, 3);
    context.fill();
    if (width > 10) {
      context.fillStyle = ghost ? "#8fd6bc" : "#66e0b6";
      context.fillRect(left + 3, top, width - 6, 1);
    }
  }

  function drawSteps(context, geo) {
    const erased = new Set();
    const ghosts = [];
    if (stroke?.mode === "erase") for (const change of stroke.changes.values()) erased.add(change.id);
    if (stroke?.mode === "draw") ghosts.push(...stroke.changes.values());
    for (const note of notes) {
      if (erased.has(note.id)) continue;
      const stepInBar = stepOf(note) - bar * stepsPerBar;
      if (stepInBar < 0 || stepInBar >= stepsPerBar) continue;
      const row = kit.findIndex((entry) => entry.pitch === note.pitch);
      if (row < 0) continue;
      paintStep(context, geo, row, stepInBar, false);
    }
    for (const change of ghosts) {
      const row = kit.findIndex((entry) => entry.pitch === change.pitch);
      const stepInBar = change.step - bar * stepsPerBar;
      if (row < 0 || stepInBar < 0 || stepInBar >= stepsPerBar) continue;
      paintStep(context, geo, row, stepInBar, true);
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
    drawBackground(context, geo);
    drawGrid(context, geo);
    drawRuler(context, geo);
    drawSteps(context, geo);
  }

  function pointerPosition(event) {
    const box = canvas.getBoundingClientRect();
    return { x: event.clientX - box.left, y: event.clientY - box.top };
  }

  function cellAt(point, geo) {
    if (point.y < RULER_HEIGHT) return null;
    const row = Math.floor((point.y - RULER_HEIGHT) / ROW_HEIGHT);
    if (row < 0 || row >= kit.length) return null;
    if (point.x < LABEL_WIDTH) return { row, pitch: kit[row].pitch, step: null };
    const stepInBar = Math.floor((point.x - LABEL_WIDTH) / geo.stepWidth);
    if (stepInBar < 0 || stepInBar >= stepsPerBar) return null;
    return { row, pitch: kit[row].pitch, step: bar * stepsPerBar + stepInBar };
  }

  function recordCell(cell) {
    if (!stroke || cell.step === null) return;
    const key = `${cell.pitch}:${cell.step}`;
    if (stroke.changes.has(key)) return;
    if (stroke.mode === "erase") {
      const note = noteAt(cell.pitch, cell.step);
      if (note) stroke.changes.set(key, { action: "erase", id: note.id });
    } else if (!noteAt(cell.pitch, cell.step)) {
      stroke.changes.set(key, { action: "draw", pitch: cell.pitch, step: cell.step });
      onAudition(cell.pitch);
    }
    paint();
  }

  canvas.addEventListener("contextmenu", (event) => event.preventDefault());
  canvas.addEventListener("pointerdown", (event) => {
    if (!canvas.clientWidth) return;
    const geo = geometry();
    const cell = cellAt(pointerPosition(event), geo);
    if (!cell) return;
    if (event.button === 2) {
      if (cell.step === null) return;
      const note = noteAt(cell.pitch, cell.step);
      if (note) onRemove(note.id);
      return;
    }
    if (event.button !== 0) return;
    if (cell.step === null) {
      onAudition(cell.pitch);
      return;
    }
    stroke = {
      mode: noteAt(cell.pitch, cell.step) ? "erase" : "draw",
      changes: new Map(),
      pointerId: event.pointerId,
    };
    canvas.setPointerCapture(event.pointerId);
    recordCell(cell);
  });
  canvas.addEventListener("pointermove", (event) => {
    const point = pointerPosition(event);
    if (!stroke) {
      canvas.style.cursor = point.y >= RULER_HEIGHT && point.x < LABEL_WIDTH ? "pointer" : "crosshair";
      return;
    }
    if (stroke.pointerId !== event.pointerId) return;
    const cell = cellAt(point, geometry());
    if (cell) recordCell(cell);
  });
  const finishStroke = (event, cancelled) => {
    if (!stroke || stroke.pointerId !== event.pointerId) return;
    const current = stroke;
    stroke = null;
    canvas.style.cursor = "crosshair";
    if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
    paint();
    if (cancelled) return;
    for (const change of current.changes.values()) {
      if (change.action === "erase") onRemove(change.id);
      else onAdd(change.pitch, Math.round(change.step * stepFrames()), Math.max(1, Math.round(stepFrames())));
    }
  };
  canvas.addEventListener("pointerup", (event) => finishStroke(event, false));
  canvas.addEventListener("pointercancel", (event) => finishStroke(event, true));
  canvas.addEventListener("wheel", (event) => {
    event.preventDefault();
    const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY;
    if (!delta) return;
    setBar(bar + (delta > 0 ? 1 : -1));
  }, { passive: false });
  window.requestAnimationFrame(paint);
  return section;
}
