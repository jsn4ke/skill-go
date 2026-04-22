## ADDED Requirements

### Requirement: Toggle status indicator in action bar
The system SHALL display toggle state on spell buttons in the action bar. When a toggle spell's aura is active on the caster, the corresponding action bar button SHALL show a highlighted border or glow effect. When inactive, the button SHALL display normally.

#### Scenario: Active toggle visual feedback
- **WHEN** caster has a toggle spell's aura active (e.g. Battle Stance)
- **THEN** the action bar button for that spell SHALL display a highlighted/glowing border

#### Scenario: Inactive toggle visual feedback
- **WHEN** caster does NOT have a toggle spell's aura active
- **THEN** the action bar button SHALL display with normal styling

### Requirement: Toggle state updates via SSE
The system SHALL emit `toggle.activated`, `toggle.deactivated`, and `toggle.broken` events via SSE. The frontend SHALL subscribe to these events and update the action bar button styling in real-time.

#### Scenario: Toggle activated updates UI
- **WHEN** frontend receives `toggle.activated` SSE event
- **THEN** the corresponding spell button SHALL update to active styling

#### Scenario: Toggle deactivated updates UI
- **WHEN** frontend receives `toggle.deactivated` SSE event
- **THEN** the corresponding spell button SHALL update to inactive styling
