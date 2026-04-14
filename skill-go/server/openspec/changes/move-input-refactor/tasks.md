## 1. WASD Input System

- [x] 1.1 Add WASD key state tracking in `web/app.js`
  - File: `web/app.js` — add `const wasdKeys = { w: false, a: false, s: false, d: false };`
  - Add `document.addEventListener('keydown', ...)` to set `wasdKeys[e.key.toLowerCase()] = true`
  - Add `document.addEventListener('keyup', ...)` to set `wasdKeys[e.key.toLowerCase()] = false`

- [x] 1.2 Add WASD movement processing in animation loop
  - File: `web/app.js` — in the `addUpdatable({ update(dt) { ... } })` block:
    - Check each WASD key and calculate direction vector (dx, dz)
    - Normalize direction (handle W+A or W+D as diagonal)
    - Call `moveCasterDirection(dx, dz)` when any key is pressed

- [x] 1.3 Implement `moveCasterDirection(dx, dz)` function
  - File: `web/app.js` — calculate new position: `x + dx * MOVE_SPEED * dt`, `z + dz * MOVE_SPEED * dt`
  - Call `moveUnit(casterGroup, newX, newZ)` from `character.js`
  - Send `POST /api/units/move` with `{ guid: activeCasterGUID, x: newX, z: newZ }`

- [x] 1.4 WASD movement interrupts channeling
  - File: `web/app.js` — in `moveCasterDirection`:
    - If `castingState === 'channeling'`, call `apiPost('/api/cast/cancel', null)` first
    - Then proceed with movement

## 2. Mouse Click Refactor (Remove Caster Ground Move)

- [x] 2.1 Remove caster ground click movement from `onCanvasClick`
  - File: `web/app.js` — in `onCanvasClick`, remove the case where clicking ground calls `moveUnit(caster)` or similar
  - Keep raycastGround() and character selection logic intact
  - Clicking ground now does nothing (unless targeting mode)

- [x] 2.2 Add target ground click movement for selected non-caster
  - File: `web/app.js` — in `onCanvasClick`, after `!hitCharacter` branch:
    - Check `selectedTargetGUID` is not null and not equal to `activeCasterGUID`
    - Call `raycastGround()` to get world position
    - Find target character group: `const targetGroup = characters.find(c => c.userData.guid == selectedTargetGUID)`
    - Call `moveUnit(targetGroup, hit.x, hit.z)` from `character.js`
    - Send `POST /api/units/move` with `{ guid: selectedTargetGUID, x: hit.x, z: hit.z }`

## 3. Move Sync to Server (reuse existing)

- [x] 3.1 Verify `/api/units/move` works with arbitrary GUIDs
  - File: `api/server.go` — `handleMoveUnit` already supports any GUID, no changes needed

## 4. Animation Loop Multi-Unit Support

- [x] 4.1 Ensure animation loop handles multiple moving units
  - File: `web/character.js` — `updateUnitMovement` already handles individual `isMoving` flags per group
  - WASD moves caster via `moveUnit(casterGroup, ...)` which sets `isMoving = true`
  - Mouse click moves target via `moveUnit(targetGroup, ...)` which sets `isMoving = true`
  - Each call sets its own `targetPosition` — no interference
  - No changes needed — this works out of the box

## 5. Remove Old Caster Movement

- [x] 5.1 Remove any existing click-to-move-caster logic in `onCanvasClick`
  - File: `web/app.js` — the ground click handler no longer moves caster
  - Old behavior was: click ground → moveUnit(caster) — now removed

## 6. Verification

- [x] 6.1 Test WASD moves caster
  - Open browser, press W — caster should move in +Z direction
  - Press S — caster moves in -Z
  - Press A/D — caster moves in -X/+X

- [x] 6.2 Test WASD interrupts channeling
  - Cast Blizzard (channeling)
  - Press W during channel — channel should cancel, bar disappears

- [x] 6.3 Test click ground moves selected target (not caster)
  - Select Target Dummy
  - Click ground near dummy — dummy should move, caster stays still

- [x] 6.4 Test click ground does not interrupt channel
  - Cast Blizzard, select Target Dummy, click ground — channel continues

- [x] 6.5 Test click ground with caster selected does nothing
  - Select Mage (caster), click ground — no movement

- [x] 6.6 Test server position sync
  - Move units, query `/api/units` — positions match visual position