## ADDED Requirements

### Requirement: 3D character model
Each unit SHALL be represented as a 3D model consisting of a cylinder body and sphere head, grouped together. The model color SHALL differ by unit role (caster=blue, warrior=brown, target=red).

#### Scenario: Characters visible in scene
- **WHEN** the scene loads and units are fetched from /api/units
- **THEN** each unit SHALL appear as a distinct 3D model at its position

#### Scenario: Mage appearance
- **WHEN** the Mage unit is rendered
- **THEN** it SHALL have a blue (#4488ff) cylinder body and sphere head

### Requirement: Head nameplate
Each character SHALL display a nameplate above its head showing the unit name. The nameplate SHALL always face the camera (billboard behavior).

#### Scenario: Name visible
- **WHEN** a unit is in the scene
- **THEN** its name SHALL be displayed above its head in white text

### Requirement: HP/MP bars above character
Each character SHALL display HP and MP bars below its name. HP bar SHALL be green/yellow/red based on health percentage (>50% green, 25-50% yellow, <25% red). MP bar SHALL be blue.

#### Scenario: HP bar reflects damage
- **WHEN** a unit takes damage
- **THEN** its HP bar SHALL decrease proportionally after the next unit state update

#### Scenario: HP bar color changes
- **WHEN** a unit's health drops below 50%
- **THEN** the HP bar SHALL turn yellow, and below 25% SHALL turn red

### Requirement: Character position from API
Characters SHALL be positioned in the 3D scene based on their X/Y coordinates from the API (Z ignored or used for height offset).

#### Scenario: Units at correct positions
- **WHEN** units are loaded from /api/units
- **THEN** each unit SHALL be placed at its (X, 0, Y) position in the scene

### Requirement: Update character state
After each spell cast, character models SHALL be updated to reflect new HP, MP, alive status, and aura changes.

#### Scenario: Dead character visual
- **WHEN** a unit's alive field is false
- **THEN** the character model SHALL appear grayed out or fallen

#### Scenario: Aura glow
- **WHEN** a unit has active auras
- **THEN** a colored ring or glow effect SHALL appear at the unit's feet
