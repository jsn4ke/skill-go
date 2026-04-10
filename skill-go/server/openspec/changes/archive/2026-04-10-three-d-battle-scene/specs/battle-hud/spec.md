## ADDED Requirements

### Requirement: Action bar
The HUD SHALL display an action bar at the bottom center of the screen containing a button for each available spell. Each button SHALL show the spell name and school color indicator.

#### Scenario: Spell buttons displayed
- **WHEN** the page loads and spells are fetched from /api/spells
- **THEN** one button per spell SHALL appear in the action bar with the spell name

#### Scenario: Cast on click
- **WHEN** the user clicks a spell button
- **THEN** the system SHALL call POST /api/cast with that spell's ID

### Requirement: Cooldown overlay
When a spell is on cooldown, its action bar button SHALL display a sweeping cooldown overlay (clockwise from top, like WoW) that reveals the button as the cooldown expires.

#### Scenario: CD overlay after cast
- **WHEN** a spell with 6s cooldown is cast
- **THEN** the button SHALL show a dark overlay that shrinks clockwise over 6 seconds

#### Scenario: Button disabled during CD
- **WHEN** a spell is on cooldown
- **THEN** the button SHALL be non-clickable until the cooldown expires

### Requirement: Statistics overlay
A semi-transparent statistics panel SHALL be displayed in the top-right corner showing total casts, total damage, crits, misses, and crit rate.

#### Scenario: Stats update after cast
- **WHEN** a spell cast completes
- **THEN** the statistics panel SHALL update with cumulative values

### Requirement: Reset button
A "Reset" button SHALL be available in the HUD to reset the game session.

#### Scenario: Reset restores scene
- **WHEN** the user clicks Reset
- **THEN** all character HP/MP bars SHALL be restored to full, aura effects SHALL be removed, and cooldown overlays SHALL be cleared

### Requirement: Fullscreen layout
The 3D canvas SHALL fill the entire viewport. The HUD elements SHALL be overlaid on top using absolute positioning with pointer-events only on interactive elements.

#### Scenario: Canvas fills viewport
- **WHEN** the page loads
- **THEN** no scrollbars SHALL appear and the 3D scene SHALL fill the entire window

#### Scenario: HUD does not block scene interaction
- **WHEN** the user clicks on an area of the HUD with no buttons
- **THEN** the click SHALL pass through to the 3D scene (or be ignored)
