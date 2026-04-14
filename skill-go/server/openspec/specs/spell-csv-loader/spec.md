## ADDED Requirements

### Requirement: spell-csv-loader SHALL parse CSV files from server/data/ into SpellInfo structs

The loader SHALL read `server/data/spells.csv` and `server/data/spell_effects.csv`, join them by `spellId`, and produce `[]spelldef.SpellInfo`.

#### Scenario: Load all spells from CSV
- **WHEN** `LoadSpells("data/")` is called
- **THEN** the function SHALL return a `[]spelldef.SpellInfo` containing one entry per unique `spellId` in `spells.csv`
- **AND** each SpellInfo SHALL have its `Effects` populated from matching rows in `spell_effects.csv`

#### Scenario: File not found
- **WHEN** `spells.csv` does not exist in the data directory
- **THEN** the function SHALL return an error wrapping the file system error

#### Scenario: Malformed CSV row
- **WHEN** a row has incorrect column count or unparseable values
- **THEN** the function SHALL return an error indicating the row number and parse failure reason

---

### Requirement: spell-csv-loader SHALL define spells.csv schema

The loader SHALL parse `spells.csv` with these columns (header row required):

| Column | Type | Required | Maps To |
|---|---|---|---|
| spellId | uint32 | yes | SpellInfo.ID |
| name | string | yes | SpellInfo.Name |
| school | string | yes | SpellInfo.SchoolMask (via name map) |
| castTime | int32 | yes | SpellInfo.CastTime (ms) |
| cooldown | int32 | yes | SpellInfo.RecoveryTime (ms) |
| gcd | int32 | yes | SpellInfo.CategoryRecoveryTime (ms) |
| manaCost | int32 | yes | SpellInfo.PowerCost |
| rangeYards | float64 | yes | SpellInfo.RangeMax |

#### Scenario: Fireball row
- **WHEN** parsing row `38692,火球术,fire,3500,0,1500,465,35`
- **THEN** `ID=38692`, `Name="火球术"`, `CastTime=3500`, `RecoveryTime=0`, `CategoryRecoveryTime=1500`, `PowerCost=465`, `RangeMax=35.0`, `SchoolMask=SchoolMaskFire`

---

### Requirement: spell-csv-loader SHALL define spell_effects.csv schema (12 columns)

The loader SHALL parse `spell_effects.csv` with these columns (header row required, 12 columns):

| Column | Type | Required | Maps To | Notes |
|---|---|---|---|---|
| spellId | uint32 | yes | join key to spells.csv | |
| index | int | yes | SpellEffectInfo.EffectIndex | |
| type | string | yes | SpellEffectInfo.EffectType (via name map) | |
| school | string | no | SpellEffectInfo.SchoolMask (via name map) | |
| value | int32 | no | SpellEffectInfo.BasePoints | |
| periodicType | int32 | no | internal: 0=none, 1=periodic | **NEW** |
| amplitude | int32 | no | SpellEffectInfo.BasePoints for periodic DoT | **NEW** |
| dummy1 | int32 | no | padding (unused) | **NEW** |
| dummy2 | int32 | no | padding (unused) | **NEW** |
| auraType | int32 | no | SpellEffectInfo.AuraType | |
| miscValue | int32 | no | SpellEffectInfo.MiscValue | |
| triggerSpellId | uint32 | no | SpellEffectInfo.TriggerSpellID | |

All rows SHALL be padded to 12 columns with empty trailing fields before parsing.

#### Scenario: school_damage with periodic DoT (Blizzard)
- **WHEN** parsing row `10,0,school_damage,frost,25,0,8,0,0,0,0,0`
- **THEN** `EffectIndex=0`, `EffectType=SpellEffectSchoolDamage`, `SchoolMask=SchoolMaskFrost`, `BasePoints=8` (amplitude column overwrites value)

#### Scenario: school_damage non-periodic (Fireball)
- **WHEN** parsing row `38692,0,school_damage,fire,717,0,0,0,0,0,0,0`
- **THEN** `EffectIndex=0`, `EffectType=SpellEffectSchoolDamage`, `SchoolMask=SchoolMaskFire`, `BasePoints=717`, `amplitude=0`

#### Scenario: apply_aura with aura fields
- **WHEN** parsing row `10,1,apply_aura,frost,65,0,0,0,0,1500,1,10,8`
- **THEN** `EffectIndex=1`, `EffectType=SpellEffectApplyAura`, `SchoolMask=SchoolMaskFrost`, `BasePoints=65`, `PeriodicTickInterval=1500`, `AuraDuration=1`, `AuraType=10`, `MiscValue=0`, `TriggerSpellID=8`

#### Scenario: trigger_spell
- **WHEN** parsing row `100,1,trigger_spell,physical,0,0,0,0,0,0,1,7922`
- **THEN** `EffectIndex=1`, `EffectType=SpellEffectTriggerSpell`, `SchoolMask=SchoolMaskPhysical`, `TriggerSpellID=7922`

### Requirement: Loader SHALL pad rows to 12 columns before parsing

The loader SHALL ensure each row has at least 12 columns by appending empty strings until reaching 12. This allows trailing commas in CSV rows to be omitted while maintaining consistent field positions.

#### Scenario: Row with trailing comma omitted
- **WHEN** parsing row `38692,0,school_damage,fire,717` (only 5 columns)
- **THEN** the loader pads it to 12: `["38692","0","school_damage","fire","717","","","","","","",""]`
- **AND** parses correctly as non-periodic school_damage with no extra fields

---

### Requirement: spell-csv-loader SHALL map school names to SchoolMask bitmasks

| CSV Value | SchoolMask Constant | Bit Value |
|---|---|---|
| fire | SchoolMaskFire | 1 |
| frost | SchoolMaskFrost | 2 |
| arcane | SchoolMaskArcane | 4 |
| nature | SchoolMaskNature | 8 |
| shadow | SchoolMaskShadow | 16 |
| holy | SchoolMaskHoly | 32 |
| physical | SchoolMaskPhysical | 64 |

#### Scenario: Unknown school name
- **WHEN** CSV contains `school: unknown_school`
- **THEN** the loader SHALL return an error listing the valid school names

---

### Requirement: spell-csv-loader SHALL map effect type names to SpellEffectType

| CSV type | SpellEffectType |
|---|---|
| school_damage | SpellEffectSchoolDamage |
| heal | SpellEffectHeal |
| apply_aura | SpellEffectApplyAura |
| trigger_spell | SpellEffectTriggerSpell |
| energize | SpellEffectEnergize |
| weapon_damage | SpellEffectWeaponDamage |

#### Scenario: Unknown effect type
- **WHEN** CSV contains `type: unknown_effect`
- **THEN** the loader SHALL return an error listing the valid effect types
