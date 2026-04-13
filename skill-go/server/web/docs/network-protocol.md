# skill-go Network Protocol Reference

> Last updated: 2026-04-13

---

## Overview

skill-go uses HTTP REST API + SSE for communication between web client and Go server.

- **Base URL**: Same origin (e.g. `http://localhost:13001`)
- **Data format**: JSON (`Content-Type: application/json`)
- **Streaming**: Server-Sent Events (SSE) for real-time trace events
- **Client library**: `web/net.js` (`NetClient` class)

---

## NetClient Usage

```javascript
import { netClient, NetError, Subscription } from '/net.js';

// REST calls
const spells = await netClient.get('/api/spells');
const result = await netClient.post('/api/cast', { spellID: 42833, targetIDs: [3] });
await netClient.put('/api/spells/42833', { name: 'Fireball', castTime: 3500 });
await netClient.del('/api/spells/1001');

// Query parameters
const history = await netClient.get('/api/trace/history', { flow_id: '123', limit: '50' });

// SSE subscription
const sub = netClient.subscribe('/api/trace/stream', (event) => {
  console.log('Trace event:', event);
});
sub.onReconnect = (status) => console.log('SSE:', status); // 'connected' | 'reconnecting'
sub.close(); // manual close (auto-reconnect stops)

// Global error handler
netClient.onError((err) => {
  console.error('Network error:', err.code, err.message, err.httpStatus);
});
```

---

## REST API Endpoints

### Spell Operations

| Method | Path | Request Body | Response | Description |
|--------|------|-------------|----------|-------------|
| GET | `/api/spells` | - | `SpellJSON[]` | List all spells |
| POST | `/api/spells` | `CreateSpellRequest` | `SpellJSON` (201) | Create a new spell |
| GET | `/api/spells/{id}` | - | `SpellJSON` | Get spell details |
| PUT | `/api/spells/{id}` | `UpdateSpellRequest` | `{status: "ok"}` | Update a spell |
| DELETE | `/api/spells/{id}` | - | `{status: "deleted"}` | Delete a spell |

### Cast Operations

| Method | Path | Request Body | Response | Description |
|--------|------|-------------|----------|-------------|
| POST | `/api/cast` | `CastRequest` | `CastPrepareResponse` or `CastResponse` | Prepare cast (two-phase for cast-time spells) |
| POST | `/api/cast/complete` | - | `CastResponse` | Complete pending cast |
| POST | `/api/cast/cancel` | - | `{result: "cancelled"}` | Cancel pending cast |

### Unit Operations

| Method | Path | Request Body | Response | Description |
|--------|------|-------------|----------|-------------|
| GET | `/api/units` | - | `UnitJSON[]` | List all units |
| POST | `/api/units/add` | `AddUnitRequest` | `UnitJSON[]` | Spawn new unit |
| PUT | `/api/units/update` | `UpdateUnitRequest` | `UnitJSON[]` | Update unit (e.g. level) |
| POST | `/api/units/move` | `MoveUnitRequest` | `UnitJSON[]` | Move unit to position |
| DELETE | `/api/units/{guid}` | - | `UnitJSON[]` | Remove unit |

### Trace Operations

| Method | Path | Params | Response | Description |
|--------|------|--------|----------|-------------|
| GET | `/api/trace` | `?clear=true` | `TraceEventJSON[]` | Get/clear trace events |
| GET | `/api/trace/stream` | - | SSE stream | Real-time trace event stream |
| GET | `/api/trace/history` | `?flow_id=&span=&limit=` | `TraceEventJSON[]` | Query trace history |

### System

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| GET | `/api` | `RouteInfo[]` | List all API routes |
| GET | `/api/docs` | `DocSection[]` | Spell system config reference |
| POST | `/api/reset` | `{status: "ok"}` | Reset session |

---

## Request/Response Types

### CastRequest
```json
{
  "spellID": 42833,
  "targetIDs": [3]
}
```

### CastResponse (instant cast result)
```json
{
  "result": "success",
  "units": [...],
  "events": [...]
}
```

### CastPrepareResponse (cast-time spell, preparing)
```json
{
  "result": "preparing",
  "castTimeMs": 3500,
  "spellID": 42833,
  "spellName": "Fireball",
  "schoolName": "Fire"
}
```

### UnitJSON
```json
{
  "guid": 3,
  "name": "Target Dummy",
  "health": 12000,
  "maxHealth": 15000,
  "mana": 0,
  "maxMana": 0,
  "alive": true,
  "level": 63,
  "teamId": 0,
  "armor": 5000,
  "resistances": { "Fire": 100, "Frost": 0, ... },
  "auras": [
    {
      "spellID": 9001,
      "name": "Fireball",
      "duration": 8000,
      "auraType": 1,
      "stacks": 1,
      "timerStart": 1712345678900
    }
  ],
  "position": { "x": 40, "y": 0, "z": 0 }
}
```

### SpellJSON
```json
{
  "id": 42833,
  "name": "Fireball",
  "schoolMask": 1,
  "schoolName": "Fire",
  "castTime": 3500,
  "cooldown": 0,
  "powerCost": 400,
  "maxTargets": 1,
  "categoryCD": 0,
  "effects": ["SchoolDamage", "ApplyAura"],
  "effectsDetail": [...]
}
```

### TraceEventJSON (SSE / trace response)
```json
{
  "flowId": 1,
  "timestamp": 1712345678900,
  "span": "combat",
  "event": "school_damage_hit",
  "spellId": 42833,
  "spellName": "Fireball",
  "fields": {
    "damage": 1234,
    "result": 2,
    "target": "Target Dummy"
  }
}
```

---

## Error Format

All errors return structured JSON:

```json
{
  "error": {
    "code": "bad_request",
    "message": "invalid JSON"
  }
}
```

### Error Codes

| HTTP Status | Code | Description |
|-------------|------|-------------|
| 400 | `bad_request` | Invalid request body or parameters |
| 400 | `invalid_json` | Malformed JSON in request body |
| 400 | `missing_field` | Required field is missing |
| 404 | `not_found` | Resource not found (spell/unit) |
| 405 | `method_not_allowed` | HTTP method not supported for this endpoint |
| 500 | `internal_error` | Server-side error |

---

## SSE Event Stream

### Connection

```
GET /api/trace/stream
Accept: text/event-stream
```

### Event Format

Each event is a JSON object prefixed with `data: `:

```
data: {"flowId":1,"timestamp":1712345678900,"span":"cooldown","event":"add_cooldown","spellId":42833,"spellName":"Fireball","fields":{"duration_ms":0,"category":0}}

data: {"flowId":1,"timestamp":1712345678901,"span":"combat","event":"school_damage_hit","spellId":42833,"spellName":"Fireball","fields":{"damage":1234,"result":2,"target":"Target Dummy"}}
```

### Reconnection

The client auto-reconnects after 3 seconds on connection loss.

---

## Span Types (Trace Events)

| Span | Description |
|------|-------------|
| `cooldown` | Cooldown/GCD events |
| `prepare` | Spell preparation phase |
| `combat` | Combat resolution (damage/heal/miss) |
| `aura` | Aura application/removal |
| `effect` | Effect execution |
