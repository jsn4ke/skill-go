## 1. Spelldef 字段扩展

- [x] 1.1 在 `spelldef/spelldef.go` 的 SpellInfo 中添加 `IsToggle bool` 和 `ToggleGroup string` 字段
- [x] 1.2 在 `spelldef/loader.go` 中解析 CSV 新列 `isToggle` 和 `toggleGroup`
- [x] 1.3 更新 `data/spells.csv` 添加 `isToggle` 和 `toggleGroup` 列头，现有技能填 0 和空
- [x] 1.4 更新 `api/docs.go` 文档端点添加 toggle 相关字段描述
- [x] 1.5 `go build ./...` 编译通过

## 2. Toggle 施法逻辑

- [x] 2.1 在 `spell/spell.go` Cast() 中添加 toggle 分支：检测 IsToggle → 检查 caster 是否已有同 spellID 的 toggle aura → 有则移除(off)，无则施加(on)
- [x] 2.2 在 toggle 施加逻辑中处理 ToggleGroup 互斥：查找同组其他 toggle aura 并先移除
- [x] 2.3 添加 toggle 相关 trace 事件：`toggle.activated`、`toggle.deactivated`
- [x] 2.4 确保 toggle spell 不触发 GCD 和 cooldown（在 Cast() 中跳过 CD 逻辑）
- [x] 2.5 在 `api/game_loop.go` handleCast() 中识别 toggle spell 的特殊响应格式（返回 activated/deactivated 状态）

## 3. BreakOnDamage 自动退出

- [x] 3.1 在 aura struct 中添加 `BreakOnDamage bool` 字段，通过 apply_aura effect 的 MiscValue 位标记解析
- [x] 3.2 在 `unit/unit.go` TakeDamage() 中添加回调机制或在 `api/game_loop.go` 的伤害处理后检查 BreakOnDamage toggle aura
- [x] 3.3 BreakOnDamage 触发时 emit `toggle.broken` trace event 并 RemoveAura

## 4. 测试技能数据

- [x] 4.1 添加盗贼潜行 spell (spellID: 1784)：isToggle=1, toggleGroup=""，effect=apply_aura(speedMod+stealth)，BreakOnDamage=true
- [x] 4.2 添加战士战斗姿态 (spellID: 2457)：isToggle=1, toggleGroup="warrior_stance"，effect=apply_aura(+AP buff)
- [x] 4.3 添加战士防御姿态 (spellID: 71)：isToggle=1, toggleGroup="warrior_stance"，effect=apply_aura(+armor buff)
- [x] 4.4 添加战士狂暴姿态 (spellID: 2458)：isToggle=1, toggleGroup="warrior_stance"，effect=apply_aura(+crit buff)

## 5. Web 前端 Toggle UI

- [x] 5.1 在 `web/app.js` SSE handler 中添加 `toggle.activated` / `toggle.deactivated` / `toggle.broken` 事件处理
- [x] 5.2 在 `web/config.js` action bar 配置中添加 toggle 技能按钮（潜行、三个姿态）
- [x] 5.3 在 `web/config.css` 中添加 toggle-active 按钮样式（高亮边框/glow 效果）
- [x] 5.4 更新 `web/config.html` 确保 toggle 按钮可点击

## 6. 集成测试

- [x] 6.1 编写 `spell/flow_test.go` 测试：toggle on → aura applied, toggle off → aura removed
- [x] 6.2 编写测试：ToggleGroup 互斥 — 激活同组新 toggle 自动移除旧的
- [x] 6.3 编写测试：BreakOnDamage — 受击自动退出 toggle
- [x] 6.4 编写测试：toggle 不触发 GCD/cooldown
- [x] 6.5 `go test ./...` 全部通过
- [ ] 6.6 启动服务器，Web 端验证：点击潜行按钮 on/off，切换战士姿态互斥，受击退出潜行
