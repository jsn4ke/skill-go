## Why

The 3D battle scene currently lacks user interaction — spells auto-target all predefined enemies, characters are fixed in place, and there's no way to manage targets. This makes it a passive demo rather than an interactive experience. Adding target selection, character movement, and dynamic target management transforms it into a functional battle simulator where users can actively control combat.

## What Changes

- **Target selection**: Click on 3D characters to select a target; show selection ring/highlight on the selected unit; pass the selected target's GUID in the cast request
- **Spell targeting**: Cast spells at the selected target instead of all enemies; support self-targeting for buffs/heals; show spell range indication
- **Ground-click movement**: Click on the ground/floor to move the caster to a new position; smooth interpolation animation between positions; update position in the API
- **Dynamic target management**: Add API endpoints for creating and removing target units; UI panel to spawn new enemies with configurable level/stats; right-click or delete button to remove targets
- **Target info panel**: Display selected target's detailed stats (HP, armor, resistances, etc.) in a side panel

## Capabilities

### New Capabilities

- `target-selection`: Click-to-select targeting with raycasting, selection ring VFX, target frame display, and passing targetID to cast API
- `unit-movement`: Ground-click movement with raycasting to floor plane, smooth position interpolation animation, and position sync with API
- `target-management`: API endpoints for adding/removing units, spawn panel UI for creating custom enemies, dynamic scene object creation/removal
- `target-info-panel`: Selected target detail display showing stats, resistances, level, HP bar, and combat info

### Modified Capabilities

- `battle-hud`: Add target frame to HUD, add spawn controls panel, update action bar spell buttons to reflect selected target state
- `animation-timeline`: Update VFX to use selected target instead of auto-targeting all units
- `battle-scene`: Add mouse event handlers for click-to-select and ground-click movement

## Impact

- **API layer** (`server/api/server.go`): New endpoints POST `/api/units/add`, DELETE `/api/units/{guid}`, POST `/api/units/move`; modify cast endpoint to use provided targetIDs
- **Web frontend** (all `web/*.js` files): New interaction systems for mouse picking, movement, target management
- **3D scene** (`scene.js`, `character.js`): Raycasting support, selection ring mesh, movement animation
- **VFX** (`vfx.js`): Selection ring, range indicator circle
- **HUD** (`app.js`, `style.css`, `index.html`): Target frame, spawn panel, updated action bar
- **Timeline** (`timeline.js`): Update to use selected target GUID
