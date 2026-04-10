## Context

当前 spell web demo 使用纯 HTML 三栏布局（文字 trace + 数字 HP 条），无法直观展示战斗过程。Go 后端 API 已经完整（/api/cast 返回 trace events + unit states），只需要一个 3D 前端来可视化这些数据。

现有前端文件（web/index.html, style.css, app.js）将被完全替换。Go 后端代码零改动。

## Goals / Non-Goals

**Goals:**
- 全屏 3D 场景替代所有文字面板，提供直观的战斗可视化
- 角色用简单几何体表示，不同颜色/大小区分身份
- 投射物、粒子、浮动伤害数字等视觉反馈覆盖所有命中结果
- 底部 HUD 动作条支持施法和 CD 显示
- 保持零构建依赖（CDN 引入 Three.js）

**Non-Goals:**
- 不做真实角色模型（WoW .m2 模型加载）
- 不做骨骼动画/动作混合
- 不做多相机/自由视角
- 不做多人/实时同步
- 不做音效/背景音乐
- 不引入 npm/webpack/vite 等构建工具

## Decisions

### D1: Three.js 通过 CDN 引入，不用 npm

项目坚持零构建依赖原则。Three.js r160+ 的 ES module 版本可通过 `importmap` + CDN（esm.sh 或 unpkg）直接在浏览器中使用，无需打包工具。

```
<script type="importmap">
{ "imports": { "three": "https://cdn.jsdelivr.net/npm/three@0.170.0/build/three.module.js" } }
</script>
```

### D2: 角色用组合几何体（圆柱+球体），不用外部模型

Mage=蓝色圆柱，Warrior=棕色圆柱，Target Dummy=红色圆柱。头顶球体作为"头部"。每个角色附带一个 Group，包含模型、名牌、HP/MP 条。名牌使用 CSS2DRenderer 始终面向相机。

### D3: 动画由 trace events 驱动（API-driven）

施法后从 /api/cast 获取 trace events 和 updated units。前端将 trace events 推入 AnimationTimeline，Timeline 根据事件类型和延迟调度 3D 动画：

- `spell.prepare` → Caster 播放施法前摇（发光/粒子聚集）
- `effect.effect_launch` → 生成投射物，沿直线飞向 Target
- `combat.hit/crit/miss` → 目标受击反馈 + 浮动数字
- `aura.apply` → 目标脚下光环

投射物飞行时间固定为 600ms（与 spell CastTime 无关，因为当前 demo 所有技能都是 instant cast）。

### D4: 浮动伤害数字用 CSS2DObject 实现

不用 3D 文字（Three.js TextGeometry 需要 font 文件），改用 HTML 元素叠加到 3D 空间（CSS2DRenderer）。这样可以用任意字体、CSS 动画，且性能更好。

### D5: 固定正交 45° 俯视相机

正交投影（OrthographicCamera），角度 (30, 45, 0)，distance=40。正交投影的好处是角色大小不随距离变化，更适合俯视战斗场景。不提供旋转/缩放。

### D6: HUD 叠加层使用 HTML/CSS，不走 Three.js

底部动作条、统计信息用 HTML div + CSS 绝对定位叠加在 canvas 上方。Three.js canvas 是底层，HUD HTML 是上层。这样 HUD 的样式和交互完全用熟悉的 HTML/CSS 实现。

### D7: 前端模块拆分为 5 个文件

```
web/
  index.html     ← 全屏布局 + importmap + script 引入
  style.css      ← HUD 样式（动作条、统计、叠加层）
  app.js         ← 入口：API 调用、初始化场景、HUD 交互
  scene.js       ← Three.js 场景/相机/光照/地面/渲染循环
  character.js   ← 角色创建、HP 名牌、角色更新
  vfx.js         ← 投射物、粒子、浮动数字、受击效果
  timeline.js    ← trace event → 动画调度队列
```

所有 JS 文件使用 ES module（`<script type="module">`），通过相对路径 import。

### D8: 投射物外观按法术学派区分

| 学派 | 颜色 | 粒子效果 |
|------|------|----------|
| Fire | #ff4400 橙红 | 火焰粒子尾迹 |
| Frost | #44aaff 浅蓝 | 冰晶碎片尾迹 |
| Arcane | #cc44ff 紫色 | 奥术光点尾迹 |
| Shadow | #8844aa 暗紫 | 暗影漩涡尾迹 |
| Nature | #44ff44 绿色 | 自然光点尾迹 |
| Holy | #ffcc00 金色 | 神圣光芒尾迹 |
| Physical | #cc8844 棕色 | 武器挥砍弧线（非投射物） |

### D9: 地面使用 GridHelper

Three.js 内置 GridHelper，暗灰色线条。地面下方添加一个半透明平面作为"地板"。

## Risks / Trade-offs

- **[CDN 依赖]** → Three.js 从 CDN 加载，离线环境不可用。可接受，这是 demo 不是生产系统。
- **[正交投影无深度感]** → 远处角色和近处一样大，损失空间感。可接受，俯视角下正交更适合等距战斗。
- **[无骨骼动画]** → 角色不会"跑动"或"挥砍"，只有整体位移和特效。可接受，这是 spell 系统验证工具不是动画 demo。
- **[投射物飞行时间固定]** → 所有技能投射物飞行时间相同（600ms），与实际 CastTime 无关。可接受，当前 demo 全是 instant cast。
- **[CSS2DRenderer 性能]** → 大量浮动数字时可能卡顿。通过限制同屏数字数量（自动回收）缓解。
