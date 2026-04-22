## Why

当前 spell 系统只支持"一次性"施法流程（Prepare→Cast→Hit→Finish），无法表达切换型（Toggle）技能。WoW 中战士姿态、德鲁伊变形、盗贼潜行、圣骑士光环等 toggle 技能是核心玩法。需要添加 toggle 施法流程，使同一技能可以反复按来切换 on/off 状态。

## What Changes

- **spelldef 新增字段**：`IsToggle bool` 和 `ToggleGroup string`，标记技能为 toggle 类型并支持互斥组
- **spell 施法流程**：handleCast 识别 toggle 类型，已有同组 aura 则移除（off），否则施加（on）
- **aura 退出条件**：ToggleAura 支持 `BreakOnDamage` 条件（如潜行被打退出）
- **示例技能数据**：添加盗贼潜行（独立 toggle）和战士姿态（互斥组 toggle）作为测试用例
- **Web 前端**：在 HUD 中显示当前激活的 toggle 状态，toggle 按钮有激活/未激活视觉反馈

## Capabilities

### New Capabilities
- `toggle-spell`: Toggle 施法流程 — 同一技能 ID 的 on/off 切换、ToggleGroup 互斥逻辑、BreakOnDamage 退出条件

### Modified Capabilities
- `spell-data-schema`: spelldef 结构新增 IsToggle、ToggleGroup、ToggleBreakCondition 字段
- `battle-hud`: 前端 HUD 新增 toggle 状态栏，显示当前激活的 toggle aura

## Impact

- `spelldef/spelldef.go` — 新增字段定义
- `spelldef/loader.go` — CSV 解析新字段
- `data/spells.csv` — 列扩展
- `data/spell_effects.csv` — toggle 技能数据
- `spell/spell.go` — handleCast 中的 toggle 分支
- `api/game_loop.go` — cast 命令的 toggle 逻辑
- `aura/manager.go` — toggle aura 的互斥移除
- `web/app.js` — toggle 状态 UI 更新
- `web/config.js` / `web/config.html` — toggle 按钮配置
