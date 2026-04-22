## ADDED Requirements

### Requirement: Toggle spell on/off switching
The system SHALL support toggle-type spells where casting the same spell ID toggles between active and inactive states. When a toggle spell is cast and the caster does NOT have the corresponding toggle aura, the system SHALL apply the aura. When the caster already has the toggle aura, the system SHALL remove it.

#### Scenario: Activate toggle (on)
- **WHEN** caster casts a toggle spell (IsToggle=true) and does not have the spell's toggle aura
- **THEN** the system SHALL apply the toggle aura to the caster
- **AND** the spell SHALL emit a `toggle.activated` trace event
- **AND** the spell SHALL proceed through Prepare→Cast→Finish normally

#### Scenario: Deactivate toggle (off)
- **WHEN** caster casts a toggle spell (IsToggle=true) and already has the spell's toggle aura
- **THEN** the system SHALL remove the toggle aura from the caster
- **AND** the spell SHALL emit a `toggle.deactivated` trace event
- **AND** the spell SHALL proceed through Prepare→Cast→Finish normally

### Requirement: ToggleGroup mutual exclusion
The system SHALL support named ToggleGroup strings on toggle spells. When a toggle spell is activated and the caster already has a toggle aura from the same ToggleGroup (but a different spell ID), the system SHALL remove the existing toggle aura before applying the new one.

#### Scenario: Switch between same-group toggles
- **WHEN** caster has "Battle Stance" aura (ToggleGroup="warrior_stance") active
- **AND** caster casts "Defensive Stance" (ToggleGroup="warrior_stance")
- **THEN** the system SHALL remove "Battle Stance" aura first
- **AND** then apply "Defensive Stance" aura
- **AND** emit `toggle.deactivated` for Battle Stance and `toggle.activated` for Defensive Stance

#### Scenario: Independent toggles do not interfere
- **WHEN** caster has "Stealth" aura (ToggleGroup="", independent) active
- **AND** caster casts "Battle Stance" (ToggleGroup="warrior_stance")
- **THEN** both auras SHALL coexist without interference

### Requirement: BreakOnDamage auto-deactivation
The system SHALL support a BreakOnDamage flag on toggle auras. When a unit with a BreakOnDamage toggle aura takes damage, the system SHALL automatically remove the toggle aura.

#### Scenario: Stealth broken by damage
- **WHEN** caster has "Stealth" toggle aura (BreakOnDamage=true) active
- **AND** caster takes damage from any source
- **THEN** the system SHALL remove the "Stealth" toggle aura
- **AND** emit `toggle.broken` trace event with reason="damage"

#### Scenario: Non-breakable toggle persists through damage
- **WHEN** caster has "Battle Stance" toggle aura (BreakOnDamage=false) active
- **AND** caster takes damage
- **THEN** the toggle aura SHALL remain active

### Requirement: Toggle aura is permanent until removed
A toggle aura SHALL have implicit permanent duration. It SHALL only be removed by: (1) casting the same toggle spell again (off), (2) activating a same-group toggle, (3) BreakOnDamage trigger, or (4) caster death.

#### Scenario: Toggle persists across other spell casts
- **WHEN** caster has "Battle Stance" toggle aura active
- **AND** caster casts Fireball
- **THEN** the "Battle Stance" toggle aura SHALL remain active

### Requirement: Toggle spell ignores GCD and cooldown
Toggle spells SHALL NOT trigger GCD or cooldown. They are instant state switches.

#### Scenario: Toggle does not block other spells
- **WHEN** caster activates a toggle spell
- **THEN** no GCD SHALL be triggered
- **AND** caster can immediately cast another spell
