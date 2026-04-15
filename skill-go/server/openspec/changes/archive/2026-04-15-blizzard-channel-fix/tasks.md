## 1. CSV 数据更新

- [x] 1.1 `data/spells.csv` header 追加 `isChanneled,channelDuration,tickInterval` 三列，Blizzard 行填 `1,8000,1000`，其余行补空
- [x] 1.2 `data/spell_effects.csv` header 追加 `radius` 列，Blizzard school_damage 行填 `8`，其余行补空

## 2. CSV Loader 解析

- [x] 2.1 `spelldef/loader.go` `LoadSpells` padding 从 9 改为 12（spells.csv 行补齐到 12 列）
- [x] 2.2 `spelldef/loader.go` `parseSpellRow` 解析 col[9] `isChanneled`（非空且 parseInt!=0 则 true）、col[10] `channelDuration`、col[11] `tickInterval`
- [x] 2.3 `spelldef/loader.go` `LoadSpells` padding spell_effects.csv 行从 10 改为 13
- [x] 2.4 `spelldef/loader.go` `parseEffectRow` 解析 col[12] `radius`（parseFloat64 填入 `eff.Radius`）

## 3. 前端去硬编码

- [x] 3.1 `web/timeline.js` 删除 `const isChanneledAoE = lastSpellID === 10` 硬编码，改为从 `spellMap_entry(lastSpellID)` 读取 `isChanneled` + `radius > 0` 判断是否跳过投射物

## 4. 验证

- [x] 4.1 启动服务器，确认 Blizzard 施法触发 channel bar（8 秒倒计时）
- [x] 4.2 确认 Fireball 等非 channel 技能行为不变（投射物 + 延迟伤害）
- [x] 4.3 确认无 spell ID 硬编码残留（grep `=== 10` 或 `spell.*10` 在 timeline.js 中无结果）
