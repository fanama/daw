package sessionapp_test

import (
	"testing"

	"gorecord/domain/session"
)

func TestSystemAudioTrackRecordsItsOwnStereoTake(t *testing.T) {
	service := newService(t)
	if err := service.Create("PC idea", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	system, err := service.AddTrackWithSource("Browser", session.SystemSource())
	if err != nil {
		t.Fatalf("add system track: %v", err)
	}
	if err := service.SetArm(system.ID(), true); err != nil {
		t.Fatalf("arm: %v", err)
	}
	take, plan, err := service.BeginTake(0)
	if err != nil {
		t.Fatalf("begin take: %v", err)
	}
	if len(plan) != 1 || !plan[0].Source.IsSystem() {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if take.Channels() != 2 {
		t.Fatalf("system take must be stereo, got %d channels", take.Channels())
	}
	if err := service.FinishTake(take.ID(), 9600, false); err != nil {
		t.Fatalf("finish take: %v", err)
	}
	root := service.Root()
	service.Close()
	reopened := newService(t)
	if err := reopened.Open(root); err != nil {
		t.Fatalf("open: %v", err)
	}
	tracks := reopened.Current().Tracks()
	if len(tracks) != 2 || !tracks[1].IsSystemAudio() {
		t.Fatalf("system source was not persisted: %+v", tracks)
	}
	if !tracks[1].IsArmed() {
		t.Fatal("arming was not persisted")
	}
}

func TestSystemAudioRefusesChainedEffects(t *testing.T) {
	service := newService(t)
	if err := service.Create("PC idea", 48000); err != nil {
		t.Fatalf("create: %v", err)
	}
	system, err := service.AddTrackWithSource("Browser", session.SystemSource())
	if err != nil {
		t.Fatalf("add system track: %v", err)
	}
	if err := service.SetPrintEffects(system.ID(), true); err == nil {
		t.Fatal("system audio takes must stay dry")
	}
	if chains := service.Chains(); len(chains) != 1 {
		t.Fatalf("system track leaked into the input chains: %+v", chains)
	}
}
