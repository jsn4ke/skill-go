## ADDED Requirements

### Requirement: CSV loader parses channel fields
The spell CSV loader SHALL parse `isChanneled`, `channelDuration`, and `tickInterval` columns from `spells.csv` (columns 9, 10, 11). These fields SHALL be stored on the `SpellInfo` struct. When `isChanneled` is truthy and `channelDuration > 0`, the spell SHALL enter the channeling execution path in `Cast()`.

#### Scenario: Channeled spell loaded from CSV
- **WHEN** `spells.csv` contains a row with `isChanneled=1, channelDuration=8000, tickInterval=1000`
- **THEN** the loaded `SpellInfo` has `IsChanneled=true`, `ChannelDuration=8000`, `TickInterval=1000`

#### Scenario: Non-channeled spell unaffected
- **WHEN** `spells.csv` contains a row without channel columns (existing 9-column format)
- **THEN** the loaded `SpellInfo` has `IsChanneled=false`, `ChannelDuration=0`, `TickInterval=0`

### Requirement: Channeled spell executes ticks via game loop
When a channeled spell completes casting (enters `StateChanneling`), the game loop SHALL start a channel ticker that fires `channel_tick` commands at the spell's `TickInterval`. Each tick SHALL execute all spell effect hit handlers on all resolved targets. After `ChannelDuration` milliseconds, a `channel_elapsed` command SHALL fire, transitioning to `StateFinished`.

#### Scenario: Blizzard channels for 8 seconds with 1-second ticks
- **WHEN** Blizzard (channelDuration=8000, tickInterval=1000) enters channeling
- **THEN** 8 tick commands fire at 1-second intervals, followed by a channel_elapsed command

#### Scenario: Channel stops when all targets dead
- **WHEN** all targets die during channeling
- **THEN** the next channel tick returns false, channel stops, `channel_stopped` event is emitted

### Requirement: Per-tick target re-resolution
On each channel tick, the game loop SHALL re-resolve targets using the spell's AoE targeting context (destination position + radius). The `SpellContext.Targets` SHALL be updated before each tick execution. This ensures units that move into/out of the AoE area are correctly included/excluded.

#### Scenario: Unit walks into Blizzard area during channel
- **WHEN** a unit outside the Blizzard radius moves into the radius between ticks
- **THEN** the next channel tick includes that unit in the target list and applies damage

#### Scenario: Unit walks out of Blizzard area during channel
- **WHEN** a unit inside the Blizzard radius moves out of the radius between ticks
- **THEN** the next channel tick excludes that unit from the target list

### Requirement: SpellContext supports dynamic target updates
The `SpellContext` SHALL expose a `SetTargets(targets []*unit.Unit)` method that replaces the internal target slice. This is used by the game loop before each channel tick.

#### Scenario: SetTargets updates targets for next tick
- **WHEN** `SetTargets(newTargets)` is called before `ExecuteChannelTick()`
- **THEN** `ExecuteChannelTick()` iterates over `newTargets`, not the original targets

### Requirement: Channel tick events include target list
Each `channel_tick` trace event SHALL include the list of hit target names and the tick number. This enables the frontend to display per-tick damage correctly.

#### Scenario: Channel tick event contains target info
- **WHEN** a channel tick fires and hits 2 targets
- **THEN** the trace event includes `targets: ["Target Dummy", "Elite"]` and `tick: 3`

### Requirement: Channel bar shows progress in frontend
When the server responds to a cast with `result: "channeling"`, the frontend SHALL show the cast bar counting down from `channelDuration` to 0 (fill bar shrinks right-to-left). On receiving `channel_elapsed` or `channel_stopped` SSE events, the frontend SHALL hide the bar and re-enable spell buttons.

#### Scenario: Channel bar displays during Blizzard
- **WHEN** Blizzard cast succeeds and enters channeling
- **THEN** the cast bar shows "暴风雪" with 8s countdown, fill bar shrinks from 100% to 0%

#### Scenario: Channel bar hides on completion
- **WHEN** `channel_elapsed` SSE event is received
- **THEN** the cast bar is hidden and spell buttons are re-enabled
