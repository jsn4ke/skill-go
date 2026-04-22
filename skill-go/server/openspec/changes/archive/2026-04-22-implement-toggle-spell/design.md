## Context

当前 spell 系统只支持一次性施法流程：Prepare→Cast→Hit→Finish。WoW 中大量技能（战士姿态、德鲁伊变形、盗贼潜行、圣骑士光环）是 Toggle 型 — 按 on/off 切换，不是一次性事件。

Toggle 的本质是"永久 aura + 互斥逻辑"。当前 aura 系统已支持 ApplyAura / RemoveAura / 属性修改，核心差距在于 spell 层没有 toggle 分支。

## Goals / Non-Goals

**Goals:**
- spelldef 支持 IsToggle / ToggleGroup 字段定义
- cast 命令处理 toggle：已有同 aura → 移除(off)，没有 → 施加(on)
- ToggleGroup 互斥：同组只能有一个 active，切新的自动移除旧的
- BreakOnDamage 条件：受击自动退出 toggle（用于潜行）
- 前端显示当前 active toggle 状态
- 添加两个测试技能：盗贼潜行（独立 toggle）和 战士姿态（互斥组）

**Non-Goals:**
- 不实现"形态改变可用技能列表"（如熊形态替换技能栏），只做属性 buff
- 不实现 ToggleGroup 的 UI 选择面板，只在 action bar 中显示状态
- 不实现被动触发（如条件满足自动激活 toggle）
- 不实现 BreakOnCast（施法时退出 toggle），仅 BreakOnDamage

## Decisions

### 1. Toggle 复用 channel 的 aura 机制，不新增 spell 类型

**选择**：Toggle 不新增 SpellState，复用现有 ApplyAura + RemoveAura。

**理由**：Toggle 本质就是"施加一个永久 aura" / "移除那个 aura"。aura 系统已有属性修改、持续效果、stack 等能力。新增 spell 类型（如 StateToggle）反而增加状态机复杂度。

**替代方案**：新增 StateActive / StateInactive 状态 → 拒绝，因为 toggle 施法后立即结束，不需要长期持有 SpellContext。

### 2. ToggleGroup 互斥由 aura manager 处理

**选择**：在 `aura.AuraManager.ApplyAura()` 中检查 ToggleGroup，互斥时先 RemoveAura 旧的再 Apply 新的。

**理由**：互斥是 aura 层面的约束，不是 spell 层。spell 只负责说"我要施加这个 aura"，互斥逻辑由 aura manager 统一处理。

**替代方案**：在 game_loop handleCast 中处理互斥 → 可以工作但会分散逻辑，不如集中在 aura manager。

### 3. Toggle aura 的 spell 生命周期

**选择**：Toggle spell 仍然走 Prepare→Cast→Finish 流程，只是 Cast 时检查 toggle 状态决定是 apply 还是 remove aura。

**流程**：
```
首次 cast (on):
  Prepare → checkcast → Cast → apply aura → Finish → StateFinished

二次 cast (off):
  Prepare → checkcast → Cast → remove aura → Finish → StateFinished
```

**理由**：复用现有施法流程，只改变 Cast 的 effect 执行逻辑。

### 4. CSV 数据格式

**选择**：在 spells.csv 新增列 `isToggle` (0/1) 和 `toggleGroup` (string)。spell_effects.csv 中 toggle 技能使用 apply_aura effect，auraDuration=0 表示永久。

### 5. BreakOnDamage 通过 AuraEffect.MiscValue 标记

**选择**：不新增 spelldef 字段，在 aura effect 的 MiscValue 中用位标记表达退出条件。

**理由**：保持 spelldef 简洁，退出条件是 aura 行为，不是 spell 定义。

## Risks / Trade-offs

- [Risk] 互斥移除 aura 时，旧 aura 的 removeEffect 清理可能不完整 → Mitigation: RemoveAura 已有 removeEffect 逻辑，确保 toggle aura 的 effect 是可逆的（如 ModifyStat(amount) 对应 ModifyStat(-amount))
- [Risk] BreakOnDamage 需要在 TakeDamage 中检查 toggle aura → Mitigation: 只检查 caster 身上 MiscValue 包含 break flag 的 aura，遍历开销小
- [Trade-off] 不支持"形态改变技能列表"意味着战士姿态只是属性 buff，不改变可用技能 → 可接受，后续可扩展
