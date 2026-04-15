## ADDED Requirements

### Requirement: Frontend SHALL determine channel behavior from spell data, not spell ID

The frontend `timeline.js` SHALL read spell channel properties from the spell data map (`window.__spellMap`) to determine whether to skip projectile animation and show immediate damage. It SHALL NOT hardcode spell IDs.

#### Scenario: Channeled AoE spell skips projectile
- **WHEN** a `school_damage_hit` event arrives and the spell data indicates `isChanneled=true` and the effect has `radius > 0`
- **THEN** `spawnProjectile()` SHALL be skipped and damage number SHALL display immediately (delay=0)

#### Scenario: Non-channel spell shows projectile
- **WHEN** a `school_damage_hit` event arrives and the spell data indicates `isChanneled=false`
- **THEN** `spawnProjectile()` SHALL be called and damage number SHALL display with normal delay

#### Scenario: spellMap has no entry for spell
- **WHEN** `spellMap_entry(spellID)` returns null
- **THEN** default to showing projectile with normal delay (existing behavior)
