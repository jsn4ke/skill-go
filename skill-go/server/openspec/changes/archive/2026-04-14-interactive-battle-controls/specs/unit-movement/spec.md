## ADDED Requirements

### Requirement: Move caster on ground click
The system SHALL allow the user to click on the ground/floor to move the caster (Mage) to that world position. Movement SHALL be animated with smooth interpolation.

#### Scenario: Move caster by clicking ground
- **WHEN** user clicks on the ground plane at position (15, 0, 5)
- **THEN** the caster smoothly moves from current position to (15, 0, 5)
- **AND** the movement animation completes in approximately 1-2 seconds

#### Scenario: Movement updates server position
- **WHEN** caster movement begins
- **THEN** a `POST /api/units/move` request SHALL be sent with `{guid: casterGUID, x: targetX, z: targetZ}`
- **AND** the server response updates the unit's position

#### Scenario: Click on character does not trigger movement
- **WHEN** user clicks on a character (target selection)
- **THEN** the caster SHALL NOT move

### Requirement: Smooth movement animation
The system SHALL interpolate the caster's position toward the target position each frame. Movement speed SHALL be approximately 10 world units per second.

#### Scenario: Movement speed
- **WHEN** caster moves 20 units
- **THEN** movement SHALL complete in approximately 2 seconds

#### Scenario: Movement completion
- **WHEN** caster position is within 0.1 units of target
- **THEN** movement animation stops and position snaps to target
