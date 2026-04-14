# WoW TBC Spell Data
# Schema version: 1.0
# Source: https://www.wowhead.com/tbc/cn/spells
# Locale: zh-CN

# spell=38692 火球术
spellId: 38692
name: 火球术
nameEn: Fireball
icon: spell_fire_flamebolt
school: fire
mechanic: ""
dispelType: ""
gcdCategory: 普通
duration: 8000
manaCost: 465
range: "35码 (中远程)"
rangeYards: 35
castTime: 3500
cooldown: 0
gcd: 1500
flags:
  - 变形时无法使用
effects:
  - index: 0
    type: school_damage
    school: fire
    value: 717
    pvpMultiplier: 1
  - index: 1
    type: apply_aura
    auraType: periodic_damage
    school: fire
    value: 21
    tickInterval: 2000
    pvpMultiplier: 1

---

# spell=27088 冰霜新星
spellId: 27088
name: 冰霜新星
nameEn: Frost Nova
icon: spell_frost_frostnova
school: frost
mechanic: ""
dispelType: magic
gcdCategory: 普通
duration: 8000
manaCost: 185
range: "0码 (自身)"
rangeYards: 0
castTime: 0
cooldown: 25000
gcd: 1500
flags:
  - 形变时无法使用
effects:
  - index: 0
    type: school_damage
    school: frost
    value: 99
    pvpMultiplier: 1
  - index: 1
    type: apply_aura
    auraType: root
    duration: 8000
    pvpMultiplier: 1

---

# spell=27126 奥术智慧
spellId: 27126
name: 奥术智慧
nameEn: Arcane Intellect
icon: spell_holy_magicalsentry
school: arcane
mechanic: ""
dispelType: magic
gcdCategory: 普通
duration: 1800000
manaCost: 700
range: "30码 (中远程)"
rangeYards: 30
castTime: 0
cooldown: 0
gcd: 1500
flags:
  - 变形时无法使用
effects:
  - index: 0
    type: apply_aura
    auraType: mod_stat
    school: arcane
    value: 40
    duration: 1800000
    pvpMultiplier: 1

---

# spell=100 冲锋
spellId: 100
name: 冲锋
nameEn: Charge
icon: ability_warrior_charge
school: physical
mechanic: ""
dispelType: ""
gcdCategory: ""
duration: 0
manaCost: 0
range: "8-25码 (近战)"
rangeYards: 25
castTime: 0
cooldown: 15000
gcd: 0
flags:
  - 是技能
  - 形变时无法使用
  - 不自动收剑
  - 战斗中不可用
  - 无视无敌效果
  - 不产生仇恨
  - 冲锋到目标时自动攻击
effects:
  - index: 0
    type: charge
    value: 1
  - index: 1
    type: dummy
    value: 90
  - index: 2
    type: trigger_spell
    value: 0
    triggerSpellId: 7922  # Charge Stun

---

# spell=7922 冲锋眩晕
spellId: 7922
name: 冲锋眩晕
nameEn: Charge Stun
icon: ability_warrior_charge
school: physical
mechanic: stun
dispelType: ""
gcdCategory: ""
duration: 1500
manaCost: 0
range: "8-25码"
rangeYards: 25
castTime: 0
cooldown: 0
gcd: 0
flags: []
effects:
  - index: 0
    type: apply_aura
    auraType: stun
    duration: 1500
