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

### Requirement: spell-csv-loader SHALL define spell_effects.csv schema

The loader SHALL parse `spell_effects.csv` with these columns (header row required):

| Column | Type | Required | Maps To |
|---|---|---|---|
| spellId | uint32 | yes | join key to spells.csv |
| index | int | yes | SpellEffectInfo.EffectIndex |
| type | string | yes | SpellEffectInfo.EffectType (via name map) |
| school | string | no | SpellEffectInfo.SchoolMask (via name map) |
| value | int32 | no | SpellEffectInfo.BasePoints |
| tickInterval | int32 | no | SpellEffectInfo.PeriodicTickInterval (ms) |
| duration | int32 | no | SpellEffectInfo.AuraDuration (ms) |

#### Scenario: Fireball school_damage effect
- **WHEN** parsing row `38692,0,school_damage,fire,717,,`
- **THEN** `EffectIndex=0`, `EffectType=SpellEffectSchoolDamage`, `SchoolMask=SchoolMaskFire`, `BasePoints=717`

#### Scenario: Fireball apply_aura effect with periodic tick
- **WHEN** parsing row `38692,1,apply_aura,fire,21,2000,8000`
- **THEN** `EffectIndex=1`, `EffectType=SpellEffectApplyAura`, `SchoolMask=SchoolMaskFire`, `BasePoints=21`, `PeriodicTickInterval=2000`, `AuraDuration=8000`

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
