## MODIFIED Requirements

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

isToggle: bool               # 是否为切换型技能. default: false
toggleGroup: string          # 互斥组名. 空字符串 = 独立开关. 仅 isToggle=true 时有效

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

#### Scenario: Toggle spell with mutual exclusion group
- **WHEN** a toggle spell belongs to a mutual exclusion group (e.g. warrior stances)
- **THEN** `isToggle` SHALL be `true`
- **AND** `toggleGroup` SHALL be the group name (e.g. "warrior_stance")

#### Scenario: Toggle spell with independent on/off
- **WHEN** a toggle spell is independent (e.g. Stealth)
- **THEN** `isToggle` SHALL be `true`
- **AND** `toggleGroup` SHALL be empty string
