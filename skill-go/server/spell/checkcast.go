package spell

import (
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// CheckCast performs the full validation chain for spell casting.
// Returns CastErrNone if all checks pass.
func CheckCast(info *spelldef.SpellInfo, caster *unit.Unit, targets []*unit.Unit, historyProvider SpellHistoryProvider, t *trace.Trace) spelldef.CastError {
	if err := checkCooldown(info, historyProvider, t); err != spelldef.CastErrNone {
		return err
	}
	if err := checkRange(info, caster, targets, t); err != spelldef.CastErrNone {
		return err
	}
	if err := checkSilence(info, caster, t); err != spelldef.CastErrNone {
		return err
	}
	if err := checkDisarm(info, caster, t); err != spelldef.CastErrNone {
		return err
	}
	if err := checkShapeshift(info, caster, t); err != spelldef.CastErrNone {
		return err
	}
	if err := checkItems(info, caster, t); err != spelldef.CastErrNone {
		return err
	}
	if err := checkArea(info, caster, t); err != spelldef.CastErrNone {
		return err
	}
	if err := checkMounted(info, caster, t); err != spelldef.CastErrNone {
		return err
	}

	t.Event(trace.SpanCheckCast, "passed", info.ID, info.Name, nil)
	return spelldef.CastErrNone
}

// SpellHistoryProvider abstracts access to cooldown/charge/GCD/lockout state.
// Implemented by cooldown.SpellHistory.
type SpellHistoryProvider interface {
	IsReady(spellID uint32, schoolMask spelldef.SchoolMask) bool
}

func checkCooldown(info *spelldef.SpellInfo, history SpellHistoryProvider, t *trace.Trace) spelldef.CastError {
	if history == nil {
		return spelldef.CastErrNone
	}
	if !history.IsReady(info.ID, info.SchoolMask) {
		t.Event(trace.SpanCheckCast, "failed", info.ID, info.Name, map[string]interface{}{
			"reason": "not_ready",
		})
		return spelldef.CastErrNotReady
	}
	return spelldef.CastErrNone
}

func checkRange(info *spelldef.SpellInfo, caster *unit.Unit, targets []*unit.Unit, t *trace.Trace) spelldef.CastError {
	if info.RangeMax <= 0 && info.RangeMin <= 0 {
		return spelldef.CastErrNone
	}
	for _, tgt := range targets {
		dist := caster.DistanceTo(tgt)
		if dist > info.RangeMax {
			t.Event(trace.SpanCheckCast, "failed", info.ID, info.Name, map[string]interface{}{
				"reason": "out_of_range",
				"target": tgt.Name,
				"dist":   dist,
				"max":    info.RangeMax,
			})
			return spelldef.CastErrOutOfRange
		}
		if dist < info.RangeMin {
			t.Event(trace.SpanCheckCast, "failed", info.ID, info.Name, map[string]interface{}{
				"reason": "too_close",
				"target": tgt.Name,
				"dist":   dist,
				"min":    info.RangeMin,
			})
			return spelldef.CastErrOutOfRange
		}
	}
	return spelldef.CastErrNone
}

func checkSilence(info *spelldef.SpellInfo, caster *unit.Unit, t *trace.Trace) spelldef.CastError {
	if info.PreventionType&spelldef.PreventionTypeSilence == 0 {
		return spelldef.CastErrNone
	}
	if caster.HasUnitState(spelldef.UnitStateSilenced) {
		t.Event(trace.SpanCheckCast, "failed", info.ID, info.Name, map[string]interface{}{
			"reason": "silenced",
		})
		return spelldef.CastErrSilenced
	}
	return spelldef.CastErrNone
}

func checkDisarm(info *spelldef.SpellInfo, caster *unit.Unit, t *trace.Trace) spelldef.CastError {
	if info.PreventionType&spelldef.PreventionTypePacify == 0 {
		return spelldef.CastErrNone
	}
	if caster.HasUnitState(spelldef.UnitStateDisarmed) {
		t.Event(trace.SpanCheckCast, "failed", info.ID, info.Name, map[string]interface{}{
			"reason": "disarmed",
		})
		return spelldef.CastErrDisarmed
	}
	return spelldef.CastErrNone
}

func checkShapeshift(info *spelldef.SpellInfo, caster *unit.Unit, t *trace.Trace) spelldef.CastError {
	if info.Stances == 0 {
		return spelldef.CastErrNone
	}
	if spelldef.StancesBit(caster.CurrentForm) & info.Stances == 0 {
		t.Event(trace.SpanCheckCast, "failed", info.ID, info.Name, map[string]interface{}{
			"reason":   "wrong_shapeshift",
			"form":     int(caster.CurrentForm),
			"required": info.Stances,
		})
		return spelldef.CastErrShapeshifted
	}
	return spelldef.CastErrNone
}

func checkItems(_ *spelldef.SpellInfo, _ *unit.Unit, _ *trace.Trace) spelldef.CastError {
	return spelldef.CastErrNone
}

func checkArea(info *spelldef.SpellInfo, _ *unit.Unit, t *trace.Trace) spelldef.CastError {
	if info.RequiredAreaID == 0 {
		return spelldef.CastErrNone
	}
	t.Event(trace.SpanCheckCast, "failed", info.ID, info.Name, map[string]interface{}{
		"reason":  "wrong_area",
		"area_id": info.RequiredAreaID,
	})
	return spelldef.CastErrWrongArea
}

func checkMounted(info *spelldef.SpellInfo, caster *unit.Unit, t *trace.Trace) spelldef.CastError {
	if info.CastTime == 0 {
		return spelldef.CastErrNone
	}
	if caster.IsMounted() {
		t.Event(trace.SpanCheckCast, "failed", info.ID, info.Name, map[string]interface{}{
			"reason":  "mounted",
			"mountID": caster.MountID,
		})
		return spelldef.CastErrMounted
	}
	return spelldef.CastErrNone
}

// ReCheckRange verifies caster-target distance is still valid during Launched state.
func ReCheckRange(info *spelldef.SpellInfo, caster *unit.Unit, targets []*unit.Unit, t *trace.Trace) bool {
	if info.RangeMax <= 0 {
		return true
	}
	for _, tgt := range targets {
		if caster.DistanceTo(tgt) > info.RangeMax {
			t.Event(trace.SpanCheckCast, "recheck_failed", info.ID, info.Name, map[string]interface{}{
				"target": tgt.Name,
			})
			return false
		}
	}
	t.Event(trace.SpanCheckCast, "recheck_passed", info.ID, info.Name, nil)
	return true
}
