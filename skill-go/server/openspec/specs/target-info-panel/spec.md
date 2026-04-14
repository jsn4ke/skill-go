## ADDED Requirements

### Requirement: Display selected target info panel
The system SHALL display a target info panel in the HUD showing the selected target's name, level, HP bar, and key combat stats. The panel SHALL update whenever a new target is selected or the selected target's state changes.

#### Scenario: Target selected — show info
- **WHEN** user selects the Target Dummy
- **THEN** the target frame shows: name "Target Dummy", level 63, HP bar, armor value, resistances

#### Scenario: Target deselected — show empty
- **WHEN** no target is selected
- **THEN** the target frame shows "No Target" placeholder text

#### Scenario: Target HP updates after damage
- **WHEN** the selected target takes damage from a spell
- **THEN** the target frame HP bar updates to reflect the new HP value

### Requirement: Target frame displays key stats
The target info panel SHALL display: name, level, HP/MaxHP bar, armor, and school resistances.

#### Scenario: Full stats display
- **WHEN** Target Dummy (level 63, 8657 HP, 5000 armor, 100 Fire resist) is selected
- **THEN** the panel shows name "Target Dummy", level "63", HP bar at 57.7%, armor "5000", Fire resistance "100"
