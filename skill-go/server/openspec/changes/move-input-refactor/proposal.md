## Why

当前移动系统存在设计问题：鼠标左键点击地面移动施法者，同时这个移动会打断引导类法术。这与 WoW 的实际体验不符。在 WoW 中，施法者移动（WASD）是打断引导的唯一方式，而鼠标点击主要用于选择目标。重新设计输入层，将 WASD 用于施法者移动（带打断逻辑），鼠标点击用于目标选择和移动辅助（非打断）。

## What Changes

- **新增 WASD 移动系统**：监听键盘 WASD 输入，推动施法者向对应方向移动。移动速度与现有 `MOVE_SPEED` 一致（10 units/s）。
- **移动打断引导逻辑**：当施法者处于 `StateChanneling` 时，WASD 移动触发 `ctx.Cancel()` 打断引导。
- **鼠标点击重构为纯选择/移动**：
  - 点击角色 → 选中为目标（不触发 caster 移动）
  - 点击地面 → 移动当前选中目标（不是 caster）；此移动**不打断** caster 的引导
- **移除旧的点击地面移动 caster 逻辑**：原 `onCanvasClick` 中点击地面时调用 `moveUnit(caster)` 的行为被删除。
- **服务端位置同步**：WASD 移动和选中目标的移动都需要同步到服务端 `POST /api/units/move`。

## Capabilities

### New Capabilities
- `caster-wasd-movement`: WASD 输入驱动施法者（activeCasterGUID）移动，带施法打断逻辑
- `target-click-move`: 鼠标点击移动当前选中目标（非施法者），非打断性移动

### Modified Capabilities
- `unit-movement`: 当前"点击地面移动施法者"的行为被 WASD 取代。点击地面不再移动施法者，而是移动选中的非施法者目标。
- `target-selection`: 当前点击角色选中并移动 caster，改为：点击角色仅选中目标，不触发任何移动

## Impact

- **前端**：`web/app.js` — 重构 `onCanvasClick`，新增 `keydown`/`keyup` WASD 监听，`animationLoop` 处理持续移动
- **前端**：`web/character.js` — 已有 `moveUnit` 逻辑可复用；需处理多个单位同时移动
- **前端**：`web/scene.js` — 无需改动（鼠标射线投射逻辑不变）
- **前端**：`web/timeline.js` — 无需改动
- **后端**：`api/server.go` — 无需改动（`/api/units/move` 已支持任意 GUID）
- **后端**：`spell/spell.go` — 新增 `IsChanneling()` 方法供前端判断是否打断
- **OpenSpec**：`unit-movement/spec.md` — 更新需求描述；`target-selection/spec.md` — 更新需求描述

## Breaking Changes

- 鼠标左键点击地面不再移动施法者（Mage）。这是用户可见的行为变化。