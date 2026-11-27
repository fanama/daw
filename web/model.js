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
// unless the region was widened past its last note. Instrument patterns pass
// the length of one beat so the loop closes on the beat after the last note
// instead of restarting between two beats.
export function patternLength(notes, patternFrames = 0, beatFrames = 0) {
  const span = noteSpan(notes);
  if (!span) return 0;
  const length = Math.max(span.length, Math.max(0, Number(patternFrames) || 0));
  const beat = Number(beatFrames) || 0;
  if (beat <= 0) return length;
  return Math.ceil((span.start + length) / beat) * beat - span.start;
}

// patternBeatFrames is the grid every instrument pattern closes on: one beat, so
// a loop never restarts between two beats. It is 0 without a sample rate, which
// leaves the loop on the exact span of the notes.
export function patternBeatFrames(state) {
  const sampleRate = Number(state?.sampleRate) || 0;
  if (!sampleRate) return 0;
  const bpm = Math.max(20, Number(state?.click?.BPM) || 120);
  return (sampleRate * 60) / bpm;
}

// repeatedNotes returns the pattern plus its looped copies. Repetitions are
// derived, so the piano roll always edits the original pattern.
export function repeatedNotes(notes, repeats = 0, patternFrames = 0, beatFrames = 0) {
  const span = noteSpan(notes);
  if (!span || repeats <= 0) return notes;
  const length = patternLength(notes, patternFrames, beatFrames);
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
      selection.push({ track, notes: repeatedNotes(trackNotes, Number(track.repeat) || 0, track.patternFrames, patternBeatFrames(state)) });
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
    end = Math.max(end, span.start + patternLength(trackNotes, track.patternFrames, patternBeatFrames(state)) * (1 + repeats));
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
