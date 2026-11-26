// Session helpers shared by the audio engine, the timeline and the page.

export function takeLength(take) {
  const end = take.trim.endFrame > 0 ? take.trim.endFrame : take.lengthFrames;
  return Math.max(0, end - take.trim.startFrame);
}

export function audibleTakes(state) {
  const selection = [];
  for (const track of state.tracks) {
    if (track.muted || (state.anySolo && !track.soloed)) {
      continue;
    }
    for (const take of state.takes) {
      if (take.trackId !== track.id || take.muted) continue;
      if (take.id !== track.activeTakeId && !take.layered) continue;
      if (takeLength(take) === 0) continue;
      selection.push({ track, take });
    }
  }
  return selection;
}

// noteSpan is the pattern a track's notes occupy: its first start and last end.
export function noteSpan(notes) {
  let start = Infinity;
  let end = 0;
  for (const note of notes) {
    start = Math.min(start, note.startFrame);
    end = Math.max(end, note.startFrame + note.lengthFrames);
  }
  return end > start ? { start, length: end - start } : null;
}

// patternLength is the looped length of an instrument pattern: the notes' span,
// unless the region was widened past its last note.
export function patternLength(notes, patternFrames = 0) {
  const span = noteSpan(notes);
  if (!span) return 0;
  return Math.max(span.length, Math.max(0, Number(patternFrames) || 0));
}

// repeatedNotes returns the pattern plus its looped copies. Repetitions are
// derived, so the piano roll always edits the original pattern.
export function repeatedNotes(notes, repeats = 0, patternFrames = 0) {
  const span = noteSpan(notes);
  if (!span || repeats <= 0) return notes;
  const length = patternLength(notes, patternFrames);
  const copies = [];
  for (let copy = 1; copy <= repeats; copy++) {
    for (const note of notes) copies.push({ ...note, startFrame: note.startFrame + length * copy });
  }
  return notes.concat(copies);
}

// audibleNoteTracks lists instrument tracks that should sound, paired with
// their notes. Instrument tracks play notes instead of recorded takes.
export function audibleNoteTracks(state) {
  const selection = [];
  const notes = Array.isArray(state.notes) ? state.notes : [];
  for (const track of state.tracks) {
    if (track.source !== "instrument") continue;
    if (track.muted || (state.anySolo && !track.soloed)) continue;
    const trackNotes = notes.filter((note) => note.trackId === track.id);
    if (trackNotes.length) {
      selection.push({ track, notes: repeatedNotes(trackNotes, Number(track.repeat) || 0, track.patternFrames) });
    }
  }
  return selection;
}

export function notesEndFrame(state) {
  let end = 0;
  const notes = Array.isArray(state.notes) ? state.notes : [];
  for (const track of state.tracks) {
    const trackNotes = notes.filter((note) => note.trackId === track.id);
    const span = noteSpan(trackNotes);
    if (!span) continue;
    const repeats = Math.max(0, Number(track.repeat) || 0);
    end = Math.max(end, span.start + patternLength(trackNotes, track.patternFrames) * (1 + repeats));
  }
  return end;
}

export function contentEndFrame(state, extraFrame = 0) {
  let end = extraFrame;
  for (const take of state.takes) {
    end = Math.max(end, take.startFrame + takeLength(take));
  }
  return Math.max(end, notesEndFrame(state));
}

export function formatDuration(seconds) {
  const total = Math.max(0, Math.round((Number(seconds) || 0) * 1000));
  const minutes = Math.floor(total / 60000);
  const rest = total % 60000;
  const wholeSeconds = Math.floor(rest / 1000);
  const milliseconds = rest % 1000;
  return `${String(minutes).padStart(2, "0")}:${String(wholeSeconds).padStart(2, "0")}.${String(milliseconds).padStart(3, "0")}`;
}
