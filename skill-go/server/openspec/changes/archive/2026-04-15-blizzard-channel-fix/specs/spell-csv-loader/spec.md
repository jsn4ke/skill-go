## MODIFIED Requirements

### Requirement: spell-csv-loader SHALL define spells.csv schema

The loader SHALL parse `spells.csv` with these columns (header row required, 12 columns):

| Column | Type | Required | Maps To |
|---|---|---|---|
| spellId | uint32 | yes | SpellInfo.ID |
| name | string | yes | SpellInfo.Name |
| school | string | yes | SpellInfo.SchoolMask (via name map) |
| castTime | int32 | yes | SpellInfo.CastTime (ms) |
| cooldown | int32 | yes | SpellInfo.RecoveryTime (ms) |
| gcd | int32 | yes | SpellInfo.CategoryRecoveryTime (ms) |
| manaCost | int32 | yes | SpellInfo.PowerCost |
| powerType | int32 | no | SpellInfo.PowerType |
| rangeYards | float64 | yes | SpellInfo.RangeMax |
| isChanneled | bool (1/0) | no | SpellInfo.IsChanneled |
| channelDuration | int32 | no | SpellInfo.ChannelDuration (ms) |
| tickInterval | int32 | no | SpellInfo.TickInterval (ms) |

All rows SHALL be padded to 12 columns with empty trailing fields before parsing.

#### Scenario: Fireball row (non-channel, 9 columns)
- **WHEN** parsing row `38692,火球术,fire,3500,0,1500,465,,35` (9 columns, no channel fields)
- **THEN** `ID=38692`, `Name="火球术"`, `IsChanneled=false`, `ChannelDuration=0`, `TickInterval=0`

#### Scenario: Blizzard row (channel, 12 columns)
- **WHEN** parsing row `10,暴风雪,frost,0,0,1500,320,,30,1,8000,1000`
- **THEN** `ID=10`, `Name="暴风雪"`, `IsChanneled=true`, `ChannelDuration=8000`, `TickInterval=1000`

---

### Requirement: spell-csv-loader SHALL define spell_effects.csv schema (13 columns)

The loader SHALL parse `spell_effects.csv` with these columns (header row required, 13 columns):

| Column | Type | Required | Maps To | Notes |
|---|---|---|---|---|
| spellId | uint32 | yes | join key to spells.csv | |
| index | int | yes | SpellEffectInfo.EffectIndex | |
| type | string | yes | SpellEffectInfo.EffectType (via name map) | |
| school | string | no | SpellEffectInfo.SchoolMask (via name map) | |
| value | int32 | no | SpellEffectInfo.BasePoints | |
| periodicType | int32 | no | internal: 0=none, 1=periodic | |
| amplitude | int32 | no | SpellEffectInfo.BasePoints for periodic DoT | |
| dummy1 | int32 | no | padding (unused) | |
| dummy2 | int32 | no | padding (unused) | |
| auraType | int32 | no | SpellEffectInfo.AuraType | |
| miscValue | int32 | no | SpellEffectInfo.MiscValue | |
| triggerSpellId | uint32 | no | SpellEffectInfo.TriggerSpellID | |
| radius | float64 | no | SpellEffectInfo.Radius | **NEW** |

All rows SHALL be padded to 13 columns with empty trailing fields before parsing.

#### Scenario: Blizzard school_damage with radius
- **WHEN** parsing row `10,0,school_damage,frost,25,0,8,0,0,0,0,0,8`
- **THEN** `EffectIndex=0`, `EffectType=SpellEffectSchoolDamage`, `SchoolMask=SchoolMaskFrost`, `BasePoints=8`, `Radius=8.0`

#### Scenario: Fireball school_damage without radius (12 columns)
- **WHEN** parsing row `38692,0,school_damage,fire,717,0,0,0,0,0,0,0` (12 columns, no radius)
- **THEN** `EffectIndex=0`, `EffectType=SpellEffectSchoolDamage`, `SchoolMask=SchoolMaskFire`, `BasePoints=717`, `Radius=0`
