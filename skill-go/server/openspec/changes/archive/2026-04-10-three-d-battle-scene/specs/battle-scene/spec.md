## ADDED Requirements

### Requirement: Three.js scene initialization
The system SHALL initialize a Three.js scene with WebGL renderer filling the entire viewport. The scene SHALL use a fixed orthographic camera at 45° elevation and 30° azimuth, positioned to view the combat area.

#### Scenario: Scene renders on page load
- **WHEN** the page loads
- **THEN** a full-viewport WebGL canvas SHALL be created with a dark background (#1a1a2e)

#### Scenario: Camera view
- **WHEN** the scene is initialized
- **THEN** the orthographic camera SHALL show all combat units within the viewport with no user interaction required

### Requirement: Ground rendering
The scene SHALL render a ground plane with a grid pattern to provide spatial reference.

#### Scenario: Grid visible
- **WHEN** the scene is rendered
- **THEN** a dark grid pattern SHALL be visible on the ground plane

### Requirement: Lighting
The scene SHALL include ambient light and directional light to illuminate 3D objects and cast shadows.

#### Scenario: Objects are lit
- **WHEN** 3D objects are placed in the scene
- **THEN** they SHALL be visible with shading, not flat-colored

### Requirement: Render loop
The system SHALL run a continuous render loop (requestAnimationFrame) that updates the scene and CSS2D overlay each frame.

#### Scenario: Animation updates
- **WHEN** active animations exist (projectiles, floating numbers, particles)
- **THEN** the render loop SHALL update their positions and opacity each frame

#### Scenario: Idle when no animations
- **WHEN** no animations are active
- **THEN** the render loop SHALL continue running but the scene SHALL appear static
