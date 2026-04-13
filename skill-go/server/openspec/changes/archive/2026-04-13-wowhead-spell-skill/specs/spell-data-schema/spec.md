## ADDED Requirements

### Requirement: spell-data-schema SHALL define a YAML schema mapping Wowhead TBC spell attributes to structured data

The schema SHALL use YAML format with the following top-level fields per spell document:

```yaml
# spell=<wowheadId> <name>
spellId: uint32              # Wowhead spell ID, e.g. 38692
name: string                 # CN locale name, e.g. "火球术"
nameEn: string               # EN locale name, e.g. "Fireball" (optional)
icon: string                 # Wowhead icon name (optional)
```

#### Scenario: Minimal spell document
- **WHEN** a spell has only basic attributes
- **THEN** the document SHALL contain `spellId` and `name` fields at minimum

#### Scenario: Document separator
- **WHEN** multiple spells exist in skill.md
- **THEN** each spell document SHALL be separated by `---` YAML document separator

---

### Requirement: spell-data-schema SHALL define spell attribute fields matching Wowhead page layout

The schema SHALL include these fields corresponding to Wowhead spell details table:

```yaml
school: string               # 魔法学校: fire, frost, arcane, nature, shadow, holy, physical
mechanic: string             # 机制: "n/a" or mechanic name (e.g. "stun", "snare")
dispelType: string           # 驱散类型: "n/a", "magic", "curse", "disease", "poison"
gcdCategory: string          # GCD目录: "普通" or category name

duration: int32              # 持续时间, 毫秒. 0 = 无持续时间
manaCost: int32              # 法力消耗. 0 = 无消耗
range: string                # 范围描述, e.g. "35码 (中远程)"
rangeYards: float64          # 最大范围(码), for calculation. -1 = melee range
castTime: int32              # 施法时间, 毫秒. 0 = 瞬发
cooldown: int32              # 冷却时间, 毫秒. 0 = 无冷却
gcd: int32                   # GCD, 毫秒. 0 = no GCD

flags: string[]              # 标记列表, e.g. ["变形时无法使用"]
```

#### Scenario: Direct damage spell (Fireball)
- **WHEN** spell has cast time, mana cost, range, duration, and multiple effects
- **THEN** all corresponding fields SHALL be populated with correct millisecond values
  - `duration: 8000` (8秒)
  - `castTime: 3500` (3.5秒)
  - `manaCost: 465`
  - `rangeYards: 35`
  - `gcd: 1500`

#### Scenario: Instant spell with cooldown
- **WHEN** spell is instant cast with a cooldown (e.g. Frost Nova)
- **THEN** `castTime` SHALL be `0` and `cooldown` SHALL be the cooldown in milliseconds

#### Scenario: n/a fields
- **WHEN** a Wowhead field shows "n/a" (e.g. mechanic, dispel type for Fireball)
- **THEN** the field SHALL be omitted or set to empty string / 0 as appropriate

---

### Requirement: spell-data-schema SHALL define effect list with tagged union pattern

Each spell SHALL have an `effects` array. Each effect SHALL use a `type` field as discriminator:

```yaml
effects:
  - index: int               # effect index (0-based)
    type: string             # effect type discriminator
    # ... type-specific fields
```

Supported effect types and their fields:

**school_damage** (SpellEffectSchoolDamage):
```yaml
    type: school_damage
    school: string           # fire, frost, etc.
    value: int32             # base damage
    pvpMultiplier: float64   # PVP 倍率
    coef: float64            # spell coefficient (optional)
```

**heal** (SpellEffectHeal):
```yaml
    type: heal
    school: string
    value: int32
    pvpMultiplier: float64
    coef: float64            # optional
```

**apply_aura** (SpellEffectApplyAura):
```yaml
    type: apply_aura
    auraType: string         # periodic_damage, periodic_heal, stun, snare, etc.
    school: string           # damage school for damage auras
    value: int32             # tick value
    tickInterval: int32      # ms between ticks (for periodic auras)
    duration: int32          # aura duration in ms (overrides spell duration if set)
    pvpMultiplier: float64
```

**trigger_spell** (SpellEffectTriggerSpell):
```yaml
    type: trigger_spell
    triggerSpellId: uint32   # spell ID to trigger
```

**energize** (SpellEffectEnergize):
```yaml
    type: energize
    powerType: string        # mana, rage, energy
    value: int32
```

**weapon_damage** (SpellEffectWeaponDamage):
```yaml
    type: weapon_damage
    weaponPercent: float64   # weapon damage percentage
```

#### Scenario: Fireball with two effects
- **WHEN** spell=38692 (Fireball) has School Damage + Apply Aura: Periodic Damage
- **THEN** the effects array SHALL contain:
  - `effects[0]`: `type: school_damage`, `school: fire`, `value: 717`, `pvpMultiplier: 1`
  - `effects[1]`: `type: apply_aura`, `auraType: periodic_damage`, `value: 21`, `tickInterval: 2000`, `pvpMultiplier: 1`

#### Scenario: Periodic aura with tick interval
- **WHEN** an apply_aura effect has "每2秒" (every 2 seconds) tick rate
- **THEN** `tickInterval` SHALL be `2000` (milliseconds)

---

### Requirement: spell-data-schema SHALL provide a complete Fireball example

The schema SHALL include a complete, validated example for spell=38692 (火球术/Fireball) that demonstrates all field types. This example SHALL be usable as a template for adding new spells.

#### Scenario: Fireball example is self-contained
- **WHEN** an agent reads the Fireball example
- **THEN** all fields SHALL be populated with correct values matching the Wowhead page
- **AND** the example SHALL serve as the canonical reference for field usage

---

### Requirement: spell-data-schema SHALL align with Go SpellInfo struct field types

The YAML schema field types SHALL map directly to the Go `spelldef.SpellInfo` struct:

| YAML Field | Go Field | Go Type |
|---|---|---|
| spellId | ID | uint32 |
| school | SchoolMask | bitmask enum |
| castTime | CastTime | int32 (ms) |
| cooldown | RecoveryTime | int32 (ms) |
| gcd | CategoryRecoveryTime | int32 (ms) |
| manaCost | PowerCost | int32 |
| duration | (via aura) | int32 (ms) |
| rangeYards | RangeMax | float64 |
| effects[].type | SpellEffectType | enum |
| effects[].value | BasePoints | int32 |

School names SHALL map to `SchoolMask` bit values:
- `fire` = 1, `frost` = 2, `arcane` = 4, `nature` = 8, `shadow` = 16, `holy` = 32, `physical` = 64

#### Scenario: Field type consistency
- **WHEN** a spell document is parsed into SpellInfo
- **THEN** each field SHALL be directly assignable without type conversion (except school name → bitmask)
