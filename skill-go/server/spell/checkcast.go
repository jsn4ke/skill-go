package spell

import (
	"log"

	"skill-go/server/spelldef"
	"skill-go/server/unit"
)

// CheckCast performs the full validation chain for spell casting.
// Returns CastErrNone if all checks pass.
func CheckCast(info *spelldef.SpellInfo, caster *unit.Unit, targets []*unit.Unit, historyProvider SpellHistoryProvider) spelldef.CastError {
	if err := checkCooldown(info, historyProvider); err != spelldef.CastErrNone {
		return err
	}
	if err := checkRange(info, caster, targets); err != spelldef.CastErrNone {
		return err
	}
	if err := checkSilence(info, caster); err != spelldef.CastErrNone {
		return err
	}
	if err := checkDisarm(info, caster); err != spelldef.CastErrNone {
		return err
	}
	if err := checkShapeshift(info, caster); err != spelldef.CastErrNone {
		return err
	}
	if err := checkItems(info, caster); err != spelldef.CastErrNone {
		return err
	}
	if err := checkArea(info, caster); err != spelldef.CastErrNone {
		return err
	}
	if err := checkMounted(info, caster); err != spelldef.CastErrNone {
		return err
	}
	return spelldef.CastErrNone
}

// SpellHistoryProvider abstracts access to cooldown/charge/GCD/lockout state.
// Implemented by cooldown.SpellHistory.
type SpellHistoryProvider interface {
	IsReady(spellID uint32, schoolMask spelldef.SchoolMask) bool
}

func checkCooldown(info *spelldef.SpellInfo, history SpellHistoryProvider) spelldef.CastError {
	if history == nil {
		return spelldef.CastErrNone
	}
	if !history.IsReady(info.ID, info.SchoolMask) {
		log.Printf("[CheckCast] %s: spell not ready (cooldown/GCD/lockout)", info.Name)
		return spelldef.CastErrNotReady
	}
	return spelldef.CastErrNone
}

func checkRange(info *spelldef.SpellInfo, caster *unit.Unit, targets []*unit.Unit) spelldef.CastError {
	if info.RangeMax <= 0 && info.RangeMin <= 0 {
		return spelldef.CastErrNone
	}
	for _, t := range targets {
		dist := caster.DistanceTo(t)
		if dist > info.RangeMax {
			log.Printf("[CheckCast] %s: target %s out of range (%.1f > %.1f)",
				info.Name, t.Name, dist, info.RangeMax)
			return spelldef.CastErrOutOfRange
		}
		if dist < info.RangeMin {
			log.Printf("[CheckCast] %s: target %s too close (%.1f < %.1f)",
				info.Name, t.Name, dist, info.RangeMin)
			return spelldef.CastErrOutOfRange
		}
	}
	return spelldef.CastErrNone
}

func checkSilence(info *spelldef.SpellInfo, caster *unit.Unit) spelldef.CastError {
	if info.PreventionType&spelldef.PreventionTypeSilence == 0 {
		return spelldef.CastErrNone
	}
	if caster.HasUnitState(spelldef.UnitStateSilenced) {
		log.Printf("[CheckCast] %s: caster %s is silenced", info.Name, caster.Name)
		return spelldef.CastErrSilenced
	}
	return spelldef.CastErrNone
}

func checkDisarm(info *spelldef.SpellInfo, caster *unit.Unit) spelldef.CastError {
	if info.PreventionType&spelldef.PreventionTypePacify == 0 {
		return spelldef.CastErrNone
	}
	if caster.HasUnitState(spelldef.UnitStateDisarmed) {
		log.Printf("[CheckCast] %s: caster %s is disarmed", info.Name, caster.Name)
		return spelldef.CastErrDisarmed
	}
	return spelldef.CastErrNone
}

func checkShapeshift(info *spelldef.SpellInfo, caster *unit.Unit) spelldef.CastError {
	if info.RequiresShapeshiftMask == 0 {
		return spelldef.CastErrNone
	}
	if caster.CurrentForm == 0 || (caster.CurrentForm&info.RequiresShapeshiftMask) == 0 {
		log.Printf("[CheckCast] %s: caster %s wrong shapeshift (form=%d, required=%d)",
			info.Name, caster.Name, caster.CurrentForm, info.RequiresShapeshiftMask)
		return spelldef.CastErrShapeshifted
	}
	return spelldef.CastErrNone
}

func checkItems(_ *spelldef.SpellInfo, _ *unit.Unit) spelldef.CastError {
	return spelldef.CastErrNone
}

func checkArea(info *spelldef.SpellInfo, caster *unit.Unit) spelldef.CastError {
	if info.RequiredAreaID == 0 {
		return spelldef.CastErrNone
	}
	_ = caster
	log.Printf("[CheckCast] %s: area restriction not met (required=%d)", info.Name, info.RequiredAreaID)
	return spelldef.CastErrWrongArea
}

func checkMounted(info *spelldef.SpellInfo, caster *unit.Unit) spelldef.CastError {
	if info.CastTime == 0 {
		return spelldef.CastErrNone
	}
	if caster.IsMounted() {
		log.Printf("[CheckCast] %s: caster %s is mounted (mountID=%d)",
			info.Name, caster.Name, caster.MountID)
		return spelldef.CastErrMounted
	}
	return spelldef.CastErrNone
}

// ReCheckRange verifies caster-target distance is still valid during Launched state.
func ReCheckRange(info *spelldef.SpellInfo, caster *unit.Unit, targets []*unit.Unit) bool {
	if info.RangeMax <= 0 {
		return true
	}
	for _, t := range targets {
		if caster.DistanceTo(t) > info.RangeMax {
			return false
		}
	}
	return true
}
