## ADDED Requirements

### Requirement: Projectile effect
When a spell is cast, a projectile SHALL fly from the caster to the target over 600ms. The projectile appearance SHALL vary by spell school (Fire=orange sphere with fire particles, Frost=blue sphere with ice crystals, Shadow=purple sphere, etc.).

#### Scenario: Fireball projectile
- **WHEN** a Fire spell (school Fire) is cast
- **THEN** an orange glowing sphere with fire particle trail SHALL fly from caster to target

#### Scenario: Melee no projectile
- **WHEN** a Physical school spell is cast
- **THEN** no projectile SHALL be spawned; instead a melee swing arc effect SHALL appear at the target

### Requirement: Floating damage numbers
When a hit occurs, a floating number SHALL appear above the target and drift upward over 1.5 seconds, then fade out. The number format SHALL vary by hit result.

#### Scenario: Normal hit number
- **WHEN** a spell hits for normal damage
- **THEN** a white number showing the damage value SHALL float upward from the target

#### Scenario: Critical hit number
- **WHEN** a spell critically hits
- **THEN** a large red number (1.5x normal size) with "CRIT!" text SHALL appear and float upward

#### Scenario: Miss display
- **WHEN** a spell misses
- **THEN** gray "MISS" text SHALL float horizontally across the target

#### Scenario: Dodge display
- **WHEN** a spell is dodged
- **THEN** orange "DODGE" text SHALL appear at the target

#### Scenario: Heal number
- **WHEN** a heal spell heals a target
- **THEN** a green "+" number SHALL float upward from the target

### Requirement: Hit flash effect
When a target is hit, it SHALL briefly flash white/red to indicate impact.

#### Scenario: Hit flash
- **WHEN** a spell hits a target
- **THEN** the target model SHALL flash red for 200ms then return to normal

### Requirement: Healing visual
When a heal spell is cast, a green light beam or rising particle effect SHALL appear between caster and target.

#### Scenario: Heal beam
- **WHEN** a Holy school heal spell is cast
- **THEN** a green beam of light SHALL connect caster to target briefly

### Requirement: Aura visual
When an aura is applied to a target, a glowing ring SHALL appear at the target's feet for the aura duration.

#### Scenario: Buff ring
- **WHEN** a buff aura is applied
- **THEN** a green/arcane colored ring SHALL appear at the target's base
