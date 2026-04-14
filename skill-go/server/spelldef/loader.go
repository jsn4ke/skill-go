package spelldef

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var schoolNameMap = map[string]SchoolMask{
	"fire":     SchoolMaskFire,
	"frost":    SchoolMaskFrost,
	"arcane":   SchoolMaskArcane,
	"nature":   SchoolMaskNature,
	"shadow":   SchoolMaskShadow,
	"holy":     SchoolMaskHoly,
	"physical": SchoolMaskPhysical,
}

var effectTypeMap = map[string]SpellEffectType{
	"school_damage": SpellEffectSchoolDamage,
	"heal":          SpellEffectHeal,
	"apply_aura":    SpellEffectApplyAura,
	"trigger_spell": SpellEffectTriggerSpell,
	"energize":      SpellEffectEnergize,
	"weapon_damage": SpellEffectWeaponDamage,
	"charge":        SpellEffectCharge,
}

// LoadSpells reads spells.csv and spell_effects.csv from dataDir, joins them by spellId,
// and returns a slice of SpellInfo.
func LoadSpells(dataDir string) ([]SpellInfo, error) {
	spellsPath := filepath.Join(dataDir, "spells.csv")
	effectsPath := filepath.Join(dataDir, "spell_effects.csv")

	spellsFile, err := os.Open(spellsPath)
	if err != nil {
		return nil, fmt.Errorf("open spells.csv: %w", err)
	}
	defer spellsFile.Close()

	effectsFile, err := os.Open(effectsPath)
	if err != nil {
		return nil, fmt.Errorf("open spell_effects.csv: %w", err)
	}
	defer effectsFile.Close()

	// Parse spells.csv
	spellsReader := csv.NewReader(spellsFile)
	spellsReader.FieldsPerRecord = -1
	spellRows, err := spellsReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read spells.csv: %w", err)
	}
	if len(spellRows) < 2 {
		return nil, fmt.Errorf("spells.csv: no data rows")
	}

	// Pad rows to 9 columns so optional fields are always at fixed indices
	for i := range spellRows {
		for len(spellRows[i]) < 9 {
			spellRows[i] = append(spellRows[i], "")
		}
	}

	// Build spell map by ID
	spellMap := make(map[uint32]*SpellInfo)
	for i, row := range spellRows[1:] {
		if len(row) < 8 {
			return nil, fmt.Errorf("spells.csv row %d: expected 8 columns, got %d", i+2, len(row))
		}
		si := &SpellInfo{
			MaxTargets: 1,
		}
		if err := parseSpellRow(row, si); err != nil {
			return nil, fmt.Errorf("spells.csv row %d: %w", i+2, err)
		}
		spellMap[si.ID] = si
	}

	// Parse spell_effects.csv
	effectsReader := csv.NewReader(effectsFile)
	effectsReader.FieldsPerRecord = -1 // allow variable field count (new columns are optional)
	effectRows, err := effectsReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read spell_effects.csv: %w", err)
	}

	// Pad rows to 10 columns so optional fields are always at fixed indices
	for i := range effectRows {
		for len(effectRows[i]) < 10 {
			effectRows[i] = append(effectRows[i], "")
		}
	}

	// Group effects by spellId
	for i, row := range effectRows[1:] {
		if len(row) < 7 {
			return nil, fmt.Errorf("spell_effects.csv row %d: expected 7 columns, got %d", i+2, len(row))
		}
		spellID, effect, err := parseEffectRow(row)
		if err != nil {
			return nil, fmt.Errorf("spell_effects.csv row %d: %w", i+2, err)
		}
		if err != nil {
			return nil, fmt.Errorf("spell_effects.csv row %d: %w", i+2, err)
		}
		si, ok := spellMap[spellID]
		if !ok {
			return nil, fmt.Errorf("spell_effects.csv row %d: spellId %d not found in spells.csv", i+2, spellID)
		}
		// Energize effects inherit amount from BasePoints and power type from spell
		if effect.EffectType == SpellEffectEnergize {
			if effect.EnergizeAmount == 0 {
				effect.EnergizeAmount = effect.BasePoints
			}
			if effect.EnergizeType == 0 {
				effect.EnergizeType = si.PowerType
			}
		}
		si.Effects = append(si.Effects, effect)
	}
	// Convert map to slice
	result := make([]SpellInfo, 0, len(spellMap))
	for _, si := range spellMap {
		result = append(result, *si)
	}
	return result, nil
}

func parseSpellRow(row []string, si *SpellInfo) error {
	id, err := strconv.ParseUint(strings.TrimSpace(row[0]), 10, 32)
	if err != nil {
		return fmt.Errorf("parse spellId: %w", err)
	}
	si.ID = uint32(id)
	si.Name = strings.TrimSpace(row[1])

	mask, err := parseSchoolMask(strings.TrimSpace(row[2]))
	if err != nil {
		return err
	}
	si.SchoolMask = mask

	si.CastTime, err = parseInt32(row[3], "castTime")
	if err != nil {
		return err
	}
	si.RecoveryTime, err = parseInt32(row[4], "cooldown")
	if err != nil {
		return err
	}
	si.CategoryRecoveryTime, err = parseInt32(row[5], "gcd")
	if err != nil {
		return err
	}
	si.PowerCost, err = parseInt32(row[6], "manaCost")
	if err != nil {
		return err
	}

	if v := strings.TrimSpace(row[7]); v != "" {
		pt, perr := strconv.ParseInt(v, 10, 32)
		if perr != nil {
			return fmt.Errorf("parse powerType: %w", perr)
		}
		si.PowerType = PowerType(pt)
	}

	si.RangeMax, err = parseFloat64(row[8], "rangeYards")
	if err != nil {
		return err
	}
	return nil
}

func parseEffectRow(row []string) (uint32, SpellEffectInfo, error) {
	var eff SpellEffectInfo
	var err error

	spellID, err := strconv.ParseUint(strings.TrimSpace(row[0]), 10, 32)
	if err != nil {
		return 0, eff, fmt.Errorf("parse spellId: %w", err)
	}

	eff.EffectIndex, err = strconv.Atoi(strings.TrimSpace(row[1]))
	if err != nil {
		return 0, eff, fmt.Errorf("parse index: %w", err)
	}

	typeName := strings.TrimSpace(row[2])
	et, ok := effectTypeMap[typeName]
	if !ok {
		return 0, eff, fmt.Errorf("unknown effect type %q: valid types are %v", typeName, effectTypeNames())
	}
	eff.EffectType = et

	schoolStr := strings.TrimSpace(row[3])
	if schoolStr != "" {
		mask, err := parseSchoolMask(schoolStr)
		if err != nil {
			return 0, eff, err
		}
		eff.SchoolMask = mask
	}

	valueStr := strings.TrimSpace(row[4])
	if valueStr != "" {
		eff.BasePoints, err = parseInt32(row[4], "value")
		if err != nil {
			return 0, eff, err
		}
	}

	tickStr := strings.TrimSpace(row[5])
	if tickStr != "" {
		eff.PeriodicTickInterval, err = parseInt32(row[5], "tickInterval")
		if err != nil {
			return 0, eff, err
		}
	}

	durationStr := strings.TrimSpace(row[6])
	if durationStr != "" {
		eff.AuraDuration, err = parseInt32(row[6], "duration")
		if err != nil {
			return 0, eff, err
		}
	}

	// Optional columns (7+): auraType, miscValue, triggerSpellId
	if len(row) > 7 {
		if v := strings.TrimSpace(row[7]); v != "" {
			eff.AuraType, err = parseInt32(row[7], "auraType")
			if err != nil {
				return 0, eff, fmt.Errorf("parse auraType: %w", err)
			}
		}
	}
	if len(row) > 8 {
		if v := strings.TrimSpace(row[8]); v != "" {
			eff.MiscValue, err = parseInt32(row[8], "miscValue")
			if err != nil {
				return 0, eff, fmt.Errorf("parse miscValue: %w", err)
			}
		}
	}
	if len(row) > 9 {
		if v := strings.TrimSpace(row[9]); v != "" {
			id, perr := strconv.ParseUint(v, 10, 32)
			if perr != nil {
				return 0, eff, fmt.Errorf("parse triggerSpellId: %w", perr)
			}
			eff.TriggerSpellID = uint32(id)
		}
	}

	return uint32(spellID), eff, nil
}

func parseSchoolMask(name string) (SchoolMask, error) {
	mask, ok := schoolNameMap[name]
	if !ok {
		return 0, fmt.Errorf("unknown school %q: valid schools are %v", name, schoolNames())
	}
	return mask, nil
}

func schoolNames() []string {
	names := make([]string, 0, len(schoolNameMap))
	for k := range schoolNameMap {
		names = append(names, k)
	}
	return names
}

func effectTypeNames() []string {
	names := make([]string, 0, len(effectTypeMap))
	for k := range effectTypeMap {
		names = append(names, k)
	}
	return names
}

func parseInt32(s, field string) (int32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}
	return int32(v), nil
}

func parseFloat64(s, field string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}
	return v, nil
}
