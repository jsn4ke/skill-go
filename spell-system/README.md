# TrinityCore 技能系统技术架构

本目录包含 TrinityCore 技能系统（Spell System）的完整技术设计文档，覆盖从施法到效果执行的完整生命周期。

## 文档索引

| # | 文档 | 覆盖范围 | 行数 |
|---|------|----------|------|
| 01 | [施法状态机](01-spell-state-machine.md) | Spell 生命周期、6 状态 FSM、prepare/cast/finish 流程、CheckCast 检查链 | 546 |
| 02 | [目标选择系统](02-target-selection.md) | 5 维度分解、显式/隐式目标、Area/Cone/Chain/Line/Traj 算法、TargetA+B 组合 | 729 |
| 03 | [效果处理管线](03-effect-pipeline.md) | Launch/Hit 两阶段、分发表、~120 种效果处理器、延迟/蓄力施法、伤害计算 | 890 |
| 04 | [Aura 系统架构](04-aura-architecture.md) | 三层结构、生命周期、堆叠规则、Proc 管线、PPM、充能管理 | 936 |
| 05 | [脚本扩展系统](05-spell-scripting.md) | 注册/加载、18 个 SpellScript hook、AuraScript hooks、类型擦除、PreventDefault | 742 |
| 06 | [冷却与充能管理](06-spell-history.md) | 4 子系统、充能队列、GCD、法术系锁定、暂停/恢复、持久化 | 760 |

## 阅读顺序

**入门路径**（按技能施放的执行顺序）：
```
01-spell-state-machine  →  02-target-selection  →  03-effect-pipeline
        ↓                        ↓                       ↓
  理解施法过程             理解目标怎么选           理解效果怎么执行
        ↓                        ↓                       ↓
  06-spell-history        04-aura-architecture    05-spell-scripting
  冷却/充能什么时候管      持续效果怎么管           怎么扩展/自定义
```

**按需查阅路径**：
- 想实现自定义技能 → 05-spell-scripting
- 想理解某个技能效果怎么工作的 → 03-effect-pipeline → 对应的 Effect* 方法
- 想理解目标为什么选不中 → 02-target-selection
- 想理解 Aura 堆叠问题 → 04-aura-architecture
- 想理解冷却/充能异常 → 06-spell-history

## 代码位置

```
src/server/game/Spells/
├── Spell.h / Spell.cpp           — 施法状态机、目标选择、效果管线
├── SpellInfo.h / SpellInfo.cpp   — 技能原型（只读数据）、目标维度分解
├── SpellDefines.h                — 枚举、标志、CastSpellTargets
├── SpellScript.h / SpellScript.cpp — 脚本扩展系统基类
├── SpellEffects.cpp              — 效果处理器分发表与实现
├── SpellMgr.h / SpellMgr.cpp     — 全局管理、DBC 加载
├── SpellHistory.h / SpellHistory.cpp — 冷却与充能管理
├── SpellCastRequest.h            — 客户端施法请求解析
├── TraitMgr.h / TraitMgr.cpp     — 天赋树系统
└── Auras/
    ├── SpellAuras.h / SpellAuras.cpp     — Aura/AuraApplication
    ├── SpellAuraEffects.h / .cpp         — AuraEffect
    └── SpellAuraDefines.h               — 枚举与标志
```

## 文档规范

- 所有文档遵循 5 段式结构：概述 → 核心数据结构 → 状态流转/执行流程 → 关键算法与分支 → 扩展点与脚本集成
- 代码引用格式：`src/server/game/Spells/Spell.cpp:3854`（文件:行号）
- 状态流转使用 ASCII art 图
