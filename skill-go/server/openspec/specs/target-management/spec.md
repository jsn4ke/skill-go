## ADDED Requirements

### Requirement: Add new target unit via API
The system SHALL provide a `POST /api/units/add` endpoint that creates a new unit and adds it to the game state. The request body SHALL accept `name` and `level` fields.

#### Scenario: Add a new Target Dummy
- **WHEN** `POST /api/units/add` is called with `{name: "Target Dummy", level: 60}`
- **THEN** a new unit is created with level-appropriate stats
- **AND** the response returns the full updated unit list including the new unit
- **AND** the new unit has a unique GUID

#### Scenario: Add unit with missing fields
- **WHEN** `POST /api/units/add` is called with `{name: ""}`
- **THEN** the system SHALL use default name "Unknown" and default level 60

### Requirement: Remove target unit via API
The system SHALL provide a `DELETE /api/units/{guid}` endpoint that removes a unit from the game state. The caster (GUID 1) SHALL NOT be removable.

#### Scenario: Remove a target
- **WHEN** `DELETE /api/units/3` is called
- **THEN** the unit with GUID 3 is removed from the game state
- **AND** the response returns the remaining unit list
- **AND** if the removed unit was the selected target, selection is cleared

#### Scenario: Attempt to remove caster
- **WHEN** `DELETE /api/units/1` is called (caster GUID)
- **THEN** the request SHALL return HTTP 400 with error "cannot remove caster"

### Requirement: Spawn panel UI
The system SHALL provide a spawn panel in the HUD with preset enemy templates and a level selector. Clicking a spawn button SHALL call the add unit API and create the 3D character in the scene.

#### Scenario: Spawn a Target Dummy from panel
- **WHEN** user clicks "Target Dummy" button in spawn panel with level set to 65
- **THEN** a new Target Dummy character appears in the scene at a random position
- **AND** the unit list updates to include the new unit

#### Scenario: Remove selected target from panel
- **WHEN** user clicks "Remove Target" button while Target Dummy is selected
- **THEN** the selected unit is removed from the scene and game state
