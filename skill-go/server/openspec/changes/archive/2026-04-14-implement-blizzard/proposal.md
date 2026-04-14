## Why

The engine currently supports 3 spells (Fireball, Charge, Charge Stun) but lacks any channeled AoE spell — a core Mage archetype ability. Blizzard (暴风雪, spell=10) is the ideal next spell because it exercises the **channeling system** that already has a complete state machine and ticker infrastructure in `spell.go`/`game_loop.go`, but has never been wired end-to-end. Implementing it reveals and forces completion of the channeling pipeline, ground-targeted AoE, and per-tick target re-resolution.

## What Changes

- **Channel CSV loading**: Extend `spelldef/loader.go` to parse `isChanneled`, `channelDuration`, `tickInterval` columns from `spells.csv`
- **Spell radius field**: Add `Radius` field to `SpellEffectInfo` and parse from `spell_effects.csv`
- **Ground-targeted casting**: Add `DestX`/`DestZ` destination position to `CastRequest` in the API, propagate through pending cast to channel ticks
- **AoE target resolution**: Wire the existing `targeting.SelectArea()` into `game_loop.go`'s cast path, re-resolve targets per channel tick (units can walk in/out of the Blizzard area)
- **Per-tick channel execution**: Verify and fix the channel tick pipeline (`startChannelTicker` → `handleChannelTick` → `ExecuteChannelTick`) for actual end-to-end execution
- **Frontend ground targeting UI**: Add ground-click targeting mode with area radius indicator for Blizzard
- **Frontend channel bar**: Extend the existing cast bar to show channel progress (counting down duration)
- **Frontend area visual**: Render a persistent frost AoE circle on the ground during channel
- **Blizzard spell data**: Add spell=10 rows to `spells.csv` and `spell_effects.csv` matching DBC values

## Capabilities

### New Capabilities
- `channeled-spell`: Engine support for channeled spells end-to-end: CSV loading, channel tick execution, per-tick target re-resolution, channel completion/interruption
- `ground-targeted-aoe`: Ground-targeted AoE casting system: destination position in API, area target resolution, frontend ground click UI with radius indicator

### Modified Capabilities

## Impact

- **Backend**: `spelldef/loader.go` (CSV columns), `spelldef/spelldef.go` (Radius field), `api/server.go` (CastRequest), `api/game_loop.go` (AoE targeting + channel tick wiring), `targeting/select.go` (called from game loop)
- **Frontend**: `web/app.js` (ground targeting mode, channel bar, Blizzard area visual), `web/character.js` (ground raycasting)
- **Data**: `data/spells.csv` (+1 row), `data/spell_effects.csv` (+1 row)
- **No breaking changes**: Existing spells (Fireball, Charge) are unaffected
