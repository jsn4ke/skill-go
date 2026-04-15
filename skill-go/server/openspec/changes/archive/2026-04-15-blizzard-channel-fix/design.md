## Context

当前暴风雪（spell ID 10）在 `spells.csv` 中只有 9 列，缺少 channel 相关字段。后端 `SpellInfo` 已定义 `IsChanneled`、`ChannelDuration`、`TickInterval`，`SpellEffectInfo` 已定义 `Radius`，但 CSV loader 不解析这些字段，全部为零值。前端 `timeline.js` 用 `lastSpellID === 10` 硬编码跳过投射物动画。

## Goals / Non-Goals

**Goals:**
- `spells.csv` 新增 `isChanneled`、`channelDuration`、`tickInterval` 三列，CSV loader 解析并填充 SpellInfo
- `spell_effects.csv` 新增 `radius` 列，CSV loader 解析并填充 SpellEffectInfo
- 前端 `timeline.js` 根据 spell 的 channel 属性（非 spell ID）判断是否跳过投射物
- 暴风雪通过 CSV 数据驱动实现 8 秒引导、1 秒 tick 的 channel 行为

**Non-Goals:**
- 不修改后端 channel 框架（已完整实现）
- 不修改前端 channel bar UI（已完整实现）
- 不新增其他 channel 技能（仅修复暴风雪）
- 不修改 YAML spell-data-schema（那是独立的数据源格式）

## Decisions

### 1. spells.csv 列追加位置

**决策**: 在现有 9 列之后追加 `isChanneled`、`channelDuration`、`tickInterval`（第 10、11、12 列）。

**理由**: 保持向后兼容，旧 9 列格式不受影响（新增列为空时为零值）。loader 已有 padding 逻辑，会将不足列数的行补齐。

**替代方案**: 新建 `spell_channels.csv` 单独存储 — 拒绝，因为 channel 是 spell 的固有属性，不应拆分文件。

### 2. isChanneled 布尔值格式

**决策**: 使用 `1`/`0` 整数（非空时 `parseInt != 0` 为 true），与 `powerType` 等字段风格一致。空字符串为 false。

**理由**: CSV 中布尔值常见格式，Go 端 `strconv.ParseInt` + `!= 0` 即可。

### 3. spell_effects.csv radius 列位置

**决策**: 在现有 12 列之后追加为第 13 列（`radius`）。

**理由**: 现有 12 列已固定（spec 文档约束），追加不影响已有解析逻辑。loader padding 到 13 列即可。

### 4. 前端去硬编码策略

**决策**: 将 spell 的 `isChanneled` 和 `channelDuration` 属性存入 `window.__spellMap`（已有机制），`timeline.js` 通过 `spellMap_entry(lastSpellID)` 读取 channel 属性判断行为。

**理由**: spell 数据已通过 `/api/spells` 接口返回并缓存在 `spellMap` 中，无需额外 API。

## Risks / Trade-offs

- **[CSV 格式变更]** spells.csv 从 9 列变为 12 列 → loader padding 逻辑已处理（从 9 改为 12），旧格式行自动补零值，无需迁移
- **[前端 spellMap 依赖]** timeline.js 依赖 `window.__spellMap` 有数据 → `app.js` 中 `loadSpells()` 在 `initScene` 前调用，时序已保证
- **[Radius 列追加]** spell_effects.csv 从 12 列变为 13 列 → 同上，padding 已处理
