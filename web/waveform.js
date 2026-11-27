// Timeline rendering: peak envelopes per take, lane layout, ruler and playhead.

import { contentEndFrame, noteSpan, patternBeatFrames, patternLength, takeLength } from "./model.js";

const RULER_HEIGHT = 22;
const BUCKET_FRAMES = 256;

export function computePeaks(samples, channels) {
  const frames = Math.floor(samples.length / channels);
  const count = Math.max(1, Math.ceil(frames / BUCKET_FRAMES));
  const min = new Float32Array(count);
  const max = new Float32Array(count);
  for (let index = 0; index < count; index++) {
    const start = index * BUCKET_FRAMES;
    const end = Math.min(frames, start + BUCKET_FRAMES);
    let peak = 0;
    for (let frame = start; frame < end; frame++) {
      const base = frame * channels;
      for (let channel = 0; channel < channels; channel++) {
        const value = Math.abs(samples[base + channel]);
        if (value > peak) peak = value;
      }
    }
    max[index] = peak;
    min[index] = -peak;
  }
  return { bucket: BUCKET_FRAMES, min, max };
}

function chooseStep(seconds, width, sampleRate) {
  const steps = [0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 15, 30, 60, 120, 300, 600];
  for (const step of steps) {
    if ((step * width) / seconds >= 80) return step;
  }
  return steps[steps.length - 1];
}

function rulerLabel(second, step) {
  const minutes = Math.floor(second / 60);
  const rest = second - minutes * 60;
  if (step < 1) return `${minutes}:${rest.toFixed(2).padStart(5, "0")}`;
  return `${minutes}:${String(Math.round(rest)).padStart(2, "0")}`;
}

function drawRuler(context, width, seconds, sampleRate, height = RULER_HEIGHT, startSecond = 0) {
  if (height <= 0) return;
  const step = chooseStep(seconds, width, sampleRate);
  context.fillStyle = "#0d1118";
  context.fillRect(0, 0, width, height);
  context.strokeStyle = "#1b2230";
  context.fillStyle = "#5c6a7e";
  context.font = "10px sans-serif";
  context.textBaseline = "middle";
  const labelY = Math.round(height / 2);
  const first = Math.floor(startSecond / step) * step;
  for (let second = first; second <= startSecond + seconds; second += step) {
    const x = Math.round(((second - startSecond) / seconds) * width) + 0.5;
    context.beginPath();
    context.moveTo(x, 0);
    context.lineTo(x, height);
    context.stroke();
    context.fillText(rulerLabel(second, step), x + 4, labelY);
  }
  context.beginPath();
  context.moveTo(0, height - 0.5);
  context.lineTo(width, height - 0.5);
  context.stroke();
}

function waveFor(context, data, x, startPixel, endPixel, pxPerFrame, trimStart, middle, amplitude) {
  if (!data || endPixel <= startPixel) return;
  context.beginPath();
  for (let pixel = startPixel; pixel < endPixel; pixel++) {
    const firstFrame = trimStart + (pixel / pxPerFrame);
    const lastFrame = trimStart + ((pixel + 1) / pxPerFrame);
    const firstBucket = Math.floor(firstFrame / data.bucket);
    const lastBucket = Math.max(firstBucket, Math.floor(lastFrame / data.bucket));
    let low = 0;
    let high = 0;
    for (let bucket = firstBucket; bucket <= lastBucket && bucket < data.max.length; bucket++) {
      if (data.max[bucket] > high) high = data.max[bucket];
      if (data.min[bucket] < low) low = data.min[bucket];
    }
    context.moveTo(x + pixel + 0.5, middle - high * amplitude);
    context.lineTo(x + pixel + 0.5, middle - low * amplitude);
  }
  context.stroke();
}

function drawTake(context, take, track, y, laneHeight, layout, view) {
  const trimmedLength = takeLength(take);
  if (trimmedLength <= 0) return;
  const x = (take.startFrame - layout.startFrame) * layout.pxPerFrame;
  const width = Math.max(2, trimmedLength * layout.pxPerFrame);
  const left = Math.max(0, x);
  const right = Math.min(layout.width, x + width);
  if (right <= left) return;
  const startPixel = Math.max(0, Math.floor(left - x));
  const endPixel = Math.min(width, Math.ceil(right - x));
  const selected = take.id === view.selectedTakeId;
  const playing = take.id === track.activeTakeId || take.layered;
  context.fillStyle = selected ? "#245445" : playing ? "#1f4a3f" : "#202836";
  context.fillRect(left, y + 14, right - left, laneHeight - 20);
  context.strokeStyle = take.incomplete ? "#f2555a" : playing ? "#3c8f74" : "#2b3444";
  context.lineWidth = 1;
  context.strokeRect(x + 0.5, y + 14.5, Math.max(1, width - 1), laneHeight - 21);
  if (selected) {
    // A bright top edge and a thick outline make the selected clip obvious
    // even when it is only partly visible at the current zoom.
    context.fillStyle = "#8be8c5";
    context.fillRect(left, y + 14, right - left, 2);
    context.strokeStyle = "#8be8c5";
    context.lineWidth = 2;
    context.strokeRect(left + 1, y + 15, Math.max(1, right - left - 2), laneHeight - 22);
  }
  const data = view.peaks.get(take.file);
  if (!data) return;
  context.save();
  context.beginPath();
  context.rect(left, y + 14, right - left, laneHeight - 20);
  context.clip();
  context.strokeStyle = selected ? "#d9fff2" : playing ? "#8be8c5" : "#7c8ba0";
  waveFor(context, data, x, startPixel, endPixel, layout.pxPerFrame, take.trim.startFrame, y + 14 + (laneHeight - 20) / 2, (laneHeight - 26) / 2);
  context.restore();
}

const NOTE_LOW = 36;
const NOTE_HIGH = 84;

// Instrument lanes show their notes as a compact pitch map, so the timeline
// previews the part without opening the piano roll.
function drawNotes(context, notes, y, laneHeight, layout, offsetFrames = 0) {
  const top = y + 16;
  const area = Math.max(6, laneHeight - 22);
  context.fillStyle = "#4fbf98";
  for (const note of notes) {
    const x = (note.startFrame + offsetFrames - layout.startFrame) * layout.pxPerFrame;
    const width = Math.max(2, note.lengthFrames * layout.pxPerFrame);
    if (x + width < 0 || x > layout.width) continue;
    const pitch = Math.max(NOTE_LOW, Math.min(NOTE_HIGH, note.pitch));
    const row = (pitch - NOTE_LOW) / (NOTE_HIGH - NOTE_LOW);
    const noteY = top + (1 - row) * (area - 4);
    context.fillRect(Math.max(0, x), noteY, width, 3);
  }
}

const LOOP_HANDLE = 10;

// An instrument lane reads as one MIDI region: a tinted block from the first
// note to the last, one dashed copy per loop, and a square loop handle at the
// end that is dragged to add or remove repetitions.
function drawMidiRegion(context, track, notes, y, laneHeight, layout, view, beatFrames = 0) {
  const span = noteSpan(notes);
  if (!span) return null;
  const top = y + 14;
  const height = Math.max(8, laneHeight - 20);
  const x = (span.start - layout.startFrame) * layout.pxPerFrame;
  const previewFrames = view.patternPreview && view.patternPreview.trackId === track.id ? view.patternPreview.frames : null;
  // Instrument regions end on the beat, so the drawn block matches what loops.
  const pattern = Math.max(span.length, patternLength(notes, previewFrames === null ? track.patternFrames : previewFrames, beatFrames));
  const width = Math.max(3, pattern * layout.pxPerFrame);
  const preview = view.loopPreview && view.loopPreview.trackId === track.id ? view.loopPreview.repeats : null;
  const repeats = Math.max(0, Math.min(32, preview === null ? Number(track.repeat) || 0 : preview));
  const selected = track.id === view.selectedTrackId;
  const motifSelected = track.id === view.selectedMotifTrackId;
  const total = width * (1 + repeats);
  context.fillStyle = motifSelected ? "#256b53" : selected ? "#1c4b3e" : "#173c32";
  context.fillRect(x, top, total, height);
  context.strokeStyle = motifSelected ? "#d9fff2" : selected ? "#8be8c5" : "#3c8f74";
  context.lineWidth = motifSelected ? 2 : selected ? 1.5 : 1;
  context.strokeRect(x + 0.5, top + 0.5, Math.max(1, total - 1), Math.max(1, height - 1));
  context.strokeStyle = "#2f6f5b";
  context.setLineDash([4, 3]);
  for (let copy = 1; copy <= repeats; copy++) {
    context.beginPath();
    context.moveTo(x + copy * width + 0.5, top);
    context.lineTo(x + copy * width + 0.5, top + height);
    context.stroke();
  }
  context.setLineDash([]);
  drawNotes(context, notes, y, laneHeight, layout);
  for (let copy = 1; copy <= repeats; copy++) drawNotes(context, notes, y, laneHeight, layout, pattern * copy);
  if (repeats) {
    context.fillStyle = "#8be8c5";
    context.font = "9px sans-serif";
    context.textBaseline = "alphabetic";
    context.fillText(`×${1 + repeats}`, x + 5, top + 10);
  }
  const handle = {
    trackId: track.id,
    x: x + total - LOOP_HANDLE,
    y: top,
    width: LOOP_HANDLE,
    height: Math.min(12, height),
    patternFrames: pattern,
    repeats,
  };
  context.fillStyle = selected || motifSelected ? "#8be8c5" : "#5fae91";
  context.fillRect(handle.x, handle.y, handle.width, handle.height);
  const resize = {
    trackId: track.id,
    x: x + width - 10,
    y: top + height - 10,
    width: 10,
    height: 10,
    patternFrames: pattern,
    minFrames: span.length,
  };
  context.beginPath();
  context.moveTo(resize.x, resize.y + resize.height);
  context.lineTo(resize.x + resize.width, resize.y + resize.height);
  context.lineTo(resize.x + resize.width, resize.y);
  context.closePath();
  context.fill();
  const region = { trackId: track.id, x, y: top, width: total, height };
  return { ...handle, resize, region };
}

function drawLive(context, live, y, laneHeight, layout) {
  if (!live || !live.peaks || live.peaks.length === 0) return;
  const x = (live.startFrame - layout.startFrame) * layout.pxPerFrame;
  const blockWidth = Math.max(1, live.blockFrames * layout.pxPerFrame);
  const middle = y + 14 + (laneHeight - 20) / 2;
  const amplitude = (laneHeight - 26) / 2;
  context.fillStyle = "#4a1d24";
  context.fillRect(x, y + 14, blockWidth * (live.peaks.length / 2), laneHeight - 20);
  context.strokeStyle = "#ff8a90";
  context.beginPath();
  for (let index = 0; index < live.peaks.length; index += 2) {
    const blockX = x + (index / 2) * blockWidth;
    context.moveTo(blockX + 0.5, middle - live.peaks[index + 1] * amplitude);
    context.lineTo(blockX + 0.5, middle - live.peaks[index] * amplitude);
  }
  context.stroke();
}

function livesOf(live) {
  if (!live) return [];
  return Array.isArray(live) ? live : [live];
}

function drawMarkers(context, markers, width, height) {
  for (const marker of markers) {
    if (marker.x < 0 || marker.x > width) continue;
    context.strokeStyle = "rgba(216, 179, 106, 0.5)";
    context.setLineDash([3, 5]);
    context.beginPath();
    context.moveTo(marker.x, 6);
    context.lineTo(marker.x, height);
    context.stroke();
    context.setLineDash([]);
    context.fillStyle = "#d8b36a";
    context.beginPath();
    context.moveTo(marker.x - 4, 0);
    context.lineTo(marker.x + 4, 0);
    context.lineTo(marker.x, 6);
    context.closePath();
    context.fill();
    context.font = "10px sans-serif";
    context.textBaseline = "middle";
    const labelWidth = context.measureText(marker.name).width;
    context.fillStyle = "#2a2417";
    context.fillRect(marker.x, 8, labelWidth + 8, 12);
    context.fillStyle = "#d8b36a";
    context.fillText(marker.name, marker.x + 4, 14);
  }
}

export function drawTimeline(canvas, view) {
  const width = canvas.clientWidth || 900;
  const height = canvas.clientHeight || 360;
  const ratio = window.devicePixelRatio || 1;
  if (canvas.width !== Math.round(width * ratio) || canvas.height !== Math.round(height * ratio)) {
    canvas.width = Math.round(width * ratio);
    canvas.height = Math.round(height * ratio);
  }
  const context = canvas.getContext("2d");
  context.setTransform(ratio, 0, 0, ratio, 0, 0);
  context.clearRect(0, 0, width, height);
  const state = view.state;
  const lives = livesOf(view.live);
  const liveEnd = lives.reduce((end, live) => Math.max(end, live.startFrame + live.blockFrames * (live.peaks.length / 2)), 0);
  const markers = Array.isArray(state.markers) ? state.markers : [];
  const markerEnd = markers.reduce((end, marker) => Math.max(end, marker.frame), 0);
  const baseSeconds = Math.max(
    15,
    Math.max(contentEndFrame(state, liveEnd), markerEnd) / state.sampleRate + 2,
    (Number(view.frame) || 0) / state.sampleRate + 1,
  );
  const zoom = Math.max(1, Math.min(128, Number(view.zoom) || 1));
  const seconds = baseSeconds / zoom;
  const visibleFrames = seconds * state.sampleRate;
  const baseFrames = baseSeconds * state.sampleRate;
  const maxStart = Math.max(0, baseFrames - visibleFrames);
  let startFrame = Math.max(0, Math.min(Number(view.startFrame) || 0, maxStart));
  if (view.follow) {
    const frame = Number(view.frame) || 0;
    if (frame < startFrame) startFrame = Math.max(0, frame - visibleFrames * 0.1);
    else if (frame > startFrame + visibleFrames * 0.85) startFrame = Math.max(0, frame - visibleFrames * 0.85);
    startFrame = Math.min(startFrame, maxStart);
  }
  const laneRows = Array.isArray(view.lanes) && view.lanes.length ? view.lanes : null;
  const rulerHeight = laneRows
    ? Math.max(RULER_HEIGHT, Math.round(laneRows[0].top))
    : RULER_HEIGHT;
  const layout = {
    pxPerFrame: width / visibleFrames,
    seconds,
    startFrame,
    baseFrames,
    width,
    laneHeight: 0,
  };
  const hits = [];
  const midiHandles = [];
  canvas.layout = layout;
  canvas.clipHits = hits;
  canvas.midiHandles = midiHandles;
  canvas.markerHits = markers.map((marker) => ({
    ...marker,
    x: (marker.frame - startFrame) * layout.pxPerFrame,
  }));
  const lanes = state.tracks.length;
  if (lanes === 0) {
    drawRuler(context, width, seconds, state.sampleRate, rulerHeight, startFrame / state.sampleRate);
    context.fillStyle = "#5c6a7e";
    context.font = "13px sans-serif";
    context.fillText("Add a track, arm it, then press record.", 18, rulerHeight + 34);
    return;
  }
  layout.laneHeight = Math.max(44, (height - rulerHeight) / lanes);
  state.tracks.forEach((track, index) => {
    const row = laneRows && laneRows[index];
    const y = row ? Math.round(row.top) : rulerHeight + index * layout.laneHeight;
    const laneHeight = row ? Math.max(1, Math.round(row.height)) : layout.laneHeight;
    const midiLane = track.source === "instrument";
    context.fillStyle = midiLane
      ? (track.id === view.selectedTrackId ? "#16241f" : "#121b19")
      : (track.id === view.selectedTrackId ? "#151c27" : "#0f141c");
    context.fillRect(0, y, width, Math.max(1, laneHeight - 1));
    context.fillStyle = "#66758b";
    context.font = "11px sans-serif";
    context.textBaseline = "alphabetic";
    context.fillText(track.name, 10, y + 13);
    if (midiLane) {
      const notes = Array.isArray(state.notes) ? state.notes : [];
      const trackNotes = notes.filter((note) => note.trackId === track.id);
      const handle = drawMidiRegion(context, track, trackNotes, y, laneHeight, layout, view, patternBeatFrames(state));
      if (handle) midiHandles.push(handle);
    }
    for (const take of state.takes) {
      if (take.trackId !== track.id) continue;
      const length = takeLength(take);
      if (length > 0) {
        hits.push({
          takeId: take.id,
          trackId: track.id,
          startFrame: take.startFrame,
          x: (take.startFrame - startFrame) * layout.pxPerFrame,
          y: y + 14,
          width: Math.max(2, length * layout.pxPerFrame),
          height: Math.max(1, laneHeight - 20),
        });
      }
      drawTake(context, take, track, y, laneHeight, layout, view);
    }
    for (const live of lives) {
      if (live.trackId === track.id) drawLive(context, live, y, laneHeight, layout);
    }
  });
  drawRuler(context, width, seconds, state.sampleRate, rulerHeight, startFrame / state.sampleRate);
  drawMarkers(context, canvas.markerHits, width, height);
  const playheadX = (view.frame - startFrame) * layout.pxPerFrame;
  context.strokeStyle = "#f2555a";
  context.lineWidth = 1.5;
  context.beginPath();
  context.moveTo(playheadX, 0);
  context.lineTo(playheadX, height);
  context.stroke();
}

export function frameAtX(canvas, x) {
  if (!canvas.layout) return 0;
  return Math.max(0, Math.round(canvas.layout.startFrame + x / canvas.layout.pxPerFrame));
}

// clipAt returns the top-most take under a canvas point, if any.
export function clipAt(canvas, x, y) {
  const hits = canvas.clipHits || [];
  for (let index = hits.length - 1; index >= 0; index--) {
    const clip = hits[index];
    if (x < clip.x || x > clip.x + clip.width || y < clip.y || y > clip.y + clip.height) continue;
    return clip;
  }
  return null;
}

// midiLoopHandleAt returns the loop handle of a MIDI region under the pointer.
export function midiLoopHandleAt(canvas, x, y) {
  for (const handle of canvas.midiHandles || []) {
    if (x < handle.x || x > handle.x + handle.width || y < handle.y || y > handle.y + handle.height) continue;
    return handle;
  }
  return null;
}

// midiRegionAt returns the MIDI motif under a canvas point, if any.
export function midiRegionAt(canvas, x, y) {
  for (const handle of canvas.midiHandles || []) {
    const region = handle.region;
    if (!region) continue;
    if (x < region.x || x > region.x + region.width || y < region.y || y > region.y + region.height) continue;
    return region;
  }
  return null;
}

// midiResizeHandleAt returns the pattern-resize handle under the pointer.
export function midiResizeHandleAt(canvas, x, y) {
  for (const handle of canvas.midiHandles || []) {
    const box = handle.resize;
    if (!box) continue;
    if (x < box.x || x > box.x + box.width || y < box.y || y > box.y + box.height) continue;
    return box;
  }
  return null;
}

// markerAtX returns the marker whose flag is close enough to click, if any.
export function markerAtX(canvas, x) {
  let nearest = null;
  for (const marker of canvas.markerHits || []) {
    const distance = Math.abs(marker.x - x);
    if (distance > 8) continue;
    if (!nearest || distance < nearest.distance) nearest = { ...marker, distance };
  }
  return nearest;
}

export { RULER_HEIGHT };
