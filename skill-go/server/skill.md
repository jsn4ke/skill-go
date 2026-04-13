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
