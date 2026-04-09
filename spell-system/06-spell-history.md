# 法术冷却与充能管理系统 (Spell Cooldown & Charge Management)

> 源文件: `src/server/game/Spells/SpellHistory.h` / `SpellHistory.cpp`

## 1. 概述 (Overview)

SpellHistory 类是 TrinityCore 中管理所有法术冷却、充能恢复、全局冷却(GCD)、法术系封锁(school lockout)的核心系统。每个 `Unit` 实例持有一个 `SpellHistory` 对象，负责追踪该单位所有法术的时间状态。

### 1.1 四个子系统

SpellHistory 管理以下四个紧密协作的子系统:

```
+---------------------------------------------------------------+
|                      SpellHistory                              |
|                                                                |
|  +------------------+  +------------------+  +---------------+ |
|  |  法术冷却管理      |  |  充能系统          |  |  GCD 管理     | |
|  |  Spell Cooldowns  |  |  Charge System   |  |  Global CD    | |
|  +------------------+  +------------------+  +---------------+ |
|                                                                |
|  +------------------------------------------------------------+|
|  |  法术系封锁 (School Lockout)                                ||
|  +------------------------------------------------------------+|
+---------------------------------------------------------------+
```

| 子系统 | 职责 | 核心存储 |
|--------|------|----------|
| **法术冷却** | 追踪每个法术的独立冷却和分类冷却(category cooldown) | `_spellCooldowns`, `_categoryCooldowns` |
| **充能系统** | 管理多次充能法术的消费与恢复队列 | `_categoryCharges` |
| **全局冷却(GCD)** | 追踪按恢复分类的全局冷却 | `_globalCooldowns` |
| **法术系封锁** | 追踪因打断效果导致的法术系锁定 | `_schoolLockouts[MAX_SPELL_SCHOOL]` |

### 1.2 与 Spell 的关系

SpellHistory 作为 `Unit` 的成员，在 Spell 执行完成后由 `SpellHistory::HandleCooldowns()` 触发冷却逻辑。Spell 系统负责法术施放本身，而 SpellHistory 负责施放后的时间状态管理。法术施放成功后会调用 `HandleCooldowns()` 来启动相应的冷却、消费充能、或标记 OnHold 状态。

跨文档引用: 法术状态机相关内容参见 [01-spell-state-machine.md](./01-spell-state-machine.md)。

---

## 2. 核心数据结构 (Core Data Structures)

### 2.1 CooldownEntry — 冷却条目

`src/server/game/Spells/SpellHistory.h:65-73`

```cpp
struct CooldownEntry
{
    uint32 SpellId = 0;                    // 法术ID
    TimePoint CooldownEnd = TimePoint::min(); // 法术冷却结束时间
    uint32 ItemId = 0;                     // 关联物品ID(物品施法时使用)
    uint32 CategoryId = 0;                 // 冷却分类ID
    TimePoint CategoryEnd = TimePoint::min(); // 分类冷却结束时间
    bool OnHold = false;                   // 是否处于等待触发状态
};
```

关键字段说明:
- **CooldownEnd / CategoryEnd**: 使用 `std::chrono::system_clock` 的时间点，精度为毫秒级别 (`Milliseconds`)
- **OnHold**: 标记冷却是否处于"等待事件触发"状态。OnHold 的冷却不会立即开始倒计时，而是等到事件触发后才转为正常冷却
- **ItemId**: 区分同一法术由不同物品触发时的独立冷却(如不同药水)

### 2.2 ChargeEntry — 充能条目

`src/server/game/Spells/SpellHistory.h:75-83`

```cpp
struct ChargeEntry
{
    ChargeEntry() = default;
    ChargeEntry(TimePoint startTime, Duration rechargeTime)
        : RechargeStart(startTime), RechargeEnd(startTime + rechargeTime) { }
    ChargeEntry(TimePoint startTime, TimePoint endTime)
        : RechargeStart(startTime), RechargeEnd(endTime) { }

    TimePoint RechargeStart;  // 恢复开始时间
    TimePoint RechargeEnd;    // 恢复结束时间(即可用时间)
};
```

充能条目记录每次消费后的一次恢复周期。恢复开始时间与上一次恢复结束时间对齐，确保多个充能按顺序依次恢复。

### 2.3 四种存储类型

`src/server/game/Spells/SpellHistory.h:86-89`

```cpp
using ChargeEntryCollection = std::deque<ChargeEntry>;
using CooldownStorageType = std::unordered_map<uint32 /*spellId*/, CooldownEntry>;
using CategoryCooldownStorageType = std::unordered_map<uint32 /*categoryId*/, CooldownEntry*>;
using ChargeStorageType = std::unordered_map<uint32 /*categoryId*/, ChargeEntryCollection>;
using GlobalCooldownStorageType = std::unordered_map<uint32 /*categoryId*/, TimePoint>;
```

| 存储类型 | 键 | 值 | 用途 |
|----------|------|------|------|
| `CooldownStorageType` | spellId | `CooldownEntry` | 按法术ID存储冷却 |
| `CategoryCooldownStorageType` | categoryId | `CooldownEntry*` | 按分类ID索引冷却(指针指向 spellCooldowns 中的条目) |
| `ChargeStorageType` | categoryId | `ChargeEntryCollection` (deque) | 按分类存储充能恢复队列 |
| `GlobalCooldownStorageType` | categoryId | `TimePoint` | 按恢复分类存储GCD结束时间 |

设计要点:
- `_categoryCooldowns` 中的指针指向 `_spellCooldowns` 中的条目，二者共享同一份数据，确保分类冷却和法术冷却的一致性
- `ChargeEntryCollection` 使用 `std::deque`，支持高效的队首弹出(pop_front)和队尾追加(emplace_back)

### 2.4 成员变量总览

`src/server/game/Spells/SpellHistory.h:221-228`

```cpp
Unit* _owner;                                    // 所属单位
CooldownStorageType _spellCooldowns;             // 法术冷却存储
CooldownStorageType _spellCooldownsBeforeDuel;   // 决斗前的冷却快照
CategoryCooldownStorageType _categoryCooldowns;  // 分类冷却索引
TimePoint _schoolLockouts[MAX_SPELL_SCHOOL];     // 法术系封锁时间点数组
ChargeStorageType _categoryCharges;              // 充能恢复队列存储
GlobalCooldownStorageType _globalCooldowns;      // GCD 存储
Optional<TimePoint> _pauseTime;                  // 冷却暂停时间点(有值表示已暂停)
```

### 2.5 SpellCooldownFlags — 冷却标志

`src/server/game/Spells/SpellHistory.h:49-56`

```cpp
enum SpellCooldownFlags
{
    SPELL_COOLDOWN_FLAG_NONE                    = 0x0,  // 无标志
    SPELL_COOLDOWN_FLAG_INCLUDE_GCD             = 0x1,  // 包含GCD
    SPELL_COOLDOWN_FLAG_INCLUDE_EVENT_COOLDOWNS = 0x2,  // 包含事件触发的冷却
    SPELL_COOLDOWN_FLAG_LOSS_OF_CONTROL_UI      = 0x4,  // 显示为失控UI中的打断冷却
    SPELL_COOLDOWN_FLAG_ON_HOLD                 = 0x8   // 强制冷却按事件触发方式处理
};
```

这些标志用于客户端发包 `SMSG_SPELL_COOLDOWN`，控制客户端UI的冷却显示行为。

---

## 3. 冷却管理流程 (Cooldown Management Flow)

### 3.1 HandleCooldowns — 冷却处理入口

`src/server/game/Spells/SpellHistory.cpp:245-277`

`HandleCooldowns()` 是 Spell 施放成功后调用的主入口方法，负责决定是否启动冷却以及如何处理特殊法术:

```
HandleCooldowns(spellInfo, itemId, spell)
        |
        v
+-- Spell 正在忽略冷却? --(是)--> return
|
+-- 消费充能 ConsumeCharge()
|
+-- 有忽略冷却光环? --(是)--> return
|
+-- 玩家 && 药水/事件触发冷却? --(是)--> 记录药水ID, return (OnHold 由战斗退出事件触发)
|
+-- 法术是事件触发冷却或被动法术? --(是)--> return
|
+-- StartCooldown(spellInfo, itemId, spell)
```

核心逻辑说明:
1. **忽略冷却检查**: 如果 Spell 对象标记为忽略冷却，直接返回
2. **充能消费**: 无论如何先消费充能(如果法术有充能分类)
3. **忽略冷却光环**: `SPELL_AURA_IGNORE_SPELL_COOLDOWN` 光环可以完全跳过冷却
4. **药水处理**: 药水和 `IsCooldownStartedOnEvent()` 的法术不立即启动冷却，而是进入 OnHold 状态
5. **被动法术**: 被动法术不需要冷却
6. **启动冷却**: 调用 `StartCooldown()` 开始正常的冷却流程

### 3.2 OnHold 事件触发冷却机制

OnHold 是一种特殊的冷却状态，用于药水和"事件触发冷却"类法术:

- **药水**: 在战斗中饮用后，冷却不会立即开始倒计时，而是等到离开战斗后才开始
- **事件触发冷却**: 某些法术标记了 `IsCooldownStartedOnEvent()`，冷却在特定事件发生时才真正启动

OnHold 的实现方式:
- 在 `StartCooldown()` 中，当 `onHold = true` 时，冷却结束时间被设为 `curTime + InfinityCooldownDelay`(一个月)，实质上是"无限期"
- 在 `SaveToDB()` 中，OnHold 状态的冷却不会被持久化到数据库
- 在 `RestoreCooldownStateAfterDuel()` 中，OnHold 冷却不会被决斗恢复覆盖

### 3.3 StartCooldown — 冷却启动与修正器链

`src/server/game/Spells/SpellHistory.cpp:397-548`

`StartCooldown()` 是实际计算并注册冷却的核心方法，包含一个完整的冷却时间修正器(modifier)链:

```
GetCooldownDurations()       -- 获取基础冷却时间(D优先查ItemEffect.db2, 其次查SpellInfo)
        |
        v
[OnHold?] --(是)--> 设为 InfinityCooldownDelay
        |
        v
ApplySpellMod(Cooldown)     -- 法术修正器(SPELL_MOD_COOLDOWN)
ApplySpellMod(CategoryCD)   -- 分类冷却修正器
        |
        v
SPELL_AURA_MOD_SPELL_COOLDOWN_BY_HASTE    -- 急速影响冷却
        |
        v
SPELL_AURA_MOD_COOLDOWN_BY_HASTE_REGEN    -- 急速恢复影响冷却
        |
        v
SPELL_AURA_MOD_RECOVERY_RATE               -- 冷却恢复速率修正
SPELL_AURA_MOD_RECOVERY_RATE_BY_SPELL_LABEL -- 按标签的恢复速率修正
        |
        v
SPELL_AURA_MOD_COOLDOWN                    -- 固定冷却时间增减
        |
        v
SPELL_AURA_MOD_SPELL_CATEGORY_COOLDOWN     -- 分类冷却增减
        |
        v
[以天为单位的分类冷却?] --(是)--> 使用每日任务重置时间
        |
        v
[修正后冷却为0?] --(是)--> return (无冷却)
        |
        v
AddCooldown()               -- 注册到 _spellCooldowns 和 _categoryCooldowns
[需要发包?] --(是)--> 发送 SMSG_SPELL_COOLDOWN
```

### 3.4 全局冷却 (GCD)

`src/server/game/Spells/SpellHistory.h:196-199`

GCD 是按 `StartRecoveryCategory` 分类的全局冷却:

```cpp
bool HasGlobalCooldown(SpellInfo const* spellInfo) const;   // 检查GCD
void AddGlobalCooldown(SpellInfo const* spellInfo, Duration duration);  // 添加GCD
void CancelGlobalCooldown(SpellInfo const* spellInfo);      // 取消GCD
Duration GetRemainingGlobalCooldown(SpellInfo const* spellInfo) const;  // 剩余GCD时间
```

GCD 存储在 `_globalCooldowns` 中，以 `spellInfo->StartRecoveryCategory` 为键。注意 GCD 和法术冷却是独立管理的:
- GCD 检查: `HasGlobalCooldown()` 使用 `Clock::now()` (系统时钟)
- 法术冷却检查: `HasCooldown()` / `Update()` 使用 `GameTime::GetTime<Clock>()` (游戏时间)

### 3.5 法术系封锁 (School Lockout)

`src/server/game/Spells/SpellHistory.cpp:781-849`

法术系封锁发生在施法被打断时(如法师的 counterspell)，会锁定指定法术系的所有法术:

```cpp
void LockSpellSchool(SpellSchoolMask schoolMask, Duration lockoutTime);
bool IsSchoolLocked(SpellSchoolMask schoolMask) const;
```

存储方式: `_schoolLockouts[MAX_SPELL_SCHOOL]` 是一个固定大小的数组，每个元素对应一个法术系的封锁结束时间。

封锁逻辑:
1. 遍历所有被封锁的法术系位，设置封锁结束时间
2. 收集该单位已知的所有法术(Player/Pet/Creature)
3. 对每个已知法术检查: 是否受沉默预防类型影响、是否忽略封锁、是否属于被封锁法术系
4. 如果封锁时间大于已有冷却的剩余时间，则注册新冷却
5. 发送带有 `SPELL_COOLDOWN_FLAG_LOSS_OF_CONTROL_UI` 标志的冷却包

`IsReady()` 中的法术系封锁检查:
```
IsReady(spellInfo, itemId)
    |
    +-- 法术不忽略法术系封锁 && PreventionType 包含 SILENCE?
    |       |
    |       +-- IsSchoolLocked()? --(是)--> return false
    |
    +-- HasCooldown()? --(是)--> return false
    |
    +-- HasCharge()? --(否)--> return false
    |
    +-- return true
```

### 3.6 冷却暂停与恢复 (Pause/Resume)

`src/server/game/Spells/SpellHistory.cpp:1068-1090`

冷却暂停机制允许在特定场景下冻结所有冷却计时器:

```cpp
void PauseCooldowns();    // 暂停: 记录当前时间到 _pauseTime
void ResumeCooldowns();   // 恢复: 计算暂停时长并延长所有冷却
```

**PauseCooldowns()**: 仅记录暂停开始时间:
```cpp
_pauseTime = time_point_cast<Duration>(GameTime::GetTime<Clock>());
```

**ResumeCooldowns()**: 计算暂停持续时间，并将所有法术冷却和充能恢复时间向后推移:
```
ResumeCooldowns()
    |
    +-- _pauseTime 有值? --(否)--> return
    |
    +-- 计算 pausedDuration = now - _pauseTime
    |
    +-- 遍历 _spellCooldowns: 每个 CooldownEnd += pausedDuration
    |
    +-- 遍历 _categoryCharges: 每个 ChargeEntry.RechargeEnd += pausedDuration
    |
    +-- 重置 _pauseTime
    |
    +-- Update()  // 清理已过期的冷却
```

注意: 恢复时只延长 `CooldownEnd` 和 `RechargeEnd`，不延长 `CategoryEnd`。`CategoryEnd` 的偏移通过 `_categoryCooldowns` 指针自动获得，因为 `_categoryCooldowns` 指向 `_spellCooldowns` 中的 `CooldownEntry`。但仔细审查代码发现，遍历 `_spellCooldowns` 只修改了 `CooldownEnd` 而非 `CategoryEnd`，这是实际实现的行为。

### 3.7 冷却流转完整 ASCII 流程图

```
                         法术施放成功 (Spell cast success)
                                    |
                                    v
                      HandleCooldowns(spellInfo, item, spell)
                                    |
                    +---------------+----------------+
                    |                                |
            spell->IsIgnoringCooldowns()    ConsumeCharge(chargeCategoryId)
                    |                                |
               (是)--> return                     有充能? --> 追加到恢复队列
                    |
               (否)--> HasAura(IGNORE_COOLDOWN)?
                    |                     |
               (是)--> return        (否)--> 玩家 && (药水 || 事件触发)?
                                              |              |
                                         (是)--> OnHold    (否)--> 被动法术?
                                          return                      |
                                                              (是)--> return
                                                              (否)-->
                                                                    |
                                                      StartCooldown(spellInfo, itemId)
                                                                    |
                                                    +---------------+---------------+
                                                    |                               |
                                              GetCooldownDurations()          onHold == true?
                                                    |                       (是)--> InfinityCooldownDelay
                                          ItemEffect.db2 优先               |
                                          回退到 SpellInfo             修正器链:
                                                    |                 - SpellMod
                                                    +-------->         - 急速光环
                                                              - 恢复速率光环
                                                              - 固定冷却增减
                                                              - 分类冷却增减
                                                                    |
                                                    [修正后冷却 > 0?]
                                                        |           |
                                                    (是)           (否)--> return
                                                        |
                                                AddCooldown() --> 注册到存储
                                                        |
                                                [需要发包?] --> SMSG_SPELL_COOLDOWN
```

### 3.8 Update — 定时清理

`src/server/game/Spells/SpellHistory.cpp:221-243`

`Update()` 在每个游戏时间 tick 中被调用，负责清理已过期的条目:

```
Update()
    |
    +-- 清理 _categoryCooldowns: 删除 CategoryEnd < now 的条目
    |
    +-- 清理 _spellCooldowns: 删除 CooldownEnd < now 的条目
    |       (通过 EraseCooldown 同时移除 _categoryCooldowns 中的指针)
    |
    +-- 清理 _categoryCharges: pop_front 已恢复的充能(RechargeEnd <= now)
```

---

## 4. 充能系统 (Charge System)

### 4.1 概述

充能系统允许某些法术拥有多次使用次数，每次使用后逐个恢复。典型例子如牧师的"真言术:盾"(Power Word: Shield)在特定天赋下拥有多次充能。

### 4.2 充能存储结构

充能按 `chargeCategoryId` 组织，每个分类下维护一个 `std::deque<ChargeEntry>`:

```
_categoryCharges[categoryId] = deque<ChargeEntry>
                                |
                                +-- ChargeEntry { RechargeStart, RechargeEnd }
                                +-- ChargeEntry { RechargeStart, RechargeEnd }
                                +-- ...
```

deque 的特性确保了:
- 队首(first)是即将恢复的充能
- 队尾(back)是最近消费的充能
- `Update()` 时从队首弹出已恢复的条目

### 4.3 充能恢复时间计算

`src/server/game/Spells/SpellHistory.cpp:999-1035`

`GetChargeRecoveryTime()` 包含完整的修正器链，计算实际恢复时间:

```
基础恢复时间 = SpellCategoryEntry.ChargeRecoveryTime
        |
        v
+ SPELL_AURA_CHARGE_RECOVERY_MOD         -- 固定时间增减
+ SPELL_AURA_CHARGE_RECOVERY_BY_TYPE_MASK -- 按类型掩码增减
        |
        v
* SPELL_AURA_CHARGE_RECOVERY_MULTIPLIER  -- 乘数修正
        |
        v
[有 SPELL_AURA_CHARGE_RECOVERY_AFFECTED_BY_HASTE?]
        |                               -- 急速影响
        v
[有 SPELL_AURA_CHARGE_RECOVERY_AFFECTED_BY_HASTE_REGEN?]
        |                               -- 急速恢复影响
        v
* SPELL_AURA_MOD_CHARGE_RECOVERY_RATE            -- 按分类ID恢复速率
* SPELL_AURA_MOD_CHARGE_RECOVERY_RATE_BY_TYPE_MASK -- 按类型掩码恢复速率
        |
        v
[基础恢复时间 <= 1h && 非忽略时间速率标志?]
        |                               -- ModTimeRate 全局时间速率
        v
floor(最终值)
```

### 4.4 ConsumeCharge — 消费充能

`src/server/game/Spells/SpellHistory.cpp:851-871`

```
ConsumeCharge(chargeCategoryId)
        |
        +-- 分类不存在? --> return
        |
        +-- 恢复时间 <= 0 或 最大充能 <= 0? --> return
        |
        +-- 有忽略充能冷却光环? --> return
        |
        +-- charges 队列为空?
        |       |
        |   (是)--> recoveryStart = now
        |   (否)--> recoveryStart = charges.back().RechargeEnd  // 对齐到队尾
        |       |
        v
charges.emplace_back(recoveryStart, chargeRecovery)
```

关键设计: 新充能的恢复开始时间 = 队尾条目的恢复结束时间。这确保了多个消费的充能按顺序依次恢复，而非并行恢复。

### 4.5 充能队列恢复 ASCII 图解

```
时间轴 ---->

初始状态 (最大充能 = 3):
  [可用] [可用] [可用]
  queue: (空)

第1次施法 (t=0):
  [恢复中..] [可用] [可用]
  queue: [ Entry{start:0, end:5s} ]

第2次施法 (t=1s):
  [恢复中..] [恢复中..] [可用]
  queue: [ Entry{start:0, end:5s}, Entry{start:5s, end:10s} ]
                     ^--- 对齐到前一个的end

第3次施法 (t=2s):
  [恢复中..] [恢复中..] [恢复中..]
  queue: [ Entry{0, 5s}, Entry{5s, 10s}, Entry{10s, 15s} ]
                                          ^--- 对齐到前一个的end

t=5s (Update 触发, front.RechargeEnd <= now):
  [可用] [恢复中..] [恢复中..]
  queue: [ Entry{5s, 10s}, Entry{10s, 15s} ]   // pop_front

t=10s (Update 触发):
  [可用] [可用] [恢复中..]
  queue: [ Entry{10s, 15s} ]

t=15s (Update 触发):
  [可用] [可用] [可用]
  queue: (空)
```

### 4.6 充能操作 API

| 方法 | 位置 | 功能 |
|------|------|------|
| `ConsumeCharge(categoryId)` | SpellHistory.h:185 | 消费一个充能，追加到恢复队列 |
| `RestoreCharge(categoryId)` | SpellHistory.h:188 | 恢复一个充能，从队列尾部弹出 |
| `ModifyChargeRecoveryTime(categoryId, mod)` | SpellHistory.cpp:873-895 | 修改所有待恢复充能的时间 |
| `UpdateChargeRecoveryRate(categoryId, mod, apply)` | SpellHistory.cpp:897-932 | 按比率修改恢复速度 |
| `ResetCharges(categoryId)` | SpellHistory.cpp:945-960 | 重置指定分类的所有充能 |
| `ResetAllCharges()` | SpellHistory.cpp:962-972 | 重置所有充能 |
| `HasCharge(categoryId)` | SpellHistory.cpp:974-986 | 检查是否还有可用充能 |
| `GetMaxCharges(categoryId)` | SpellHistory.cpp:988-997 | 获取最大充能数(含光环修正) |
| `GetChargeRecoveryTime(categoryId)` | SpellHistory.cpp:999-1035 | 获取充能恢复时间(含全部修正器) |

**HasCharge 的判断逻辑** (`SpellHistory.cpp:974-986`):
```cpp
bool HasCharge(uint32 chargeCategoryId) const
{
    if (!sSpellCategoryStore.LookupEntry(chargeCategoryId))
        return true;  // 分类不存在视为有充能

    int32 maxCharges = GetMaxCharges(chargeCategoryId);
    if (maxCharges <= 0)
        return true;  // 当前未启用充能机制(如未点天赋)

    auto itr = _categoryCharges.find(chargeCategoryId);
    return itr == _categoryCharges.end() || int32(itr->second.size()) < maxCharges;
    // 队列为空 或 已消费数 < 最大数 --> 有充能可用
}
```

### 4.7 ModifyChargeRecoveryTime

`src/server/game/Spells/SpellHistory.cpp:873-895`

修改充能恢复时间时，每个 `ChargeEntry` 的 `RechargeStart` 和 `RechargeEnd` 都会加上相同的偏移量，保持恢复时长不变但时间窗口整体平移:

```
修改前:  [Entry{start:0, end:5s}, Entry{start:5s, end:10s}]
修改 +3s: [Entry{start:3, end:8s}, Entry{start:8s, end:13s}]
```

修改后会重新检查是否有已过期条目需要弹出，并向客户端发送 `UpdateChargeCategoryCooldown` 包。

### 4.8 UpdateChargeRecoveryRate

`src/server/game/Spells/SpellHistory.cpp:897-932`

恢复速率修改使用乘数方式，第一个充能基于当前时间点缩放，后续充能基于恢复时长缩放并重新链接:

```
修改前 (modChange=0.5, 即冷却减半):
  now=2s
  [Entry{start:0, end:5s}, Entry{start:5s, end:10s}]

第一个: newEnd = now + (5s - now) * 0.5 = 2s + 3s*0.5 = 3.5s
后续: newDuration = (end - start) * 0.5 = 5s * 0.5 = 2.5s
  [Entry{start:?, end:3.5s}, Entry{start:3.5s, end:6.0s}]
```

---

## 5. 持久化与特殊场景 (Persistence & Special Cases)

### 5.1 数据库持久化 (Load/Save)

#### 模板化设计

`src/server/game/Spells/SpellHistory.h:100-104`

SpellHistory 的数据库操作采用模板特化设计，支持 `Player` 和 `Pet` 两种所有者类型:

```cpp
template<class OwnerType>
void LoadFromDB(PreparedQueryResult cooldownsResult, PreparedQueryResult chargesResult);

template<class OwnerType>
void SaveToDB(CharacterDatabaseTransaction trans);
```

#### PersistenceHelper 特化

`src/server/game/Spells/SpellHistory.cpp:38-139`

两个特化版本 (`PersistenceHelper<Player>` 和 `PersistenceHelper<Pet>`) 提供了不同的 SQL 语句和字段映射:

| 特化 | 冷却删除 | 冷却插入 | 充能删除 | 充能插入 |
|------|----------|----------|----------|----------|
| Player | `CHAR_DEL_CHAR_SPELL_COOLDOWNS` | `CHAR_INS_CHAR_SPELL_COOLDOWN` | `CHAR_DEL_CHAR_SPELL_CHARGES` | `CHAR_INS_CHAR_SPELL_CHARGES` |
| Pet | `CHAR_DEL_PET_SPELL_COOLDOWNS` | `CHAR_INS_PET_SPELL_COOLDOWN` | `CHAR_DEL_PET_SPELL_CHARGES` | `CHAR_INS_PET_SPELL_CHARGES` |

Player 和 Pet 的字段差异:
- Player 冷却包含 5 个字段: spellId, itemId, cooldownEnd, categoryId, categoryEnd
- Pet 冷却包含 4 个字段: spellId, cooldownEnd, categoryId, categoryEnd (无 itemId)
- 充能字段两者一致: categoryId, rechargeStart, rechargeEnd

#### LoadFromDB 流程

`src/server/game/Spells/SpellHistory.cpp:147-180`

```
LoadFromDB<Player/Pet>(cooldownsResult, chargesResult)
    |
    +-- 遍历冷却结果集:
    |       ReadCooldown() 解析字段
    |       验证 SpellInfo 存在性
    |       存入 _spellCooldowns[spellId]
    |       如果有 CategoryId，建立 _categoryCooldowns[categoryId] 指针
    |
    +-- 遍历充能结果集:
            ReadCharge() 解析字段
            验证 SpellCategory 存在性
            追加到 _categoryCharges[categoryId]
```

#### SaveToDB 流程

`src/server/game/Spells/SpellHistory.cpp:182-219`

```
SaveToDB<Player/Pet>(trans)
    |
    +-- 删除旧的冷却记录
    |
    +-- 遍历 _spellCooldowns:
    |       跳过 OnHold 状态的冷却(!)
    |       插入新的冷却记录
    |
    +-- 删除旧的充能记录
    |
    +-- 遍历 _categoryCharges:
            插入所有充能记录
```

关键: **OnHold 状态的冷却不会被持久化到数据库**。这意味着如果玩家在 OnHold 状态下登出，对应的冷却会丢失。这是有意为之的设计，因为 OnHold 冷却通常由运行时事件(如战斗结束)触发。

### 5.2 决斗冷却保存与恢复

`src/server/game/Spells/SpellHistory.cpp:1169-1218`

决斗系统需要保存和恢复冷却状态，确保决斗不影响玩家正常的冷却计时:

#### SaveCooldownStateBeforeDuel

`src/server/game/Spells/SpellHistory.h:207` / `SpellHistory.cpp:1169-1172`

```cpp
void SaveCooldownStateBeforeDuel()
{
    _spellCooldownsBeforeDuel = _spellCooldowns;  // 深拷贝当前所有冷却
}
```

#### RestoreCooldownStateAfterDuel

`src/server/game/Spells/SpellHistory.h:208` / `SpellHistory.cpp:1174-1218`

恢复逻辑:
```
RestoreCooldownStateAfterDuel()
    |
    +-- 遍历当前冷却: 找出决斗中新增的"长冷却"(>10分钟)
    |       --> 合并到 _spellCooldownsBeforeDuel
    |       (保留决斗中触发的专业冷却等)
    |
    +-- 遍历决斗前冷却快照:
    |       跳过 OnHold 状态的条目
    |       写入回 _spellCooldowns (不覆盖已有的 OnHold 冷却)
    |
    +-- 发送 SMSG_SPELL_COOLDOWN 给客户端刷新UI
            (只发送 0~10分钟范围的冷却，避免视觉bug)
```

设计要点:
- 决斗中新增的超过10分钟的冷却(如专业制作冷却)会被保留
- OnHold 状态的冷却在恢复时不会被覆盖，保持其等待触发的状态
- 客户端发包限制在10分钟范围内，避免UI显示异常

### 5.3 GetCooldownDurations — 冷却时长获取

`src/server/game/Spells/SpellHistory.h:205` / `SpellHistory.cpp:1128-1167`

这是一个静态方法，用于获取法术的原始冷却时长:

```
GetCooldownDurations(spellInfo, itemId, cooldown, categoryId, categoryCooldown)
    |
    +-- itemId 非零?
    |       (是)--> 查找 ItemTemplate 的 ItemEffect 数组
    |               找到匹配 spellId 的 ItemEffect
    |               使用 ItemEffect 的 CoolDownMSec / SpellCategoryID / CategoryCoolDownMSec
    |
    +-- 以上未找到有效冷却?
            (是)--> 使用 SpellInfo 的 RecoveryTime / Category / CategoryRecoveryTime
```

优先级: ItemEffect.db2(物品专属冷却) > SpellInfo(DBC基础冷却)

### 5.4 关键 API 方法汇总

| 方法 | 声明位置 | 功能 |
|------|----------|------|
| `HandleCooldowns()` | SpellHistory.h:108 | 冷却处理主入口，由 Spell 施放后调用 |
| `StartCooldown()` | SpellHistory.h:118 | 启动冷却(含完整修正器链) |
| `AddCooldown()` | SpellHistory.h:121-127 | 直接添加冷却条目到存储 |
| `SendCooldownEvent()` | SpellHistory.h:119 | 发送冷却事件包并可选启动冷却 |
| `ModifyCooldown()` | SpellHistory.h:128-129 | 修改法术冷却和充能恢复时间 |
| `ResetCooldown()` | SpellHistory.h:152 | 重置指定法术冷却 |
| `ResetAllCooldowns()` | SpellHistory.h:173 | 重置所有冷却 |
| `HasCooldown()` | SpellHistory.h:174-175 | 检查法术是否在冷却中 |
| `IsReady()` | SpellHistory.h:110 | 综合检查法术是否可用(封锁+冷却+充能) |
| `LockSpellSchool()` | SpellHistory.h:181 | 锁定法术系 |
| `IsSchoolLocked()` | SpellHistory.h:182 | 检查法术系是否被锁定 |
| `AddGlobalCooldown()` | SpellHistory.h:197 | 添加GCD |
| `HasGlobalCooldown()` | SpellHistory.h:196 | 检查GCD |
| `ConsumeCharge()` | SpellHistory.h:185 | 消费充能 |
| `HasCharge()` | SpellHistory.h:191 | 检查充能可用性 |
| `PauseCooldowns()` | SpellHistory.h:202 | 暂停所有冷却 |
| `ResumeCooldowns()` | SpellHistory.h:203 | 恢复所有冷却 |
| `Update()` | SpellHistory.h:106 | 定时清理过期条目 |
| `LoadFromDB<OwnerType>()` | SpellHistory.h:101 | 从数据库加载冷却和充能 |
| `SaveToDB<OwnerType>()` | SpellHistory.h:104 | 保存冷却和充能到数据库 |
| `SaveCooldownStateBeforeDuel()` | SpellHistory.h:207 | 决斗前保存冷却快照 |
| `RestoreCooldownStateAfterDuel()` | SpellHistory.h:208 | 决斗后恢复冷却状态 |
| `GetCooldownDurations()` | SpellHistory.h:205 | 获取法术基础冷却时长 |

### 5.5 IsReady — 法术可用性综合判定

`src/server/game/Spells/SpellHistory.cpp:279-292`

`IsReady()` 是判断法术是否可以施放的核心方法，综合了三个子系统的检查:

```
IsReady(spellInfo, itemId)
    |
    +-- [1] 法术系封锁检查
    |   不忽略封锁 && PreventionType 包含 SILENCE
    |   && IsSchoolLocked(schoolMask) --> return false
    |
    +-- [2] 冷却检查
    |   HasCooldown(spellInfo, itemId) --> return false
    |
    +-- [3] 充能检查
    |   !HasCharge(chargeCategoryId) --> return false
    |
    +-- return true
```

这三个检查的顺序反映了优先级: 法术系封锁 > 法术冷却 > 充能可用性。

### 5.6 与 01-spell-state-machine.md 的交叉引用

SpellHistory 与法术状态机紧密关联:

- **Spell 施放完成后** 调用 `HandleCooldowns()` 进入冷却管理流程
- **IsReady()** 在 Spell 状态机的 "检查是否可以施放" 阶段被调用
- **OnHold 冷却** 对应状态机中"等待事件"的异步行为
- **GCD** 在 Spell 开始施放前由状态机检查
- **法术系封锁** 由打断效果(aura effect)触发，影响后续的施放判定
- **Pause/Resume** 可能由状态机中的某些状态转换触发(如载具进入/离开)

详细的状态转换逻辑请参考 [01-spell-state-machine.md](./01-spell-state-machine.md)。
