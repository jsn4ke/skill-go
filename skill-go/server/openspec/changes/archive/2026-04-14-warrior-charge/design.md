## Context

当前系统已有完整的施法→命中管线、Aura 系统（挂载/周期伤害/过期/控制效果反转）、CSV 驱动的技能配置、3D 场景和位移命令。`UnitStateStunned` 已在 `spelldef/unitstate.go` 定义，`aura/manager.go` 的 `applyControlEffect` 已支持通过 `MiscValue` 映射到 `UnitState`。

TBC 原版 spell=100 冲锋的调用链：`Charge(100)` → trigger_spell → `Charge Stun(7922)` → apply_aura stun 1.5s。

## Goals / Non-Goals

**Goals:**
- 冲锋施法后，施法者瞬移到目标面前（目标面朝方向偏移 1 码）
- 眩晕目标 1.5 秒，通过 `trigger_spell` 触发 spell=7922（Charge Stun）→ `apply_aura` stun 实现
- 眩晕到期自动移除，恢复目标行动能力
- 前端实时更新施法者 3D 位置和目标眩晕状态

**Non-Goals:**
- 不实现怒气系统（冲锋产怒）
- 不实现冲锋不可用条件（已定身/变形/战斗中不可用等）
- 不实现冲锋 CD 共享组
- 不实现冲锋最小射程（TBC 要求 8-25 码，当前简化为 0-25 码）

## Decisions

### 1. 位移实现：直接修改 Position，通过 cast response 同步

冲锋位移不引入新的 effect type。在 `processCastResult` 阶段，对冲绷新技能做特殊处理：命中后直接修改施法者 `Position`，然后通过 `/api/units` 返回新的单位数据，前端 `reconcileUnits` 会调用 `moveUnit` 更新 3D 位置。

**替代方案**：新增 `SpellEffectCharge` effect type + handler。评估后认为过早抽象，冲锋是目前唯一的位移技能，直接在 cast 完成后处理更简单。

### 2. 眩晕机制：trigger_spell → spell=7922 → apply_aura stun

spell=100 的效果[2] 为 `trigger_spell`，触发 spell=7922（Charge Stun）。spell=7922 在 CSV 中定义为 `apply_aura` stun，duration=1500ms。命中时触发链：`Charge(100)` → `trigger_spell` → `Charge Stun(7922)` → `apply_aura stun` → `AuraTypeDebuff` + `MiscValue=UnitStateStunned`。复用已有 `applyControlEffect` / `removeControlEffect` 逻辑。

**TBC 原版对比**：TBC spell=100 有 3 个效果（charge 位移、dummy 服务端逻辑、trigger_spell 眩晕），gcd=0。当前引擎没有 charge effect type，简化为：位移在代码中直接处理（Decision 1），眩晕通过 trigger_spell → spell=7922 → apply_aura 实现，完整复刻 TBC 调用链。

### 3. 位移偏移量：目标面朝方向 -1 码

冲锋结束后施法者应站在目标"面前"。使用目标面朝方向向量，施法者落在 `target.Position - forward * 1.0` 的位置。如果目标面朝为 (0,0,0)（默认），回退到施法者方向向量。

## Risks / Trade-offs

- **[位移精确度]** → 当前 3D 场景是俯视角，Z 轴旋转有限，位移效果可能不够明显。接受这个限制。
- **[施法者 stun]** → 冲锋只晕目标不晕自己。如果目标对施法者冲锋时施加了 stun，需要在 `handleCastComplete` 中检查施法者状态。
