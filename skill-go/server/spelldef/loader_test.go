package spelldef

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSpells_Fireball(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "spells.csv"), "spellId,name,school,castTime,cooldown,gcd,manaCost,powerType,rangeYards\n"+
		"38692,火球术,fire,3500,0,1500,465,,35\n")
	writeCSV(t, filepath.Join(dir, "spell_effects.csv"), "spellId,index,type,school,value,tickInterval,duration\n"+
		"38692,0,school_damage,fire,717,,\n"+
		"38692,1,apply_aura,fire,21,2000,8000\n")

	spells, err := LoadSpells(dir)
	if err != nil {
		t.Fatalf("LoadSpells failed: %v", err)
	}
	if len(spells) != 1 {
		t.Fatalf("expected 1 spell, got %d", len(spells))
	}

	s := spells[0]
	assertEqual(t, "ID", uint32(38692), s.ID)
	assertEqual(t, "Name", "火球术", s.Name)
	assertEqual(t, "SchoolMask", SchoolMaskFire, s.SchoolMask)
	assertEqual(t, "CastTime", int32(3500), s.CastTime)
	assertEqual(t, "RecoveryTime", int32(0), s.RecoveryTime)
	assertEqual(t, "CategoryRecoveryTime", int32(1500), s.CategoryRecoveryTime)
	assertEqual(t, "PowerCost", int32(465), s.PowerCost)
	assertEqual(t, "RangeMax", 35.0, s.RangeMax)

	if len(s.Effects) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(s.Effects))
	}

	e0 := s.Effects[0]
	assertEqual(t, "EffectIndex", 0, e0.EffectIndex)
	assertEqual(t, "EffectType", SpellEffectSchoolDamage, e0.EffectType)
	assertEqual(t, "BasePoints", int32(717), e0.BasePoints)

	e1 := s.Effects[1]
	assertEqual(t, "EffectIndex", 1, e1.EffectIndex)
	assertEqual(t, "EffectType", SpellEffectApplyAura, e1.EffectType)
	assertEqual(t, "BasePoints", int32(21), e1.BasePoints)
	assertEqual(t, "PeriodicTickInterval", int32(2000), e1.PeriodicTickInterval)
	assertEqual(t, "AuraDuration", int32(8000), e1.AuraDuration)
}

func TestLoadSpells_MultipleSpells(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "spells.csv"), "spellId,name,school,castTime,cooldown,gcd,manaCost,powerType,rangeYards\n"+
		"38692,火球术,fire,3500,0,1500,465,,35\n"+
		"100,冲锋,physical,0,15000,0,0,1,25\n")
	writeCSV(t, filepath.Join(dir, "spell_effects.csv"), "spellId,index,type,school,value,tickInterval,duration\n"+
		"38692,0,school_damage,fire,717,,\n"+
		"100,0,charge,physical,1,,\n")

	spells, err := LoadSpells(dir)
	if err != nil {
		t.Fatalf("LoadSpells failed: %v", err)
	}
	if len(spells) != 2 {
		t.Fatalf("expected 2 spells, got %d", len(spells))
	}
}

func TestLoadSpells_FileNotFound(t *testing.T) {
	_, err := LoadSpells("/nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestLoadSpells_UnknownSchool(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "spells.csv"), "spellId,name,school,castTime,cooldown,gcd,manaCost,powerType,rangeYards\n"+
		"1,test,invalid,0,0,0,0,,0\n")

	_, err := LoadSpells(dir)
	if err == nil {
		t.Fatal("expected error for unknown school")
	}
}

func TestLoadSpells_UnknownEffectType(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "spells.csv"), "spellId,name,school,castTime,cooldown,gcd,manaCost,powerType,rangeYards\n"+
		"1,test,fire,0,0,0,0,,0\n")
	writeCSV(t, filepath.Join(dir, "spell_effects.csv"), "spellId,index,type,school,value,tickInterval,duration\n"+
		"1,0,invalid_type,,,,\n")

	_, err := LoadSpells(dir)
	if err == nil {
		t.Fatal("expected error for unknown effect type")
	}
}

func writeCSV(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertEqual[T comparable](t *testing.T, field string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %v, got %v", field, want, got)
	}
}
