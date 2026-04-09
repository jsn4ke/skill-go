package unit

import (
	"testing"

	"skill-go/server/spelldef"
)

// --- 4.1 Default attributes ---

func TestNewUnit_DefaultAttributes(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)

	if u.Str != 0 || u.Agi != 0 || u.Sta != 0 || u.Int != 0 || u.Spi != 0 {
		t.Errorf("base attributes should be 0, got Str=%d Agi=%d Sta=%d Int=%d Spi=%d",
			u.Str, u.Agi, u.Sta, u.Int, u.Spi)
	}
	if u.AttackPower != 0 || u.SpellPower != 0 {
		t.Errorf("combat stats should be 0, got AP=%d SP=%d", u.AttackPower, u.SpellPower)
	}
	if u.CritMelee != 0 || u.CritSpell != 0 || u.Dodge != 0 || u.Parry != 0 || u.Block != 0 {
		t.Error("percentage stats should be 0")
	}
}

// --- 4.2 ModifyStat ---

func TestModifyStat_Increase(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)

	u.ModifyStat(StatAttackPower, 50)
	if u.AttackPower != 50 {
		t.Errorf("AttackPower = %d, want 50", u.AttackPower)
	}

	u.ModifyStat(StatIntellect, 20)
	if u.Int != 20 {
		t.Errorf("Int = %d, want 20", u.Int)
	}

	u.ModifyStat(StatCritMelee, 5)
	if u.CritMelee != 5.0 {
		t.Errorf("CritMelee = %v, want 5.0", u.CritMelee)
	}
}

func TestModifyStat_Decrease(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)
	u.SpellPower = 100
	u.ModifyStat(StatSpellPower, -20)
	if u.SpellPower != 80 {
		t.Errorf("SpellPower = %d, want 80", u.SpellPower)
	}
}

// --- 4.3 Armor ---

func TestSetArmor(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)
	u.SetArmor(500)
	if u.Armor != 500 {
		t.Errorf("Armor = %d, want 500", u.Armor)
	}
}

// --- 4.4 Resistance ---

func TestResistance_SetAndGet(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)
	u.SetResistance(spelldef.SchoolMaskFire, 75.0)
	if got := u.GetResistance(spelldef.SchoolMaskFire); got != 75.0 {
		t.Errorf("Fire resistance = %v, want 75.0", got)
	}
}

func TestResistance_OtherUnchanged(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)
	u.SetResistance(spelldef.SchoolMaskFire, 75.0)
	if got := u.GetResistance(spelldef.SchoolMaskFrost); got != 0.0 {
		t.Errorf("Frost resistance should be 0, got %v", got)
	}
}

func TestResistance_AllSchools(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)
	schools := []spelldef.SchoolMask{
		spelldef.SchoolMaskFire,
		spelldef.SchoolMaskFrost,
		spelldef.SchoolMaskArcane,
		spelldef.SchoolMaskNature,
		spelldef.SchoolMaskShadow,
		spelldef.SchoolMaskHoly,
		spelldef.SchoolMaskPhysical,
	}
	for i, s := range schools {
		u.SetResistance(s, float64(i*10)+10)
	}
	for i, s := range schools {
		want := float64(i*10) + 10
		if got := u.GetResistance(s); got != want {
			t.Errorf("school %d: resistance = %v, want %v", i, got, want)
		}
	}
}

// --- 4.5 Level ---

func TestLevel_Default(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)
	if u.Level != 1 {
		t.Errorf("Level = %d, want 1", u.Level)
	}
}

func TestLevel_Set(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)
	u.SetLevel(60)
	if u.Level != 60 {
		t.Errorf("Level = %d, want 60", u.Level)
	}
}

// --- 4.6 IsFriendly ---

func TestIsFriendly_SameTeam(t *testing.T) {
	a := NewUnit(1, "A", 100, 50)
	b := NewUnit(2, "B", 100, 50)
	a.TeamID = 1
	b.TeamID = 1
	if !a.IsFriendly(b) {
		t.Error("units on same team should be friendly")
	}
}

func TestIsFriendly_DifferentTeam(t *testing.T) {
	a := NewUnit(1, "A", 100, 50)
	b := NewUnit(2, "B", 100, 50)
	a.TeamID = 1
	b.TeamID = 2
	if a.IsFriendly(b) {
		t.Error("units on different teams should not be friendly")
	}
}

// --- 4.7 SetWeaponDamage ---

func TestSetWeaponDamage(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)
	u.SetWeaponDamage(50, 100)
	if u.MinWeaponDamage != 50 || u.MaxWeaponDamage != 100 {
		t.Errorf("weapon damage = %d–%d, want 50–100", u.MinWeaponDamage, u.MaxWeaponDamage)
	}
}

// --- 4.8 NewUnitWithStats ---

func TestNewUnitWithStats(t *testing.T) {
	u := NewUnitWithStats(1, "Player", 1000, 500, 60, 1)
	if u.Level != 60 {
		t.Errorf("Level = %d, want 60", u.Level)
	}
	if u.TeamID != 1 {
		t.Errorf("TeamID = %d, want 1", u.TeamID)
	}
	if u.Health != 1000 || u.Mana != 500 {
		t.Errorf("HP=%d MP=%d, want 1000/500", u.Health, u.Mana)
	}
}

// --- 4.9 Backward compatibility ---

func TestNewUnit_BackwardCompatible(t *testing.T) {
	u := NewUnit(1, "Test", 100, 50)

	if u.Level != 1 {
		t.Errorf("default Level = %d, want 1", u.Level)
	}
	if u.TeamID != 0 {
		t.Errorf("default TeamID = %d, want 0", u.TeamID)
	}
	if u.Armor != 0 {
		t.Errorf("default Armor = %d, want 0", u.Armor)
	}
	for i := 0; i < spelldef.SchoolMax; i++ {
		if u.Resistances[i] != 0 {
			t.Errorf("default Resistances[%d] = %v, want 0", i, u.Resistances[i])
		}
	}
	if u.MinWeaponDamage != 0 || u.MaxWeaponDamage != 0 {
		t.Error("default weapon damage should be 0")
	}
	if u.BlockValue != 0 {
		t.Error("default BlockValue should be 0")
	}
}

// --- StatType.String ---

func TestStatType_String(t *testing.T) {
	tests := []struct {
		stat StatType
		want string
	}{
		{StatStrength, "Strength"},
		{StatIntellect, "Intellect"},
		{StatAttackPower, "AttackPower"},
		{StatCritMelee, "CritMelee"},
		{StatBlockValue, "BlockValue"},
	}
	for _, tt := range tests {
		if got := tt.stat.String(); got != tt.want {
			t.Errorf("StatType(%d).String() = %q, want %q", int(tt.stat), got, tt.want)
		}
	}
}
