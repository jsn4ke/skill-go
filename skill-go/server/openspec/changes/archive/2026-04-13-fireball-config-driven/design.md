## Context

当前 Fireball 硬编码在 `game_loop.go` 的 `initSpellBook()` 中，与 Wowhead 数据不一致。`aura_update` ticker 仅处理光环过期，不处理周期性伤害 tick。`AuraEffect.PeriodicTimer` 字段存在但未被使用。

`skill.md` 是 agent 参考文档（描述 Wowhead 页面结构和字段映射规则），**不是**运行时数据源。实际数据以 CSV 格式存储在 `server/data/` 目录。

已有的 Web 配置 UI（`config.js`）通过 REST API 对 `spellBook` 进行运行时 CRUD 操作，修改仅存内存。这层能力不需要改动。

## Goals / Non-Goals

**Goals:**
- 定义 CSV 表结构：`spells.csv`（法术主表）+ `spell_effects.csv`（效果表）
- 新建 `spelldef/loader.go`，启动时解析 CSV 为 `[]SpellInfo`
- `initSpellBook()` 改为调用 CSV loader，替换硬编码
- Fireball 使用 CSV 数据（ID=38692, damage=717, mana=465）
- `aura_update` 支持周期性伤害 tick（按 `AuraEffect.PeriodicTimer` 间隔造成伤害）

**Non-Goals:**
- 不实现所有 WoW TBC 法术，仅 Fireball
- 不实现 CSV 热重载（重启加载即可）
- 不改动 Web 配置 UI 的 API 接口
- 不实现法力值系数、PVP 倍率等高级计算

## Decisions

### 1. CSV 多表结构：spells + spell_effects

**选择**: 两个 CSV 文件，通过 `spellId` 关联。

**spells.csv**（法术主表）:
```csv
spellId,name,school,castTime,cooldown,gcd,manaCost,rangeYards
38692,火球术,fire,3500,0,1500,465,35
27088,冰霜新星,frost,0,25000,1500,185,0
27126,奥术智慧,arcane,0,0,1500,700,30
```

**spell_effects.csv**（效果表）:
```csv
spellId,index,type,school,value,tickInterval,duration
38692,0,school_damage,fire,717,,
38692,1,apply_aura,fire,21,2000,8000
27088,0,school_damage,frost,99,,
27088,1,apply_aura,,,,8000
27126,0,apply_aura,arcane,40,,1800000
```

**理由**: 与 WoW DBC 表结构一致，一法术多效果自然表达，避免单行 JSON 嵌套。字符串值（fire/school_damage）对 agent 友好。

**备选**: 单表 JSON — agent 写入复杂，嵌套解析不如 CSV 直观。

### 2. CSV loader 在 spelldef 包内

**选择**: 新建 `spelldef/loader.go`，使用标准库 `encoding/csv` 解析。

**理由**: `SpellInfo` 定义在 `spelldef` 包内，loader 放同一包可直接访问内部字段。标准库 `encoding/csv` 已足够，无需额外依赖。

### 3. 字符串到枚举的映射

CSV 中存储可读字符串，loader 内置映射表：
- `school`: `"fire"` → `SchoolMaskFire`，`"frost"` → `SchoolMaskFrost`，...
- `effect type`: `"school_damage"` → `SpellEffectSchoolDamage`，`"apply_aura"` → `SpellEffectApplyAura`，...

### 4. 周期性伤害在事件循环中实现

**选择**: 扩展 `handleAuraUpdate` 逻辑，对每个 aura 检查 `PeriodicTimer > 0` 的效果，按 tick 间隔造成伤害。

```go
for _, eff := range a.Effects {
    if eff.PeriodicTimer > 0 {
        elapsed := now - timerStart
        ticks := int(elapsed / int64(eff.PeriodicTimer))
        if ticks > eff.AppliedTicks {
            // 造成伤害，更新 AppliedTicks
        }
    }
}
```

**理由**: 与现有 aura 过期逻辑同在事件循环中，无并发问题。

### 5. 新增字段

- `SpellEffectInfo.PeriodicTickInterval int32`: 从 CSV `tickInterval` 列加载，传递给 `AuraEffect.PeriodicTimer`
- `AuraEffect.AppliedTicks int32`: 记录已执行的 tick 数量，防止重复伤害

## Risks / Trade-offs

- **CSV 无 schema 验证**: 列顺序错误会导致字段错位 → loader 按列名映射（header 行），不依赖列顺序
- **PeriodicTickInterval 字段新增**: 修改 SpellEffectInfo struct → 检查是否有序列化/反序列化依赖
- **周期伤害数值**: 当前仅使用 BasePoints，不考虑法力值系数 → 可通过 Web UI 运行时调整 BasePoints 弥补
