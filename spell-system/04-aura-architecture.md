# 04 - Aura 系统架构

## 1. 概述 (Overview)

Aura 是 TrinityCore 法术系统的持久化效应层。当一个 Spell 效果类型为 `SPELL_EFFECT_APPLY_AURA` 时，法术命中后不再是一次性执行，而是在目标身上创建一个 **Aura 对象**，持续对目标施加影响。

### Aura 与 Spell 的关系

```
Spell（瞬态）
  |
  | 法术命中 → SPELL_EFFECT_APPLY_AURA
  v
Aura（持久态）
  |
  |--- AuraEffect[0..N]    每个效果槽位一个
  |--- AuraApplication      每个受影响目标一个
  |
  | 生命周期内持续：
  |   - 周期性触发（Tick）
  |   - Proc 触发
  |   - 持续时间倒计时
  |   - 层数/充能管理
```

**关键区分：**

- **Spell** 是法术施放的完整过程，从 `Spell::prepare()` 到 `Spell::finish()`，生命周期短暂。
- **Aura** 是法术命中后在目标上留下的持久化效果，生命周期从创建到过期/移除。
- 一个 Spell 可以产生多个 Aura（如多目标 AOE），一个 Aura 可以持有多个 AuraEffect。

### 核心约束

- 每个单位最多持有 **300** 个 Aura（`MAX_AURAS`，`SpellAuraDefines.h:22`）
- Area Aura 的目标列表每 **500ms** 更新一次（`UPDATE_TARGET_MAP_INTERVAL`，`SpellAuras.h:58`）
- Aura 的 owner 可以是 `Unit`（`UnitAura`）或 `DynamicObject`（`DynObjAura`）

---

## 2. 核心数据结构 (Core Data Structures)

### 2.1 三层架构：Aura → AuraEffect → AuraApplication

```
+------------------------------------------------------+
| Aura (SpellAuras.h:167-319)                          |
|   持有者: WorldObject (Unit 或 DynamicObject)          |
|   身份: SpellInfo + CastId + CasterGUID               |
|   状态: Duration / StackAmount / ProcCharges          |
|                                                       |
|   +--- AuraEffect[0]  (SpellAuraEffects.h:29-374)     |
|   |      效果类型: AuraType (e.g. SPELL_AURA_MOD_STAT)|
|   |      数值: Amount / BaseAmount                    |
|   |      周期: Period / PeriodicTimer / TicksDone      |
|   |      处理: HandleEffect() → 具体类型 Handler       |
|   |                                                    |
|   +--- AuraEffect[1]                                  |
|   +--- AuraEffect[2]                                  |
|   |                                                    |
|   +--- AuraApplication → Target A                     |
|   +--- AuraApplication → Target B                     |
|   +--- AuraApplication → Target C                     |
+------------------------------------------------------+
```

**设计意图：** Aura 与 AuraEffect 分离是为了支持"一个法术产生多种效果"（如同时减速+减攻击速度），Aura 与 AuraApplication 分离是为了支持 Area Aura 的多目标共享。

### 2.2 Aura 类 (`SpellAuras.h:167-319`)

Aura 是整个系统的核心中枢，管理效果、应用、持续时间和 Proc。

```cpp
class TC_GAME_API Aura {
    // 身份信息
    SpellInfo const* const m_spellInfo;
    ObjectGuid const m_casterGuid;
    ObjectGuid const m_castId;
    WorldObject* const m_owner;          // Unit 或 DynamicObject

    // 时间管理
    int32 m_maxDuration;                  // 最大持续时间
    int32 m_duration;                     // 当前剩余时间
    int32 m_updateTargetMapInterval;     // Area Aura 目标刷新计时器

    // 层数与充能
    uint8 m_stackAmount;                 // 当前层数
    uint8 m_procCharges;                 // Proc 充能次数

    // 目标管理
    ApplicationMap m_applications;       // GUID → AuraApplication*

    // 效果数组
    AuraEffectVector _effects;           // 最多 MAX_SPELL_EFFECTS (3) 个
};
```

**关键方法：**

| 方法 | 位置 | 职责 |
|------|------|------|
| `TryRefreshStackOrCreate()` | `SpellAuras.cpp:353-391` | 入口：尝试刷新/堆叠或创建新 Aura |
| `CanStackWith()` | `SpellAuras.cpp:1641-1730` | 判断能否与已有 Aura 共存 |
| `UpdateOwner()` | `SpellAuras.cpp:820-856` | 每帧更新入口 |
| `Update()` | `SpellAuras.cpp:858-869` | 持续时间和资源消耗更新 |
| `ModStackAmount()` | `SpellAuras.cpp:1096-1132` | 层数增减与刷新 |
| `GetProcEffectMask()` | `SpellAuras.cpp:1842-1987` | Proc 效果掩码计算 |
| `PrepareProcToTrigger()` | `SpellAuras.cpp:1801-1816` | Proc 触发准备 |
| `TriggerProcOnEvent()` | `SpellAuras.cpp:2018-2040` | Proc 实际触发 |
| `CalcPPMProcChance()` | `SpellAuras.cpp:2042-2055` | PPM 触发概率计算 |

### 2.3 AuraEffect 类 (`SpellAuraEffects.h:29-374`)

每个 Spell 的每个有效效果槽位对应一个 AuraEffect，封装具体效果的行为。

```cpp
class TC_GAME_API AuraEffect {
    Aura* const m_base;                  // 所属 Aura
    SpellInfo const* const m_spellInfo;
    SpellEffectInfo const& m_effectInfo;

    int32 const m_baseAmount;            // 基础数值（DBC 定义）
    int32 _amount;                       // 当前实际数值（含系数计算）

    // 周期性效果
    int32 _periodicTimer;                // 下次 Tick 剩余时间
    int32 _period;                       // Tick 间隔
    uint32 _ticksDone;                   // 已完成 Tick 数

    bool m_canBeRecalculated;            // 是否可重新计算
    bool m_isPeriodic;                   // 是否为周期性效果
};
```

**效果处理核心函数：**

`AuraEffect::HandleEffect()` (`SpellAuraEffects.cpp:1125-1169`) 是所有效果应用的分发中心：

```cpp
void AuraEffect::HandleEffect(AuraApplication* aurApp, uint8 mode, bool apply,
                              AuraEffect const* triggeredBy) {
    // 1. 注册/注销效果到 Unit 的效果列表
    if (mode & AURA_EFFECT_HANDLE_REAL)
        aurApp->GetTarget()->_RegisterAuraEffect(this, apply);

    // 2. 应用/移除 SpellMod
    if (mode & AURA_EFFECT_HANDLE_CHANGE_AMOUNT_MASK)
        ApplySpellMod(aurApp->GetTarget(), apply, triggeredBy);

    // 3. AuraScript OnEffectApply/Remove Hook
    bool prevented = apply
        ? GetBase()->CallScriptEffectApplyHandlers(...)
        : GetBase()->CallScriptEffectRemoveHandlers(...);

    // 4. 默认效果处理器（按 AuraType 分发）
    if (!prevented)
        (*this.*AuraEffectHandler[GetAuraType()].Value)(aurApp, mode, apply);

    // 5. AuraScript AfterEffectApply/Remove Hook
    if (apply)
        GetBase()->CallScriptAfterEffectApplyHandlers(...);
    else
        GetBase()->CallScriptAfterEffectRemoveHandlers(...);
}
```

**数值变更流程：**

`AuraEffect::ChangeAmount()` (`SpellAuraEffects.cpp:1082-1123`)：

```
ChangeAmount(newAmount, mark, onStackOrReapply)
  |
  |→ 对所有 AuraApplication：
  |     1. _RegisterAuraEffect(this, false)    // 注销旧效果
  |     2. HandleEffect(aurApp, mask, false)   // 移除旧效果
  |
  |→ _amount = newAmount                       // 更新数值
  |→ CalculateSpellMod()                       // 重算 SpellMod
  |
  |→ 对所有 AuraApplication：
  |     3. _RegisterAuraEffect(this, true)     // 注册新效果
  |     4. HandleEffect(aurApp, mask, true)    // 应用新效果
  |
  |→ SetNeedClientUpdateForTargets()           // 通知客户端
```

### 2.4 AuraApplication 类 (`SpellAuras.h:60-104`)

AuraApplication 表示 Aura 在某个具体目标上的"应用实例"。对于非 Area Aura，一个 Aura 只有一个 AuraApplication（施放者自己）。对于 Area Aura，一个 Aura 可能有多个 AuraApplication。

```cpp
class TC_GAME_API AuraApplication {
    Unit* const _target;            // 应用目标
    Aura* const _base;              // 所属 Aura
    uint16 _slot;                   // Aura 在目标上的显示槽位
    uint16 _flags;                  // AFLAG_POSITIVE / AFLAG_NEGATIVE 等
    uint32 _effectMask;             // 当前生效的效果掩码
    uint32 _effectsToApply;         // 待应用效果掩码（命中时使用）
    AuraRemoveMode _removeMode;8;   // 移除原因
    bool _needClientUpdate:1;       // 是否需要发送客户端更新
};
```

**关键标志位：** `AFLAG_POSITIVE` (0x0002) 决定该 Aura 在客户端显示为增益还是减益。

### 2.5 AuraCreateInfo 结构体 (`SpellAuras.h:106-148`)

Builder 模式创建 Aura 的参数容器：

```cpp
struct TC_GAME_API AuraCreateInfo {
    ObjectGuid CasterGUID;
    Unit* Caster = nullptr;
    int32 const* BaseAmount = nullptr;
    ObjectGuid CastItemGUID;
    uint32 CastItemId = 0;
    int32 CastItemLevel = -1;
    bool* IsRefresh = nullptr;       // 输出：是否为刷新已有 Aura
    int32 StackAmount = 1;           // 初始层数
    bool ResetPeriodicTimer = true;

    // Builder 链式设置
    AuraCreateInfo& SetCaster(Unit* caster);
    AuraCreateInfo& SetBaseAmount(int32 const* bp);
    AuraCreateInfo& SetStackAmount(int32 stackAmount);
    // ...
};
```

### 2.6 AuraKey 结构体 (`SpellAuras.h:151-159`)

用于数据库持久化的唯一标识：

```cpp
struct AuraKey {
    ObjectGuid Caster;       // 施放者 GUID
    ObjectGuid Item;         // 施放物品 GUID
    uint32 SpellId;          // 法术 ID
    uint32 EffectMask;       // 效果掩码

    // C++20 默认比较运算符
    friend std::strong_ordering operator<=>(AuraKey const&, AuraKey const&) = default;
};
```

### 2.7 UnitAura vs DynObjAura

```
                  Aura (基类)
                 /          \
          UnitAura        DynObjAura
     (SpellAuras.h:434)  (SpellAuras.h:461)
               |                |
         owner = Unit     owner = DynamicObject
```

| 特性 | UnitAura (`SpellAuras.h:434-459`) | DynObjAura (`SpellAuras.h:461-472`) |
|------|------|------|
| **Owner 类型** | Unit (Player/NPC) | DynamicObject |
| **创建时机** | 法术命中 Unit 时 | 法术产生 DynamicObject 时 |
| **目标来源** | `FillTargetMap()` 根据效果半径 | `FillTargetMap()` 基于动态对象位置 |
| **Diminishing** | 支持（`m_AuraDRGroup`） | 不支持 |
| **Static Application** | 有（`_staticApplications`） | 无（纯 Area） |
| **心跳** | 支持（`Heartbeat()`） | 不支持 |
| **典型场景** | 大多数 Buff/Debuff | 萨满图腾光环、法师冰环等 |

**DynObjAura 的关键限制：** 同一施放者、同一法术 ID 的 DynObjAura 不能共存（`CanStackWith()`，`SpellAuras.cpp:1651-1656`），且不会命中飞行中的目标（`UpdateTargetMap()`，`SpellAuras.cpp:728`）。

---

## 3. 生命周期管理 (Lifecycle Management)

### 3.1 创建流程

```
Spell::finish()
  |
  |→ Spell::HandleEffects() → SPELL_EFFECT_APPLY_AURA
  |     |
  |     |→ 构建 AuraCreateInfo
  |     |   .SetCaster(caster)
  |     |   .SetBaseAmount(m_spellValue->EffectBasePoints)
  |     |   .SetStackAmount(1)
  |     |   .SetIsRefresh(&isRefresh)
  |     |
  |     v
  |   Aura::TryRefreshStackOrCreate(createInfo)     [SpellAuras.cpp:353-391]
  |     |
  |     |→ BuildEffectMaskForOwner()  // 过滤 Owner 不适用的效果
  |     |
  |     |→ _TryStackingOrRefreshingExistingAura()   [Unit.cpp:3385-3446]
  |     |     |
  |     |     |→ GetOwnedAura(spellId, casterGUID, castItemGUID, 0)
  |     |     |   // 查找已有的同 ID Aura
  |     |     |
  |     |     |  [找到] → ModStackAmount() → 返回已存在 Aura
  |     |     |  [未找到] → 返回 nullptr
  |     |
  |     |  [找到已存在 Aura] → 设置 IsRefresh = true → 返回
  |     |  [未找到]
  |        |
  |        v
  |   Aura::Create(createInfo)                        [SpellAuras.cpp:406-459]
  |     |
  |     |→ 解析 Caster（ObjectAccessor 查找）
  |     |→ 检查 Owner 是否在世界中
  |     |
  |     |→ switch (owner->GetTypeId()):
  |     |     TYPEID_UNIT/PLAYER → new UnitAura(createInfo)
  |     |     TYPEID_DYNAMICOBJECT → new DynObjAura(createInfo)
  |     |
  |     |→ [UnitAura] AddStaticApplication(owner, effMask)
  |     |    // 为 Owner 创建 AuraApplication 并应用
  |
  v
Unit::_AddAura(aura, caster)                         [Unit.cpp:3448]
  |
  |→ m_ownedAuras.emplace(id, aura)       // 加入拥有 Aura 列表
  |→ _RemoveNoStackAurasDueToAura()       // 移除互斥的已有 Aura
  |→ aura->SetIsSingleTarget()            // 处理单一目标标记
  |→ aura->_ApplyForTargets()             // 对所有目标应用
  |
  v
Unit::_ApplyAura(aurApp, effMask)
  |
  |→ aurApp->_HandleEffect(effIndex, true)    // 应用每个效果
  |→ aurApp->SetNeedClientUpdate()            // 通知客户端
```

### 3.2 完整生命周期 ASCII 图

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Aura 生命周期                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  [创建]                                                            │
│    TryRefreshStackOrCreate ──→ 已存在? ──[是]──→ ModStackAmount    │
│         │                                                     │    │
│         [否]                                                    v    │
│         v                                                   刷新   │
│    Aura::Create ──→ new UnitAura / DynObjAura               计时器  │
│         │                                                     │    │
│         v                                                     v    │
│    _InitEffects() ←── 创建 AuraEffect[0..N]                        │
│         │                                                          │
│         v                                                          │
│    Unit::_AddAura() ←── 加入 m_ownedAuras                          │
│         │                                                          │
│         v                                                          │
│    _ApplyForTargets() ←── 对每个目标创建 AuraApplication            │
│         │                                                          │
│         v                                                          │
│    _HandleEffect(apply=true) ←── AuraEffect::HandleEffect          │
│         │                      ←── 注册到 Unit 效果系统             │
│         │                      ←── AuraScript OnEffectApply Hook   │
│         │                      ←── 默认 Handler 执行               │
│         │                      ←── AuraScript After Hook           │
│         │                                                          │
│  ═══════╪══════════════════════════════════════════════════════════  │
│         │               [运行中 - 每帧更新]                        │
│         v                                                          │
│    UpdateOwner(diff) ── SpellAuras.cpp:820-856                    │
│         │                                                          │
│         ├──→ Update(diff)              持续时间/资源消耗            │
│         │     └──→ duration <= 0? ──→ 过期移除                    │
│         │                                                          │
│         ├──→ UpdateTargetMap(caster)   每500ms刷新Area目标         │
│         │     └──→ FillTargetMap() → 新目标应用/旧目标移除         │
│         │                                                          │
│         ├──→ AuraEffect::Update()      周期Tick                    │
│         │     └──→ PeriodicTick() → 治疗/伤害/触发法术            │
│         │                                                          │
│         └──→ _DeleteRemovedApplications() 清理已移除的应用         │
│                                                                     │
│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │
│         │               [移除触发]                                 │
│         │    • 持续时间到期 (AURA_REMOVE_BY_EXPIRE)                │
│         │    • 主动取消 (AURA_REMOVE_BY_CANCEL)                    │
│         │    • 驱散 (AURA_REMOVE_BY_ENEMY_SPELL)                  │
│         │    • 充能耗尽 (ModStackAmount → stackAmount <= 0)       │
│         │    • 死亡 (AURA_REMOVE_BY_DEATH)                         │
│         v                                                          │
│    Remove(removeMode)                                              │
│         │                                                          │
│         v                                                          │
│    _UnapplyForTargets() ←── 对每个目标反应用                        │
│         │                                                          │
│         v                                                          │
│    _HandleEffect(apply=false) ←── 移除效果                         │
│         │                      ←── 从 Unit 效果系统注销             │
│         │                      ←── AuraScript OnEffectRemove Hook  │
│         │                      ←── 默认 Handler 反向执行           │
│         │                      ←── AuraScript After Hook           │
│         │                                                          │
│         v                                                          │
│    ~Aura() ←── 析构，释放所有 AuraEffect 和 AuraApplication        │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.3 持续时间管理

**设置与刷新：**

```
SetDuration(duration, withMods)     直接设置当前持续时间
RefreshDuration(withMods)           重新计算并设置为最大持续时间
CalcMaxDuration()                   基于施放者和法术信息计算最大持续时间
RefreshTimers(resetPeriodicTimer)   同时刷新持续时间、周期计时器
```

**特殊规则：**
- 被动法术（Passive）没有 DurationEntry 时，持续时间为 -1（永久）
- 被动法术的永久持续时间定义：`maxDuration = -1`（`CalcMaxDuration()`，`SpellAuras.cpp:928-929`）
- 充能消耗完后的移除通过 `ModStackAmount(-1)` 触发（`SpellAuras.cpp:1111-1115`）

### 3.4 层数管理

**`ModStackAmount()` 流程** (`SpellAuras.cpp:1096-1132`)：

```
ModStackAmount(num, removeMode, resetPeriodicTimer)
  |
  |→ stackAmount = m_stackAmount + num
  |→ maxStackAmount = CalcMaxStackAmount()
  |
  |→ [num > 0 && stackAmount > max]
  |     [不可堆叠] → stackAmount = 1
  |     [可堆叠]   → stackAmount = max
  |
  |→ [stackAmount <= 0] → Remove(removeMode) → return true
  |
  |→ refresh = (新层数 >= 旧层数) && (可堆叠 || 非唯一)
  |
  |→ SetStackAmount(stackAmount)
  |
  |→ [refresh]
  |     RefreshTimers(resetPeriodicTimer)   // 刷新持续时间和周期
  |     SetCharges(CalcMaxCharges())        // 重置充能
  |
  |→ SetNeedClientUpdateForTargets()        // 通知客户端
```

### 3.5 充能管理

```
SetCharges(charges)              设置充能数
ModCharges(num, removeMode)      增减充能
DropCharge(removeMode)           消耗一次充能 = ModCharges(-1)
ModChargesDelayed(num)           延迟增减充能（通过事件系统）
DropChargeDelayed(delay)         延迟消耗充能
CalcMaxCharges(caster)           计算最大充能数
```

**Proc 充能消耗流程：**

```
PrepareProcChargeDrop(procEntry, eventInfo)      [SpellAuras.cpp:1818-1826]
  |
  |→ [PROC_ATTR_USE_STACKS_FOR_CHARGES 未设置 && IsUsingCharges()]
  |     --m_procCharges
  |     SetNeedClientUpdateForTargets()
  |
ConsumeProcCharges(procEntry)                     [SpellAuras.cpp:1828-1840]
  |
  |→ [PROC_ATTR_USE_STACKS_FOR_CHARGES 已设置]
  |     ModStackAmount(-1)     // 消耗层数代替充能
  |
  |→ [IsUsingCharges() && GetCharges() == 0]
  |     Remove()               // 充能耗尽 → 移除 Aura
```

### 3.6 Area Aura 目标更新循环

Area Aura 每 500ms（`UPDATE_TARGET_MAP_INTERVAL`）执行一次目标列表刷新。

**`UpdateTargetMap()` 流程** (`SpellAuras.cpp:661-796`)：

```
UpdateTargetMap(caster, apply=true)
  |
  |→ m_updateTargetMapInterval = UPDATE_TARGET_MAP_INTERVAL (500ms)
  |
  |→ FillTargetMap(targets, caster)  // 虚函数：子类填充目标列表
  |   UnitAura:  基于效果半径搜索周围 Unit
  |   DynObjAura: 基于 DynamicObject 位置搜索
  |
  |→ 遍历已有 Application:
  |     [目标不在新列表中] → 加入 targetsToRemove
  |     [目标免疫]        → 加入 targetsToRemove
  |     [效果掩码变化]    → 更新效果掩码
  |     [无变化]          → 从 targets 中移除（无需处理）
  |
  |→ 遍历新目标列表:
  |     [免疫检查]        → 排除
  |     [CanStackWith]    → 排除
  |     [IsHighestExclusiveAura] → 排除
  |     [DynObjAura && 目标飞行] → 排除
  |     [通过]            → _CreateAuraApplication()
  |
  |→ 移除旧目标:
  |     _UnapplyAura(aurApp, AURA_REMOVE_BY_DEFAULT)
  |
  |→ 应用新目标:
  |     _ApplyAura(aurApp, effMask)
```

**在 `UpdateOwner()` 中的调度** (`SpellAuras.cpp:820-856`)：

```cpp
void Aura::UpdateOwner(uint32 diff, WorldObject* owner) {
    Update(diff, caster);                    // 持续时间更新

    if (m_updateTargetMapInterval <= diff)
        UpdateTargetMap(caster);             // 500ms 到期 → 刷新目标
    else
        m_updateTargetMapInterval -= diff;   // 累减计时

    for (AuraEffect* effect : GetAuraEffects())
        effect->Update(diff, caster);        // 周期 Tick 更新

    _DeleteRemovedApplications();            // 延迟清理
}
```

---

## 4. 关键算法与分支 (Key Algorithms & Branches)

### 4.1 堆叠规则：CanStackWith

`CanStackWith()` (`SpellAuras.cpp:1641-1730`) 决定两个 Aura 能否在同一目标上共存。返回 `false` 意味着新 Aura 将取代旧 Aura。

```
CanStackWith(existingAura)
  |
  |→ [自身] → return true
  |
  |→ [同一施放者 && DynObjAura] → return false
  |   // DynObjAura: 同一施放者同一法术不共存
  |
  |→ [被动法术 && 同一施放者 && 同等级/同法术] → return false
  |
  |→ [已有法术的 TriggerSpell == 自身法术ID] → return true
  |   // 防止触发法术移除触发源
  |
  |→ [自身法术的 TriggerSpell == 已有法术ID] → return true
  |   // 防止刷新触发法术时被触发源移除
  |
  |→ [IsAuraExclusiveBySpecificWith] → return false
  |   // 法术专有互斥规则（DBC 定义）
  |
  |→ CheckSpellGroupStackRules():
  |     EXCLUSIVE                    → return false
  |     EXCLUSIVE_HIGHEST           → return false（已有 Aura 等级 >= 新的）
  |     EXCLUSIVE_FROM_SAME_CASTER  → [同施放者] return false
  |     DEFAULT / EXCLUSIVE_SAME_EFFECT → 继续
  |
  |→ [不同 SpellFamilyName] → return true
  |   // 不同法术家族永远共存
  |
  |→ [不同施放者]:
  |     [引导法术] → return true
  |     [DOT_STACKING_RULE] → return true
  |     [都有非区域周期效果] → return true
  |     // DOT/HOT 从不同施放者可堆叠
  |
  |→ [CONTROL_VEHICLE 效果]:
  |     [无空座位] → return false
  |     [有空座位] → return true
  |
  |→ [SHOW_CONFIRMATION_PROMPT 效果] → return false
  |
  |→ [同一法术等级链]:
  |     [MultiSlot && 非区域] → return true
  |     [不同附魔物品] → return true
  |     [其他] → return false
  |     // 同一施放者同法术同等级默认不堆叠
  |
  |→ return true  // 默认允许共存
```

### 4.2 Proc 管线

Proc 是 Aura 的被动触发机制，当满足特定条件时自动触发效果。

#### 完整 Proc 流程图

```
Unit::ProcDamageAndSpell()
  |
  |→ 遍历身上所有 AuraApplication
  |     |
  |     v
  |   Aura::GetProcEffectMask(aurApp, eventInfo, now)
  |     [SpellAuras.cpp:1842-1987]
  |     |
  |     |→ [无 SpellProcEntry] → return 0
  |     |
  |     |→ [触发的法术就是自身] → return 0
  |     |   // 防止自触发循环
  |     |
  |     |→ [触发法术有 SPELL_ATTR3_SUPPRESS_* 属性] → return 0
  |     |
  |     |→ [隐身法术 && 不破隐法术] → return 0
  |     |
  |     |→ [无充能可消耗] → return 0
  |     |
  |     |→ [Proc 冷却中] → return 0
  |     |
  |     |→ [SpellMgr::CanSpellTriggerProcOnEvent] → 失败 → return 0
  |     |   // DBC 数据检查：TypeMask、SpellFamilyMask 等
  |     |
  |     |→ [ConditionMgr 检查] → 失败 → return 0
  |     |
  |     |→ [AuraScript CheckProc Hook] → return 0
  |     |
  |     |→ 逐效果检查: CheckEffectProc()
  |     |   [DisableEffectsMask 或效果检查失败] → 排除该效果
  |     |
  |     |→ [无效果通过] → return 0
  |     |
  |     |→ [装备要求检查] (被动法术+玩家) → return 0
  |     |
  |     |→ [SPELL_ATTR3_ONLY_PROC_OUTDOORS && 室内] → return 0
  |     |
  |     |→ [SPELL_ATTR3_ONLY_PROC_ON_CASTER && 非自身] → return 0
  |     |
  |     |→ [非允许坐下 Proc && 坐着] → return 0
  |     |
  |     v
  |   CalcProcChance(procEntry, eventInfo)
  |     [SpellAuras.cpp:1989-2016]
  |     |
  |     |→ chance = procEntry.Chance
  |     |
  |     |→ [有伤害信息 && ProcsPerMinute != 0]
  |     |     chance = GetPPMProcChance(WeaponSpeed, PPM)
  |     |
  |     |→ [ProcBasePPM > 0]
  |     |     chance = CalcPPMProcChance(caster)
  |     |
  |     |→ [SpellModOp::ProcChance] 修改 chance
  |     |
  |     |→ [PROC_ATTR_REDUCE_PROC_60 && level > 60]
  |     |     chance *= (1 - (level-60)/30)
  |     |
  |     |→ roll_chance_f(chance)
  |     |
  |     v
  |   [成功] → return procEffectMask
  |   [失败] → return 0
  |
  |→ [procEffectMask != 0]
  |     |
  |     v
  |   Aura::PrepareProcToTrigger(aurApp, eventInfo, now)
  |     [SpellAuras.cpp:1801-1816]
  |     |
  |     |→ CallScriptPrepareProcHandlers(aurApp, eventInfo)
  |     |   [被阻止] → return
  |     |
  |     |→ PrepareProcChargeDrop(procEntry, eventInfo)
  |     |   // 预扣充能（非 PROC_ATTR_USE_STACKS_FOR_CHARGES）
  |     |
  |     |→ AddProcCooldown(procEntry, now)
  |     |   // 设置 Proc 冷却
  |     |
  |     |→ SetLastProcSuccessTime(now)
  |     |
  |     v
  |   Aura::TriggerProcOnEvent(procEffectMask, aurApp, eventInfo)
  |     [SpellAuras.cpp:2018-2040]
  |     |
  |     |→ CallScriptProcHandlers(aurApp, eventInfo)
  |     |   [被阻止] → 跳过默认处理
  |     |
  |     |→ 遍历通过的效果:
  |     |     GetEffect(i)->HandleProc(aurApp, eventInfo)
  |     |     // 执行具体的 Proc Handler（如触发法术、触发伤害）
  |     |
  |     |→ CallScriptAfterProcHandlers(aurApp, eventInfo)
  |     |
  |     |→ ConsumeProcCharges(procEntry)
  |     |   // 最终消耗充能或层数
```

#### Proc 管线 ASCII 总览

```
┌──────────────────────────────────────────────────────────────┐
│                     Proc 管线                                │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  事件触发 (攻击/施法/受到伤害...)                             │
│       │                                                      │
│       v                                                      │
│  ┌─────────────────┐                                        │
│  │ GetProcEffectMask│  效果掩码筛选                          │
│  │ (1842-1987)     │  ─ SpellProcEntry 检查                 │
│  └────────┬────────┘  ─ 防自触发检查                        │
│           │           ─ 冷却检查                             │
│           │           ─ DBC/Condition 检查                   │
│           │           ─ AuraScript CheckProc Hook           │
│           │           ─ 逐效果 CheckEffectProc              │
│           v                                                  │
│  ┌─────────────────┐                                        │
│  │ CalcProcChance  │  概率计算                               │
│  │ (1989-2016)     │  ─ PPM/武器速度 → chance               │
│  └────────┬────────┘  ─ ProcBasePPM → chance                │
│           │           ─ SpellMod 修改                       │
│           │           ─ 等级衰减                             │
│           v                                                  │
│     roll_chance_f(chance)                                    │
│        │           │                                        │
│     [失败]       [成功]                                      │
│        │           │                                        │
│       丢弃         v                                        │
│              ┌──────────────────────┐                       │
│              │PrepareProcToTrigger  │  触发准备              │
│              │(1801-1816)          │  ─ PrepareProc Hook    │
│              └──────────┬───────────┘  ─ 预扣充能            │
│                         │              ─ 设置冷却            │
│                         v                                     │
│              ┌──────────────────────┐                       │
│              │TriggerProcOnEvent    │  实际触发              │
│              │(2018-2040)          │  ─ Proc Hook           │
│              └──────────┬───────────┘  ─ HandleProc (逐效果) │
│                         │              ─ AfterProc Hook      │
│                         v                                     │
│              ┌──────────────────────┐                       │
│              │ConsumeProcCharges    │  消耗结算              │
│              │(1828-1840)          │  ─ 充能耗尽→Remove     │
│              └──────────────────────┘  ─ 层数消耗           │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 4.3 PPM (Procs Per Minute) 机制

PPM 是一种基于时间的 Proc 概率调节机制，使触发频率趋于稳定。

**公式来源：** http://us.battle.net/wow/en/forum/topic/8197741003#1

**`CalcPPMProcChance()`** (`SpellAuras.cpp:2042-2055`)：

```cpp
float Aura::CalcPPMProcChance(Unit* actor) const {
    float ppm = m_spellInfo->CalcProcPPM(actor, GetCastItemLevel());
    float averageProcInterval = 60.0f / ppm;

    float secondsSinceLastAttempt = std::min(
        (currentTime - m_lastProcAttemptTime).count(), 10.0f);
    float secondsSinceLastProc = std::min(
        (currentTime - m_lastProcSuccessTime).count(), 1000.0f);

    // 核心公式：运气保护机制
    float chance = std::max(1.0f,
        1.0f + ((secondsSinceLastProc / averageProcInterval - 1.5f) * 3.0f))
        * ppm * secondsSinceLastAttempt / 60.0f;

    RoundToInterval(chance, 0.0f, 1.0f);
    return chance * 100.0f;  // 转为百分比
}
```

**PPM 特点：**

1. **自适应概率：** 距离上次成功触发越久，概率越高（运气保护）
2. **时间上限：** `secondsSinceLastAttempt` 最大 10 秒，`secondsSinceLastProc` 最大 1000 秒
3. **最低概率：** `std::max(1.0f, ...)` 确保最低 100% 基础概率
4. **两类 PPM：**
   - `SpellProcEntry.ProcsPerMinute`：基于武器速度的传统 PPM（`CalcProcChance` 中调用）
   - `SpellInfo.ProcBasePPM`：基于法术的 PPM（调用 `CalcPPMProcChance`）

### 4.4 AuraEffectHandleModes 位标志

`AuraEffectHandleModes` (`SpellAuraDefines.h:39-53`) 是一组位标志，控制效果处理器的调用模式：

```
位标志值                    含义
───────────────────────────────────────────────────────────────
AURA_EFFECT_HANDLE_DEFAULT = 0x00    默认（不触发任何处理）

AURA_EFFECT_HANDLE_REAL    = 0x01    实际应用/移除效果到 Unit
                                       注册/注销效果到 Unit 效果系统

AURA_EFFECT_HANDLE_SEND_FOR_CLIENT = 0x02
                                       向客户端发送应用/移除数据包

AURA_EFFECT_HANDLE_CHANGE_AMOUNT = 0x04
                                       效果数值变更时更新

AURA_EFFECT_HANDLE_REAPPLY = 0x08    Aura 重新应用时（堆叠/刷新）

AURA_EFFECT_HANDLE_STAT    = 0x10    属性重算时调用

AURA_EFFECT_HANDLE_SKILL   = 0x20    技能重算时调用
```

**常用组合掩码：**

```cpp
SEND_FOR_CLIENT_MASK = REAL | SEND_FOR_CLIENT           // 0x03
CHANGE_AMOUNT_MASK   = CHANGE_AMOUNT | REAL              // 0x05
CHANGE_AMOUNT_SEND_MASK = CHANGE_AMOUNT_MASK | SEND_FOR_CLIENT_MASK  // 0x07
REAL_OR_REAPPLY_MASK = REAPPLY | REAL                    // 0x09
```

**`HandleEffect()` 中的断言** (`SpellAuraEffects.cpp:1128-1134`)：调用时只允许以下几种模式：
- 单一标志：`REAL`, `SEND_FOR_CLIENT`, `CHANGE_AMOUNT`, `STAT`, `SKILL`, `REAPPLY`
- 组合标志：`CHANGE_AMOUNT | REAPPLY`（堆叠且数值变更时）

**各模式的典型调用场景：**

| 场景 | 调用模式 | 触发位置 |
|------|----------|----------|
| Aura 首次应用 | `REAL \| SEND_FOR_CLIENT` | `Unit::_ApplyAura` |
| Aura 被移除 | `REAL \| SEND_FOR_CLIENT` | `Unit::_UnapplyAura` |
| 层数堆叠刷新 | `CHANGE_AMOUNT \| REAPPLY` | `ChangeAmount()` |
| 仅数值变更 | `CHANGE_AMOUNT` | `ChangeAmount()` |
| 属性重算 | `STAT` | `Unit::_ApplyAllAuraStatMods` |
| 技能重算 | `SKILL` | `Unit::_ApplyAllAuraSkillMods` |

### 4.5 AuraRemoveMode 枚举

`AuraRemoveMode` (`SpellAuraDefines.h:55-64`) 记录 Aura 被移除的原因：

```cpp
enum AuraRemoveMode {
    AURA_REMOVE_NONE = 0,                // 未移除
    AURA_REMOVE_BY_DEFAULT = 1,          // 脚本移除 / 堆叠替换
    AURA_REMOVE_BY_INTERRUPT,            // 被中断（移动/施法等打断）
    AURA_REMOVE_BY_CANCEL,               // 被取消（右键点击 Buff）
    AURA_REMOVE_BY_ENEMY_SPELL,          // 被敌方驱散 / 吸收盾摧毁
    AURA_REMOVE_BY_EXPIRE,               // 持续时间到期
    AURA_REMOVE_BY_DEATH                 // 死亡移除
};
```

移除原因影响 AuraScript Hook 的行为（如 `OnRemove` 可以根据 `removeMode` 执行不同逻辑）。

---

## 5. 扩展点与脚本集成 (Extension Points & Script Integration)

### 5.1 AuraScript Hook 体系

Aura 通过 AuraScript 提供丰富的 Hook 点供脚本扩展。脚本通过 `sScriptMgr->CreateAuraScripts()` 加载，在 `Aura::LoadScripts()` (`SpellAuras.cpp:2065-2073`) 中初始化。

**核心 Hook 分类：**

#### 生命周期 Hook

| Hook | 调用位置 | 用途 |
|------|----------|------|
| `CheckAreaTarget` | `UpdateTargetMap` | 过滤 Area Aura 目标 |
| `AfterCheckAreaTarget` | `UpdateTargetMap` | 补充过滤逻辑 |
| `Dispel` / `AfterDispel` | `Unit::RemoveAurasDueToDispel` | 驱散时自定义行为 |

#### 效果应用/移除 Hook

| Hook | 调用位置 | 用途 |
|------|----------|------|
| `CheckApplyProc` | `GetProcEffectMask` | 控制 Proc 是否可触发 |
| `PrepareProc` | `PrepareProcToTrigger` | 准备 Proc（可阻止） |
| `Proc` | `TriggerProcOnEvent` | Proc 触发（可阻止默认行为） |
| `AfterProc` | `TriggerProcOnEvent` | Proc 后处理 |
| `CheckEffectProc` | `CheckEffectProc` | 单效果 Proc 检查 |
| `EffectProc` / `AfterEffectProc` | `AuraEffect::HandleProc` | 单效果 Proc 处理 |

#### 效果计算 Hook

| Hook | 调用位置 | 用途 |
|------|----------|------|
| `CalcAmount` | `AuraEffect::CalculateAmount` | 自定义效果数值 |
| `CalcPeriodic` | `AuraEffect::CalculatePeriodic` | 自定义周期间隔 |
| `CalcSpellMod` | `AuraEffect::CalculateSpellMod` | 自定义 SpellMod |
| `CalcCritChance` | `PeriodicTick` | 自定义周期暴击率 |
| `CalcDamageAndHealing` | `PeriodicTick` | 自定义伤害/治疗量 |

#### 吸收/分摊 Hook

| Hook | 调用位置 | 用途 |
|------|----------|------|
| `Absorb` / `AfterAbsorb` | `Unit::CalcAbsorbResist` | 自定义伤害吸收 |
| `ManaShield` / `AfterManaShield` | `Unit::CalcAbsorbResist` | 法力护盾 |
| `Split` | `Unit::CalcAbsorbResist` | 伤害分摊 |

#### 战斗事件 Hook

| Hook | 调用位置 | 用途 |
|------|----------|------|
| `EnterLeaveCombat` | `Unit::CombatStop` / `EnterCombat` | 进出战斗处理 |
| `OnHeartbeat` | `UnitAura::Heartbeat` | 心跳（定期执行） |

### 5.2 脚本加载与注册

```cpp
// Aura::LoadScripts() - SpellAuras.cpp:2065-2073
void Aura::LoadScripts() {
    sScriptMgr->CreateAuraScripts(m_spellInfo->Id, m_loadedScripts, this);
    for (AuraScript* script : m_loadedScripts) {
        script->Register();
    }
}
```

脚本通过 `Register()` 方法绑定具体的 Handler 到上述 Hook 点。每个 Hook 可以绑定多个 Handler，按注册顺序执行。

### 5.3 脚本阻止机制

多个关键 Hook 支持脚本"阻止"默认行为：

- `CheckProc` → 返回 `false` 阻止整个 Proc
- `PrepareProc` → 返回 `false` 阻止触发
- `Proc` → 返回 `true` 阻止默认 Proc 处理
- `CheckEffectProc` → 返回 `false` 排除该效果
- `EffectApply` → 返回 `true` 阻止默认应用 Handler
- `EffectRemove` → 返回 `true` 阻止默认移除 Handler

### 5.4 与其他模块的交叉引用

**与效果管线（03-effect-pipeline.md）的关系：**

- Spell 效果管线负责决定法术命中时执行什么操作。当效果类型为 `SPELL_EFFECT_APPLY_AURA` 时，管线调用 `Aura::TryRefreshStackOrCreate()` 创建 Aura。
- `SpellEffectInfo` 提供了 Aura 的基础参数（`ApplyAuraName` 即 AuraType，`BasePoints` 等）。
- AuraEffect 持有 `SpellEffectInfo` 的引用，效果数值计算依赖效果管线中的公式。

**与法术脚本（05-spell-scripting.md）的关系：**

- SpellScript 和 AuraScript 是两套独立的脚本体系。SpellScript 处理法术施放过程（准备、命中、效果执行），AuraScript 处理 Aura 的生命周期（应用、Tick、Proc、移除）。
- 法术效果执行时如果创建 Aura，则 AuraScript 接管后续逻辑。
- `OnEffectApply`/`OnEffectRemove` Hook 允许脚本在效果应用/移除时介入，与 SpellScript 的 `OnEffectHit` 形成时序衔接：`OnEffectHit` 在法术命中时触发，`OnEffectApply` 在 Aura 应用到目标时触发。
- Proc 触发产生的法术（如 `SPELL_AURA_PROC_TRIGGER_SPELL`）会创建新的 Spell 实例，此时新 Spell 的 SpellScript 独立运行。
