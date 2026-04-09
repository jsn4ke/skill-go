# 03 - 法术效果处理管线 (Spell Effect Processing Pipeline)

## 1. 概述 (Overview)

TrinityCore 的法术效果处理管线是整个法术系统的核心执行引擎。一个法术从施法完成到最终作用于目标，需要经过一系列严格的阶段划分与调度。管线的核心职责包括：

- **效果分派 (Effect Dispatch)**：将 DBC 中定义的 `SpellEffectName` 映射到对应的 C++ 成员函数，通过函数指针表实现零开销分派。
- **多阶段执行 (Multi-Phase Execution)**：将效果处理分为 Launch（发射）和 Hit（命中）两大阶段，每个阶段再细分为无目标（全局）和有目标（单目标）两种模式。
- **即时与延迟路径 (Immediate vs Delayed Paths)**：根据法术是否有飞行时间（`TimeDelay`），走不同的执行时序。
- **数值计算 (Value Calculation)**：通过 `SpellEffectInfo::CalcValue` 链完成基础数值、方差、资源系数、天赋修正等复合计算。

管线的设计哲学是 **数据驱动 + 模板分派**：所有效果处理器以函数指针数组存储，运行时通过 `SpellEffectInfo::Effect` 索引直接调用，避免了庞大的 `switch-case` 结构。

---

## 2. 核心数据结构 (Core Data Structures)

### 2.1 EffectHandleMode 枚举

```
src/server/game/Spells/Spell.h:264-270
```

```cpp
enum SpellEffectHandleMode
{
    SPELL_EFFECT_HANDLE_LAUNCH,          // 发射阶段 - 无目标上下文
    SPELL_EFFECT_HANDLE_LAUNCH_TARGET,   // 发射阶段 - 针对特定目标
    SPELL_EFFECT_HANDLE_HIT,             // 命中阶段 - 无目标上下文
    SPELL_EFFECT_HANDLE_HIT_TARGET       // 命中阶段 - 针对特定目标
};
```

四种模式构成了管线执行的四维分派维度。效果处理器内部通过检查 `Spell::effectHandleMode` 成员变量来决定当前执行处于哪个阶段，从而选择性地执行对应逻辑。

| 模式 | 调用时机 | 典型用途 |
|------|---------|---------|
| `LAUNCH` | `HandleLaunchPhase()` 遍历所有效果 | 全局预计算、日志、统计 |
| `LAUNCH_TARGET` | `DoEffectOnLaunchTarget()` 对每个目标调用 | 预计算伤害/治疗值 |
| `HIT` | `_handle_immediate_phase()` 遍历所有效果 | 无目标的效果（地面效果、区域触发器等） |
| `HIT_TARGET` | `DoSpellEffectHit()` 对每个目标调用 | 实际施加伤害/治疗/光环 |

### 2.2 SpellEffectInfo 类

```
src/server/game/Spells/SpellInfo.h:209-297
```

`SpellEffectInfo` 是单个法术效果的完整运行时描述，从 DBC `SpellEffectEntry` 构造而来。关键字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `EffectIndex` | `SpellEffIndex` | 效果在法术中的索引 (0/1/2) |
| `Effect` | `SpellEffectName` | 效果类型枚举，用作分派表的索引 |
| `ApplyAuraName` | `AuraType` | 若为光环效果，指定光环类型 |
| `BasePoints` | `float` | 效果基础值 |
| `Scaling` | `ScalingInfo` | 缩放系数、方差、资源系数 |
| `BonusCoefficient` | `float` | 天赋/精通加成系数 |
| `TargetA/B` | `SpellImplicitTargetInfo` | 隐式目标信息 |
| `EffectAttributes` | `EnumFlag<SpellEffectAttributes>` | 效果属性标志 |

**关键计算方法：**

- `CalcBaseValue()` (`src/server/game/Spells/SpellInfo.h:267`): 返回 `SpellEffectValue` 类型的基础值，根据缩放系数从装备等级/法术等级曲线查表计算。
- `CalcValue()` (`src/server/game/Spells/SpellInfo.cpp:515-602`): 完整数值计算链，依次处理天赋修正、方差随机化、每级加成、连击点加成、精通修正、效果修正器。
- `CalcDamageMultiplier()` / `CalcValueMultiplier()`: 计算伤害/数值乘数。

### 2.3 SpellEffectHandlers 分派表

```
src/server/game/Spells/SpellEffects.cpp:90-447
```

```cpp
NonDefaultConstructible<SpellEffectHandlerFn> SpellEffectHandlers[TOTAL_SPELL_EFFECTS] =
{
    &Spell::EffectNULL,                                     //  0
    &Spell::EffectInstaKill,                                //  1 SPELL_EFFECT_INSTAKILL
    &Spell::EffectSchoolDMG,                                //  2 SPELL_EFFECT_SCHOOL_DAMAGE
    &Spell::EffectDummy,                                    //  3 SPELL_EFFECT_DUMMY
    // ... 共 TOTAL_SPELL_EFFECTS (355) 个条目
};
```

其中 `SpellEffectHandlerFn` 类型定义为 (`src/server/game/Spells/Spell.h:1121`)：

```cpp
using SpellEffectHandlerFn = void(Spell::*)();
```

这是一个成员函数指针，签名为无参数、无返回值。效果处理所需的所有数据通过 `Spell` 类的成员变量传递：
- `unitTarget`, `itemTarget`, `gameObjTarget`, `m_corpseTarget`, `destTarget` -- 当前效果的目标
- `damage` -- 计算后的伤害/效果数值
- `effectHandleMode` -- 当前执行模式
- `effectInfo` -- 当前效果信息
- `variance` -- 方差值
- `_spellAura` -- 关联的光环对象

### 2.4 TargetInfoBase 层次结构

```
src/server/game/Spells/Spell.h:833-903
```

```
TargetInfoBase (虚基类)
  |-- TargetInfo         (Unit 目标，包含伤害/治疗/暴击/反射/DR 等完整数据)
  |-- GOTargetInfo       (GameObject 目标)
  |-- ItemTargetInfo     (Item 目标)
  |-- CorpseTargetInfo   (Corpse 目标)
```

**虚接口：**

| 方法 | 说明 | TargetInfo 实现 |
|------|------|----------------|
| `PreprocessTarget(Spell*)` | 命中前预处理（免疫检查、DR计算、光环基础值） | 完整实现 |
| `DoTargetSpellHit(Spell*, SpellEffectInfo const&)` | 执行效果命中 | 调用 `DoSpellEffectHit()` |
| `DoDamageAndTriggers(Spell*)` | 施加伤害/治疗并触发法术系统 | 完整实现 |

**TargetInfo** (`src/server/game/Spells/Spell.h:846-875`) 是最关键的子类，携带：

- `Damage` / `Healing` -- 发射阶段预计算的伤害/治疗值
- `MissCondition` / `ReflectResult` -- 未命中结果（闪避、免疫、反射等）
- `IsCrit` -- 发射阶段计算的暴击结果
- `AuraDuration` / `AuraBasePoints[]` / `HitAura` -- 光环创建数据
- `DRGroup` -- 递减效果分组
- `TimeDelay` -- 投射物飞行延迟时间

**DoProcessTargetContainer 模板** (`src/server/game/Spells/Spell.h:905-906`)：

```cpp
template <class Container>
void DoProcessTargetContainer(Container& targetContainer);
```

这是一个泛型处理模板，对任意 `TargetInfoBase` 容器执行统一流程：
1. 逐目标调用 `PreprocessTarget()` -- 命中前预处理
2. 遍历所有效果，逐目标调用 `DoTargetSpellHit()` -- 执行效果
3. 逐目标调用 `DoDamageAndTriggers()` -- 施加伤害/治疗并触发

---

## 3. 执行流程 (Execution Flow)

### 3.1 完整管线流程图

```
                           cast()/go()
                               |
                               v
                    +----------------------+
                    |   Prepare()          |  法术准备阶段
                    |   SelectSpellTargets |  目标选取（见 02 文档）
                    +----------------------+
                               |
                   +-----------+-----------+
                   |  Has LaunchDelay?     |
                   |  or Has TravelTime?   |
                   +-----+----------+------+
                   NO   |          |  YES
                   |    |          |    |
                   v    |          v    v
         +------------------+  +--------------------+
         | handle_immediate |  | handle_delayed     |
         | :3967-4055       |  | :4057-4160         |
         +------------------+  +--------------------+
                   |                    |
                   |           +--------+--------+
                   |           | LaunchDelay     |
                   |           | elapsed?        |
                   |           +--+----------+---+
                   |           NO|          |YES
                   |           |  return    v
                   |           |  next_time  |
                   |           |  +-----> HandleLaunchPhase()
                   |           |           |
                   |           |  TimeDelay    |
                   |           |  elapsed?     |
                   |           +-+--------+---+
                   |            NO       YES
                   |           return    |
                   |           next_time |
                   |                    |
                   +-------+------------+
                           |
                           v
                 +---------------------+
                 | HandleLaunchPhase() |  :8532-8566
                 +---------------------+
                          |
             +------------+------------+
             |  LAUNCH (per effect)    |
             |  HandleEffects(         |
             |    mode=LAUNCH)         |
             +------------+------------+
                          |
             +------------+------------+
             | PreprocessSpellLaunch() |
             | (per target)            |
             +------------+------------+
                          |
             +------------+------------+
             | LAUNCH_TARGET            |
             | (per effect per target)  |
             | DoEffectOnLaunchTarget() |
             +------------+------------+
                          |
                          v
               +--------------------+
               | _handle_immediate  |  :4162-4180
               | _phase()           |
               +--------------------+
                    |           |
         +----------+   +-------+----------+
         | HIT (per    | | DoProcessTarget |
         | effect)     | | Container       |
         | HandleEff.  | | (TargetInfo)    |
         | mode=HIT    | | (GOTargetInfo)  |
         +----------+   | (CorpseTarget   |
                    |   |  Info)           |
                    |   +-------+----------+
                    |           |
                    +-----+-----+
                          |
                          v
               +--------------------+
               | DoSpellEffectHit() |  :3208-3294
               | (per target)       |
               |                    |
               |  1. Aura 创建      |
               |  2. HANDLE_HIT_    |
               |     TARGET         |
               |  3. HandleEffects()|
               +---------+----------+
                         |
                         v
               +--------------------+
               | DoDamageAndTriggers|  :849-850
               | (per target)       |
               |                    |
               |  1. DealDamage     |
               |  2. DealHealing    |
               |  3. TriggerSystem  |
               +---------+----------+
                         |
                         v
               +--------------------+
               | _handle_finish     |  :4182-4197
               | _phase()           |
               |                    |
               |  1. Extra Attacks  |
               |  2. Proc on Finish |
               +--------------------+
                         |
                         v
                    finish()
```

### 3.2 Launch 阶段详解

`HandleLaunchPhase()` (`src/server/game/Spells/Spell.cpp:8532-8566`) 分为三个子步骤：

**步骤 1: LAUNCH 模式处理（全局，无目标）**

```cpp
for (SpellEffectInfo const& spellEffectInfo : m_spellInfo->GetEffects())
{
    if (!spellEffectInfo.IsEffect())
        continue;
    HandleEffects(nullptr, nullptr, nullptr, nullptr, spellEffectInfo, SPELL_EFFECT_HANDLE_LAUNCH);
}
```

遍历所有有效效果，以 `SPELL_EFFECT_HANDLE_LAUNCH` 模式调用 `HandleEffects`。注意此时所有目标参数均为 `nullptr`，效果处理器只能执行不依赖具体目标的逻辑。

**步骤 2: PreprocessSpellLaunch（每目标预处理）**

```cpp
PrepareTargetProcessing();
for (TargetInfo& target : m_UniqueTargetInfo)
    PreprocessSpellLaunch(target);
```

对每个目标执行发射预处理 (`src/server/game/Spells/Spell.cpp:8568-8606`)：
- 免疫性检查
- 进入战斗状态判定
- 暴击率计算与判定（结果存入 `targetInfo.IsCrit`）

**步骤 3: LAUNCH_TARGET 模式处理（每效果每目标）**

```cpp
for (SpellEffectInfo const& spellEffectInfo : m_spellInfo->GetEffects())
{
    float multiplier = ...;
    for (TargetInfo& target : m_UniqueTargetInfo)
        DoEffectOnLaunchTarget(target, multiplier, spellEffectInfo);
}
```

`DoEffectOnLaunchTarget()` (`src/server/game/Spells/Spell.cpp:8608-8654`)：
1. 解析实际命中单位（考虑反射情况）
2. 调用 `HandleEffects(..., SPELL_EFFECT_HANDLE_LAUNCH_TARGET)` -- 效果处理器在此阶段预计算伤害值（`m_damage`）和治疗值（`m_healing`）
3. 对 `m_damage` 应用 AoE 伤害衰减（`sqrt` 目标上限削减、`AOE_DAMAGE_TARGET_CAP` 线性削减）
4. 对 `m_healing` 应用类似的 AoE 治疗衰减
5. 将最终值存入 `targetInfo.Damage` / `targetInfo.Healing`

### 3.3 Hit 阶段详解

Hit 阶段通过 `DoProcessTargetContainer` 模板 (`src/server/game/Spells/Spell.h:905-906`) 统一调度：

```cpp
template <class Container>
void Spell::DoProcessTargetContainer(Container& targetContainer)
{
    for (TargetInfoBase& target : targetContainer)
        target.PreprocessTarget(this);              // 命中前预处理

    for (SpellEffectInfo const& spellEffectInfo : m_spellInfo->GetEffects())
        for (TargetInfoBase& target : targetContainer)
            if (target.EffectMask & (1 << spellEffectInfo.EffectIndex))
                target.DoTargetSpellHit(this, spellEffectInfo);  // 执行效果

    for (TargetInfoBase& target : targetContainer)
        target.DoDamageAndTriggers(this);           // 伤害/治疗与触发
}
```

**PreprocessTarget** (对 `TargetInfo` 实现，即 `PreprocessSpellHit`，`src/server/game/Spells/Spell.cpp:3091-3207`)：
- 重新检查目标 evade/免疫状态
- 调用 `CallScriptBeforeHitHandlers()` -- **脚本 BeforeHit 钩子**
- PvP 合法性重检
- 光环基础值计算（`CalcBaseValue`）
- 递减效果 (Diminishing Returns) 计算
- 光环持续时间计算与递减
- 若目标因 DR 导致持续时间为 0 且法术仅有光环效果，返回 `SPELL_MISS_IMMUNE`

**DoTargetSpellHit** (对 `TargetInfo` 实现，即 `DoSpellEffectHit`，`src/server/game/Spells/Spell.cpp:3208-3294`)：
- 创建/刷新光环（`Aura::TryRefreshStackOrCreate`）
- 设置光环持续时间、DR 分组
- 调用 `HandleEffects(..., SPELL_EFFECT_HANDLE_HIT_TARGET)` -- 效果处理器在此阶段实际施加效果

**DoDamageAndTriggers** (`TargetInfo::DoDamageAndTriggers`，声明于 `src/server/game/Spells/Spell.h:849-850`)：
- 施加伤害（`targetInfo.Damage`）
- 施加治疗（`targetInfo.Healing`）
- 调用 `DoTriggersOnSpellHit()` -- 命中触发系统

### 3.4 即时路径 vs 延迟路径

**即时路径** `handle_immediate()` (`src/server/game/Spells/Spell.cpp:3967-4055`)：

```
handle_immediate()
  |
  |-- 处理引导 (IsChanneled) -> 设置 m_channelDuration
  |-- 通道法术的 Empower 初始化
  |
  |-- PrepareTargetProcessing()
  |-- _handle_immediate_phase()
  |     |-- HandleThreatSpells()
  |     |-- for each effect: HandleEffects(mode=HIT)
  |     |-- DoProcessTargetContainer(m_UniqueItemInfo)
  |
  |-- DoProcessTargetContainer(m_UniqueTargetInfo)   // 包含完整的 Preprocess/Hit/Damage
  |-- DoProcessTargetContainer(m_UniqueGOTargetInfo)
  |-- DoProcessTargetContainer(m_UniqueCorpseTargetInfo)
  |
  |-- FinishTargetProcessing()
  |-- _handle_finish_phase()
  |-- finish()
```

所有目标同步命中，一次调用完成全部处理。

**延迟路径** `handle_delayed(uint64 t_offset)` (`src/server/game/Spells/Spell.cpp:4057-4160`)：

```
handle_delayed(t_offset)
  |
  |-- 若 !m_launchHandled:
  |     检查 LaunchDelay 是否已过
  |     HandleLaunchPhase()
  |     m_launchHandled = true
  |
  |-- 检查 m_delayMoment vs t_offset
  |
  |-- 若 !m_immediateHandled && m_delayMoment <= t_offset:
  |     _handle_immediate_phase()
  |     m_immediateHandled = true
  |
  |-- 按时间筛选到期目标 (TimeDelay <= t_offset)
  |     (对单投射物: HasDst() -> 所有目标同时到达)
  |     DoProcessTargetContainer(delayedTargets)
  |
  |-- 若所有目标已处理 (next_time == 0):
  |     _handle_finish_phase()
  |     finish()
  |     return 0
  |-- 否则:
  |     return next_time  // 返回下次执行时间
```

延迟路径通过事件系统周期性回调 `handle_delayed()`，每次处理飞行时间已到的目标子集。单投射物法术（`HasDst()`）则忽略 `TimeDelay`，所有目标同时命中。

**状态控制标志：**

| 标志 | 说明 |
|------|------|
| `m_launchHandled` | Launch 阶段是否已执行（仅延迟路径使用） |
| `m_immediateHandled` | Immediate 阶段是否已执行（仅延迟路径使用） |
| `m_executedCurrently` | 防止重复执行的守卫标志 |
| `m_referencedFromCurrentSpell` | 防止法术被引用时被删除 |

---

## 4. 关键算法与分支 (Key Algorithms & Branches)

### 4.1 函数指针分派链

从 `HandleEffects()` 到最终效果处理器的完整调用链：

```
HandleEffects()                                              // Spell.cpp:5754-5770
  |
  |-- 设置当前效果上下文:
  |     effectHandleMode = mode
  |     unitTarget / itemTarget / gameObjTarget / m_corpseTarget
  |     destTarget / effectInfo
  |
  |-- damage = CalculateDamage(spellEffectInfo, unitTarget)  // Spell.cpp:7247-7251
  |     |
  |     +-> m_caster->CalculateSpellDamage(target, spellEffectInfo, ...)
  |           |
  |           +-> spellEffectInfo.CalcValue(caster, basePoints, target, variance, ...)
  |                 |
  |                 +-> CalcBaseValue()     // 基础值（缩放曲线）
  |                 +-> Trait 修正          // 天赋树效果点数覆盖
  |                 +-> Variance 随机化     // 方差计算
  |                 +-> 等级/每级加成       // RealPointsPerLevel
  |                 +-> 连击点加成          // PointsPerResource
  |                 +-> 精通修正            // BonusCoefficient
  |                 +-> ApplyEffectModifiers // 全局效果修正器
  |
  |-- preventDefault = CallScriptEffectHandlers(effIndex, mode)  // Spell.cpp:8902-8946
  |     |
  |     |-- 遍历 m_loadedScripts
  |     |-- 根据 mode 选择对应钩子列表:
  |     |     LAUNCH       -> OnEffectLaunch
  |     |     LAUNCH_TARGET-> OnEffectLaunchTarget
  |     |     HIT          -> OnEffectHit
  |     |     HIT_TARGET   -> OnEffectHitTarget
  |     |-- 调用匹配的效果处理器
  |     |-- 检查 _IsDefaultEffectPrevented()
  |
  |-- if (!preventDefault)
        (this->*SpellEffectHandlers[spellEffectInfo.Effect].Value)()
```

### 4.2 伤害/治疗计算链

```
CalculateDamage()  // Spell.cpp:7247-7251
  |
  +-> WorldObject::CalculateSpellDamage()
        |
        +-> SpellEffectInfo::CalcValue()  // SpellInfo.cpp:515-602
              |
              |-- CalcBaseValue(caster, target, itemId, itemLevel)
              |     |-- Scaling.Coefficient != 0:
              |     |     查 SpellScalingEntry, 按装备等级/法术等级取值
              |     |-- Scaling.Coefficient == 0:
              |     |     直接使用 BasePoints
              |
              |-- 天赋修正 (TraitDefinitionEffectPointsEntry)
              |     Set / Multiply / None 三种操作类型
              |
              |-- 方差随机化 (Scaling.Variance)
              |     delta = |Variance * 0.5|
              |     value += basePoints * frand(-delta, delta)
              |
              |-- 资源系数 (Scaling.ResourceCoefficient)
              |     comboDamage = value * ResourceCoefficient
              |
              |-- 等级加成 (RealPointsPerLevel)
              |     value += level * basePointsPerLevel
              |
              |-- 连击点加成 (PointsPerResource)
              |     value += comboDamage * comboPoints
              |
              |-- 精通修正 (SPELL_ATTR8_MASTERY_AFFECTS_POINTS)
              |     value += mastery * BonusCoefficient
              |
              |-- ApplyEffectModifiers(caster, effIndex, value)
                    全局效果修正器最终调整
```

**CalcBaseValue 返回值变更说明：** 该方法在近期提交 (`62a459d508`) 中返回类型从 `int32` 改为 `double` (`SpellEffectValue`)，提升了高数值场景下的精度。

### 4.3 Empower 法术机制

Empower 法术（蓄力施法）的运行时数据存储在 `EmpowerData` 结构中 (`src/server/game/Spells/Spell.h:774-782`)：

```cpp
struct EmpowerData
{
    Milliseconds MinHoldTime = 0ms;          // 最小蓄力时间
    std::vector<Milliseconds> StageDurations; // 各阶段持续时间
    int32 CompletedStages = 0;               // 已完成阶段数
    bool IsReleasedByClient = false;         // 客户端是否已释放
    bool IsReleased = false;                 // 服务端是否已确认释放
};
```

**初始化流程** (`src/server/game/Spells/Spell.cpp:3989-4008`)，在 `handle_immediate()` 的引导法术处理中：

1. 法术构造时检测 `IsEmpowerSpell()`，创建 `m_empower` 对象
2. 在计算引导持续时间时，若为 Empower 法术：
   - 计算原始持续时间的缩放比例 `ratio = duration / originalDuration`
   - 按比例转换每个阶段的阈值时间 `StageDurations[i] = Thresholds[i] * ratio`
   - 确保总持续时间等于修改后的 duration（最后一阶段为余量）
   - 计算最小蓄力时间 `MinHoldTime = StageDurations[0] * EmpowerMinHoldStagePercent`
   - 额外追加 `SPELL_EMPOWER_HOLD_TIME_AT_MAX` 以允许最大蓄力后的释放窗口

**效果增强体现：** Empower 阶段数通过 `CompletedStages` 传递给效果处理器，效果处理器根据完成阶段调整伤害系数、效果数值等。

### 4.4 效果处理器分类 (~120 个 Effect* 方法)

`SpellEffectHandlers` 分派表 (`src/server/game/Spells/SpellEffects.cpp:90-447`) 定义了 `TOTAL_SPELL_EFFECTS` (355) 个条目。其中有效的处理器约 120 个，其余为 `EffectNULL`（空实现）或 `EffectUnused`（废弃）。以下按功能分类：

#### 4.4.1 伤害类 (Damage)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectSchoolDMG` | 2 | 魔法学校伤害 |
| `EffectWeaponDmg` | 17, 31, 58, 121 | 武器伤害（多种变体：无学校、百分比、标准、标准化） |
| `EffectEnvironmentalDMG` | 7 | 环境伤害 |
| `EffectHealthLeech` | 9 | 生命汲取 |
| `EffectPowerBurn` | 62 | 法力燃烧 |
| `EffectDamageFromMaxHealthPCT` | 165 | 最大生命值百分比伤害 |
| `EffectGameObjectDamage` | 87 | 游戏对象伤害 |
| `EffectDurabilityDamage` | 111 | 耐久度伤害 |
| `EffectDurabilityDamagePCT` | 115 | 耐久度百分比伤害 |

#### 4.4.2 治疗类 (Healing)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectHeal` | 10 | 标准治疗 |
| `EffectHealPct` | 136 | 百分比治疗 |
| `EffectHealMaxHealth` | 67 | 按最大生命值治疗 |
| `EffectHealMechanical` | 75 | 机械单位治疗 |
| `EffectHealBattlePetPct` | 200 | 战宠百分比治疗 |
| `EffectSpiritHeal` | 117 | 灵魂治疗（墓地复活相关） |

#### 4.4.3 光环类 (Aura)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectApplyAura` | 6, 174 | 施加光环（含宠物光环变体） |
| `EffectRemoveAura` | 164, 203 | 移除光环 |
| `EffectModifyAuraStacks` | 289 | 修改光环层数 |
| `EffectDispel` | 38 | 驱散 |
| `EffectDispelMechanic` | 108 | 按机制驱散 |
| `EffectStealBeneficialBuff` | 126 | 窃取增益 |
| `EffectRemoveAuraBySpellLabel` | 212 | 按标签移除光环 |

#### 4.4.4 传送与位移类 (Movement & Teleport)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectTeleportUnits` | 252 | 传送单位 |
| `EffectTeleportUnitsWithVisualLoadingScreen` | 15 | 带加载画面的传送 |
| `EffectTeleportToReturnPoint` | 13 | 传送回出发点 |
| `EffectTeleUnitsFaceCaster` | 43 | 传送到施法者面前 |
| `EffectJump` | 41 | 跳跃 |
| `EffectJumpDest` | 42, 213 | 跳跃到目标点 |
| `EffectLeap` | 29 | 冲刺 |
| `EffectLeapBack` | 138 | 后跳 |
| `EffectCharge` | 96 | 冲锋 |
| `EffectChargeDest` | 149 | 冲锋到目标点 |
| `EffectJumpCharge` | 254 | 跳跃冲锋 |
| `EffectKnockBack` | 98, 144 | 击退 |
| `EffectPullTowards` | 124 | 拉向施法者 |
| `EffectPullTowardsDest` | 145 | 拉向目标点 |
| `EffectSendTaxi` | 123 | 出租车/飞行路径 |

#### 4.4.5 资源与能量类 (Power & Resource)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectPowerDrain` | 8 | 能量汲取 |
| `EffectPowerBurn` | 62 | 法力燃烧 |
| `EffectEnergize` | 30 | 恢复能量 |
| `EffectEnergizePct` | 137 | 百分比恢复能量 |

#### 4.4.6 召唤类 (Summon)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectSummonType` | 28 | 召唤（通用） |
| `EffectSummonPet` | 56 | 召唤宠物 |
| `EffectSummonObject` | 104 | 召唤对象（有槽位） |
| `EffectSummonObjectWild` | 76 | 召唤对象（野外） |
| `EffectSummonPlayer` | 85 | 召唤玩家 |
| `EffectSummonChangeItem` | 34 | 召唤变身物品 |
| `EffectSummonRaFFriend` | 152 | 召唤 RAF 好友 |
| `EffectSummonPersonalGameObject` | 171 | 召唤个人游戏对象 |
| `EffectCreateTamedPet` | 153 | 创建可驯养宠物 |
| `EffectDismissPet` | 102 | 解散宠物 |
| `EffectDestroyAllTotems` | 110 | 销毁所有图腾 |

#### 4.4.7 物品与制造类 (Item & Crafting)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectCreateItem` | 24 | 创建物品 |
| `EffectCreateItem2` | 157 | 创建物品 v2 |
| `EffectCreateRandomItem` | 59 | 创建随机物品 |
| `EffectCreateHeirloomItem` | 222 | 创建传家宝 |
| `EffectEnchantItemPerm` | 53 | 永久附魔 |
| `EffectEnchantItemTmp` | 54 | 临时附魔 |
| `EffectEnchantItemPrismatic` | 156 | 棱彩附魔 |
| `EffectEnchantHeldItem` | 92 | 附魔手持物品 |
| `EffectDisEnchant` | 99 | 分解 |
| `EffectTradeSkill` | 47 | 专业技能 |
| `EffectProspecting` | 127 | 选矿 |
| `EffectMilling` | 158 | 研磨 |
| `EffectDestroyItem` | 169 | 销毁物品 |
| `EffectRechargeItem` | 66 | 充能物品 |
| `EffectApplyEnchantIllusion` | 243 | 应用附魔幻象 |
| `EffectUpgradeHeirloom` | 245 | 升级传家宝 |

#### 4.4.8 光环效果与区域类 (Persistent & Area)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectPersistentAA` | 27 | 持久区域光环 |
| `EffectCreateAreaTrigger` | 179, 353 | 创建区域触发器 |
| `EffectActivateObject` | 86 | 激活游戏对象 |
| `EffectTransmitted` | 50 | 传送门/门 |

#### 4.4.9 战斗机制类 (Combat Mechanics)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectInstaKill` | 1 | 即时击杀 |
| `EffectInterruptCast` | 68 | 打断施法 |
| `EffectTaunt` | 114 | 嘲讽 |
| `EffectThreat` | 63 | 仇恨 |
| `EffectModifyThreatPercent` | 125 | 修改仇恨百分比 |
| `EffectRedirectThreat` | 130 | 重定向仇恨 |
| `EffectParry` | 22 | 招架 |
| `EffectBlock` | 23 | 格挡 |
| `EffectDualWield` | 40 | 双持 |
| `EffectAddExtraAttacks` | 19 | 额外攻击 |
| `EffectDistract` | 69 | 分心 |
| `EffectSanctuary` | 79, 176 | 圣所 |

#### 4.4.10 任务与声望类 (Quest & Reputation)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectQuestComplete` | 16 | 完成任务 |
| `EffectQuestStart` | 150 | 开始任务 |
| `EffectQuestFail` | 147 | 任务失败 |
| `EffectQuestClear` | 139 | 清除任务 |
| `EffectKillCredit` | 134 | 击杀计数 |
| `EffectKillCreditPersonal` | 90 | 个人击杀计数 |
| `EffectKillCreditLabel` | 316, 317 | 标签击杀计数 |
| `EffectReputation` | 103, 184 | 声望修改 |

#### 4.4.11 增益与状态类 (Buffs & State)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectApplyGlyph` | 74 | 施加铭文 |
| `EffectTitanGrip` | 155 | 泰坦之握 |
| `EffectActivateSpec` | 162 | 激活专精 |
| `EffectSelfResurrect` | 94 | 自我复活 |
| `EffectResurrect` | 18 | 复活他人 |
| `EffectResurrectPet` | 109 | 复活宠物 |
| `EffectResurrectNew` | 378 | 新版复活 |
| `EffectResurrectWithAura` | 172 | 带光环复活 |
| `EffectInebriate` | 100 | 醉酒 |
| `EffectPickPocket` | 71 | 扒窃 |
| `EffectSkinning` | 95 | 剥皮 |
| `EffectSkinPlayerCorpse` | 116 | 剥玩家尸体 |
| `EffectFeedPet` | 101 | 喂养宠物 |
| `EffectTameCreature` | 55 | 驯服生物 |

#### 4.4.12 触发与连锁类 (Trigger & Chain)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectTriggerSpell` | 64, 142 | 触发法术 |
| `EffectTriggerMissileSpell` | 32, 148 | 触发投射物法术 |
| `EffectForceCast` | 140, 141 | 强制施法 |
| `EffectForceCast2` | 160 | 强制施法 v2 |
| `EffectTriggerRitualOfSummoning` | 151 | 触发召唤仪式 |
| `EffectSendEvent` | 61 | 发送事件 |
| `EffectScriptEffect` | 77 | 脚本效果 |
| `EffectDummy` | 3 | 虚效果（脚本处理） |

#### 4.4.13 冷却与充能类 (Cooldown & Charges)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectModifyCooldown` | 290 | 修改冷却时间 |
| `EffectModifyCooldowns` | 291 | 批量修改冷却 |
| `EffectModifyCooldownsByCategory` | 292 | 按类别修改冷却 |
| `EffectModifySpellCharges` | 293 | 修改法术充能 |

#### 4.4.14 视觉与场景类 (Visual & Scene)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectPlayMovie` | 45 | 播放电影 |
| `EffectPlayMusic` | 132 | 播放音乐 |
| `EffectPlaySound` | 131 | 播放音效 |
| `EffectPlaySceneScriptPackage` | 195 | 播放场景脚本包 |
| `EffectPlayScene` | 198 | 播放场景 |
| `EffectCreateSceneObject` | 196 | 创建场景对象 |
| `EffectCreatePrivateSceneObject` | 197 | 创建私有场景对象 |
| `EffectCreateConversation` | 219 | 创建对话 |
| `EffectCancelConversation` | 113 | 取消对话 |
| `EffectCreatePrivateConversation` | 267 | 创建私有对话 |
| `EffectAddFarsight` | 72 | 远视 |

#### 4.4.15 货币与经验类 (Currency & Experience)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectGiveCurrency` | 166 | 给予货币 |
| `EffectIncreaseCurrencyCap` | 14 | 提高货币上限 |
| `EffectGiveExperience` | 236 | 给予经验 |
| `EffectGiveRestedExperience` | 237 | 给予休息经验 |
| `EffectGiveHonor` | 253 | 给予荣誉 |
| `EffectGiveArtifactPower` | 240 | 给予神器能量 |
| `EffectGiveArtifactPowerNoBonus` | 242 | 给予神器能量（无加成） |

#### 4.4.16 学习与专业技能类 (Learning & Skills)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectLearnSpell` | 36 | 学习法术 |
| `EffectLearnSkill` | 44 | 学习技能 |
| `EffectLearnPetSpell` | 57 | 学习宠物法术 |
| `EffectUntrainTalents` | 73 | 重置天赋 |
| `EffectUnlearnSpecialization` | 133 | 遗忘专精 |
| `EffectProficiency` | 60 | 熟练度 |
| `EffectSkill` | 118 | 技能 |
| `EffectLearnTransmogSet` | 255 | 学习幻化套装 |
| `EffectLearnTransmogIllusion` | 276 | 学习幻化幻象 |
| `EffectRemoveTalent` | 181 | 移除天赋 |
| `EffectLearnGarrisonBuilding` | 210 | 学习要塞建筑 |

#### 4.4.17 现代系统扩展类 (Modern System Extensions)

Dragonflight / 战妆天赋系统及后续扩展：

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectCreateTraitTreeConfig` | 303 | 创建天赋树配置 |
| `EffectChangeActiveCombatTraitConfig` | 304 | 切换战斗天赋配置 |
| `EffectRespecAzeriteEmpoweredItem` | 259 | 重铸艾泽里特 |
| `EffectLearnAzeriteEssencePower` | 265 | 学习艾泽里特精华 |
| `EffectApplyMountEquipment` | 268 | 应用坐骑装备 |
| `EffectUpdatePlayerPhase` | 167 | 更新玩家相位 |
| `EffectUpdateZoneAurasAndPhases` | 170 | 更新区域光环和相位 |
| `EffectUpdateInteractions` | 306 | 更新交互状态 |
| `EffectSkipCampaign` | 283 | 跳过战役 |
| `EffectSkipQuestLine` | 311 | 跳过任务线 |
| `EffectLaunchQuestChoice` | 205 | 启动任务选择 |
| `EffectGrantBattlePetLevel` | 225 | 提升战宠等级 |
| `EffectGrantBattlePetExperience` | 286 | 给予战宠经验 |
| `EffectChangeBattlePetQuality` | 204 | 改变战宠品质 |
| `EffectEnableBattlePets` | 201 | 启用战宠 |
| `EffectUncageBattlePet` | 192 | 释放战宠 |
| `EffectEquipTransmogOutfit` | 347 | 穿戴幻化套装 |
| `EffectSendChatMessage` | 284 | 发送聊天消息 |
| `EffectLearnWarbandScene` | 341 | 学习战团场景 |

#### 4.4.18 玩家数据类 (Player Data)

| 方法 | 效果编号 | 说明 |
|------|---------|------|
| `EffectSetPlayerDataElementAccount` | 335 | 设置账号级玩家数据 |
| `EffectSetPlayerDataElementCharacter` | 336 | 设置角色级玩家数据 |
| `EffectSetPlayerDataFlagAccount` | 337 | 设置账号级标志 |
| `EffectSetPlayerDataFlagCharacter` | 338 | 设置角色级标志 |

#### 4.4.19 空实现与废弃 (NULL & Unused)

分派表中大量条目映射到 `EffectNULL`（约 180+ 个）或 `EffectUnused`（约 25 个）。`EffectNULL` 通常记录调试日志后返回，而 `EffectUnused` 表示该效果 ID 在 TrinityCore 中未使用或已被其他机制替代。

---

## 5. 扩展点与脚本集成 (Extension Points & Script Integration)

### 5.1 效果管线中的脚本钩子

TrinityCore 在效果处理管线的多个关键节点插入了 `SpellScript` 钩子，允许脚本在不修改引擎代码的情况下介入效果处理流程。

#### 5.1.1 CallScriptEffectHandlers -- 效果级钩子

```
src/server/game/Spells/Spell.cpp:8902-8946
```

在 `HandleEffects()` 中，于调用分派表处理器之前执行。根据 `SpellEffectHandleMode` 选择对应的钩子列表：

| SpellEffectHandleMode | SpellScript 钩子列表 | SpellScriptHookType |
|-----------------------|---------------------|---------------------|
| `SPELL_EFFECT_HANDLE_LAUNCH` | `OnEffectLaunch` | `SPELL_SCRIPT_HOOK_EFFECT_LAUNCH` |
| `SPELL_EFFECT_HANDLE_LAUNCH_TARGET` | `OnEffectLaunchTarget` | `SPELL_SCRIPT_HOOK_EFFECT_LAUNCH_TARGET` |
| `SPELL_EFFECT_HANDLE_HIT` | `OnEffectHit` | `SPELL_SCRIPT_HOOK_EFFECT_HIT` |
| `SPELL_EFFECT_HANDLE_HIT_TARGET` | `OnEffectHitTarget` | `SPELL_SCRIPT_HOOK_EFFECT_HIT_TARGET` |

**关键行为：**
- 脚本通过 `_IsEffectPrevented(effIndex)` 检查效果是否已被前置脚本阻止
- 通过 `PreventDefaultEffect(effIndex)` 阻止引擎默认效果处理器执行
- `preventDefault` 返回值控制是否跳过 `SpellEffectHandlers[...]` 调用

#### 5.1.2 BeforeHit / OnHit / AfterHit -- 命中级钩子

| 钩子 | 位置 | 说明 |
|------|------|------|
| `BeforeHit` | `src/server/game/Spells/Spell.cpp:8960-8971` | `PreprocessSpellHit()` 开头，可修改命中逻辑 |
| `OnHit` | `src/server/game/Spells/Spell.cpp:8973-8983` | 效果命中后调用 |
| `AfterHit` | `src/server/game/Spells/Spell.cpp:8985-8994` | 所有目标处理完毕后调用 |

**BeforeHit 的特殊位置：** 在 `PreprocessSpellHit()` (`src/server/game/Spells/Spell.cpp:3105`) 中最早调用，此时免疫检查、DR 计算、光环基础值计算尚未执行。脚本可以在此时改变目标状态或法术行为。

#### 5.1.3 SuccessfulDispel -- 驱散成功钩子

```
src/server/game/Spells/Spell.cpp:8948-8958
```

在驱散效果 (`EffectDispel`) 成功驱散一个光环后调用，允许脚本在驱散发生时执行自定义逻辑。

### 5.2 钩子注册模式

在脚本中使用 `.effect` 绑定语法注册效果钩子：

```cpp
// 示例：注册 OnEffectHitTarget 钩子
class spell_my_spell_SpellScript : public SpellScript
{
    PrepareSpellScript(spell_my_spell_SpellScript);

    void HandleEffect(SpellEffIndex effIndex)
    {
        // 自定义逻辑，在此可访问:
        //   GetCaster()        -- 施法者
        //   GetHitUnit()       -- 命中单位
        //   GetEffectValue()   -- 效果数值
        //   PreventHitEffect() -- 阻止后续效果
    }

    void Register() override
    {
        OnEffectHitTarget += SpellEffectFn(spell_my_spell_SpellScript::HandleEffect,
            EFFECT_0, SPELL_EFFECT_SCHOOL_DAMAGE);
    }
};

void AddSC_my_spell()
{
    RegisterSpellScript(spell_my_spell_SpellScript);
}
```

### 5.3 与其他管线文档的交叉引用

| 文档 | 交叉内容 |
|------|---------|
| **01-cast-pipeline.md** | 法术准备阶段 (`prepare()`) 在效果管线之前执行，决定法术是否能进入效果处理阶段。准备阶段计算的法力消耗 (`m_powerCost`)、施法时间等数据在效果管线中仍可被引用。 |
| **02-target-selection.md** | 目标选取生成的 `m_UniqueTargetInfo`、`m_UniqueGOTargetInfo`、`m_UniqueItemInfo`、`m_UniqueCorpseTargetInfo` 容器是效果管线的直接输入。`TargetInfoBase` 层次结构中的 `EffectMask` 决定了哪些效果作用于哪些目标。 |
| **05-script-system.md** | `SpellScript` 类是效果管线脚本集成的核心。`OnEffectLaunch`、`OnEffectLaunchTarget`、`OnEffectHit`、`OnEffectHitTarget` 四组效果钩子直接嵌入 `HandleEffects()` 分派链。`BeforeHit`、`OnHit`、`AfterHit` 三组命中钩子控制命中阶段的脚本介入点。`PreventDefaultEffect()` 机制允许脚本完全替代默认效果处理器。 |

### 5.4 扩展效果处理器的步骤

当需要新增一个效果处理器时：

1. 在 `Spell` 类中声明新的 `Effect*()` 方法 (`src/server/game/Spells/Spell.h:281-468`)
2. 在 `SpellEffects.cpp` 中实现该方法
3. 在 `SpellEffectHandlers` 分派表 (`src/server/game/Spells/SpellEffects.cpp:90-447`) 中注册函数指针
4. 若需要脚本支持，在 `SpellScript` 中注册对应钩子
5. 方法内部根据 `effectHandleMode` 判断当前阶段，选择性执行逻辑（例如 `if (effectHandleMode != SPELL_EFFECT_HANDLE_HIT_TARGET) return;`）
