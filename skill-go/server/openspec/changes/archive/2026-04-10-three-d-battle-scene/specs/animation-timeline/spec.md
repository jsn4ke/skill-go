## ADDED Requirements

### Requirement: Event-to-animation mapping
The timeline SHALL map backend trace events to corresponding 3D animations. Each trace event type SHALL trigger a specific visual action.

#### Scenario: spell.prepare triggers cast animation
- **WHEN** a trace event with span="spell" and event="prepare" is received
- **THEN** the caster SHALL display a cast preparation visual effect (glowing hands/particles gathering)

#### Scenario: effect_launch triggers projectile
- **WHEN** a trace event with span="effect" (launch or hit) is received with damage data
- **THEN** a projectile SHALL be launched from caster to target

#### Scenario: combat hit triggers damage number
- **WHEN** a trace event with span="combat" and event containing hit result is received
- **THEN** the appropriate floating damage number SHALL appear at the target

#### Scenario: cooldown.add triggers CD indicator
- **WHEN** a trace event with span="cooldown" is received
- **THEN** the corresponding spell button in the HUD SHALL start its cooldown animation

### Requirement: Animation sequencing
Multiple animations from a single cast SHALL play in correct temporal order: cast preparation → projectile launch → hit impact → damage number.

#### Scenario: Correct order
- **WHEN** Fireball is cast and trace events are processed
- **THEN** cast glow SHALL appear first, projectile SHALL launch second, hit flash and damage number SHALL appear last

### Requirement: Animation cleanup
Completed animations SHALL be removed from the scene to prevent memory leaks.

#### Scenario: Projectile removed after impact
- **WHEN** a projectile reaches its target
- **THEN** the projectile mesh and its particles SHALL be removed from the scene

#### Scenario: Floating number removed after fade
- **WHEN** a floating damage number finishes its 1.5s animation
- **THEN** the number element SHALL be removed from the DOM
