## ADDED Requirements

### Requirement: Target frame in HUD
The system SHALL add a target frame element to the HUD overlay, positioned at the top-center of the screen. The target frame SHALL display the selected unit's name, level, and HP bar.

#### Scenario: Target frame layout
- **WHEN** the battle scene loads
- **THEN** a target frame element SHALL be visible at the top-center of the screen
- **AND** it SHALL display "No Target" when nothing is selected

### Requirement: Spawn controls panel
The system SHALL add a spawn controls panel to the HUD, positioned on the left side. The panel SHALL contain preset enemy spawn buttons and a level number input.

#### Scenario: Spawn panel contains controls
- **WHEN** the battle scene loads
- **THEN** the spawn panel SHALL contain: a level number input (default 60), "Target Dummy" spawn button, "Elite" spawn button, "Boss" spawn button, "Remove Target" button

#### Scenario: Spawn panel is collapsible
- **WHEN** user clicks the spawn panel toggle button
- **THEN** the spawn panel expands/collapses to save screen space

### Requirement: Action bar shows selected target context
The system SHALL visually indicate in the action bar whether a target is currently selected, and spell buttons SHALL use the selected target for casting.

#### Scenario: Spell cast uses selected target
- **WHEN** a target is selected and user clicks a spell button
- **THEN** the spell is cast at the selected target's GUID

#### Scenario: Spell cast without target
- **WHEN** no target is selected and user clicks a spell button
- **THEN** the spell is cast with empty targetIDs (default targeting)
