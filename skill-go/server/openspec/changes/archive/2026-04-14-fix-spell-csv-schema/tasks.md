## 1. Update spell_effects.csv Header

- [x] 1.1 Change header from 10 columns to 12 columns: add `periodicType,amplitude,dummy1,dummy2` after `value`

## 2. Fix CSV Row Column Counts

- [x] 2.1 Update Fireball school_damage row: `38692,0,school_damage,fire,717,0,0,0,0,0,0,0`
- [x] 2.2 Update Fireball apply_aura row: `38692,1,apply_aura,fire,21,0,0,0,0,1,21,7922`
- [x] 2.3 Update Charge rows (4 rows): pad all to 12 columns with empty trailing fields
- [x] 2.4 Update Stun row: `7922,0,apply_aura,physical,0,0,0,0,1500,1,4,8`
- [x] 2.5 Update Blizzard school_damage row: `10,0,school_damage,frost,25,0,8,0,0,0,0,0`
- [x] 2.6 Update Blizzard apply_aura row: `10,1,apply_aura,frost,65,0,0,0,1500,1,10,8`

## 3. Fix parseEffectRow in spelldef/loader.go

- [x] 3.1 Change row pad target from 10 to 12 columns (no-op: loader uses len>N checks, already handles 12 cols)
- [x] 3.2 Add amplitude parsing for SpellEffectSchoolDamage (reads col 6, overwrites BasePoints when non-zero)
- [x] 3.3 Change auraType/miscValue/triggerSpellId from cols 7/8/9 to cols 9/10/11
- [x] 3.4 Verify: `go build ./...` compiles without errors

## 4. Verify

- [x] 4.1 Run `go test ./spelldef/...` — all tests pass
- [x] 4.2 Verify Blizzard DoT row parses with amplitude=8
