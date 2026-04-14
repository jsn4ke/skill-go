## ADDED Requirements

### Requirement: fix-fireball-dot SHALL ensure Fireball DoT deals periodic damage to the target

The periodic damage aura applied by Fireball's effect[1] SHALL deal 21 fire damage every 2 seconds for 8 seconds to the target unit.

#### Scenario: DoT damage applied after cast completes
- **WHEN** Fireball cast completes and the apply_aura effect fires
- **THEN** the target SHALL receive 21 damage at t=2s, t=4s, t=6s after aura application
- **AND** each tick SHALL emit a `periodic_damage` trace event
