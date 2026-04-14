## Context

The 3D battle scene is a Three.js-based web frontend connected to a Go spell engine API. Currently:
- Spells auto-target all predefined enemies (Mage casts on both Warrior and Target Dummy)
- Characters are static at fixed positions
- No mouse interaction beyond spell button clicks
- Only 3 units exist: Mage (caster), Warrior, Target Dummy

The API already supports `targetIDs` in the cast request but the frontend always passes an empty array. Units already have `position: {X, Y, Z}` fields. The architecture is modular: `scene.js` (3D setup), `character.js` (unit rendering), `vfx.js` (visual effects), `timeline.js` (event→VFX mapping), `app.js` (orchestration).

## Goals / Non-Goals

**Goals:**
- Click a 3D character to select it as the spell target
- Click the ground to move the caster
- Add/remove enemy targets dynamically
- Show selected target's info (stats, HP, resistances)
- Pass selected target GUID to the cast API

**Non-Goals:**
- Camera control (orbit/zoom) — fixed orthographic camera is intentional
- Pathfinding or collision avoidance — direct line movement only
- Multi-select or area-targeting
- Real-time position sync or networked multiplayer
- Drag-and-drop spell casting
- Keyboard shortcuts for abilities

## Decisions

### D1: Raycasting with Three.js Raycaster for click detection

**Choice**: Use `THREE.Raycaster` against character meshes and a ground plane.

**Rationale**: Three.js provides built-in raycasting that works with OrthographicCamera. No additional dependency needed. Characters are `THREE.Group` objects containing meshes — raycasting against children is well-supported.

**Alternative considered**: CSS-based click regions overlaying 3D positions. Rejected because it can't handle ground clicks and doesn't scale with camera changes.

### D2: Selection ring mesh for target feedback

**Choice**: A `THREE.RingGeometry` placed at the selected character's feet, rotating slowly.

**Rationale**: Consistent with existing VFX style (rings, circles). Easy to animate. Placed at Y=0.05 to sit on the ground plane. Color matches team (green=friendly, red=enemy).

**Alternative considered**: Outline effect (post-processing). Rejected — too heavy for a demo, requires additional Three.js addons.

### D3: Ground plane as invisible click target

**Choice**: Use the existing semi-transparent floor plane for ground raycasting. On click (not on a character), project the click position to Y=0 and move the caster there.

**Rationale**: The floor plane already exists. Adding an invisible larger plane is simpler than extending the visible one.

### D4: Smooth movement with lerp in the animation loop

**Choice**: Store a `targetPosition` on the character's userData. In the animation loop (scene.js), lerp `position.x` and `position.z` toward the target at a fixed speed.

**Rationale**: Simple, deterministic, no physics needed. Speed ~10 units/second feels responsive. Movement completes when distance < 0.1.

**Alternative considered**: Tween library (GSAP). Rejected — unnecessary dependency for simple linear interpolation.

### D5: New API endpoints for unit management

**Choice**: Add `POST /api/units/add` (body: name, level) and `DELETE /api/units/{guid}`.

**Rationale**: RESTful and simple. The add endpoint creates a unit with default stats scaled by level. The delete endpoint removes from the game state and returns the updated unit list. Reuse existing `UnitJSON` format.

**Alternative considered**: WebSocket for real-time sync. Rejected — overkill for single-user local demo. HTTP request/response is sufficient.

### D6: Movement via `POST /api/units/move`

**Choice**: Send `{guid, x, z}` to update unit position server-side. Response returns updated units.

**Rationale**: Keeps server authoritative. Position changes affect combat calculations (e.g., range checks in the future). Simple JSON body.

### D7: Spawn panel with preset templates

**Choice**: A collapsible panel with buttons for preset enemy types: "Target Dummy" (any level), "Elite" (higher stats), "Boss" (very high stats). Level input via number field.

**Rationale**: Avoids overwhelming users with stat configuration. Presets provide sensible defaults. Level slider is the main customization lever.

## Risks / Trade-offs

- **[Raycasting accuracy]** OrthographicCamera raycasting may not perfectly align with CSS2D labels → Use character body mesh (cylinder) as the raycast target, which is the largest geometry
- **[Movement during combat]** Moving the caster mid-combat could cause visual inconsistencies with in-flight projectiles → Ignore for now; projectiles track current position
- **[Unit GUID conflicts]** Adding/removing units while casting could cause stale references → Each cast response returns fresh unit list; frontend reconciles after each cast
- **[Ground click vs character click ambiguity]** Clicking near a character's base might hit the ground plane instead → Prioritize character raycasting; only fall through to ground click if no character hit
