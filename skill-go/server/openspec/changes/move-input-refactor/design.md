## Context

当前 `app.js` 的 `onCanvasClick` 中，左键点击地面会调用 `moveUnit(caster)` 移动施法者。这个行为在 WoW 中对应 WASD 移动，但实际 WoW 中 WASD 移动会打断引导法术。当前系统的设计混淆了"选择目标"和"移动 caster"——鼠标点击同时做了两件事。

用户现在要求重新设计输入分层：
- **WASD** → 移动施法者 + 打断引导
- **鼠标点击** → 选择目标/移动选中目标（非打断）

## Goals / Non-Goals

**Goals:**
- WASD 键盘输入驱动施法者（activeCasterGUID）持续移动
- WASD 移动时如果施法者处于引导状态，打断引导
- 鼠标左键点击地面移动当前选中的非 caster 目标（Target Dummy 等）
- 此鼠标触发的目标移动**不打断**施法者的引导
- 多单位可以同时移动（WASD 移动 caster + 鼠标移动选中目标）

**Non-Goals:**
- 不实现斜向移动（可以后续扩展）
- 不实现鼠标右键移动
- 不改变后端位置同步逻辑（复用现有 `/api/units/move`）
- 不实现施法者朝向（rotation）

## Decisions

### D1: WASD 使用固定方向向量而非速度合成

每个按键映射到固定方向：
- W = (0, 1) — 向+Z 方向（屏幕下方）
- S = (0, -1) — 向-Z 方向
- A = (-1, 0) — 向-X 方向
- D = (1, 0) — 向+X 方向

每帧 `animationLoop` 中检查按键状态，按下时按方向向量移动。每帧最多移动 `MOVE_SPEED * dt` 距离。无需速度合成（斜向）。

### D2: WASD 移动与动画循环耦合

在现有 `addUpdatable({ update(dt) { ... } })` 的循环中，添加 WASD 方向检测：

```javascript
const WASD_KEYS = { w: false, a: false, s: false, d: false };
document.addEventListener('keydown', e => { WASD_KEYS[e.key.toLowerCase()] = true; });
document.addEventListener('keyup', e => { WASD_KEYS[e.key.toLowerCase()] = false; });

// In animation loop update:
if (WASD_KEYS.w) moveDirection(0, 1);
if (WASD_KEYS.s) moveDirection(0, -1);
if (WASD_KEYS.a) moveDirection(-1, 0);
if (WASD_KEYS.d) moveDirection(1, 0);
```

移动方向基于相机朝向固定，简化处理（相机正交向下）。

### D3: WASD 移动打断引导

在 WASD 按键触发移动后，如果 `castingState === 'channeling'`，调用 `POST /api/cast/cancel` 打断引导。判断逻辑：
- `castingState` 变量在 `enterChannelingState()` 时设为 `'channeling'`
- 前端不知道后端的 `spell.StateChanneling`，但通过 `castingState` 状态可以判断
- `cancelCast()` 向后端发送取消请求，后端调用 `ctx.Cancel()` 停止 channel ticker

### D4: 鼠标点击地面移动选中目标（非施法者）

`onCanvasClick` 重构：点击地面时，检查 `selectedTargetGUID`：
- 如果选中的是 caster 自己（`selectedTargetGUID === activeCasterGUID`）：不移动
- 如果选中的是其他目标（Target Dummy 等）：调用 `moveUnit(targetGroup)` 移动该目标
- 如果没有选中目标：点击地面不产生任何效果

服务端同步：`moveUnit` 需要调用 `POST /api/units/move` 同步位置。

### D5: 移动时颜色反馈

当单位被移动时（无论是 WASD 驱动还是鼠标点击驱动），在目标头顶显示移动方向指示器（如向上箭头），表示该单位正在移动。

## Risks / Trade-offs

**[Risk] 同时移动 caster 和目标**  
→ 当前设计允许多个单位同时移动。`moveUnit` 在 `character.js` 中设置 `targetPosition` 和 `isMoving`，每个单位独立追踪自己的目标位置。动画循环遍历所有 characters 分别处理。

**[Risk] WASD 移动在瞄准模式下失效**  
→ 当 `targetingMode !== 'unit'` 时（地面选择 AoE 技能等），WASD 仍可移动施法者。瞄准模式不应阻止移动。

**[Risk] 移动打断 channeling 的时序**  
→ 客户端先移动（视觉），再取消（后端）。可能短暂出现视觉上的移动然后技能消失。但这个体验接近 WoW 真实行为（移动瞬间技能被取消）。

## Open Questions

1. 是否需要方向指示器（如角色头顶的移动箭头）？考虑作为视觉反馈添加。
2. WASD 移动是否需要碰撞检测？当前不考虑（游戏是俯视角，无碰撞）。
3. 移动时是否播放脚步声或动画？当前不考虑（简化实现）。