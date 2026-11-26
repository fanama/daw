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

// audibleNoteTracks lists instrument tracks that should sound, paired with
// their notes. Instrument tracks play notes instead of recorded takes.
export function audibleNoteTracks(state) {
  const selection = [];
  const notes = Array.isArray(state.notes) ? state.notes : [];
  for (const track of state.tracks) {
    if (track.source !== "instrument") continue;
    if (track.muted || (state.anySolo && !track.soloed)) continue;
    const trackNotes = notes.filter((note) => note.trackId === track.id);
    if (trackNotes.length) selection.push({ track, notes: trackNotes });
  }
  return selection;
}

export function notesEndFrame(state) {
  let end = 0;
  for (const note of Array.isArray(state.notes) ? state.notes : []) {
    end = Math.max(end, note.startFrame + note.lengthFrames);
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
