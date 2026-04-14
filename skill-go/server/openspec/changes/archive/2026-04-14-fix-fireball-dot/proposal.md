## Why

火球术的 apply_aura 效果（DoT 周期伤害）施放后未对目标造成伤害。根因需要排查，可能的原因：
1. `handleAuraUpdate` 中 damage target 解析逻辑有误
2. Aura 的 `SpellID` 使用 `eff.EffectIndex + 9000` 而非实际 spellId，导致与 spell 关联断裂
3. 前端 timeline 未渲染 periodic_damage 事件

## What Changes

- 排查并修复 DoT 伤害不生效的根因
- 确保 Fireball 施放后，目标每 2 秒受到 21 点火焰伤害，持续 8 秒
- 确保 periodic_damage 事件在 trace/timeline 中可见

## Capabilities

### New Capabilities
（无）

### Modified Capabilities
（无已有 spec 需要修改 — 这是 bug fix）

## Impact

- `api/game_loop.go` — handleAuraUpdate 逻辑修复
- `api/server.go` — makeAuraHandler 可能需要调整
- `web/trace.js` — 可能需要支持渲染 periodic_damage 事件
