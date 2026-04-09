# skill-go

用 Go 复刻 TrinityCore 法术系统的核心逻辑。

## 目标

基于对 TrinityCore 法术系统的深度分析，以纯 Go 实现以下子系统：

- 施法状态机
- 效果管线
- 目标选择系统
- Aura 系统
- 脚本扩展系统
- 冷却与充能管理

## 参考文档

概念性设计文章（零代码引用，适合作为复刻指南）：

| 子系统 | 文章 |
|--------|------|
| 施法状态机 | [spell-state-machine-article.md](../spell-system/spell-state-machine-article.md) |
| 效果管线 | [spell-effect-pipeline-article.md](../spell-system/spell-effect-pipeline-article.md) |
| 目标选择 | [target-selection-article.md](../spell-system/target-selection-article.md) |
| Aura 架构 | [aura-architecture-article.md](../spell-system/aura-architecture-article.md) |
| 脚本扩展 | [spell-scripting-article.md](../spell-system/spell-scripting-article.md) |
| 冷却与充能 | [spell-cooldown-charge-article.md](../spell-system/spell-cooldown-charge-article.md) |

## 目录结构

```
skill-go/
├── server/   # 服务端——技能核心逻辑
├── client/   # 模拟客户端——测试和验证服务端逻辑
└── go.work   # Go workspace
```

## 快速开始

```bash
# 运行服务端
cd server && go run .

# 运行模拟客户端
cd client && go run .
```
