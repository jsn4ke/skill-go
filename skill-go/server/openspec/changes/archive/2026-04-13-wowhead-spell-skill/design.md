## Context

skill-go 模拟器当前通过 Go 代码硬编码法术属性（Spell struct 中的 Name、Damage、ManaCost、CastTime 等字段直接赋值）。Wowhead TBC CN 页面提供了完整的法术数据（spell=38692 火球术等），但这些数据是非结构化的 HTML 页面，无法直接被程序或 agent 使用。

需要定义一个结构化的 `skill.md` 文件格式，将 Wowhead 的法术属性映射为 AI agent 可解析的数据语言（YAML），作为 spell 数据的单一数据源。

## Goals / Non-Goals

**Goals:**
- 定义清晰的 YAML schema，覆盖 Wowhead 法术页面的所有关键字段
- 以 Fireball (38692) 为完整示例
- schema 设计兼顾人工可读性和 agent 可解析性
- 效果列表支持 WoW TBC 的所有效果类型枚举

**Non-Goals:**
- 不实现从 Wowhead 自动抓取数据的爬虫
- 不实现 skill.md 到 Go Spell struct 的运行时加载器（后续 change）
- 不覆盖所有 TBC 法术，仅定义 schema 和示例

## Decisions

### 1. 使用 YAML 而非 JSON

**选择**: YAML 作为 skill.md 的格式。

**理由**: YAML 对人类更友好，支持注释，缩进结构更适合嵌套的效果列表。Agent 解析 YAML 与 JSON 同样简单。JSON 的优势（机器生成/解析）在此场景不适用。

**备选**: JSON — 更严格的 schema 验证，但可读性差。

### 2. 字段命名使用英文 camelCase

**选择**: 所有字段名使用英文 camelCase（如 `spellId`, `castTime`, `manaCost`）。

**理由**: 与 Go 代码中的 Spell struct 字段名保持一致，减少加载时的映射成本。中文名称放在 `name` 字段的值中。

**备选**: 中文 snake_case（如 `法力消耗`）— agent 可读但与代码不匹配。

### 3. 效果列表使用类型判别联合 (tagged union)

**选择**: 每个 effect 包含 `type` 字段作为判别键，根据 type 决定其他字段的存在。

```yaml
effects:
  - type: school_damage
    school: fire
    value: 717
    pvpMultiplier: 1
  - type: apply_aura
    auraType: periodic_damage
    value: 21
    tickInterval: 2  # 秒
    pvpMultiplier: 1
```

**理由**: WoW TBC 有 100+ 种效果类型，不可能用固定字段表示。tagged union 模式在 YAML 中通过 type 字段实现，agent 可根据 type 知道需要哪些字段。

### 4. 数值字段使用 number，范围使用字符串

**选择**: `manaCost: 465`（数字），`range: "35码"`（字符串）。

**理由**: 数值字段用于计算，必须是数字。范围包含单位（码/ yards）且有些是"自身体范围"等描述性文本，保留为字符串。

### 5. 文件组织：一个法术一个 YAML 文档

**选择**: `skill.md` 中每个法术用 `---` 分隔符分隔，形成多文档 YAML 文件。

```yaml
# spell=38692 火球术
spellId: 38692
name: 火球术
---
# spell=133 冰霜新星
spellId: 133
name: 冰霜新星
```

**理由**: 方便增量添加新法术，避免单一大 JSON 的合并冲突。

## Risks / Trade-offs

- **Wowhead 数据变更**: TBC Classic 的数据相对稳定，但如果 Wowhead 页面结构调整，手动维护的字段映射可能失效 → schema 中预留 `rawNote` 字段存放额外备注
- **效果类型枚举不全**: TBC 有 100+ 种 effect type，初始版本只覆盖常用的 10-15 种 → schema 允许 `type` 为任意字符串，agent 可处理未知类型
- **范围字段解析**: 字符串格式（"35码"、" melee range"）不便于程序计算 → 预留 `rangeYards: 35` 数值字段，字符串仅作显示用
