## ADDED Requirements

### Requirement: Mouse click event handling
The battle scene SHALL handle mouse click events on the 3D canvas. Clicks SHALL be classified as character clicks or ground clicks based on raycaster intersection results.

#### Scenario: Character click detection
- **WHEN** user clicks on the canvas and the raycaster intersects a character mesh
- **THEN** the click SHALL be treated as a target selection action
- **AND** the character's GUID SHALL be passed to the selection handler

#### Scenario: Ground click detection
- **WHEN** user clicks on the canvas and the raycaster does NOT intersect any character but DOES intersect the ground plane
- **THEN** the click SHALL be treated as a movement action
- **AND** the ground intersection point (X, Z) SHALL be passed to the movement handler

### Requirement: Raycaster setup
The system SHALL use a `THREE.Raycaster` configured for the OrthographicCamera to detect mouse interactions. The raycaster SHALL test against all character body meshes first, then the ground plane.

#### Scenario: Raycaster uses correct camera
- **WHEN** a click event fires
- **THEN** the raycaster SHALL use the scene's OrthographicCamera for coordinate conversion

#### Scenario: Character meshes are raycast targets
- **WHEN** raycaster intersects a character's body (cylinder) or head (sphere) mesh
- **THEN** the parent character Group SHALL be identified as the clicked character
