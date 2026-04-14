## ADDED Requirements

### Requirement: WASD keyboard input moves the caster

The system SHALL allow the user to press WASD keys to move the active caster (activeCasterGUID) in world space. Movement direction maps directly to world axes: W = +Z, S = -Z, A = -X, D = +X.

#### Scenario: Press W to move caster forward
- **WHEN** user presses W key while Mage is caster
- **THEN** the Mage moves in +Z direction at MOVE_SPEED (10 units/s)
- **AND** movement persists as long as W is held

#### Scenario: Press A to move caster left
- **WHEN** user presses A key
- **THEN** the Mage moves in -X direction at 10 units/s

#### Scenario: Multiple keys are not combined
- **WHEN** user presses W and D simultaneously
- **THEN** the caster moves in +Z only (W takes priority, or first key wins)
- **OR** the caster does not move at all (no diagonal movement)

#### Scenario: Release key stops movement
- **WHEN** user releases W
- **THEN** the Mage stops moving in +Z direction

### Requirement: WASD movement interrupts channeling

The system SHALL interrupt any active channel when the caster begins WASD movement.

#### Scenario: Move during Blizzard channel cancels spell
- **WHEN** Mage is channeling Blizzard and user presses W
- **THEN** the channel is cancelled immediately
- **AND** the channel bar disappears
- **AND** no further damage ticks occur

#### Scenario: Move during cast bar does not cancel (non-channel)
- **WHEN** Mage has a cast-time spell on cast bar (not channeling) and presses W
- **THEN** the cast bar continues
- **AND** casting is NOT interrupted

### Requirement: WASD movement is smooth and frame-rate independent

The system SHALL interpolate caster movement based on elapsed time (dt), not frame count.

#### Scenario: Frame rate independence
- **WHEN** dt = 16ms (60 FPS) and W is held
- **THEN** caster moves 10 * 0.016 = 0.16 units per frame
- **AND** at 30 FPS (dt=33ms) caster moves 10 * 0.033 = 0.33 units per frame
- **AND** both result in approximately 10 units/second average

### Requirement: WASD movement syncs to server

The system SHALL send position updates to the backend when the caster moves.

#### Scenario: Caster position synced to server
- **WHEN** WASD movement causes caster position to change
- **THEN** a `POST /api/units/move` request is sent with the new x,z position
- **AND** the server updates the caster's position in its state