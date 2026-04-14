## ADDED Requirements

### Requirement: Click on ground moves selected non-caster target

The system SHALL allow the user to left-click on the ground to move the currently selected target (if it is not the caster) to that position. This movement does NOT interrupt the caster's channeling.

#### Scenario: Click ground to move Target Dummy
- **WHEN** Target Dummy is selected (selectedTargetGUID = dummy's GUID, NOT caster's GUID)
- **AND** user clicks on ground at position (25, 0, 5)
- **THEN** the Target Dummy smoothly moves to (25, 0, 5)
- **AND** the Mage's channel (if active) continues uninterrupted

#### Scenario: Click ground with caster selected does nothing
- **WHEN** selectedTargetGUID === activeCasterGUID (caster is selected)
- **AND** user clicks on ground
- **THEN** no movement occurs

#### Scenario: Click ground with no target selected does nothing
- **WHEN** selectedTargetGUID is null
- **AND** user clicks on ground
- **THEN** no movement occurs

#### Scenario: Click on character selects but does not move that character
- **WHEN** user clicks on Target Dummy
- **THEN** Target Dummy becomes selected (selection ring appears)
- **AND** no movement is triggered for any unit

### Requirement: Click-to-move target sync to server

The system SHALL send position updates to the backend for the moved target.

#### Scenario: Target position synced
- **WHEN** mouse click causes Target Dummy to move
- **THEN** a `POST /api/units/move` request is sent with `{guid: targetGUID, x, z}`
- **AND** server updates the target's position

### Requirement: Target movement is smooth

The system SHALL interpolate the selected target's position toward the click destination using the existing movement system.

#### Scenario: Smooth movement to click destination
- **WHEN** Target Dummy receives a move command to (25, 0, 5)
- **THEN** the dummy interpolates from current position to (25, 0, 5)
- **AND** movement speed is 10 units/s (same as caster)
- **AND** movement completes when within 0.1 units of target