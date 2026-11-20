package jsonstore_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorecord/domain/mix"
	"gorecord/domain/session"
	"gorecord/infrastructure/jsonstore"
)

func TestSaveLoadPreservesTheAggregate(t *testing.T) {
	root := t.TempDir()
	sessionRoot := filepath.Join(root, "Riff idea")
	store := jsonstore.NewStore(root)

	original, err := session.New("Riff idea", 48000, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	guitar, err := original.AddTrack("Guitar", 0)
	if err != nil {
		t.Fatalf("add track: %v", err)
	}
	effects := mix.DefaultEffects()
	effects.Amp.Drive = 0.8
	effects.Reverb.Enabled = true
	effects.Reverb.Mix = 0.35
	effects.EQ.Bands[1].GainDB = -4
	if err := original.SetEffects(guitar.ID(), effects); err != nil {
		t.Fatalf("set effects: %v", err)
	}
	if err := original.SetArm(guitar.ID(), true); err != nil {
		t.Fatalf("arm: %v", err)
	}
	take, err := original.AddTake(guitar.ID(), "takes/take-001.wav", 0, 1, 48000, false, false, time.Unix(1_700_000_100, 0))
	if err != nil {
		t.Fatalf("add take: %v", err)
	}
	if err := original.CompleteTake(take.ID(), 96000); err != nil {
		t.Fatalf("complete take: %v", err)
	}
	original.SetClick(mix.Click{Enabled: true, BPM: 96, BeatsPerBar: 4})
	if err := store.Save(sessionRoot, original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load(sessionRoot)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Name() != "Riff idea" || loaded.SampleRate() != 48000 {
		t.Fatalf("header mismatch: %s %d", loaded.Name(), loaded.SampleRate())
	}
	loadedTracks := loaded.Tracks()
	if len(loadedTracks) != 1 || !loadedTracks[0].IsArmed() {
		t.Fatalf("track state lost: %+v", loadedTracks)
	}
	if loadedTracks[0].Effects().Amp.Drive != 0.8 || loadedTracks[0].Effects().Reverb.Mix != 0.35 {
		t.Fatalf("effects lost: %+v", loadedTracks[0].Effects())
	}
	loadedTakes := loaded.Takes()
	if len(loadedTakes) != 1 || loadedTakes[0].LengthFrames() != 96000 {
		t.Fatalf("take lost: %+v", loadedTakes)
	}
	if loadedTakes[0].File() != "takes/take-001.wav" {
		t.Fatalf("relative path not preserved: %q", loadedTakes[0].File())
	}
	if loaded.Click().BPM != 96 || !loaded.Click().Enabled {
		t.Fatalf("click lost: %+v", loaded.Click())
	}
}

func TestSaveKeepsOneBackup(t *testing.T) {
	root := t.TempDir()
	store := jsonstore.NewStore(root)
	created, err := session.New("Idea", 48000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := store.Save(root, created); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.Save(root, created); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "project.json.bak")); err != nil {
		t.Fatalf("expected a backup file: %v", err)
	}
}

func TestLoadRefusesANewerSchema(t *testing.T) {
	root := t.TempDir()
	raw, err := json.Marshal(map[string]any{"schemaVersion": 99})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "project.json"), raw, 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}
	store := jsonstore.NewStore(root)
	if _, loadErr := store.Load(root); loadErr == nil {
		t.Fatal("expected an unsupported schema error")
	}
}

func TestListReturnsOnlySessionFolders(t *testing.T) {
	root := t.TempDir()
	store := jsonstore.NewStore(root)
	created, err := session.New("Idea one", 48000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := store.Save(filepath.Join(root, "Idea one"), created); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "not-a-session"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	summaries, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "Idea one" {
		t.Fatalf("unexpected summaries: %+v", summaries)
	}
}
