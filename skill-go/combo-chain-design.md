# Combo Chain Design

> 连招序列系统设计文档

---

## 1. 概述

连招序列（Combo Chain）是动作游戏中的核心机制：玩家连续按下攻击键，角色按固定顺序执行不同的招式（1→2→3→4→1），不可跳步，超时或被打断则重置。

**核心设计原则：连招不是独立系统，而是现有 spell 机制的组合。**

```
连招 = trigger_spell (链式触发) + aura stacks (记住第几步) + aura duration (超时重置)
```

---

## 2. 与现有机制的关系

### 2.1 对比简单 autoRepeat

| 维度 | autoRepeat (普通平A) | Combo Chain (连招) |
|------|---------------------|-------------------|
| 每次施放 | 同一个 spell | 不同的 spell |
| 序列 | 无，每次一样 | 1→2→3→4→1 循环 |
| 跳步 | 不适用 | 禁止，必须按顺序 |
| 重置 | 无概念 | 超时/打断后回到第1步 |
| 前端按钮 | 技能栏直接绑 spellID | 一个按钮，逻辑决定施放哪个 |

### 2.2 使用的现有 building blocks

```
┌──────────────────────────────────────────────────────────┐
│  trigger_spell (已有)                                    │
│  spell hit 时触发另一个 spell                            │
│  → 实现 1→2→3→4 的链式触发                             │
│                                                          │
│  aura + stacks (已有)                                    │
│  施加可叠加的 aura，stacks 记录当前第几步                 │
│  → 记住连招进度                                         │
│                                                          │
│  aura duration + refresh (已有)                          │
│  aura 有过期时间，重复施加时刷新                          │
│  → 超时自动重置连招                                     │
│                                                          │
│  delayMode=fixed + missileSpeed (已实现)                 │
│  固定延迟，模拟挥砍接触时间                              │
│  → 每一刀的伤害不是瞬间的，是挥刀碰到才打                │
│                                                          │
│  autoRepeat (已有)                                       │
│  技能结束后保持 Idle 状态等待重新触发                    │
│  → 不需要，连招由 trigger_spell 自动驱动                 │
└──────────────────────────────────────────────────────────┘
```

---

## 3. 数据定义

### 3.1 spells.csv — 连招步骤

每一步是独立的 spell，通过 trigger_spell 链接：

```csv
spellId,name,school,castTime,cooldown,gcd,manaCost,powerType,rangeYards,isChanneled,channelDuration,tickInterval,missileSpeed,delayMode
20001,横斩,physical,0,0,1500,0,1,5,0,0,0,0.3,fixed
20002,竖斩,physical,0,0,1500,0,1,5,0,0,0,0.25,fixed
20003,回旋斩,physical,0,0,1500,0,1,5,0,0,0,0.35,fixed
20004,终结击,physical,0,0,1500,20,1,5,0,0,0,0.4,fixed
```

### 3.2 spell_effects.csv — 每一步的效果

每个步骤有两个 effect：
1. **weapon_damage** — 造成伤害（数值不同，体现招式差异）
2. **trigger_spell** — 触发下一步

```csv
spellId,index,type,school,value,periodicType,amplitude,dummy1,dummy2,auraType,miscValue,triggerSpellId,radius
20001,0,weapon_damage,physical,80,0,0,0,0,0,0,0,
20001,1,trigger_spell,physical,0,0,0,0,0,0,0,20002,
20002,0,weapon_damage,physical,120,0,0,0,0,0,0,0,
20002,1,trigger_spell,physical,0,0,0,0,0,0,0,20003,
20003,0,weapon_damage,physical,100,0,0,0,0,0,0,0,
20003,1,trigger_spell,physical,0,0,0,0,0,0,0,20004,
20004,0,weapon_damage,physical,200,0,0,0,0,0,0,0,
20004,1,trigger_spell,physical,0,0,0,0,0,0,0,20001,
```

### 3.3 连招进度追踪 aura

需要一个隐藏 spell 来管理连招状态。这个 spell 不造成伤害，只负责刷新 stacks：

```csv
spellId,name,school,castTime,cooldown,gcd,manaCost,powerType,rangeYards,isChanneled,channelDuration,tickInterval,missileSpeed,delayMode
20010,连招进度,physical,0,0,0,0,1,5,0,0,0,0,
```

```csv
spellId,index,type,school,value,periodicType,amplitude,dummy1,dummy2,auraType,miscValue,triggerSpellId,radius
20010,0,apply_aura,physical,1,0,0,0,0,1,50,0,
```

- `auraType = 1` (stun 类型复用为 combo 标记)
- `miscValue = 50` (连招的 aura 类型标识，和普通 stun 区分开)
- `value = 1` (每步叠加 1 层)
- **duration 不设置** → 永久存在，由下次步骤刷新

> 注意：实际实现时可能需要新增一个专门的 auraType（如 `AuraTypeCombo = 50`），或者在 spell 级别增加一个 `comboTimeout` 字段控制超时。具体取决于后续实现。

---

## 4. 执行流程

### 4.1 完整时序（正常连招）

```
玩家按 [攻击] 键

T=0ms     前端发送 cast(当前连招对应的 spellID)
          │
          │ 查找 caster 身上的 combo aura:
          │   stacks=0 或不存在 → 施放 20001 (横斩)
          │   stacks=1          → 施放 20002 (竖斩)
          │   stacks=2          → 施放 20003 (回旋斩)
          │   stacks=3          → 施放 20004 (终结击)
          │
          ▼
       ┌──────────────────────────────────────────┐
       │  Prepare(20001 横斩)                       │
       │  checkCast: melee range ✅                 │
       │  castTime=0 → 立刻 Cast()                  │
       │  State: None → Preparing → Launched        │
       │                                           │
       │  delayMode=fixed, missileSpeed=0.3         │
       │  → 注册 delayed hit: 300ms 后触发          │
       └──────────────┬───────────────────────────┘
                      │
                      ▼
       ┌──────────────────────────────────────────┐
       │  Launch phase                             │
       │  effect[0]: weapon_damage_launch          │
       │    → trace: "挥刀动画开始 (横斩)"         │
       │  effect[1]: trigger_spell_launch          │
       │    → trace: "下一步: 竖斩"                │
       └──────────────────────────────────────────┘
                      │
                      │ 等待 300ms (刀挥到目标身上)
                      │
                      ▼
       ┌──────────────────────────────────────────┐
       │  Hit phase                                │
       │  effect[0]: weapon_damage_hit              │
       │    → melee_roll → hit                     │
       │    → damage_calc → 80 伤害                │
       │    → 应用伤害                              │
       │                                           │
       │  effect[1]: trigger_spell_hit              │
       │    → 触发 20002 (竖斩)                     │
       │    → 但竖斩不立刻施放!                     │
       │    → 只是将 20002 标记为 "待触发"          │
       │    → 等玩家下次按攻击键                    │
       │                                           │
       │  (同时刷新 combo aura: stacks=1)           │
       └──────────────┬───────────────────────────┘
                      │
                      ▼
                  StateFinished
                      │
                      │ 玩家再次按 [攻击] 键
                      ▼
       ┌──────────────────────────────────────────┐
       │  combo aura stacks=1 → 施放 20002 (竖斩) │
       │  重复同样的流程...                         │
       │  hit 时: stacks=1 → 2                     │
       └──────────────────────────────────────────┘
```

### 4.2 触发 vs 立即施放

**关键区分：trigger_spell 在 hit 时触发，但不等于立即施放下一步。**

```
trigger_spell 的两种模式:

  模式 A — 立即施放 (如当前冲锋眩晕):
    冲锋 hit → 立即 Cast(7922) → 立即 hit → 眩晕
    适合: 被动效果、附带的即时触发

  模式 B — 标记为待触发 (连招用):
    横斩 hit → 标记"下一步=竖斩" → 等玩家按键
    适合: 需要玩家主动操作的连招下一步
```

连招中，trigger_spell 应该是模式 B：hit 时设置连招进度，但不自动施放下一步。下一步需要玩家**主动按下攻击键**才施放。

如果想要全自动连招（按住不放就自动 1→2→3→4→1），则需要模式 A + 前端长按检测。

### 4.3 超时重置

```
玩家按 [攻击] → 横斩(hit) → combo stacks=1

  ... 玩家去做别的事，3秒没按 ...

T=3000ms  combo aura 过期 → stacks 清零

玩家再次按 [攻击]
  → stacks=0 → 从头开始: 横斩
```

### 4.4 被打断重置

```
横斩 hit → combo stacks=1
  → 玩家被眩晕 (Cancel 机制)
  → 清除 combo aura
  → stacks=0

下次按攻击 → 从头开始
```

### 4.5 目标切换时的行为

```
横斩 hit 目标A → combo stacks=1

玩家切换到目标B:
  方案1: combo 重置 (大多数动作游戏的做法)
    → 换目标 = 换连招，从横斩重新开始
  方案2: combo 保留 (某些游戏允许)
    → stacks 不受目标切换影响
```

---

## 5. delayMode 字段

### 5.1 新增字段说明

`delayMode` 控制 `missileSpeed` 字段的含义：

| delayMode | missileSpeed 含义 | 延迟计算 | 适用场景 |
|-----------|------------------|---------|---------|
| `distance` (默认) | 弹道速度 (码/秒) | `distance / missileSpeed * 1000` | 火球, 寒冰箭 |
| `fixed` | 固定延迟 (秒) | `missileSpeed * 1000` | 近战挥砍, 特殊弹道 |

### 5.2 不指定 delayMode 的行为

向后兼容：如果 `delayMode` 为空或缺失，默认为 `distance` 模式（当前火球的行为不变）。

### 5.3 CSV 示例

```csv
# 弹道模式 (距离相关)
38692,火球术,fire,3500,...,14,distance

# 固定延迟模式 (近战挥砍)
20001,横斩,physical,0,...,0.3,fixed
```

---

## 6. 前端集成

### 6.1 攻击按钮逻辑

```
用户按 [攻击] 按钮:

1. 读取当前 caster 的 auras
2. 找到 combo aura (auraType=50)
3. 根据 stacks 决定施放哪个 spell:
   - 无 combo aura 或 stacks=0 → cast(20001)
   - stacks=1 → cast(20002)
   - stacks=2 → cast(20003)
   - stacks=3 → cast(20004)
4. 发送 cast 请求
```

### 6.2 前端不需要知道 combo 概念

前端只需要：
- 一个 [攻击] 按钮
- 一个查找函数：`getComboSpellID(casterAuras)`
- 其余全是后端 spell 系统自动处理

### 6.3 连招进度显示

前端可以从 caster 的 auras 中读取 combo stacks 来显示进度：
- stacks=0: 无连招
- stacks=1: 显示第1段高亮
- stacks=2: 显示第1-2段高亮
- stacks=3: 显示第1-3段高亮

---

## 7. 完整例子：战士四段连招

### 7.1 技能数据

| spellId | 名称 | 伤害 | 挥砍延迟 | 触发下一步 | 连招特效 |
|---------|------|------|---------|-----------|---------|
| 20001 | 横斩 | 80 | 300ms | 20002 | 横向挥砍 |
| 20002 | 竖斩 | 120 | 250ms | 20003 | 竖向劈砍 |
| 20003 | 回旋斩 | 100 | 350ms | 20004 | 旋转攻击 |
| 20004 | 终结击 | 200 | 400ms | 20001 | 重击收尾 |

### 7.2 完整时序图

```
         玩家操作         服务端                              目标HP
         ───────         ──────                              ──────

按攻击! ──▶ Prepare(20001)                      
           Cast() → Launch                     
           [挥刀动画开始] 300ms                 
                          ▼                  
                         Hit: 80 伤害                  15000→14920
                         combo stacks=1
                         trigger: 20002 就绪
                          │                          
按攻击! ────────────────▶│                          
           Prepare(20002) │                          
           Cast() → Launch │                          
           [竖刀动画开始] 250ms                     
                          ▼                          
                         Hit: 120 伤害                 14920→14800
                         combo stacks=2
                         trigger: 20003 就绪
                          │                          
按攻击! ────────────────▶│                          
           Prepare(20003) │                          
           Cast() → Launch │                          
           [回旋动画开始] 350ms                     
                          ▼                          
                         Hit: 100 伤害                 14800→14700
                         combo stacks=3
                         trigger: 20004 就绪
                          │                          
按攻击! ────────────────▶│                          
           Prepare(20004) │                          
           Cast() → Launch │                          
           [重击动画开始] 400ms                     
                          ▼                          
                         Hit: 200 伤害                 14700→14500
                         combo stacks=4 → 溢出 → 重置
                         trigger: 20001 就绪
                          │                          
                          │     ... 3秒没按 ...
                          │                          
按攻击! ────────────────▶│     combo 超时, stacks=0
           Prepare(20001) │     从头开始!
           Cast() → Launch │                          
           [横刀动画开始] 300ms                     
                          ▼                          
                         Hit: 80 伤害                  14500→14420
```

### 7.3 被打断的时序

```
按攻击! ──▶ 横斩 hit (combo=1) ──▶ 按攻击! ──▶ 竖斩 hit (combo=2)
                                                        │
                                     目标释放眩晕打断! ──┘
                                                        │
                                                  combo 清除
                                                  stacks=0
                                                        │
按攻击! ─────────────────────────────────────────────▶│
           combo=0 → 横斩 (从头开始)
```

---

## 8. 扩展场景

### 8.1 不同武器的连招

```csv
# 单手剑: 4 段
20001,剑-横斩, ...
20002,剑-竖斩, ...
20003,剑-回旋, ...
20004,剑-终结, ...

# 双手斧: 3 段 (更慢更重)
20101,斧-劈砍, ..., 0.5, fixed
20102,斧-横扫, ..., 0.6, fixed
20103,斧-重击, ..., 0.7, fixed

# 匕首: 5 段 (更快更轻)
20201,匕-刺, ..., 0.15, fixed
20202,匕-划, ..., 0.15, fixed
20203,匕-挑, ..., 0.2, fixed
20204,匕-旋, ..., 0.15, fixed
20205,匕-背刺, ..., 0.2, fixed
```

### 8.2 技能+连招组合

```
冲锋 (spell 100) hit 后:
  → trigger_spell: 连招-横斩 (20001)
  → 冲锋结束后自动进入连招模式
  → 玩家继续按攻击键就能接着连招

火球术 (spell 38692):
  → 无连招, 正常弹道流程
```

### 8.3 职业切换连招

不同职业的 [攻击] 按钮绑定不同的连招链：

```
战士: 20001→20002→20003→20004 (剑四段)
盗贼: 20201→20202→20203→20204→20205 (匕首五段)
法师: 无连招, 改为法术序列 (火球→冰枪→火冲)
```

前端根据当前职业/caster 的 combo aura 类型决定走哪条连招链。

---

## 9. 实现检查清单

### 服务端

- [ ] `spelldef/spelldef.go`: 新增 `DelayMode string` 字段
- [ ] `spelldef/loader.go`: 解析 `delayMode` 列 (col 13)
- [ ] `data/spells.csv`: 新增 `delayMode` 列
- [ ] `spell/spell.go`: `startDelayedHit()` 中根据 delayMode 选择计算方式
- [ ] `spell/spell.go`: trigger_spell 效果支持 "标记待触发" 模式 (combo 用)
- [ ] 连招进度 aura 的定义 (新增 auraType 或复用现有)

### 前端

- [ ] 攻击按钮: 读取 combo aura stacks → 决定 cast 哪个 spellID
- [ ] 连招进度 UI: 显示当前第几段
- [ ] 超时视觉反馈: combo 断裂时的提示

---

## 10. 与 WoW 原版 auto-attack 的对比

| 维度 | WoW auto-attack (原版) | 本方案 |
|------|----------------------|--------|
| 系统位置 | 独立 CombatHandler | spell 系统内 |
| 伤害延迟 | 服务器即时, 客户端动画 | 服务器延迟 (fixed delay) |
| 序列变化 | 每次一样 | 1→2→3→4 循环 |
| 双手分别计时 | 主手/副手独立 | 可拆成两条独立连招链 |
| 与 GCD 关系 | 不触发 GCD | 可配 (建议不触发) |
| 资源消耗 | 无 | 可配 (终结击消耗 20 怒气) |
