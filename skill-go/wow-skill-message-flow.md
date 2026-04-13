# WoW Spell Message Flow Reference

> Based on TrinityCore 3.3.5a (WotLK) spell system

---

## 1. Normal Cast-Time Spell Flow

```
CLIENT                          SERVER
  |                               |
  |-- CMSG_CAST_SPELL --------->  |
  |   {spellID, targetGUID}      |
  |                               |-- Validate()
  |                               |   - Caster alive?
  |                               |   - Not already casting?
  |                               |   - Not silenced?
  |                               |   - Not mounted?
  |                               |   - Correct shapeshift?
  |                               |   - Spell on cooldown?
  |                               |   - Not on GCD?
  |                               |   - Has charges?
  |                               |   - Sufficient mana?
  |                               |   - Target in range?
  |                               |   - Target alive?
  |                               |   - Line of sight?
  |                               |
  |                               |-- FAIL: SMSG_SPELL_FAILURE
  |                               |-- FAIL: SMSG_SPELL_FAILED_OTHER
  | <-- {result: "failed"} ------|
  |                               |
  |                               |-- PASS:
  |                               |   - Consume mana
  |                               |   - Start cooldown (some spells)
  |                               |   - Start GCD
  |                               |   - SMSG_SPELL_START (broadcast)
  |                               |
  | <-- SMSG_SPELL_START --------|
  |   {castTime, spellID,        |
  |    casterGUID, targetGUID}    |
  |                               |
  |   [=== Cast Bar Progress ===]|
  |                               |
  |   ... possible interrupts ... |
  |                               |
  |                               |-- Cast completes
  |                               |   - Consume charges (if applicable)
  |                               |   - Start recovery cooldown
  |                               |   - SMSG_SPELL_GO (broadcast)
  |                               |   - Process effects
  |                               |   - SMSG_SPELL_EXECUTE_LOG
  |                               |
  | <-- SMSG_SPELL_GO -----------|
  |   {spellID, hit/miss,        |
  |    damage, crit}             |
  | <-- SMSG_SPELL_EXECUTE_LOG --|
  |                               |
```

## 2. Instant Cast Spell Flow

```
CLIENT                          SERVER
  |                               |
  |-- CMSG_CAST_SPELL --------->  |
  |                               |-- Validate() (same checks)
  |                               |
  |                               |-- PASS:
  |                               |   - Consume mana
  |                               |   - Start GCD
  |                               |   - Start recovery cooldown
  |                               |   - SMSG_SPELL_GO (broadcast)
  |                               |   - Process effects
  |                               |   - SMSG_SPELL_EXECUTE_LOG
  |                               |
  | <-- SMSG_SPELL_GO -----------|
  | <-- SMSG_SPELL_EXECUTE_LOG --|
```

## 3. Interrupt Paths

### 3a. Movement Interrupt

```
CLIENT                          SERVER
  |                               |
  |   [=== Casting Fireball ===] |
  |                               |
  |-- Movement Start --------->  |
  |                               |-- Detect movement during cast
  |                               |-- Cancel cast
  |                               |-- Refund mana
  |                               |-- SMSG_SPELL_FAILURE
  |                               |-- SMSG_SPELL_FAILED_OTHER
  |                               |   {reason: "interrupted"}
  |                               |
  | <-- SMSG_SPELL_FAILURE ------|
  |                               |
```

### 3b. Spell Pushback (Damage Taken)

```
CLIENT                          SERVER
  |                               |
  |   [=== Casting Fireball ===] |
  |   [||||||||||||          ]   |
  |                               |
  |   (Caster takes damage)       |
  |                               |-- Apply pushback (+500ms)
  |                               |-- If total pushback >= castTime:
  |                               |     Interrupt completely
  |                               |-- Else:
  |                               |     Extend remaining cast time
  |                               |-- SMSG_SPELL_DELAYED (optional)
  |                               |
  |   [||||||||||||||        ]   |
  |   (extended cast bar)         |
  |                               |
```

### 3c. School Lockout (Counterspell)

```
CLIENT                          SERVER
  |                               |
  |   [=== Casting Fireball ===] |
  |                               |
  |   (Enemy uses Counterspell)   |
  |                               |-- Interrupt cast
  |                               |-- Lock Fire school for 6s
  |                               |-- SMSG_SPELL_INTERRUPTED
  |                               |-- SMSG_SPELL_FAILED_OTHER
  |                               |   {school: Fire, lockoutMs: 6000}
  |                               |
  | <-- SMSG_SPELL_INTERRUPTED ---|
  |   (cannot cast Fire spells    |
  |    for 6 seconds)             |
```

## 4. Channeled Spell Flow

```
CLIENT                          SERVER
  |                               |
  |-- CMSG_CAST_SPELL --------->  |
  |                               |-- Validate()
  |                               |-- SMSG_SPELL_START (with channel flag)
  |                               |
  | <-- SMSG_SPELL_START --------|
  |                               |
  |   [=== Channel Bar ==========]|
  |   [|||||||||||||||||||||||||] |
  |                               |-- Tick 1: process effects
  |                               |-- SMSG_SPELL_CHANNEL_UPDATE
  | <-- SMSG_SPELL_CHANNEL_UPDATE |
  |                               |
  |   [||||||||                  ] |
  |                               |-- Tick 2: process effects
  |                               |-- SMSG_SPELL_CHANNEL_UPDATE
  | <-- SMSG_SPELL_CHANNEL_UPDATE |
  |                               |
  |   [||||                      ] |
  |                               |-- Movement interrupt
  |                               |-- SMSG_SPELL_INTERRUPTED
  | <-- SMSG_SPELL_INTERRUPTED ---|
  |                               |
```

## 5. Key Message Types

| Message | Direction | Description |
|---------|-----------|-------------|
| CMSG_CAST_SPELL | Client→Server | 施法请求 |
| SMSG_SPELL_START | Server→Client | 施法开始（含 cast time） |
| SMSG_SPELL_GO | Server→Client | 施法执行（效果触发） |
| SMSG_SPELL_EXECUTE_LOG | Server→Client | 战斗日志（伤害/治疗数字） |
| SMSG_SPELL_FAILURE | Server→Client | 施法失败（如取消） |
| SMSG_SPELL_FAILED_OTHER | Server→Client | 其他玩家施法失败通知 |
| SMSG_SPELL_INTERRUPTED | Server→Client | 施法被中断 |
| SMSG_SPELL_DELAYED | Server→Client | 施法被推延 |
| SMSG_SPELL_CHANNEL_UPDATE | Server→Client | 引导法术剩余时间更新 |
| SMSG_SPELL_COOLDOWN | Server→Client | 冷却时间更新 |
| SMSG_SPELL_POWER | Server→Client | 法力值变化 |

## 6. Cooldown Timing

| Spell Type | CD Start | CD End |
|------------|----------|--------|
| Instant | On cast (SMSG_SPELL_GO) | RecoveryTime after cast |
| Cast-time | On complete (SMSG_SPELL_GO) | RecoveryTime after cast |
| Channeled | On complete | RecoveryTime after channel ends |
| Charge-based | On consume | ChargeRecoveryTime per charge |
| Shared CD | On cast | CategoryRecoveryTime |

## 7. skill-go Current Implementation vs WoW

| Feature | WoW | skill-go | Status |
|---------|-----|----------|--------|
| Basic cast flow (prepare→cast→execute) | ✓ | ✓ | **Match** |
| Instant cast | ✓ | ✓ | **Match** |
| Mana consumption on prepare | ✓ | ✓ | **Match** |
| Cooldown on complete | ✓ | ✓ | **Match** |
| GCD | ✓ | ✓ | **Match** |
| Cancel + mana refund | ✓ | ✓ | **Match** |
| Movement interrupt | ✓ | ✗ | **Missing** — no movement detection during cast |
| Spell pushback | ✓ | ✗ | **Missing** — no pushback on damage taken |
| Pushback interrupt (100% cap) | ✓ | ✗ | **Missing** |
| Channeled spells with tick | ✓ | Partial | Channeling code exists but no SSE tick push |
| School lockout | ✓ | Partial | Data structures exist, no interrupt spell |
| Line of sight | ✓ | ✗ | Not implemented |
| Visibility range broadcast | ✓ | ✗ | Single client, N/A |
| Spell queue | ✓ (retail) | ✗ | 3.3.5a doesn't have it either |
| Spell reflection | ✓ | ✗ | Flag exists, no logic |
| Script hooks (OnCheckCast, etc.) | ✓ | ✓ | **Match** |

## 8. Notes

### Design Decisions (2026-04-13)

1. **Movement interrupt via client notification**: Web has no continuous position sync, so movement interrupt is triggered client-side (cancel before move) rather than server-detected.

2. **Pushback via explicit API**: POST /api/cast/pushback allows external systems to trigger pushback. Not auto-detected from damage (since caster doesn't typically take damage in this demo).

3. **Channel SSE via trace stream**: Channel ticks reuse the existing SSE trace infrastructure (span="channel", event="channel_tick") rather than a new endpoint.

4. **No visibility broadcast**: Single-client web demo doesn't need the WoW visibility range broadcast mechanic. SSE trace stream serves the equivalent role.
