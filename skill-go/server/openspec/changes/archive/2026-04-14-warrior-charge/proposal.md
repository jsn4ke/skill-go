## Why

冲锋（Charge）是战士最具标志性的技能，涉及**位移 → 近身 → 眩晕**三段式效果链，是验证引擎"瞬移 + 控制效果 + 近战攻击"组合能力的理想候选。当前系统只有远程施法（火球术）和即时法术，缺少位移类近战技能。

## What Changes

- 新增 spellId=100 的战士技能"冲锋"（参考 `skill.md` 中的 TBC 数据）
- 冲锋效果：施法者瞬间移动到目标位置，通过 trigger_spell 触发 spell=7922（Charge Stun）对目标施加 1.5 秒眩晕
- TBC 原版调用链：`Charge(100)` → trigger_spell → `Charge Stun(7922)` → apply_aura stun 1.5s
- 在 `spells.csv` 和 `spell_effects.csv` 中添加冲锋和冲锋眩晕数据（gcd=0，0 法力消耗，15秒 CD）
- 复用已有的 `UnitStateStunned` 控制效果机制（通过 `trigger_spell` → `apply_aura`）
- 验证施法者位移后位置同步到前端 3D 场景

## Capabilities

### New Capabilities
- `warrior-charge`: 战士冲锋技能 — 瞬移至目标位置并施加眩晕控制效果

### Modified Capabilities
（无现有 spec 需要修改）

## Impact

- `data/spells.csv` — 新增冲锋行
- `data/spell_effects.csv` — 新增冲锋（trigger_spell）和冲锋眩晕（apply_aura stun）效果行
- `aura/manager.go` — `applyEffect` 中 `AuraTypeDebuff` 需要支持 stun 类型（通过 `MiscValue` 映射到 `UnitStateStunned`）
- `effect/effects.go` — 确认/添加 `trigger_spell` effect handler
- `api/game_loop.go` — 施法者位移后需更新 `allUnits` 状态，前端通过 `/api/units` 刷新位置
- `web/app.js` — 处理施法者位移后的 3D 模型更新
