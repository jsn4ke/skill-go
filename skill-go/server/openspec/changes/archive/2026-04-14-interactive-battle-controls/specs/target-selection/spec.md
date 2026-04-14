## ADDED Requirements

### Requirement: Click-to-select a character
The system SHALL allow the user to click on any 3D character in the scene to select it as the active target. Selection SHALL be indicated by a visible selection ring at the character's base.

#### Scenario: Select an enemy target
- **WHEN** user clicks on the Target Dummy character mesh
- **THEN** a rotating ring appears at the Target Dummy's feet
- **AND** the target frame in the HUD shows the Target Dummy's name and HP

#### Scenario: Switch target by clicking another character
- **WHEN** user clicks on the Warrior while Target Dummy is selected
- **THEN** the selection ring moves from Target Dummy to Warrior
- **AND** the target frame updates to show Warrior's info

#### Scenario: Click empty space deselects target
- **WHEN** user clicks on the ground (not on any character)
- **THEN** the selection ring is removed
- **AND** the target frame shows "No Target"

### Requirement: Selection ring visual feedback
The system SHALL render a ring geometry at the selected character's base position. The ring SHALL rotate continuously and be colored based on the target's team alignment (green for friendly, red for enemy).

#### Scenario: Enemy selection ring color
- **WHEN** an enemy unit (teamId != caster teamId) is selected
- **THEN** the selection ring SHALL be red (0xff3333)

#### Scenario: Friendly selection ring color
- **WHEN** a friendly unit is selected
- **THEN** the selection ring SHALL be green (0x33ff33)

### Requirement: Pass selected target to cast API
The system SHALL include the selected target's GUID in the spell cast request when a target is selected.

#### Scenario: Cast spell with selected target
- **WHEN** user has Target Dummy selected and clicks Fireball
- **THEN** the cast request SHALL include `targetIDs: [targetDummyGUID]`

#### Scenario: Cast spell with no target selected
- **WHEN** user has no target selected and clicks Fireball
- **THEN** the cast request SHALL include `targetIDs: []` (fallback to default targets)
