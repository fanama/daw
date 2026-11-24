package sessionapp_test

import (
	"path/filepath"
	"testing"
	"time"

	sessionapp "fanamix/application/session"
	"fanamix/infrastructure/jsonstore"
	"fanamix/infrastructure/wavio"
)

func newService(t *testing.T) *sessionapp.Service {
	t.Helper()
	root := t.TempDir()
	store := jsonstore.NewStore(root)
	return sessionapp.NewService(store, wavio.NewTakeFiles(), func() time.Time { return time.Unix(1_700_000_000, 0) })
}

func TestCreateSessionStartsWithAGuitarTrack(t *testing.T) {
	service := newService(t)
	if err := service.Create("Riff idea", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	current := service.Current()
	if current == nil {
		t.Fatal("no current session")
	}
	if current.Name() != "Riff idea" || current.SampleRate() != 48000 {
		t.Fatalf("unexpected session: %s %d", current.Name(), current.SampleRate())
	}
	tracks := current.Tracks()
	if len(tracks) != 1 || tracks[0].Name() != "Guitar" {
		t.Fatalf("expected one guitar track, got %+v", tracks)
	}
	if _, _, err := service.BeginTake(0); err == nil {
		t.Fatal("recording must be refused before a track is armed")
	}
	if err := service.SetArm(tracks[0].ID(), true); err != nil {
		t.Fatalf("arm: %v", err)
	}
	take, plan, err := service.BeginTake(0)
	if err != nil {
		t.Fatalf("begin take: %v", err)
	}
	if len(plan) != 1 || take.Channels() != 1 {
		t.Fatalf("unexpected arm plan: %+v / take %+v", plan, take)
	}
	if take.File() != "takes/take-001.wav" {
		t.Fatalf("unexpected take path: %s", take.File())
	}
	if err := service.FinishTake(take.ID(), 4800, false); err != nil {
		t.Fatalf("finish take: %v", err)
	}
}

func TestCreateRefusesADuplicateSessionName(t *testing.T) {
	service := newService(t)
	if err := service.Create("Twice", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := service.Create("Twice", 48000); err == nil {
		t.Fatal("creating the same session name twice must be refused")
	}
}

func TestSessionReopensFromDisk(t *testing.T) {
	service := newService(t)
	if err := service.Create("Layer idea", 44100); err != nil {
		t.Fatalf("create: %v", err)
	}
	tracks := service.Current().Tracks()
	if err := service.SetArm(tracks[0].ID(), true); err != nil {
		t.Fatalf("arm: %v", err)
	}
	root := service.Root()
	service.Close()

	reopened := newService(t)
	if err := reopened.Open(root); err != nil {
		t.Fatalf("open: %v", err)
	}
	loaded := reopened.Current()
	if loaded == nil || loaded.SampleRate() != 44100 {
		t.Fatalf("unexpected reopened session: %+v", loaded)
	}
	reloadedTracks := loaded.Tracks()
	if len(reloadedTracks) != 1 || !reloadedTracks[0].IsArmed() {
		t.Fatalf("arming was not persisted: %+v", reloadedTracks)
	}
	if !loaded.Tracks()[0].Effects().Amp.Enabled {
		t.Fatal("default effects were not restored")
	}
}

func TestChainsMapEveryTrackToItsChannel(t *testing.T) {
	service := newService(t)
	if err := service.Create("Chain idea", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := service.AddTrack("Guitar double", 2)
	if err != nil {
		t.Fatalf("add track: %v", err)
	}
	if err := service.SetPrintEffects(second.ID(), true); err != nil {
		t.Fatalf("print effects: %v", err)
	}
	chains := service.Chains()
	if len(chains) != 2 {
		t.Fatalf("expected two chains, got %d", len(chains))
	}
	if chains[1].Channel != 2 || !chains[1].PrintEffects {
		t.Fatalf("unexpected chain: %+v", chains[1])
	}
	if !chains[1].Effects.Amp.Enabled {
		t.Fatal("expected the default amp chain")
	}
}

func TestSummariesListTheSessionFolderName(t *testing.T) {
	service := newService(t)
	if err := service.Create("Move me", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	if filepath.Base(service.Root()) != "Move me" {
		t.Fatalf("unexpected folder: %s", service.Root())
	}
	summaries, err := service.Summaries()
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "Move me" {
		t.Fatalf("unexpected summaries: %+v", summaries)
	}
}
