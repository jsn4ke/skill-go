# 01 - 法术状态机 (Spell State Machine)

## 1. 概述

TrinityCore 的法术系统基于有限状态机 (Finite State Machine) 模型设计。每一个 `Spell` 对象从创建到销毁都严格遵循一组预定义的状态流转路径。状态机保证了法术在施法准备、施放延迟、飞行物命中、引导持续、正常完成或被中断等场景下的行为一致性。

核心状态定义位于 `src/server/game/Spells/Spell.h:254-262`：

```cpp
enum SpellState
{
    SPELL_STATE_NULL        = 0,   // 初始/默认状态
    SPELL_STATE_PREPARING   = 1,   // 正在施法（读条中）
    SPELL_STATE_LAUNCHED    = 2,   // 已施出（飞行物/延迟命中中）
    SPELL_STATE_CHANNELING  = 3,   // 引导中
    SPELL_STATE_FINISHED    = 4,   // 已完成/已终止
    SPELL_STATE_IDLE        = 5,   // 空闲（仅自动重复射击）
};
```

状态机的驱动入口是 `Spell::update(uint32 difftime)` (`src/server/game/Spells/Spell.cpp:4213-4330`)，该方法由 `SpellEvent` 事件系统周期性调用，每帧递减计时器并在条件满足时推进状态。

本文档将覆盖：
- 6 个状态的完整生命周期与所有转换路径
- `prepare()`、`update()`、`cast()`/`_cast()`、`handle_immediate()`、`handle_delayed()`、`finish()`、`cancel()` 七个核心流程
- `CheckCast()` 验证链
- 4 个目标容器（TargetInfo、GOTargetInfo、ItemTargetInfo、CorpseTargetInfo）
- 赋能法术 (Empower Spell) 机制
- 脚本集成扩展点

---

## 2. 核心数据结构

### 2.1 SpellValue — 法术运行时数值

`src/server/game/Spells/Spell.h:239-252`

`SpellValue` 是法术对象持有的运行时数值容器，存储法术的实际基础点数（BasePoint）、效果乘数、持续时间等动态计算结果。它由施法参数初始化，可在脚本系统中被修改。

```cpp
struct SpellValue
{
    int32  EffectBasePoints[MAX_SPELL_EFFECTS] = { };
    float  EffectMultiplier[MAX_SPELL_EFFECTS] = { };
    float  EffectChainAmplitude[MAX_SPELL_EFFECTS] = { };
    Optional<int32> Duration;
    float  DurationMul = 1.0f;
    // ...
};
```

关键字段：
- `EffectBasePoints[i]`：第 i 个效果的基础点数，覆盖 DBC 中的默认值
- `Duration`：可选的自定义持续时间
- `DurationMul`：持续时间乘数

### 2.2 CastSpellExtraArgs — 施法额外参数

`src/server/game/Spells/SpellDefines.h:483-545`

当服务器主动施放法术（如 Aura 触发、技能联动）时，通过此结构体传递额外上下文：

```cpp
struct CastSpellExtraArgs : public CastSpellExtraArgsInit
{
    TriggerCastFlags    TriggerFlags;
    Difficulty          CastDifficulty;
    Item*               CastItem;
    Spell const*        TriggeringSpell;
    AuraEffect const*   TriggeringAura;
    ObjectGuid          OriginalCaster;
    std::vector<SpellValueOverride> SpellValueOverrides;
    std::any            CustomArg;
    Scripting::v2::ActionResultSetter<SpellCastResult> ScriptResult;
    bool                ScriptWaitsForSpellHit;
};
```

关键字段：
- `TriggerFlags`：控制触发施法的跳过行为（跳过冷却、跳过消耗、跳过目标检查等）
- `SpellValueOverrides`：修改 SpellValue 中的特定字段
- `TriggeringSpell` / `TriggeringAura`：标记触发来源，用于递归判断和光环关联
- `ScriptResult`：v2 脚本系统异步回调，当 `ScriptWaitsForSpellHit` 为 true 时延迟到命中阶段返回结果

### 2.3 EmpowerData — 赋能数据

`src/server/game/Spells/Spell.h:774-782`

赋能法术（如龙希尔职业的蓄力法术）在引导期间根据蓄力时长动态提升效果等级：

```cpp
struct EmpowerData
{
    Milliseconds MinHoldTime = 0ms;              // 最小蓄力时间
    std::vector<Milliseconds> StageDurations;     // 每阶段持续时间
    int32 CompletedStages = 0;                    // 当前完成的阶段数
    bool IsReleasedByClient = false;              // 客户端是否释放
    bool IsReleased = false;                      // 是否已释放
};
```

赋能法术在 `handle_immediate()` 中初始化阶段持续时间（`src/server/game/Spells/Spell.cpp:3989-4008`），在 `update()` 的 CHANNELING 分支中逐阶段推进（`src/server/game/Spells/Spell.cpp:4276-4312`）。

### 2.4 TargetInfoBase 层次结构 — 目标信息容器

`src/server/game/Spells/Spell.h:833-903`

法术的目标存储采用 4 个并行容器，每种目标类型有独立的结构体，均继承自 `TargetInfoBase` 基类：

```
TargetInfoBase (虚基类)
├── TargetInfo         — Unit 目标 (m_UniqueTargetInfo)
├── GOTargetInfo       — GameObject 目标 (m_UniqueGOTargetInfo)
├── ItemTargetInfo     — Item 目标 (m_UniqueItemInfo)
└── CorpseTargetInfo   — Corpse 目标 (m_UniqueCorpseTargetInfo)
```

基类接口：
- `PreprocessTarget(Spell*)`：命中前预处理（计算减益递减、光环持续时间等）
- `DoTargetSpellHit(Spell*, SpellEffectInfo const&)`：执行效果命中逻辑
- `DoDamageAndTriggers(Spell*)`：处理伤害、治疗和触发器

| 容器 | 成员 | 延迟命中 | 用途 |
|------|------|---------|------|
| `m_UniqueTargetInfo` | `TargetInfo` | 支持 (`TimeDelay`) | 存储所有受影响的 Unit，包括命中判定、暴击、反射等完整信息 |
| `m_UniqueGOTargetInfo` | `GOTargetInfo` | 支持 (`TimeDelay`) | 存储受影响的 GameObject |
| `m_UniqueItemInfo` | `ItemTargetInfo` | 不支持 | 存储受影响的物品（如附魔、炼金） |
| `m_UniqueCorpseTargetInfo` | `CorpseTargetInfo` | 支持 (`TimeDelay`) | 存储受影响的尸体（如复活） |

`TargetInfo` 是最复杂的子结构，包含完整的命中信息：

```cpp
struct TargetInfo : public TargetInfoBase
{
    ObjectGuid      TargetGUID;
    uint64          TimeDelay;           // 延迟命中时间 (毫秒)
    int32           Damage;
    int32           Healing;
    bool            Positive;
    SpellMissInfo   MissCondition;       // 命中/闪避/招架/反射
    SpellMissInfo   ReflectResult;
    bool            IsCrit;
    // 预处理阶段填充：
    DiminishingGroup DRGroup;
    int32           AuraDuration;
    int32           AuraBasePoints[MAX_SPELL_EFFECTS];
    UnitAura*       HitAura;
};
```

对于延迟法术 (delayed spell)，目标在 `handle_delayed()` 中按 `TimeDelay` 排序逐步命中（`src/server/game/Spells/Spell.cpp:4100-4116`）。

---

## 3. 状态流转

### 3.1 ASCII 状态图

```
                         Spell 构造
                            │
                            v
                   ┌────────────────┐
                   │  SPELL_STATE   │
                   │     NULL (0)   │
                   └───────┬────────┘
                           │
                  prepare()  (Spell.h:3439 设置)
                           │
                           v
               ┌───────────────────────┐
               │  SPELL_STATE          │
     ┌────────>│  PREPARING (1)        │<────────┐
     │         └──────────┬────────────┘         │
     │                    │                      │
     │         timer==0    │     cancel()         │
     │         (读条完成)   │     (中断)            │
     │                    v                      │
     │         ┌───────────────────────┐         │
     │         │ cast() -> _cast()      │         │
     │         │ state = LAUNCHED (2)  │         │
     │         │ (Spell.cpp:3854)      │         │
     │         └──────────┬────────────┘         │
     │                    │                      │
     │                    │                      │
     │        ┌───────────┴──────────┐           │
     │        │                      │           │
     │   handle_immediate()    handle_delayed()  │
     │   (瞬时法术)             (延迟法术)        │
     │        │                      │           │
     │        │               延迟逐步命中        │
     │        │               next_time==0       │
     │        │                      │           │
     │        │                      v           │
     │        │                _handle_finish()   │
     │        │                      │           │
     │        v                      v           │
     │   ┌────────────────────────────┐          │
     │   │ IsChanneled()?             │          │
     │   └────┬───────────────┬───────┘          │
     │        │ Yes           │ No               │
     │        v               v                  │
     │ ┌──────────────┐  finish() ─────────┐    │
     │ │ SPELL_STATE  │                   │    │
     │ │ CHANNELING   │                   │    │
     │ │ (3)          │                   │    │
     │ └──────┬───────┘                   │    │
     │        │                           │    │
     │        │ update() 递减 timer       │    │
     │        │ timer==0 或中断            │    │
     │        v                           │    │
     │   finish()                         │    │
     │        │                           │    │
     │        v                           v    │
     │ ┌──────────────┐          ┌──────────┐   │
     │ │SPELL_STATE   │          │SPELL_STATE│   │
     │ │FINISHED (4)  │          │FINISHED  │   │
     │ └──┬───────────┘          │(4)       │   │
     │    │                      └─────┬────┘   │
     │    │ IsAutoRepeat()?            │         │
     │    │ && 成功施放                 │         │
     │    │                            │         │
     │    v                            │         │
     │ ┌──────────────┐                │         │
     │ │SPELL_STATE   │                │         │
     │ │IDLE (5)      │                │         │
     │ │(自动重复)     │                │         │
     │ └──────────────┘                │         │
     │                                 │         │
     │ cancel() 从任何活跃状态 ─────────┘         │
     │ -> FINISHED                                 │
     │                                             │
     └─────────> PREPARING (重新开始自动重复) <─────┘
```

### 3.2 状态转换详述

#### NULL -> PREPARING

触发：`Spell::prepare()` 调用
位置：`src/server/game/Spells/Spell.cpp:3439`

```cpp
m_spellState = SPELL_STATE_PREPARING;
```

进入 PREPARING 后，`prepare()` 会依次执行：
1. 初始化施法物品信息和等级（`3418-3435`）
2. 初始化显式目标（`3437`）
3. 注册 `SpellEvent` 到事件系统（`3448-3449`）
4. 检查法术是否被禁用（`3452-3457`）
5. 检查是否已有法术在施放（`3460-3465`）
6. 加载法术脚本（`3467`）
7. 计算消耗（`3470-3471`）
8. **调用 `CheckCast(true)` 进行严格验证**（`3474`），失败则 `finish()`
9. 准备触发器数据（`3505`）
10. 计算施法时间并应用修正（`3507-3509`）
11. 移动中检查（`3512-3513`）
12. 消耗资源（法力/物品/符文等）（后续代码）
13. 发送施法开始包
14. 若为瞬时法术 (casttime==0)，立即调用 `cast(!m_casttime)` 即 `cast(true)`

#### PREPARING -> LAUNCHED

触发：`Spell::_cast()` 执行成功
位置：`src/server/game/Spells/Spell.cpp:3854`

```cpp
m_spellState = SPELL_STATE_LAUNCHED;
```

`_cast()` 是施放的核心实现，`cast()` 仅封装一层 SpellMod 保护（`3649-3664`）。`_cast()` 的关键步骤：
1. 更新指针有效性（`3669-3674`），失败则 `cancel()`
2. 检查显式目标是否仍存在（`3677-3681`），失败则 `cancel()`
3. 通知玩家宠物 AI（`3689-3697`）
4. 设置 SpellMod 占用（`3700-3709`）
5. 调用 `CallScriptBeforeCastHandlers()`（`3711`）
6. 非瞬时法术进行第二轮 **`CheckCast(false)` 验证**（`3729`），包括交易检查和递减检查
7. 处理法力消耗（后续代码）
8. 目标选择与修正（`src/server/game/Spells/Spell.cpp:3780+`）
9. 处理反射（`3790+`）
10. 发送冷却（`3851-3852`）
11. **设置 state = SPELL_STATE_LAUNCHED**（`3854`）
12. 处理 Launch 阶段（若无 LaunchDelay 则立即执行）（`3856-3859`）
13. 发送 `SMSG_SPELL_GO` 包（`3866`）
14. 触发施法成功 proc（`3888`）
15. 通知 CreatureAI（`3891-3893`）
16. 根据是否有延迟 (`m_delayMoment`) 分支：
    - 有延迟 -> `SetDelayStart(0)`，等待 `handle_delayed()` 被事件驱动调用
    - 无延迟 -> `handle_immediate()`（`3914`）

#### LAUNCHED -> CHANNELING

触发：`Spell::handle_immediate()` 中的引导逻辑
位置：`src/server/game/Spells/Spell.cpp:4021`

```cpp
m_spellState = SPELL_STATE_CHANNELING;
```

仅当 `m_spellInfo->IsChanneled()` 为 true 且持续时间不为 0 时转换。若持续时间恰好为 0，法术会在 `handle_immediate()` 末尾直接调用 `finish()` 跳过引导状态。

#### LAUNCHED -> FINISHED

触发：
- 瞬时法术在 `handle_immediate()` 中完成所有目标处理后调用 `finish()`（`4053-4054`）
- 延迟法术在 `handle_delayed()` 中所有目标命中完毕 (`next_time==0`) 后调用 `_handle_finish_phase()` + `finish()`（`4145-4153`）

#### CHANNELING -> FINISHED

触发：
- `update()` 中引导计时器递减至 0 时调用 `finish()`（`4315-4318`）
- `cancel()` 在 CHANNELING 状态下中断引导（`3617-3644`）

#### FINISHED -> IDLE

触发：`finish()` 中自动重复法术的特殊处理
位置：`src/server/game/Spells/Spell.cpp:4349-4350`

```cpp
if (IsAutoRepeat() && unitCaster->GetCurrentSpell(CURRENT_AUTOREPEAT_SPELL) == this)
    m_spellState = SPELL_STATE_IDLE;
```

仅用于自动射击 (Auto Shot) 等 CURRENT_AUTOREPEAT_SPELL，使 Spell 对象不被销毁，持续提供重复射击能力。

#### 任何活跃状态 -> FINISHED (cancel)

触发：`Spell::cancel()`
位置：`src/server/game/Spells/Spell.cpp:3599-3647`

`cancel()` 是从 PREPARING、LAUNCHED、CHANNELING 到 FINISHED 的统一中断路径：

```cpp
void Spell::cancel()
{
    if (m_spellState == SPELL_STATE_FINISHED)
        return;
    SpellState oldState = m_spellState;
    m_spellState = SPELL_STATE_FINISHED;
    // 根据 oldState 执行不同清理：
    // PREPARING: 取消 GCD
    // PREPARING/LAUNCHED: 发送中断包
    // CHANNELING: 移除引导光环
}
```

---

## 4. 关键算法与分支

### 4.1 update() — 状态驱动的主循环

`src/server/game/Spells/Spell.cpp:4213-4330`

`update()` 是状态机的驱动函数，由 `SpellEvent` 每帧调用。它首先做前置检查（指针有效性、目标存在性、移动打断），然后根据 `m_spellState` 分发：

**PREPARING 分支** (`4236-4249`)：
- 递减 `m_timer`（施法读条剩余时间）
- 当 `m_timer == 0` 且非下次近战攻击法术时，调用 `cast(!m_casttime)`
  - 瞬时法术 (`m_casttime==0`)：`skipCheck=true`，跳过二轮 CheckCast（因为 prepare() 中已做过）
  - 非瞬时法术：`skipCheck=false`，需要二轮 CheckCast

**CHANNELING 分支** (`4251-4326`)：
- 检查引导目标列表有效性，目标全失则中断引导（`4256-4265`）
- 递减 `m_timer`
- **赋能法术处理** (`4276-4312`)：
  - 根据已过时间计算 `completedStages`
  - 阶段变更时发送 `SpellEmpowerSetStage` 包并调用 `CallScriptEmpowerStageCompletedHandlers`
  - 调用 `CanReleaseEmpowerSpell()` 判断是否可释放
  - 释放时设置 `m_timer=0` 触发 finish
- 当 `m_timer == 0` 时发送引导结束包并调用 `finish()`
- 调用 CreatureAI::OnChannelFinished 钩子

### 4.2 CheckCast() — 施法验证链

`src/server/game/Spells/Spell.cpp:5780-6156`

`CheckCast(bool strict, int32* param1, int32* param2)` 是法术能否施放的核心验证函数。它被调用两次：
1. `prepare()` 中以 `strict=true` 调用（`3474`）
2. `_cast()` 中以 `strict=false` 调用（`3729`），仅对非瞬时法术

验证链的子检查顺序：

| 序号 | 检查项 | 说明 | 位置 |
|------|--------|------|------|
| 1 | 死亡状态 | 施法者是否死亡，被动法术和特定属性豁免 | 5793 |
| 2 | 免疫交互 | 玩家处于免疫状态时禁止与特定 GameObject 交互 | 5787-5791 |
| 3 | 冷却检查 | 检查法术冷却、充能和分类冷却 | 5794+ |
| 4 | 施法禁用 Aura | 检查 SPELL_AURA_DISABLE_CASTING_EXCEPT_ABILITIES 等 | 5800-5808 |
| 5 | 攻击禁用 Aura | 检查 SPELL_AURA_DISABLE_ATTACKING_EXCEPT_ABILITIES | 5809+ |
| 6 | 专精检查 | 检查当前专精是否允许施放 | -- |
| 7 | 战斗状态 | 部分法术要求脱战施放 | -- |
| 8 | 施法物品 | 法术通过物品施放时的有效性检查 | -- |
| 9 | 骑乘/载具状态 | 是否允许在坐骑上施放 | 6102-6112 |
| 10 | 法术焦点 | RequiresSpellFocus 检查 | 6114-6123 |
| 11 | **CheckItems()** | 物品/图腾/符文/圣物检查 | 6130 |
| 12 | **CheckRange(strict)** | 射程检查（strict=true 时更严格） | 6137 |
| 13 | **CheckPower()** | 法力/能量/怒气等资源检查 | 6142-6146 |
| 14 | **CheckCasterAuras()** | 施法者 Aura 冲突检查（沉默、变形等） | 6148-6153 |
| 15 | **CallScriptCheckCastHandlers()** | 脚本自定义验证钩子 | 6156 |
| 16 | 效果专属检查 | 根据 SpellEffect 逐效果验证（DUMMY、 Summon 等） | 6160+ |

`strict` 参数的语义差异：
- `strict=true`：在 `prepare()` 阶段调用，执行完整验证，确保在开始施法前所有条件满足
- `strict=false`：在 `_cast()` 阶段（读条结束后）调用，用于捕获施法期间状态变化（如目标移出射程）

### 4.3 瞬时法术 vs 延迟法术分支

在 `_cast()` 的末尾（`3896-3915`），根据 `m_delayMoment` 是否为 0 决定走哪条路径：

**瞬时法术 (`m_delayMoment == 0`)** — `handle_immediate()` (`3967-4055`)：

```
handle_immediate()
  ├── 若引导法术且 duration > 0:
  │     ├── 计算持续时间（SpellMod + 急速）
  │     ├── 若赋能法术: 初始化阶段持续时间
  │     ├── SendChannelStart()
  │     └── state = CHANNELING
  ├── PrepareTargetProcessing()
  ├── _handle_immediate_phase()           // 处理威胁、HIT 模式效果、物品目标
  ├── DoProcessTargetContainer(Unit)      // 处理所有 Unit 目标命中
  ├── DoProcessTargetContainer(GO)        // 处理所有 GameObject 目标命中
  ├── DoProcessTargetContainer(Corpse)    // 处理所有 Corpse 目标命中
  ├── FinishTargetProcessing()
  ├── _handle_finish_phase()              // 额外攻击等收尾逻辑
  ├── TakeCastItem()
  └── 若非 CHANNELING: finish()
```

**延迟法术 (`m_delayMoment > 0`)** — `handle_delayed()` (`4057-4160`)：

```
handle_delayed(t_offset)
  ├── 若未处理 Launch 阶段:
  │     ├── 检查 LaunchDelay
  │     └── HandleLaunchPhase() + m_launchHandled = true
  ├── PrepareTargetProcessing()
  ├── 若未处理 Immediate 阶段 (m_delayMoment <= t_offset):
  │     └── _handle_immediate_phase()
  ├── 按时间筛选已到期的 Unit 目标 -> DoProcessTargetContainer
  ├── 按时间筛选已到期的 GO 目标  -> DoProcessTargetContainer
  ├── FinishTargetProcessing()
  └── 若 next_time == 0 (所有目标已命中):
        ├── _handle_finish_phase()
        └── finish()
      否则:
        └── return next_time (等待下一次事件触发)
```

延迟法术的核心特点是：目标命中被分散到多个时间点，由 `SpellEvent` 持续调度 `handle_delayed()` 直到所有目标的 `TimeDelay` 都已过期。

### 4.4 handle_immediate() 的辅助阶段

`_handle_immediate_phase()` (`4162-4180`)：
- 处理仇恨法术 `HandleThreatSpells()`
- 以 `SPELL_EFFECT_HANDLE_HIT` 模式执行所有效果（处理地面效果等无目标效果）
- 处理物品目标容器 `DoProcessTargetContainer(m_UniqueItemInfo)`

`_handle_finish_phase()` (`4182-4197`)：
- 记录额外攻击法术 `SPELL_EFFECT_ADD_EXTRA_ATTACKS` 到 Unit
- 触发 SPELL_LINK_CAST 联动法术（在 `_cast()` 的 `CallScriptAfterCastHandlers` 之后的逻辑中）

### 4.5 cancel() — 统一中断处理

`src/server/game/Spells/Spell.cpp:3599-3647`

`cancel()` 首先将状态设为 FINISHED（幂等检查），然后根据转换前的原状态执行不同清理：

| 原状态 | 清理行为 |
|--------|---------|
| PREPARING | `CancelGlobalCooldown()` + 发送中断包和失败结果 |
| LAUNCHED | 发送中断包和失败结果 |
| CHANNELING | 移除所有引导目标上的 Aura + `UpdateChanneledTargetList` + 发送引导结束包 + 移除施法者身上的赋能标记 |

### 4.6 finish() — 终态处理

`src/server/game/Spells/Spell.cpp:4332-4422`

`finish(SpellCastResult result)` 将状态设为 FINISHED，然后执行：

1. 设置脚本异步结果（`4338-4339`）
2. 自动重复法术转为 IDLE 而非销毁（`4349-4350`）
3. 清除引导中断标记（`4353`）
4. 清除 UNIT_STATE_CASTING 状态（`4355-4356`）
5. 解除被施法控制的傀儡（`4359-4366`）
6. 释放生物施法焦点（`4369`）
7. 触发施法结束 proc `PROC_FLAG_CAST_ENDED`（`4371`）
8. 赋能法术在结束时才触发 GCD（`4373-4378`）
9. 失败时回退天赋配置和退还消耗（`4380-4394`）
10. 处理召唤雕像解散（`4396-4410`）
11. 更新药水冷却（`4413-4417`）
12. 停止自动攻击（`4420-4421`）

---

## 5. 扩展点与脚本集成

### 5.1 脚本钩子一览

法术状态机在关键路径上暴露了大量脚本钩子，供 Eluna、v2 Scripting 等脚本系统使用：

| 钩子名称 | 调用位置 | 触发时机 |
|----------|---------|---------|
| `CallScriptCheckCastHandlers()` | `CheckCast()` 末尾 (`6156`) | 施法验证阶段，可自定义拒绝原因 |
| `CallScriptBeforeCastHandlers()` | `_cast()` 开头 (`3711`) | 法术实际执行前 |
| `CallScriptOnCastHandlers()` | `_cast()` 发送 SMSG_SPELL_GO 后 (`3813`) | 法术施出但效果未命中前 |
| `CallScriptAfterCastHandlers()` | `_cast()` 末尾 (`3920`) | 法术完全执行完毕后 |
| `CallScriptOnHitHandlers()` | `DoTargetSpellHit` 各命中路径 (`2771, 3063, 3073, 3087`) | 效果命中目标时 |
| `CallScriptCalcCastTimeHandlers()` | `prepare()` 计算施法时间 (`3509`) | 允许脚本修改施法时间 |
| `CallScriptEmpowerStageCompletedHandlers()` | `update()` CHANNELING 分支 (`4303`) | 赋能阶段完成时 |
| `CallScriptEmpowerCompletedHandlers()` | `update()` CHANNELING 分支 (`4310`) | 赋能完全释放时 |
| `sScriptMgr->OnPlayerSpellCast()` | `_cast()` 开头 (`3687`) | 玩家施法通知 |

### 5.2 CastSpellExtraArgs 中的脚本集成

`CastSpellExtraArgs` (`src/server/game/Spells/SpellDefines.h:483-545`) 提供了两个脚本相关字段：

- `ScriptResult` (`ActionResultSetter<SpellCastResult>`)：v2 脚本系统的异步结果回调。当 `ScriptWaitsForSpellHit=false` 时在 `_cast()` 完成时设置结果（`3917-3918`）；当 `ScriptWaitsForSpellHit=true` 时在 `finish()` 中设置结果（`4338-4339`）
- `CustomArg` (`std::any`)：传递脚本自定义数据到法术对象，用于实现脚本特定的效果逻辑（如天赋切换时传递 TraitConfig）

### 5.3 SpellEvent 事件驱动

`prepare()` 中创建 `SpellEvent` 并注册到施法者的事件队列（`3448-3449`）：

```cpp
_spellEvent = new SpellEvent(this);
m_caster->m_Events.AddEvent(_spellEvent, m_caster->m_Events.CalculateTime(1ms));
```

`SpellEvent` 负责周期性调用 `update()` 驱动状态机，并在法术完成后自动清理 Spell 对象（FINISHED/IDLE 状态除外）。对于延迟法术，`handle_delayed()` 的返回值作为下一次事件调度的延迟时间。

### 5.4 脚本自定义目标与效果

通过 `CallScriptCheckCastHandlers()` 和 `CallScriptOnHitHandlers()`，脚本可以：
- 自定义施法条件（如特定 NPC 状态才能接受某法术）
- 在命中时注入额外效果或修改命中行为
- 调整赋能法术的阶段逻辑

### 5.5 相关文档交叉引用

- **目标选择系统**：详见 [02-target-selection.md](02-target-selection.md) — 覆盖目标初始化、修正、反射、PreprocessTarget 等内容
- **效果管线**：详见 [03-effect-pipeline.md](03-effect-pipeline.md) — 覆盖 SpellEffectHandleMode 四阶段、HandleEffects 调用链、效果执行顺序
- **法术脚本编写**：详见 [05-spell-scripting.md](05-spell-scripting.md) — 覆盖如何编写自定义法术脚本、注册钩子、使用 CastSpellExtraArgs
