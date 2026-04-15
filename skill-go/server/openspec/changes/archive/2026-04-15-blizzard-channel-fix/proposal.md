## Why

暴风雪（spell ID 10）的设计意图是 channel 技能（8 秒引导、1 秒 tick），后端 channel 框架和前端 channel bar UI 都已实现。但 `spells.csv` 缺少 `isChanneled`、`channelDuration`、`tickInterval` 列，CSV loader 也不解析这些字段，导致 Blizzard 加载后 `IsChanneled=false`，实际走即时伤害路径。前端 `timeline.js` 用 `lastSpellID === 10` 硬编码判断 Blizzard 跳过投射物，而非根据 spell 数据驱动。

## What Changes

- **spells.csv 增加 3 列**：`isChanneled`（bool）、`channelDuration`（int32 ms）、`tickInterval`（int32 ms），追加在现有 9 列之后
- **CSV loader 解析 channel 字段**：`parseSpellRow` 读取新增的 3 列并填充 `SpellInfo.IsChanneled`、`ChannelDuration`、`TickInterval`
- **spell_effects.csv 增加 radius 列**：用于 AoE 范围，当前 `SpellEffectInfo.Radius` 字段存在但从未从 CSV 加载
- **删除前端硬编码**：`timeline.js` 中 `lastSpellID === 10` 改为从 spell 数据读取 channel/AoE 属性，通用判断所有 channel 技能
- **spell-csv-loader spec 更新**：反映新增列

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `spell-csv-loader`: 新增 `isChanneled`、`channelDuration`、`tickInterval` 列的解析；新增 `radius` 列到 spell_effects.csv
- `channeled-spell`: 补充 channel 字段从 CSV 加载的具体列映射，确保与 CSV loader 对齐

## Impact

- `data/spells.csv` — 新增 3 列（header + Blizzard 行数据）
- `data/spell_effects.csv` — 新增 radius 列（第 12+ 列）
- `spelldef/loader.go` — `parseSpellRow` 解析 12 列，`parseEffectRow` 解析 radius
- `spelldef/spelldef.go` — 无变更（字段已存在）
- `web/timeline.js` — 删除 `lastSpellID === 10` 硬编码，改用 spell 数据判断
- `web/app.js` — 无变更（已有通用 channel 状态处理）
- 相关 spec 文件更新
