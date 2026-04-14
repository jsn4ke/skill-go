## ADDED Requirements

### Requirement: VFX targets selected unit only
The animation timeline SHALL direct spell VFX (projectiles, damage numbers, hit flash) only toward the selected target, instead of all units.

#### Scenario: Projectile flies to selected target
- **WHEN** user casts Fireball with Target Dummy selected
- **THEN** the projectile animation SHALL travel from caster to Target Dummy only
- **AND** damage number SHALL appear only on Target Dummy

#### Scenario: No target selected — VFX on all default targets
- **WHEN** user casts Fireball with no target selected
- **THEN** the VFX SHALL apply to all default targets (current behavior)

### Requirement: Heal VFX targets self or selected friendly
Heal spell VFX SHALL target the selected unit. If no target is selected, heals SHALL target the caster.

#### Scenario: Heal with friendly target selected
- **WHEN** user casts Heal with Warrior selected
- **THEN** the heal beam SHALL go from caster to Warrior
- **AND** the heal number SHALL appear on Warrior

#### Scenario: Heal with no target selected
- **WHEN** user casts Heal with no target
- **THEN** the heal beam and number SHALL target the caster (self-heal)
