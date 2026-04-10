## 1. 场景基础设施 (scene.js)

- [x] 1.1 创建 `web/scene.js`：初始化 WebGLRenderer（全屏，抗锯齿，暗色背景 #1a1a2e）
- [x] 1.2 创建 `web/scene.js`：OrthographicCamera（45° 俯视，30° 方位，distance=40）
- [x] 1.3 创建 `web/scene.js`：添加 GridHelper 地面网格 + 半透明地板平面
- [x] 1.4 创建 `web/scene.js`：AmbientLight（0x404060，强度 0.6）+ DirectionalLight（0xffffff，强度 0.8，投射阴影）
- [x] 1.5 创建 `web/scene.js`：CSS2DRenderer 叠加层（用于名牌和浮动数字）
- [x] 1.6 创建 `web/scene.js`：requestAnimationFrame 渲染循环，支持注册/注销可更新对象

## 2. 角色渲染 (character.js)

- [x] 2.1 创建 `web/character.js`：createCharacter(unit) 函数 — 圆柱体身体 + 球体头，按角色着色
- [x] 2.2 创建 `web/character.js`：头顶名牌（CSS2DObject，白色文字，显示 unit.Name）
- [x] 2.3 创建 `web/character.js`：HP/MP 条（CSS2DObject，HTML 进度条，绿/黄/红分级）
- [x] 2.4 创建 `web/character.js`：updateCharacter(character, unitData) — 更新 HP/MP 条、alive 状态、aura 列表
- [x] 2.5 创建 `web/character.js`：死亡视觉 — alive=false 时模型灰化 + 倒下

## 3. 法术视觉特效 (vfx.js)

- [x] 3.1 创建 `web/vfx.js`：spawnProjectile(from, to, schoolColor) — 发光球体 + 粒子尾迹，600ms 飞行
- [x] 3.2 创建 `web/vfx.js`：按学派配色（Fire=#ff4400, Frost=#44aaff, Arcane=#cc44ff, Shadow=#8844aa, Nature=#44ff44, Holy=#ffcc00）
- [x] 3.3 创建 `web/vfx.js`：spawnMeleeArc(target, color) — 近战挥砍弧线（非投射物，Physical 学派专用）
- [x] 3.4 创建 `web/vfx.js`：spawnDamageNumber(target, value, result) — 浮动数字（Hit=白, Crit=红大, Miss=灰, Dodge=橙, Parry=橙, Block=蓝）
- [x] 3.5 创建 `web/vfx.js`：spawnHealNumber(target, value) — 绿色 "+" 浮动数字
- [x] 3.6 创建 `web/vfx.js`：flashHit(target) — 目标闪红 200ms
- [x] 3.7 创建 `web/vfx.js`：spawnHealBeam(from, to) — 绿色光柱连接施法者和目标
- [x] 3.8 创建 `web/vfx.js`：spawnAuraRing(target, color) — 目标脚下发光圆环
- [x] 3.9 创建 `web/vfx.js`：动画自动清理 — 完成后从 scene 移除 mesh 和 DOM 元素

## 4. 动画时间线 (timeline.js)

- [x] 4.1 创建 `web/timeline.js`：AnimationTimeline 类 — 接收 trace events 队列
- [x] 4.2 创建 `web/timeline.js`：映射 spell.prepare → 施法者发光特效
- [x] 4.3 创建 `web/timeline.js`：映射 effect_launch/effect_hit → 投射物生成（延迟 200ms）
- [x] 4.4 创建 `web/timeline.js`：映射 combat.hit/crit/miss/dodge/parry/block → 浮动数字 + 受击闪红（延迟 800ms，投射物到达时）
- [x] 4.5 创建 `web/timeline.js`：映射 aura.apply → 光环特效
- [x] 4.6 创建 `web/timeline.js`：映射 cooldown.add → 通知 HUD 开始 CD 动画

## 5. HUD 界面 (style.css + index.html)

- [x] 5.1 重写 `web/index.html`：全屏 canvas 布局 + importmap 引入 Three.js + HUD 叠加层 HTML
- [x] 5.2 重写 `web/style.css`：底部动作条样式（居中，暗色背景，圆角）
- [x] 5.3 重写 `web/style.css`：技能按钮样式（名称 + 学派色点，hover/active/disabled 状态）
- [x] 5.4 重写 `web/style.css`：CD 旋转遮罩样式（conic-gradient + animation）
- [x] 5.5 重写 `web/style.css`：右上角统计面板（半透明，小字体）
- [x] 5.6 重写 `web/style.css`：名牌和 HP/MP 条样式（面向相机的 3D 叠加层）

## 6. 入口与集成 (app.js)

- [x] 6.1 重写 `web/app.js`：import 所有模块，初始化 scene/character/vfx/timeline
- [x] 6.2 重写 `web/app.js`：页面加载时 GET /api/spells + /api/units，创建角色和动作条
- [x] 6.3 重写 `web/app.js`：castSpell(spellID) — POST /api/cast，解析 trace events，推入 timeline，更新角色状态
- [x] 6.4 重写 `web/app.js`：renderActionBar(spells) — 底部动作条渲染，绑定点击事件
- [x] 6.5 重写 `web/app.js`：startCooldown(spellID, durationMs) — 按钮上启动 CD 遮罩动画
- [x] 6.6 重写 `web/app.js`：updateStats(traceEvents) — 统计面板更新（伤害/治疗/暴击/未命中/暴击率）
- [x] 6.7 重写 `web/app.js`：resetSession() — POST /api/reset，恢复所有角色状态，清除所有 VFX 和 CD

## 7. 端到端验证

- [x] 7.1 启动 `go run .`，浏览器访问 http://localhost:8080，确认 3D 场景渲染
- [x] 7.2 确认三个角色（Mage/Warrior/Target Dummy）在场景中正确显示，头顶名牌可见
- [x] 7.3 点击 Fireball，确认投射物从 Mage 飞向 Target Dummy，命中后浮动伤害数字出现
- [x] 7.4 确认 Target Dummy HP 条缩短，Mage MP 条缩短
- [x] 7.5 连续施法两次，确认第二次按钮 CD 遮罩显示且无法点击
- [x] 7.6 点击 Reset，确认所有 HP/MP 恢复、CD 清除、VFX 清除
- [x] 7.7 运行 `go test ./... -count=1` 全部通过（后端无回归）
