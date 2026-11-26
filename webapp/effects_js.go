//go:build js && wasm

package main

import (
	"fmt"

	"fanamix/domain/mix"
	"fanamix/domain/session"
)

func (c *core) applyTrackEffect(trackID session.TrackID, req request) error {
	switch req.Op {
	case "track.effects":
		return c.setEffects(trackID, req.Effects)
	case "track.effect.add":
		return c.service.AddEffect(trackID, mix.EffectKind(req.Kind))
	case "track.effect.remove":
		return c.service.RemoveEffect(trackID, req.Index)
	case "track.effect.move":
		return c.service.MoveEffect(trackID, req.From, req.To)
	default:
		return fmt.Errorf("unknown track effect command %q", req.Op)
	}
}
