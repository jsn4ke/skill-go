## Context

The spell engine has a complete channeling state machine (`spell.go`: `startChannel()`, `ExecuteChannelTick()`, `FinishChannel()`) and a channel ticker in the game loop (`game_loop.go`: `startChannelTicker()`, `handleChannelTick()`, `handleChannelElapsed()`), but these have never been exercised end-to-end. The `targeting` package has `selectArea()` for radius-based AoE selection, but it is disconnected from the spell pipeline. The frontend has `raycastGround()` but no ground-targeting spell mode.

Spell 10 (Blizzard/暴风雪) is a channeled frost AoE: the caster places an area on the ground, then ice shards damage all enemies within 8 yards every 1 second for 8 seconds. DBC data: 320 mana, 30yd range, no cooldown, 1.5s GCD.

**Current flow gaps:**
1. `parseSpellRow()` only parses 9 columns — no `isChanneled`, `channelDuration`, `tickInterval`
2. `SpellEffectInfo` has no `Radius` field — AoE radius is undefined
3. `CastRequest` has no `DestX`/`DestZ` — ground position cannot be sent
4. `handleCast()` resolves targets from `TargetIDs` only — never calls `targeting.Select()`
5. `ExecuteChannelTick()` uses frozen `s.Targets` — no per-tick re-resolution
6. Frontend has no ground-targeting UI for AoE spells

## Goals / Non-Goals

**Goals:**
- End-to-end channeling: CSV → Prepare → Cast → channel ticks → Finish
- Ground-targeted AoE: click ground → server receives dest position → area targeting
- Per-tick re-resolution: units walking in/out of Blizzard area are correctly hit/missed
- Frontend: ground-targeting mode with radius indicator, channel progress bar, Blizzard area visual

**Non-Goals:**
- Channel pushback on damage taken (future enhancement)
- Channel interruption by movement (future enhancement)
- Spell coefficient/spellpower scaling for channel ticks (uses BasePoints directly for now)
- Multiple simultaneous channeled spells (only one `pending` cast at a time)
- Blizzard slow/debuff effect (DBC spell 10 is damage-only per wowhead)

## Decisions

### 1. Per-tick target re-resolution vs frozen target list
**Decision:** Re-resolve targets on every channel tick using `targeting.SelectArea()`.
**Rationale:** In WoW, units can walk in/out of Blizzard. Freezing the list at cast time would be incorrect behavior and would be immediately noticeable. The `targeting.SelectArea()` function already exists and is O(n) — with typical unit counts (<20), this is trivial.
**Alternative:** Freeze at cast — simpler but noticeably wrong gameplay.

### 2. Radius storage location
**Decision:** Add `Radius float64` to `SpellEffectInfo`. Parse from `spell_effects.csv` column 10.
**Rationale:** Radius is per-effect, not per-spell. A spell could have effects with different radii. Keeping it on the effect matches DBC structure where radius is per-effect.
**Alternative:** Store on `SpellInfo` — simpler but less flexible.

### 3. Ground targeting in CastRequest
**Decision:** Add `DestX *float64` and `DestZ *float64` to `CastRequest`. Use pointers so nil = no ground target (backward compatible).
**Rationale:** Non-AoE spells don't need a destination. Pointer fields with `omitempty` make existing spells unaffected.

### 4. AoE target resolution wiring
**Decision:** In `handleCast()`, when `DestX`/`DestZ` are provided, build a `targeting.SelectionContext` with `SelectArea` + `RefPosition` and call `targeting.Select()`. Store the `SelectionContext` (or dest position + radius) on `pendingCast` so ticks can re-resolve.
**Rationale:** The targeting package is already complete. We just need to call it. Storing the dest+radius on pendingCast gives `handleChannelTick()` everything it needs to re-resolve per tick.

### 5. Channel tick re-resolution approach
**Decision:** In `handleChannelTick()`, before calling `ctx.ExecuteChannelTick()`, re-resolve targets via `targeting.Select()` and replace `ctx.Targets` with the fresh list. Add a `SetTargets(targets)` method on `SpellContext`.
**Rationale:** `ExecuteChannelTick()` iterates `s.Targets` internally. Rather than modifying the tick handler's internals, we simply update `Targets` before each tick. Clean and minimal change.

### 6. Frontend ground targeting mode
**Decision:** Extend `enterTargetingMode()` to accept a mode parameter: `'unit'` (existing behavior) or `'ground'`. In ground mode, clicking the ground (not a unit) places the spell at that location. Show a ground circle indicator at the mouse position with the spell's radius. On click, send `destX`/`destZ` in the cast request.
**Rationale:** Reuses existing targeting infrastructure. The ground circle visual provides clear feedback. The mode distinction is minimal code change.

### 7. Channel bar visualization
**Decision:** Reuse the existing cast bar (`#cast-bar`) for channeling. When the cast response indicates `result === 'channeling'`, keep the bar visible and animate it counting down from `channelDuration` to 0 (instead of 0 to castTime). The bar fill shrinks from right to left. On `channel_elapsed` or `channel_stopped` events, hide the bar.
**Rationale:** The cast bar UI already exists. Extending it for channeling is natural and avoids new UI elements.

### 8. Blizzard area visual on the ground
**Decision:** Create a Three.js `CircleGeometry` mesh at the destination position when channeling starts. Color: semi-transparent frost blue. Remove it when channel ends. Use `addUpdatable` or direct scene management.
**Rationale:** Simple circle on the ground clearly shows the Blizzard area. Matches WoW's ground indicator.

### 9. Frontend spell metadata for targeting mode
**Decision:** The `/api/spells` response includes spell effects data. The frontend determines targeting mode based on whether the spell has a radius (`effect.radius > 0`) on `school_damage` effect type. If radius > 0 → ground targeting mode. Otherwise → unit targeting mode.
**Rationale:** Radius-based detection is generic — any future AoE spell with a radius will automatically get ground targeting. No hardcoded spell list.

## Risks / Trade-offs

- **[Channel state conflicts]** If a user starts a new cast while channeling, only one `pending` slot exists. → Mitigation: reject cast attempts during active channel (check `gl.pending != nil`).
- **[Per-tick re-resolution performance]** With many units, re-resolving per tick could be slow. → Mitigation: negligible at current unit counts (<20). If needed, cache with dirty flag later.
- **[Frontend ground indicator during movement]** The circle indicator needs to follow the mouse correctly in 3D space. → Mitigation: reuse existing `raycastGround()` in the `pointermove` handler.
- **[3D Y vs Z axis]** The engine uses Z for up in `unit.Position` but Three.js uses Y for up. Ground position mapping must be consistent. → Mitigation: existing code already handles this mapping in `raycastGround()`.

## Open Questions

None — all major decisions are clear. The channeling infrastructure is already built, so this is primarily integration work.
