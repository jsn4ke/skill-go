## MODIFIED Requirements

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
- **WHEN** parsing row `10,0,school_damage,frost,25,0,8,,,,,,`
- **THEN** `EffectIndex=0`, `EffectType=SpellEffectSchoolDamage`, `SchoolMask=SchoolMaskFrost`, `PeriodicTickInterval=0` (set from tickInterval col), `BasePoints=8` (amplitude column overwrites value), `periodicType=0`

#### Scenario: school_damage non-periodic (Fireball)
- **WHEN** parsing row `38692,0,school_damage,fire,717,0,0,,,,,,`
- **THEN** `EffectIndex=0`, `EffectType=SpellEffectSchoolDamage`, `SchoolMask=SchoolMaskFire`, `BasePoints=717`, `periodicType=0`, `amplitude=0`

#### Scenario: apply_aura with aura fields
- **WHEN** parsing row `10,1,apply_aura,frost,65,,1500,1,10,,8`
- **THEN** `EffectIndex=1`, `EffectType=SpellEffectApplyAura`, `SchoolMask=SchoolMaskFrost`, `BasePoints=65`, `PeriodicTickInterval=1500`, `AuraDuration=1`, `AuraType=10`, `MiscValue=0`, `TriggerSpellID=8`

#### Scenario: trigger_spell
- **WHEN** parsing row `100,1,trigger_spell,physical,0,,,,,,,7922`
- **THEN** `EffectIndex=1`, `EffectType=SpellEffectTriggerSpell`, `SchoolMask=SchoolMaskPhysical`, `TriggerSpellID=7922`

### Requirement: Loader SHALL pad rows to 12 columns before parsing

The loader SHALL ensure each row has at least 12 columns by appending empty strings until reaching 12. This allows trailing commas in CSV rows to be omitted while maintaining consistent field positions.

#### Scenario: Row with trailing comma omitted
- **WHEN** parsing row `38692,0,school_damage,fire,717` (only 5 columns)
- **THEN** the loader pads it to 12: `["38692","0","school_damage","fire","717","","","","","","",""]`
- **AND** parses correctly as non-periodic school_damage with no extra fields
