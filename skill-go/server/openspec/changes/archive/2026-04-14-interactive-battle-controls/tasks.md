## 1. API: Unit Management Endpoints

- [x] 1.1 Add `POST /api/units/add` endpoint — accept `{name, level}`, create unit with level-scaled default stats, return updated unit list
- [x] 1.2 Add `DELETE /api/units/{guid}` endpoint — remove unit (protect caster GUID 1), return remaining unit list
- [x] 1.3 Add `POST /api/units/move` endpoint — accept `{guid, x, z}`, update unit position, return updated unit list
- [x] 1.4 Add API tests for new endpoints (add, delete, move, delete caster protection)

## 2. Scene: Mouse Interaction & Raycasting

- [x] 2.1 Add `THREE.Raycaster` setup in `scene.js` — export raycast function that tests against character meshes
- [x] 2.2 Export ground plane reference from `scene.js` for ground-click detection
- [x] 2.3 Add mouse click handler in `app.js` — classify clicks as character-click or ground-click using raycaster priority

## 3. Target Selection System

- [x] 3.1 Add `createSelectionRing(unitGroup)` in `vfx.js` — rotating ring at character feet, team-colored (green/red)
- [x] 3.2 Add `updateSelectionRing()` in `vfx.js` — move ring to follow selected target position
- [x] 3.3 Add `removeSelectionRing()` in `vfx.js` — clean up ring mesh
- [x] 3.4 Add selection state management in `app.js` — `selectedTarget` variable, `selectTarget(guid)` and `deselectTarget()` functions
- [x] 3.5 Connect click handler to selection — character click calls `selectTarget()`, ground click calls `deselectTarget()`

## 4. Unit Movement System

- [x] 4.1 Add movement state to character `userData` — `targetPosition`, `isMoving` flag
- [x] 4.2 Add `moveUnit(group, x, z)` function in `character.js` — set target position to trigger movement
- [x] 4.3 Add movement interpolation in `scene.js` animation loop — lerp position at ~10 units/sec, snap when close
- [x] 4.4 Connect ground click to movement — call `POST /api/units/move` then `moveUnit()` on caster

## 5. Target Info Panel (HUD)

- [x] 5.1 Add target frame HTML in `index.html` — top-center panel with name, level, HP bar, stats section
- [x] 5.2 Add target frame CSS in `style.css` — styling for HP bar, stat labels, "No Target" placeholder
- [x] 5.3 Add `updateTargetFrame(unitData)` in `app.js` — populate target frame with selected unit data
- [x] 5.4 Clear target frame on deselect — show "No Target" when no unit selected
- [x] 5.5 Update target frame after cast response — refresh HP and stats from API response

## 6. Spawn Controls Panel (HUD)

- [x] 6.1 Add spawn panel HTML in `index.html` — left-side collapsible panel with level input, preset buttons, remove button
- [x] 6.2 Add spawn panel CSS in `style.css` — panel styling, button layout, collapse animation
- [x] 6.3 Implement `spawnUnit(name, level)` in `app.js` — call `POST /api/units/add`, create 3D character, update unit list
- [x] 6.4 Implement `removeSelectedTarget()` in `app.js` — call `DELETE /api/units/{guid}`, remove 3D character, deselect
- [x] 6.5 Handle unit creation in `character.js` — random spawn position (X: 30-50, Z: -5 to 5)

## 7. Cast Targeting Integration

- [x] 7.1 Update `castSpell()` in `app.js` — pass `selectedTarget` GUID in `targetIDs` array
- [x] 7.2 Update `processEvents()` in `timeline.js` — direct VFX to selected target only (instead of all targets)
- [x] 7.3 Update heal VFX — target selected friendly unit, or self if no target selected
- [x] 7.4 Update stats counting — count based on events from actual target, not all units

## 8. Dynamic Scene Management

- [x] 8.1 Add `addCharacterToScene(unitData)` in `app.js` — create 3D character from API data, add to scene and tracking array
- [x] 8.2 Add `removeCharacterFromScene(guid)` in `app.js` — remove 3D character, clean up meshes and aura rings
- [x] 8.3 Reconcile unit list after API responses — add new units, remove deleted units, update existing
