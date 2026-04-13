## Why

skill-go 模拟器需要精确的 WoW TBC 法术数据来驱动施法、光环、周期性效果等核心机制。当前法术数据硬编码在 Go 代码中（如 Fireball 的伤害值、持续时间、法力消耗等），修改或新增法术需要改代码并重新编译。我们需要一个结构化的 `skill.md` 文件，从 Wowhead TBC 页面提取法术信息，重新组织为 AI agent 能理解的数据语言，使法术配置数据驱动化。

## What Changes

- 新建 `skill.md`：定义 WoW TBC 法术的结构化数据格式，将 Wowhead 页面的法术属性映射为 agent 可解析的 YAML/JSON schema
- 覆盖的法术属性：名称、持续时间、类型（魔法学校）、机制、驱散类型、法力消耗、范围、施法时间、冷却、GCD、效果列表（效果类型、数值、PVP 倍率、tick 间隔）、标记/flags
- 提供示例：以 Fireball (spell=38692) 为模板，展示完整的数据结构
- 定义扩展规则：其他法术如何按此格式补充

## Capabilities

### New Capabilities
- `spell-data-schema`: 定义 Wowhead 法术数据的结构化 schema，包括字段规范、类型定义、效果枚举，以及 skill.md 文件的格式约定

### Modified Capabilities
（无已有 spec 需要修改）

## Impact

- `skill.md` — 新增文件，法术数据定义
- 后续可实现一个加载器，将 skill.md 中的数据加载到 skill-go 的 Spell struct 中，替代硬编码
- 依赖 Wowhead TBC CN 数据源 (https://www.wowhead.com/tbc/cn/spells)
