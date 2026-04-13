## Why

当前 Fireball 技能硬编码在 `game_loop.go` 的 `initSpellBook()` 中（ID=42833, damage=888, mana=400），与 Wowhead 实际数据（ID=38692, damage=717, mana=465）不一致。需要将技能数据落地到 CSV 配置表（`server/data/`），启动时加载，使技能参数可版本化、可由 agent 维护，并为后续技能扩展奠定基础。

`skill.md` 是 agent 参考文档（告知如何从 Wowhead 获取法术数据及字段映射规则），不是运行时数据源。

## What Changes

- 新增 CSV 配置文件：`server/data/spells.csv`（法术主表）和 `server/data/spell_effects.csv`（效果表）
- 新增 CSV loader（`spelldef/loader.go`），启动时解析 CSV 为 `[]SpellInfo`
- 替换 `initSpellBook()` 中的 Fireball 硬编码为从 CSV 加载
- 实现 Fireball 的完整效果链：school_damage + apply_aura(periodic_damage) 含 tick 机制
- Web UI 运行时调整仅存内存，重启后从 CSV 恢复（已有能力）

## Capabilities

### New Capabilities
- `spell-csv-loader`: 从 `server/data/` CSV 文件解析 spell 数据为 SpellInfo struct 的加载器，包含多表关联、字段映射、类型转换
- `periodic-damage-aura`: 周期性伤害光环效果（DoT），按 tickInterval 间隔在事件循环中造成伤害

### Modified Capabilities
（无已有 spec 需要修改）

## Impact

- `server/data/spells.csv` — 新增，法术主表数据文件
- `server/data/spell_effects.csv` — 新增，法术效果表数据文件
- `spelldef/spelldef.go` — 新增 PeriodicTickInterval 字段
- `spelldef/loader.go` — 新增，CSV 解析器
- `api/game_loop.go` — 替换 `initSpellBook()` 硬编码为加载器调用
- `aura/types.go` — 新增 AppliedTicks 字段
- `api/server.go` — 更新 makeAuraHandler 传递 PeriodicTickInterval
