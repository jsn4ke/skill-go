## Why

当前 spell web demo 完全基于文字展示（trace 事件流、数字 HP 条），用户无法直观看到"谁在打谁、法术飞过去没有、暴击了没有"。需要一个 3D 战斗场景来可视化 spell 系统的实际效果，使 demo 从开发者调试工具变成可展示的战斗模拟器。

## What Changes

- 用 Three.js 全屏 3D 场景完全替换现有三栏 HTML 布局
- 角色用彩色几何体表示（圆柱体+球体头），头顶 HP/MP 名牌
- 施法时播放投射物飞行动画（火球、冰箭等，按法术学派区分）
- 命中结果通过 3D 浮动数字展示（伤害值、CRIT、MISS、DODGE 等）
- 底部 HUD 动作条替代左侧技能面板，带 WoW 风格 CD 旋转遮罩
- 固定 45° 俯视相机，暗色场景（网格地面 + 环境光 + 方向光）
- 移除纯文字 trace 面板和单位状态面板，改为 3D 内嵌可视元素
- Go 后端 API 不变，复用现有 /api/* 端点
- 前端仍然零构建依赖（Three.js 通过 CDN 引入）

## Capabilities

### New Capabilities

- `battle-scene`: Three.js 3D 场景初始化、相机、光照、地面渲染
- `character-renderer`: 3D 角色模型创建、HP/MP 头顶名牌、角色定位
- `spell-vfx`: 投射物特效、粒子尾迹、命中闪红、浮动伤害数字
- `animation-timeline`: trace event 到动画的映射调度、事件队列管理
- `battle-hud`: 底部动作条、技能按钮、CD 遮罩、统计叠加层

### Modified Capabilities

（无现有 spec 需要修改）

## Impact

- **web/index.html**: 完全重写，改为全屏 canvas 布局 + HUD 叠加层
- **web/style.css**: 完全重写，改为 HUD 和叠加层样式
- **web/app.js**: 完全重写，改为 Three.js 入口 + API 调用
- **新增 web/scene.js, character.js, vfx.js, timeline.js, hud.js**: 前端模块文件
- **Go 后端**: 无变更
- **外部依赖**: Three.js 通过 CDN script 标签引入（无 npm）
