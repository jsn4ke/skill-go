## ADDED Requirements

### Requirement: periodic-damage-aura SHALL deal damage at tick intervals via the event loop

When an aura has effects with `PeriodicTimer > 0`, the `handleAuraUpdate` logic SHALL calculate the number of elapsed ticks since the aura was applied and deal damage for each new tick.

#### Scenario: Fireball DoT ticks every 2 seconds
- **WHEN** Fireball's apply_aura effect (value=21, tickInterval=2000, duration=8000) is applied to a target
- **THEN** the target SHALL take 21 damage at t=2s, t=4s, t=6s (3 total ticks before expiration at 8s)
- **AND** each tick SHALL emit a trace event with type `"periodic_damage"`

#### Scenario: No periodic damage on non-periodic aura
- **WHEN** an aura has `PeriodicTimer == 0` on all its effects
- **THEN** no periodic damage SHALL be dealt
- **AND** the aura SHALL only expire when its Duration elapses

---

### Requirement: periodic-damage-aura SHALL track applied ticks per effect

Each `AuraEffect` SHALL maintain an `AppliedTicks` counter to avoid dealing damage for already-processed ticks.

#### Scenario: Tick counter prevents double-damage
- **WHEN** `handleAuraUpdate` runs multiple times within a single tick interval
- **THEN** no damage SHALL be dealt on subsequent calls until the next tick boundary
- **AND** `AppliedTicks` SHALL only increment when a new tick boundary is crossed

#### Scenario: Aura expiration resets state
- **WHEN** a periodic aura expires and is re-applied
- **THEN** `AppliedTicks` SHALL be reset to 0 on the new aura's effects

---

### Requirement: periodic-damage-aura SHALL use SpellEffectInfo.PeriodicTickInterval

The `SpellEffectInfo` struct SHALL have a new `PeriodicTickInterval int32` field. When the aura handler creates an `AuraEffect` from a spell effect with `PeriodicTickInterval > 0`, the aura effect's `PeriodicTimer` SHALL be set to this value.

#### Scenario: Fireball spell effect to aura effect mapping
- **WHEN** Fireball's effect[1] has `PeriodicTickInterval: 2000` and `BasePoints: 21`
- **THEN** the resulting `AuraEffect` SHALL have `PeriodicTimer: 2000` and `BaseAmount: 21`
