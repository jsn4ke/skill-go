## 1. Add struct fields

- [x] 1.1 Add `PeriodicTickInterval int32` field to `SpellEffectInfo` in `spelldef/spelldef.go`
- [x] 1.2 Add `AppliedTicks int32` field to `AuraEffect` in `aura/types.go`

## 2. Create CSV data files

- [x] 2.1 Create `server/data/spells.csv` with Fireball (38692), Frost Nova (27088), Arcane Intellect (27126)
- [x] 2.2 Create `server/data/spell_effects.csv` with corresponding effect rows (Fireball: school_damage + apply_aura periodic_damage)

## 3. Implement CSV loader

- [x] 3.1 Create `spelldef/loader.go` with school name → SchoolMask and effect type name → SpellEffectType lookup functions
- [x] 3.2 Implement `LoadSpells(dataDir string) ([]SpellInfo, error)` — parse spells.csv and spell_effects.csv, join by spellId
- [x] 3.3 Write unit test for `LoadSpells` — verify Fireball CSV parses to correct SpellInfo fields

## 4. Replace hardcoded Fireball

- [x] 4.1 Modify `initSpellBook()` in `api/game_loop.go` to call `spelldef.LoadSpells("data/")` instead of hardcoded array
- [x] 4.2 Verify `go build ./...` passes

## 5. Implement periodic damage in event loop

- [x] 5.1 Update `makeAuraHandler` in `api/server.go` to pass `PeriodicTickInterval` from `SpellEffectInfo` to `AuraEffect.PeriodicTimer`
- [x] 5.2 Extend `handleAuraUpdate` in `api/game_loop.go` to check `PeriodicTimer > 0` effects, calculate elapsed ticks, deal damage via event loop
- [x] 5.3 Emit trace event `periodic_damage` for each tick with damage amount
- [x] 5.4 Write test: Fireball DoT deals 21 damage every 2s for 3 ticks over 8s duration

## 6. Integration verification

- [x] 6.1 `go build ./...` passes
- [x] 6.2 `go test ./...` passes
- [ ] 6.3 Manual test: start server, cast Fireball, observe direct damage + DoT ticks in timeline
