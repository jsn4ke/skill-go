## 1. CSV 数据

- [x] 1.1 在 `data/spells.csv` 添加冲锋行：spellId=100, name=冲锋, physical, castTime=0, cooldown=15000, gcd=0, manaCost=0, range=25
- [x] 1.2 在 `data/spell_effects.csv` 添加冲锋效果：spellId=100, index=0, type=trigger_spell, triggerSpellId=7922（TBC 原版有 3 个效果，简化为 trigger_spell 触发眩晕 + 代码中处理位移）
- [x] 1.3 在 `data/spells.csv` 添加冲锋眩晕行：spellId=7922, name=冲锋眩晕, physical, castTime=0, cooldown=0, gcd=0, manaCost=0, range=25
- [x] 1.4 在 `data/spell_effects.csv` 添加冲锋眩晕效果：spellId=7922, index=0, type=apply_aura, auraType=debuff, miscValue=UnitStateStunned, duration=1500

## 2. trigger_spell 效果支持

- [x] 2.1 确认引擎已支持 `trigger_spell` effect type，命中时自动对目标施放 triggerSpellId 指定的法术。如不支持，在 `effect/effects.go` 中添加 trigger_spell handler
- [x] 2.2 在 `aura/manager.go` 的 `applyEffect` 中为 `AuraTypeDebuff` 添加 `MiscValue=UnitStateStunned` 的处理（`applyControlEffect` 已支持 stun，确认可复用）
- [x] 2.3 确保 `AuraTypeDebuff` 的 CSV 配置能通过 `MiscValue` 字段传递 stun 状态值。如需新增 CSV 列 `miscValue`，更新 `spelldef/loader.go` 的 `parseEffectRow`

## 3. 施法者位移

- [x] 3.1 在 `api/game_loop.go` 的 `processCastResult` 或 `handleCastComplete` 中，检测冲锋技能命中后，计算目标面朝方向，将施法者 `Position` 设为 `target.Position - forward * 1.0`
- [x] 3.2 在 `handleAuraUpdate` 中，受眩晕目标不可行动（如尝试位移或施法时返回错误）

## 4. 前端更新

- [x] 4.1 `web/app.js` 的 `reconcileUnits` 已有位置更新逻辑，确认位移后 3D 模型跟随移动
- [x] 4.2 在 `web/character.js` 中为被眩晕目标添加视觉指示（如头顶显示"眩晕"文字）

## 5. 测试

- [x] 5.1 `go test ./...` 通过
- [ ] 5.2 手动测试：选中目标 → 施放冲锋 → 施法者瞬移 → 目标眩晕 1 秒 → 自动恢复
