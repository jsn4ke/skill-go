## Why

当前 spells.csv 中包含5个技能（火球术、冰霜新星、奥术智慧、冲锋、冲锋眩晕），但只有火球术和冲锋已完整实现并通过验证。冰霜新星和奥术智慧虽已配置但未实现相关特效和测试，保留会造成维护负担和困惑。

## What Changes

从 data/spells.csv 和 data/spell_effects.csv 中移除冰霜新星(spellId=27088)和奥术智慧(spellId=27126)及其所有关联效果。同步更新 loader_test.go 中的测试数据。

## Capabilities

### New Capabilities
（无）

### Modified Capabilities
（无，仅清理数据，不改变行为）

## Impact

- `data/spells.csv` — 删除2行
- `data/spell_effects.csv` — 删除2行
- `spelldef/loader_test.go` — 移除使用27088/27126的测试数据
- `skill.md` — 可选清理参考数据
