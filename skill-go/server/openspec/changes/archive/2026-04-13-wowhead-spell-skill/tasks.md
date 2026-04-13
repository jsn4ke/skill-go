## 1. Create skill.md file

- [x] 1.1 Create `skill.md` at project root with YAML front matter header (schema version, source, locale)
- [x] 1.2 Add Fireball (spell=38692) as the first complete spell entry with all fields populated per spec

## 2. Validate schema

- [x] 2.3 Verify Fireball entry fields match Wowhead page values (duration 8000, castTime 3500, manaCost 465, rangeYards 35, gcd 1500, 2 effects)
- [x] 2.4 Verify school name → SchoolMask mapping (fire=1) and effect types align with Go SpellEffectType enum

## 3. Add secondary spell examples

- [x] 3.5 Add Frost Nova (spell=27088) as instant+cooldown example
- [x] 3.6 Add Arcane Intellect (spell=27126) as apply_aura example without damage
