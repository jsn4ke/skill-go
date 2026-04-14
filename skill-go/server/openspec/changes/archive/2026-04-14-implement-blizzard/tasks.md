## Tasks

### Backend: Data & Type Changes

- [x] **T1: Add Radius field to SpellEffectInfo**
  - File: `spelldef/spelldef.go` — add `Radius float64` to `SpellEffectInfo` struct

- [x] **T2: Extend CSV loader for channel fields and radius**
  - File: `spelldef/loader.go` —
    - Pad spell rows to 12 columns (add isChanneled, channelDuration, tickInterval at cols 9, 10, 11)
    - `parseSpellRow()`: parse cols 9 (isChanneled bool), 10 (channelDuration int32), 11 (tickInterval int32)
    - Pad effect rows to 11 columns (add radius at col 10)
    - `parseEffectRow()`: parse col 10 (radius float64)

- [x] **T3: Add Blizzard spell data to CSVs**
  - File: `data/spells.csv` — add row: `10,暴风雪,frost,0,0,1500,320,,30,1,8000,1000`
  - File: `data/spell_effects.csv` — add row: `10,0,school_damage,frost,25,,,,,,,8`
  - DBC: 320 mana, 30yd range, channeled, 8s duration, 1s tick interval, 25 dmg/tick, 8yd radius

### Backend: Engine Integration

- [x] **T4: Add SetTargets method to SpellContext**
  - File: `spell/spell.go` — add `SetTargets(targets []*unit.Unit)` that replaces `s.Targets`

- [x] **T5: Add DestX/DestZ to CastRequest and castPayload**
  - File: `api/server.go` — add `DestX *float64`, `DestZ *float64` to `CastRequest`
  - File: `api/game_loop.go` — add `DestX`, `DestZ` to `castPayload` struct

- [x] **T6: Store dest position and radius on pendingCast**
  - File: `api/server.go` — add `DestX`, `DestZ`, `Radius` fields to `pendingCast` struct

- [x] **T7: Implement targeting.UnitProvider on GameLoop**
  - File: `api/game_loop.go` — add `GetAllUnits() []*unit.Unit` method returning `gl.allUnits`

- [x] **T8: Wire AoE targeting into handleCast**
  - File: `api/game_loop.go` — in `handleCast()`:
    - When `DestX`/`DestZ` are non-nil, build `targeting.SelectionContext` with `SelectArea` + `RefPosition`
    - Call `targeting.Select()` to resolve targets
    - Find the effect's `Radius` for the SelectionContext
    - Store dest position and radius on `pendingCast`
    - If non-channeled AoE, execute immediately as before

- [x] **T9: Per-tick target re-resolution in handleChannelTick**
  - File: `api/game_loop.go` — in `handleChannelTick()`:
    - If `pendingCast` has DestX/DestZ, re-resolve targets via `targeting.Select()`
    - Call `ctx.SetTargets(resolvedTargets)` before `ctx.ExecuteChannelTick()`
    - Emit trace events with target list per tick

- [x] **T10: Include channel info in cast response**
  - File: `api/game_loop.go` — in `handleCastComplete()`, when `ctx.State == StateChanneling`:
    - Include `channelDuration` and `destX`/`destZ` in the response so the frontend can render the area visual
    - Add these fields to the response struct

- [x] **T11: Reject casts during active channel**
  - File: `api/game_loop.go` — at the start of `handleCast()`, check `gl.pending != nil` and reject with error "already casting/channeling"

### Frontend: Ground Targeting & Channel UI

- [x] **T12: Ground targeting mode for AoE spells**
  - File: `web/app.js` —
    - Modify `enterTargetingMode(spellID)` to accept mode `'unit'` or `'ground'`
    - Determine mode based on spell effect radius: if any effect has `radius > 0`, use `'ground'`
    - In ground targeting click handler: call `raycastGround()`, send `destX`/`destZ` in cast request
    - Show "Select location for 暴风雪..." indicator

- [x] **T13: Ground circle indicator following cursor**
  - File: `web/app.js` —
    - Create a Three.js `CircleGeometry` mesh (semi-transparent frost blue) for the ground indicator
    - On `pointermove` during ground targeting mode, update circle position via `raycastGround()`
    - Scale circle to match spell radius
    - Remove circle on exit

- [x] **T14: Channel bar visualization**
  - File: `web/app.js` —
    - When cast response has `result === 'channeling'`, reuse cast bar for channel countdown
    - Bar counts down from `channelDuration` to 0 (fill shrinks right-to-left)
    - On `channel_elapsed` / `channel_stopped` SSE events: hide bar, re-enable buttons
    - Handle `channel_tick` events for damage display

- [x] **T15: Blizzard area circle during channel**
  - File: `web/app.js` —
    - When channeling starts, create a persistent circle mesh at `destX`/`destZ`
    - Semi-transparent frost blue, matching radius
    - Remove on channel end events
    - Use `addUpdatable` for scene lifecycle management

### Verification

- [x] **T16: End-to-end test**
  - Select Mage as caster, click Blizzard button
  - Ground targeting mode activates with circle indicator
  - Click ground near a Target Dummy
  - Channel bar appears, Blizzard area circle renders
  - Damage ticks appear in spell log every 1s for 8s
  - Dummy takes damage, HP updates
  - Channel bar disappears after 8s
  - Blizzard circle removed from scene

### Backend: Speed Reduction (Slow) System

- [x] **T17: Add SpeedMod field to Unit**
  - File: `unit/unit.go` — add `SpeedMod float64` field (1.0 = normal, multiplicative)
  - Add `RecalcSpeedMod(slows []int32)` method — computes product of (1 - pct/100) for each slow
  - Initialize `SpeedMod: 1.0` in `NewUnit`

- [x] **T18: Add speed modifier aura handling**
  - File: `aura/types.go` — add `AuraMiscModSpeed int32 = 10` constant
  - File: `aura/manager.go` — extend `applyControlEffect`/`removeControlEffect` with `AuraMiscModSpeed` case
  - Add `recalcSpeedMod(mgr)` function that scans all auras for speed modifiers and recomputes owner's SpeedMod
  - Call `recalcSpeedMod` after `ApplyAura` and `RemoveAura` (after map mutation)
  - Fix `RemoveAura` to delete from map BEFORE recalculating speed (prevents stale values)

- [x] **T19: Add Blizzard slow effect to spell_effects.csv**
  - File: `data/spell_effects.csv` — add second effect: `10,1,apply_aura,frost,65,,1500,1,10,,8`
  - 65% slow, 1.5s duration, debuff, MiscValue=10 (MOD_SPEED), radius=8

- [x] **T20: Expose SpeedMod in UnitJSON**
  - File: `api/server.go` — add `SpeedMod float64` to `UnitJSON` struct and populate in `unitToJSON`

- [x] **T21: Reset SpeedMod on unit reset**
  - File: `api/game_loop.go` — in `handleReset`, set `SpeedMod = 1.0` for all units

- [x] **T22: Frontend speed modifier visual and movement**
  - File: `web/character.js` —
    - Add `speedMod: 1.0` to `group.userData` in `createCharacter`
    - In `updateCharacter`: track `speedMod` from `unitData.speedMod`, apply blue tint when slowed
    - In `updateUnitMovement`: multiply movement speed by `d.speedMod`
  - File: `web/app.js` —
    - Add SSE handler for `aura.applied`/`aura.removed`/`aura.refreshed` events to refresh unit state
