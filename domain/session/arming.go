package session

import (
	"sort"

	"gorecord/domain/mix"
)

// ArmSlot is one recorded source in the next take, in take channel order.
type ArmSlot struct {
	TrackID      TrackID
	Source       Source
	PrintEffects bool
	Effects      mix.Effects
}

// ArmPlan returns the armed sources ordered by input channel, with PC audio
// last. Inputs and one PC audio track may share the pass.
func (s *Session) ArmPlan() ([]ArmSlot, error) {
	var slots []ArmSlot
	for _, track := range s.tracks {
		if !track.armed {
			continue
		}
		slots = append(slots, ArmSlot{
			TrackID:      track.id,
			Source:       track.source,
			PrintEffects: track.printEffects,
			Effects:      track.effects,
		})
	}
	if len(slots) == 0 {
		return nil, invalid("arm at least one track before recording")
	}
	sort.SliceStable(slots, func(left, right int) bool {
		if slots[left].Source.IsSystem() != slots[right].Source.IsSystem() {
			return !slots[left].Source.IsSystem()
		}
		return slots[left].Source.Channel < slots[right].Source.Channel
	})
	return slots, nil
}

// ArmedChannelCount reports how many channels the next take will contain.
func (s *Session) ArmedChannelCount() int {
	count := 0
	for _, track := range s.tracks {
		if !track.armed {
			continue
		}
		if track.source.IsSystem() {
			count += 2
			continue
		}
		count++
	}
	return count
}

// HasSolo reports whether any track is soloed.
func (s *Session) HasSolo() bool {
	for _, track := range s.tracks {
		if track.soloed {
			return true
		}
	}
	return false
}
