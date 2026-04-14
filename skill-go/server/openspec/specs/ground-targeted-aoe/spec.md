## ADDED Requirements

### Requirement: SpellEffectInfo includes radius field
The `SpellEffectInfo` struct SHALL have a `Radius float64` field. The CSV loader SHALL parse this from `spell_effects.csv` column 10. Spells with `radius > 0` on a damage effect are considered ground-targeted AoE spells.

#### Scenario: Blizzard effect loaded with radius
- **WHEN** `spell_effects.csv` contains an effect row with `radius=8`
- **THEN** the loaded `SpellEffectInfo` has `Radius=8.0`

#### Scenario: Non-AoE spell effect has zero radius
- **WHEN** `spell_effects.csv` contains a Fireball effect row without radius column
- **THEN** the loaded `SpellEffectInfo` has `Radius=0.0`

### Requirement: CastRequest accepts ground destination
The `CastRequest` struct SHALL include `DestX *float64` and `DestZ *float64` fields (JSON: `destX`, `destZ`). When provided, these represent the world position where the AoE spell is centered. When nil, the spell uses unit-based targeting (existing behavior).

#### Scenario: Ground-targeted cast request
- **WHEN** the frontend sends `{ spellID: 10, destX: 30.0, destZ: 0.0 }`
- **THEN** the server resolves the spell as AoE with center at (30.0, 0.0)

#### Scenario: Unit-targeted cast request unchanged
- **WHEN** the frontend sends `{ spellID: 38692, targetIDs: [3] }` without destX/destZ
- **THEN** the server resolves targets from targetIDs as before

### Requirement: Game loop resolves AoE targets via targeting package
When `handleCast()` receives a cast request with non-nil `DestX`/`DestZ`, it SHALL build a `targeting.SelectionContext` with `SelectArea` category and `RefPosition` reference, using `DestX`/`DestZ` as the origin and the spell effect's `Radius` as the selection radius. It SHALL call `targeting.Select()` to resolve targets. The destination position and radius SHALL be stored on `pendingCast` for per-tick re-resolution.

#### Scenario: AoE cast resolves targets in radius
- **WHEN** Blizzard is cast at position (30, 0) with radius 8
- **THEN** only units within 8 yards of (30, 0) are included as targets

#### Scenario: AoE cast with no targets in range
- **WHEN** Blizzard is cast at position (0, 0) with radius 8 and no units within 8 yards
- **THEN** the spell enters channeling but tick events show zero targets

### Requirement: GameLoop implements targeting.UnitProvider
The `GameLoop` SHALL implement the `targeting.UnitProvider` interface by providing a `GetAllUnits()` method that returns `gl.allUnits`. This allows the targeting package to query the full unit list.

#### Scenario: UnitProvider returns all units
- **WHEN** `targeting.Select()` calls `world.GetAllUnits()`
- **THEN** it receives the complete unit list from the game loop

### Requirement: Frontend enters ground targeting mode for AoE spells
When a spell button is clicked and the spell has an effect with `radius > 0`, the frontend SHALL enter ground targeting mode instead of unit targeting mode. In ground targeting mode:
- A translucent circle indicator (matching the spell's radius) SHALL follow the mouse cursor on the ground plane
- Clicking the ground SHALL cast the spell at that position (sending `destX`/`destZ`)
- Clicking a unit SHALL cancel ground targeting (not select the unit)
- Pressing ESC SHALL cancel ground targeting

#### Scenario: Clicking Blizzard button enters ground targeting mode
- **WHEN** the Blizzard button is clicked
- **THEN** a "Select location for 暴风雪..." indicator appears and a frost-blue circle follows the cursor on the ground

#### Scenario: Clicking ground in targeting mode casts spell
- **WHEN** the user clicks the ground at position (30, 5) during ground targeting
- **THEN** a cast request is sent with `destX=30, destZ=5` and ground targeting mode exits

#### Scenario: ESC cancels ground targeting
- **WHEN** the user presses ESC during ground targeting
- **THEN** ground targeting mode exits, no spell is cast

### Requirement: Frontend renders Blizzard area during channel
When Blizzard enters channeling, the frontend SHALL render a semi-transparent frost-blue circle mesh at the destination position on the ground. This visual SHALL persist for the duration of the channel and be removed when channeling ends.

#### Scenario: Blizzard area circle appears during channel
- **WHEN** Blizzard cast succeeds and enters channeling at position (30, 0)
- **THEN** a frost-blue circle of radius 8 appears at (30, 0) on the ground

#### Scenario: Blizzard area circle removed on channel end
- **WHEN** `channel_elapsed` or `channel_stopped` event is received
- **THEN** the Blizzard area circle is removed from the scene
