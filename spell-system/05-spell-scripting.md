# 05 - 法术脚本扩展系统 (Spell Scripting Extension System)

## 1. 概述 (Overview)

TrinityCore 的法术脚本扩展系统提供了一套类型安全、编译期校验的回调框架，允许在不修改核心引擎代码的前提下，通过派生脚本类来定制法术施放、命中、光环等各个阶段的行为。

该系统的核心设计目标:

- **零开销抽象**: 通过类型擦除 (type erasure) 将用户脚本函数指针统一存储为定长字节缓冲区，避免虚函数表膨胀
- **编译期签名校验**: 每个 Handler 类模板的构造函数中使用 `static_assert` 验证回调函数签名，签名不匹配时产生清晰的编译错误信息
- **多脚本聚合**: 同一法术可注册多个脚本，所有脚本的 Hook 按注册顺序依次执行，通过 `HookList` 容器实现
- **双轨体系**: `SpellScript` 处理瞬时法术事件（施放、命中），`AuraScript` 处理持续光环事件（周期 tick、proc、吸收）

两个主要脚本基类:

| 基类 | 文件位置 | 用途 |
|------|---------|------|
| `SpellScript` | `src/server/game/Spells/SpellScript.h:293-964` | 法术施放流程的各阶段 Hook |
| `AuraScript` | `src/server/game/Spells/SpellScript.h:1006-1908` | 光环效果生命周期的各阶段 Hook |

两者共同继承自 `SpellScriptBase` (`src/server/game/Spells/SpellScript.h:71-260`)，该基类提供:
- 脚本生命周期管理 (`_Register`, `_Unload`, `_Init`)
- 虚接口契约 (`Register()`, `Validate()`, `Load()`, `Unload()`)
- `ScriptFuncInvoker` 类型擦除模板
- `EffectHook` 效果过滤基类
- `HookList<T>` 编译屏障包装（延迟 `operator+=` 实例化）

---

## 2. 核心数据结构 (Core Data Structures)

### 2.1 ScriptFuncInvoker -- 类型擦除存储

`ScriptFuncInvoker` 是整个脚本系统的基石，定义于 `SpellScriptBase` 中 (`SpellScript.h:146-175`)。它将任意类型的函数指针（成员函数指针、静态函数指针）存储在固定大小的字节数组中，并通过统一的 Thunk 函数指针进行调用。

```
template <typename Ret, typename BaseClass, typename... Args>
struct ScriptFuncInvoker
{
    union SizeAndAlignment                           // :149-153
    {
        Ret(BaseClass::* Member)(Args...);           // 成员函数指针
        Ret(* Static)(BaseClass&, Args...);          // 静态函数指针
    };

    static constexpr std::size_t Size      = sizeof(SizeAndAlignment);       // :155
    static constexpr std::size_t Alignment = alignof(SizeAndAlignment);      // :156

    struct alignas(Alignment) StorageType  // :158
        : std::array<std::byte, Size>
    { };

    template <typename ScriptFunc>
    struct Impl                                  // :160-171
    {
        using ScriptClass = GetScriptClass_t<ScriptFunc>;

        static Ret Invoke(BaseClass& script, Args... args, StorageType callImpl)  // :165
        {
            // 从存储区读回原始函数指针， reinterpret_cast 后通过 std::invoke 调用
            return std::invoke(
                reinterpret_cast<Impl const*>(callImpl.data())->Func,
                static_cast<ScriptClass&>(script),
                args...
            );
        }

        ScriptFunc Func;  // :170 -- 实际存储的函数指针
    };

    StorageType ImplStorage;                                     // :173
    Ret(* Thunk)(BaseClass&, Args..., StorageType);             // :174
};
```

**设计要点:**

1. **StorageType** (`:158`): 继承自 `std::array<std::byte, Size>` 并使用 `alignas(Alignment)` 保证对齐。Size 和 Alignment 取 `SizeAndAlignment` union 的 sizeof/alignof，确保能容纳最大的函数指针类型。

2. **Impl<ScriptFunc>** (`:160-171`): 内嵌结构体，`Func` 成员持有实际函数指针。其 `Invoke` 静态方法作为 Thunk 的目标：从 `StorageType` 的字节缓冲区中 reinterpret_cast 回 `Impl*`，取出 `Func`，通过 `std::invoke` 调用。`static_cast<ScriptClass&>(script)` 实现从基类引用到具体脚本类引用的安全向下转型。

3. **Thunk** (`:174`): 函数指针，指向 `Impl<ScriptFunc>::Invoke`，作为统一调用入口。所有不同签名的回调都通过同一 Thunk 签名调用。

4. **static_assert 校验**: 每个 Handler 构造函数中都包含:
   ```cpp
   static_assert(ScriptFuncInvoker::Size >= sizeof(ScriptFunc));
   static_assert(ScriptFuncInvoker::Alignment >= alignof(ScriptFunc));
   ```
   确保存储区能容纳实际函数指针类型。

**类型推导辅助**: `GetScriptClass` 特化 (`:119-144`) 从函数指针类型中提取脚本类类型:
- `Ret(Class::*)(Args...)` -> `Class` (非 const 成员函数)
- `Ret(Class::*)(Args...) const` -> `Class` (const 成员函数)
- `Ret(*)(Class&, Args...)` -> `std::remove_const_t<Class>` (静态函数)

### 2.2 EffectHook -- 效果过滤基类

`EffectHook` (`SpellScript.h:100-117`) 为需要按效果索引/效果名称过滤的 Hook 提供公共基础设施:

```
class TC_GAME_API EffectHook
{
public:
    explicit EffectHook(uint8 effIndex);           // 构造，指定效果索引
    uint32 GetAffectedEffectsMask(SpellInfo const* spellInfo) const;  // :110
    bool IsEffectAffected(SpellInfo const* spellInfo, uint8 effIndex) const;  // :111
    virtual bool CheckEffect(SpellInfo const* spellInfo, uint8 effIndex) const = 0;  // :112
protected:
    uint8 _effIndex;                              // :116
};
```

两个派生类分别用于法术和光环:
- `SpellScript::EffectBase` (`:361-374`): 增加 `_effName` (uint16) 成员，`CheckEffect` 同时校验效果索引和效果名称
- `AuraScript::EffectBase` (`:1110-1123`): 增加 `_auraType` (uint16) 成员，`CheckEffect` 校验效果索引和光环类型

### 2.3 HookList -- 多脚本 Hook 聚合容器

全局 `HookList<T>` (`src/common/Utilities/Util.h:487-526`) 是一个轻量级的 `std::vector<T>` 包装器:

```
template <typename T>
class HookList
{
    typedef std::vector<T> ContainerType;
    ContainerType _container;
public:
    HookList<T>& operator+=(T&& t) { _container.emplace_back(std::move(t)); return *this; }
    size_t size() const;
    iterator begin() / end();
    const_iterator begin() / end();
};
```

`SpellScriptBase` 中定义了编译屏障版本 (`SpellScript.h:93-98`):

```cpp
template <typename T>
class HookList : public ::HookList<T>
{
public:
    HookList& operator+=(T&& t) noexcept;  // 声明于此，定义在 .cpp 中
};
```

**设计意图**: 将 `operator+=` 的实例化推迟到编译单元 (.cpp) 中进行，避免在每个包含 `SpellScript.h` 的脚本文件中都实例化所有 Handler 类型的模板代码，从而显著减少编译时间和二进制体积。

**多脚本聚合机制**: 当同一法术有多个脚本时，每个脚本实例的 `Register()` 方法独立向自己的 `HookList` 添加 Handler。Spell/Aura 在执行 Hook 时遍历 `m_loadedScripts` 中的所有脚本，依次调用每个脚本的对应 HookList 中的所有 Handler:

```
对于 SpellScript 中 OnHit Hook 的执行:
  for (SpellScript* script : m_loadedScripts)
      for (HitHandler& handler : script->OnHit)
          handler.Call(script);
```

### 2.4 Handler 类一览

每个 Handler 类都遵循统一的构造模式:

```
class XxxHandler final
{
public:
    using ScriptFuncInvoker = SpellScript::ScriptFuncInvoker<Ret, Args...>;
    template<typename ScriptFunc>
    explicit XxxHandler(ScriptFunc handler) { /* static_assert + placement new + Thunk */ }
    Ret Call(SpellScript* script, Args...) const { /* Thunk dispatch */ }
private:
    ScriptFuncInvoker _invoker;
};
```

**SpellScript Handler 类** (`SpellScript.h:293-710`):

| Handler 类 | 行号 | 函数签名 | 用途 |
|-----------|------|---------|------|
| `CastHandler` | 301-327 | `void HandleCast()` | 施放/施放前后回调 |
| `CheckCastHandler` | 329-359 | `SpellCastResult CheckCast()` | 施放条件校验 |
| `EffectHandler` | 376-403 | `void HandleEffect(SpellEffIndex)` | 效果 Launch/Hit 回调 |
| `BeforeHitHandler` | 405-431 | `void HandleBeforeHit(SpellMissInfo)` | 命中前回调 |
| `HitHandler` | 433-459 | `void HandleHit()` | 命中/命中后回调 |
| `OnCalcCritChanceHandler` | 461-491 | `void CalcCritChance(Unit const*, float&)` | 暴击率计算 |
| `TargetHook` | 493-509 | (基类) | 目标选择 Hook 基类 |
| `ObjectAreaTargetSelectHandler` | 511-547 | `void SetTargets(std::list<WorldObject*>&)` | 区域目标选择 |
| `ObjectTargetSelectHandler` | 549-585 | `void SetTarget(WorldObject*&)` | 单目标选择 |
| `DestinationTargetSelectHandler` | 587-618 | `void SetTarget(SpellDestination&)` | 目的地选择 |
| `DamageAndHealingCalcHandler` | 620-650 | `void CalcDamage(SpellEffectInfo const&, Unit*, int32&, int32&, float&)` | 伤害/治疗计算 |
| `OnCalculateResistAbsorbHandler` | 652-682 | `void CalcAbsorbResist(DamageInfo const&, uint32&, int32&)` | 抵抗/吸收计算 |
| `EmpowerStageCompletedHandler` | 684-710 | `void HandleEmpowerStageCompleted(int32)` | 蓄力阶段完成 |

**AuraScript Handler 类** (`SpellScript.h:1006-1908`):

| Handler 类 | 行号 | 函数签名 | 用途 |
|-----------|------|---------|------|
| `CheckAreaTargetHandler` | 1014-1044 | `bool CheckTarget(Unit*)` | 区域光环目标检查 |
| `AuraDispelHandler` | 1046-1076 | `void HandleDispel(DispelInfo*)` | 驱散回调 |
| `AuraHeartbeatHandler` | 1078-1108 | `void HandleHeartbeat()` | 心跳回调 |
| `EffectPeriodicHandler` | 1125-1156 | `void HandlePeriodic(AuraEffect const*)` | 周期效果 tick |
| `EffectUpdatePeriodicHandler` | 1158-1189 | `void HandleUpdatePeriodic(AuraEffect*)` | 周期效果更新 |
| `EffectCalcAmountHandler` | 1191-1222 | `void CalcAmount(AuraEffect const*, int32&, bool&)` | 效果数值计算 |
| `EffectCalcPeriodicHandler` | 1224-1255 | `void CalcPeriodic(AuraEffect const*, bool&, int32&)` | 周期数据计算 |
| `EffectCalcSpellModHandler` | 1257-1288 | `void CalcSpellMod(AuraEffect const*, SpellModifier*&)` | 法术修正计算 |
| `EffectCalcCritChanceHandler` | 1290-1321 | `void CalcCritChance(AuraEffect const*, Unit const*, float&)` | DoT/Hot 暴击率 |
| `EffectCalcDamageAndHealingHandler` | 1323-1354 | `void CalcDamageAndHealing(AuraEffect const*, Unit*, int32&, int32&, float&)` | DoT/Hot 伤害/治疗 |
| `EffectApplyHandler` | 1356-1391 | `void HandleApplyOrRemove(AuraEffect const*, AuraEffectHandleModes)` | 效果应用/移除 |
| `EffectAbsorbHandler` | 1393-1424 | `void HandleAbsorb(AuraEffect*, DamageInfo&, uint32&)` | 伤害吸收 |
| `EffectAbsorbHealHandler` | 1426-1457 | `void HandleAbsorb(AuraEffect*, HealInfo&, uint32&)` | 治疗吸收 |
| `CheckProcHandler` | 1459-1489 | `bool CheckProc(ProcEventInfo&)` | Proc 触发检查 |
| `CheckEffectProcHandler` | 1491-1522 | `bool CheckProc(AuraEffect const*, ProcEventInfo&)` | 效果级 Proc 检查 |
| `AuraProcHandler` | 1524-1554 | `void HandleProc(ProcEventInfo&)` | Proc 触发回调 |
| `EffectProcHandler` | 1556-1587 | `void HandleProc(AuraEffect*, ProcEventInfo&)` | 效果级 Proc 回调 |
| `EnterLeaveCombatHandler` | 1589-1615 | `void HandleEnterLeaveCombat(bool)` | 进出战斗回调 |

---

## 3. 注册与加载流程 (Registration & Loading Flow)

### 3.1 宏注册机制

脚本通过 `RegisterSpellScript` 系列宏在脚本源文件中注册 (`ScriptMgr.h:1382-1383`):

```cpp
#define RegisterSpellScript(spell_script) \
    RegisterSpellAndAuraScriptPairWithArgs(spell_script, void, #spell_script)

#define RegisterSpellScriptWithArgs(spell_script, script_name, ...) \
    RegisterSpellAndAuraScriptPairWithArgs(spell_script, void, script_name, __VA_ARGS__)

#define RegisterSpellAndAuraScriptPairWithArgs(script_1, script_2, script_name, ...) \
    new GenericSpellAndAuraScriptLoader<...>(script_name, std::make_tuple(__VA_ARGS__))
```

`GenericSpellAndAuraScriptLoader` (`ScriptMgr.h:1350-1366`) 继承自 `SpellScriptLoader` (`ScriptMgr.h:201`)，后者继承自 `ScriptObject`。宏展开后在全局作用域创建一个 `GenericSpellAndAuraScriptLoader` 实例，其构造函数将自身注册到 `ScriptRegistry<SpellScriptLoader>`。

### 3.2 完整注册与加载流程

```
[编译期]                          [启动期]                           [运行期]
                                 (服务器启动)
                                     |
                                     v
+-------------------+      +--------------------+      +--------------------------+
| 脚本源文件 .cpp    |      | ScriptRegistry     |      | ObjectMgr::_spellScriptsStore |
|                   |      | <SpellScriptLoader>|      | (multimap<uint32,        |
| RegisterSpellScript|----->| .cpp 全局实例       |      |  pair<uint32, bool>>)    |
| (MySpellScript)   |      | 自动注册            |      +--------------------------+
+-------------------+      +--------------------+                ^
                                                                 |
                                    数据库加载                    |
                              +--------------------+              |
                              | spell_script       |              |
                              | 表中 spell_id +    |-------------+
                              | ScriptName 映射    |  建立 spell_id -> script_name
                              +--------------------+
                                                              法术施放时
                                                                     |
                                                                     v
                                                         +----------------------+
                                                         | Spell::LoadScripts() |  Spell.cpp:3467
                                                         |   或                 |
                                                         | Aura::LoadScripts()  |  SpellAuras.cpp:2065
                                                         +----------------------+
                                                                     |
                                                                     v
                                                         +---------------------------+
                                                         | sScriptMgr->              |
                                                         |  CreateSpellScripts(      |  ScriptMgr.cpp:1454
                                                         |    spellId, scripts, this) |
                                                         +---------------------------+
                                                                     |
                                                                     v
                                                         +---------------------------+
                                                         | sObjectMgr->              |
                                                         |  GetSpellScriptsBounds(   |  ObjectMgr.cpp:9029
                                                         |    spellId)               |
                                                         |  返回 multimap 迭代器对     |
                                                         +---------------------------+
                                                                     |
                                                                     v
                                                    +----------------------------------------+
                                                    | 遍历 bounds, 对每个条目:                |
                                                    |   1. 检查是否启用 (.second == true)     |
                                                    |   2. GetSpellScriptLoader(name)         |
                                                    |   3. loader->GetSpellScript() 创建实例   |
                                                    |   4. script->_Init(name, spellId)       |
                                                    |   5. script->_Load(spell/aura)          |
                                                    |   6. scriptVector.push_back(script)     |
                                                    +----------------------------------------+
                                                                     |
                                                                     v
                                                    +----------------------------------------+
                                                    | Spell/Aura 持有 m_loadedScripts 向量    |
                                                    | (Spell.h:959)                           |
                                                    | 对每个 script 调用 script->Register()   |
                                                    | 脚本在 Register() 中通过                |
                                                    | OnHit += SpellHitFn(&Class::HandleHit) |
                                                    | 等 HookList::operator+= 注册回调        |
                                                    +----------------------------------------+
```

### 3.3 关键函数说明

**`ScriptMgr::CreateSpellOrAuraScripts`** (`ScriptMgr.cpp:1425-1452`): 泛型模板函数，被 `CreateSpellScripts` 和 `CreateAuraScripts` 调用。流程:
1. 通过 `sObjectMgr->GetSpellScriptsBounds(spellId)` 获取该法术的所有脚本注册条目
2. 检查脚本是否启用（`itr->second.second`）
3. 通过 `ScriptRegistry<SpellScriptLoader>::GetScriptById()` 获取 Loader 实例
4. 调用 Loader 的 `GetSpellScript()` / `GetAuraScript()` 创建脚本实例
5. 调用 `_Init()` 和 `_Load()` 完成初始化
6. 加入 `scriptVector`

**`Spell::LoadScripts()`** (`Spell.cpp:8817-8825`): 调用 `sScriptMgr->CreateSpellScripts()` 获取脚本列表，然后对每个脚本调用 `Register()` 完成回调注册。

**`Aura::LoadScripts()`** (`SpellAuras.cpp:2065-2073`): 调用 `sScriptMgr->CreateAuraScripts()` 获取脚本列表，然后对每个脚本调用 `Register()`。

### 3.4 Register() 标准模式

每个脚本子类必须实现 `Register()` 纯虚函数，在其中通过 `HookList::operator+=` 注册所需 Hook:

```cpp
class spell_mage_fireball : public SpellScript
{
    PrepareSpellScript(spell_mage_fireball);

    bool Validate(SpellInfo const* spellInfo) override
    {
        return ValidateSpellInfo({ spellInfo->Id });  // 验证法术存在
    }

    void HandleHit()
    {
        Unit* target = GetHitUnit();
        if (target)
            SetHitDamage(CalcByValue(target));
    }

    void Register() override
    {
        OnHit += SpellHitFn(spell_mage_fireball::HandleHit);
        // 也可注册多个效果 Hook:
        OnEffectHitTarget += SpellEffectFn(spell_mage_fireball::HandleEffectHit,
                                           EFFECT_0, SPELL_EFFECT_SCHOOL_DAMAGE);
    }
};

// 效果级 Hook 注册示例:
void Register() override
{
    OnEffectLaunch += SpellEffectFn(Class::OnLaunch, EFFECT_0, SPELL_EFFECT_ANY);
    OnEffectHitTarget += SpellEffectFn(Class::OnHitTarget, EFFECT_ALL, SPELL_EFFECT_APPLY_AURA);

    // 目标选择 Hook:
    OnObjectAreaTargetSelect += SpellObjectAreaTargetSelectFn(
        Class::FilterTargets, EFFECT_0, TARGET_UNIT_DEST_AREA_ENEMY);

    // 伤害计算 Hook:
    CalcDamage += SpellCalcDamageFn(Class::CalcDamage);
}
```

宏辅助定义:
- `SpellCastFn(F)` -> `CastHandler(&F)`
- `SpellCheckCastFn(F)` -> `CheckCastHandler(&F)`
- `SpellEffectFn(F, I, N)` -> `EffectHandler(&F, I, N)` (I=效果索引, N=效果名称)
- `SpellHitFn(F)` -> `HitHandler(&F)`
- `SpellObjectAreaTargetSelectFn(F, I, N)` -> `ObjectAreaTargetSelectHandler(&F, I, N)`
- `SpellCalcDamageFn(F)` -> `DamageAndHealingCalcHandler(&F)`

---

## 4. Hook 体系 (Hook System)

### 4.1 SpellScriptState 与 SpellScriptHookType

脚本状态枚举 (`SpellScript.h:61-68`):

```
enum SpellScriptState
{
    SPELL_SCRIPT_STATE_NONE         = 0,
    SPELL_SCRIPT_STATE_REGISTRATION = 1,
    SPELL_SCRIPT_STATE_LOADING      = 2,
    SPELL_SCRIPT_STATE_UNLOADING    = 3
};
```

`SpellScriptHookType` (`SpellScript.h:263-288`) 从 `SPELL_SCRIPT_STATE_END` 开始枚举，共定义 18 个 Hook 类型（加上 `OnPrecast` 和 `CalcCastTime` 虚函数），与 `SpellScriptState` 共享同一数值空间用于运行时阶段检查。

`AuraScriptHookType` (`SpellScript.h:967-1000`) 同样从 `SPELL_SCRIPT_STATE_END` 开始，定义 28 个 Hook 类型。

### 4.2 SpellScript 18 个 Hook 执行顺序

`SpellScript.h:825-846` 中的注释完整描述了执行顺序:

```
法术施放时间线
=====================================================================>

[阶段1: 准备]
  1. OnPrecast               -- 法术准备期间（施法条开始前）
                              虚函数 override, 非 HookList

[阶段2: 施放前]
  2. BeforeCast              -- 施法条满时，法术处理前
  3. OnCheckCast             -- 覆盖 CheckCast 结果（返回 SpellCastResult）

[阶段3: 目标选择]
  4a. OnObjectAreaTargetSelect  -- 区域目标筛选（可修改 target list）
  4b. OnObjectTargetSelect      -- 单目标筛选（可替换 target）
  4c. OnDestinationTargetSelect -- 目的地筛选（可修改 destination）

[阶段4: 发射]
  5. OnCast                  -- 法术发射（创建投射物）或立即执行前
  6. AfterCast               -- 投射物发射后、立即法术完成

[阶段5: 效果发射 (Launch)]
  7. OnEffectLaunch          -- 效果处理前（发射阶段）
  8. OnCalcCritChance        -- 暴击率计算（发射后，逐目标）

[阶段6: 发射目标处理 (LaunchTarget)]
  9. OnEffectLaunchTarget    -- 发射阶段逐目标效果处理

[阶段7: 伤害/治疗计算]
  10a. CalcDamage            -- 伤害计算（逐目标）
  10b. CalcHealing           -- 治疗计算（逐目标）

[阶段8: 抵抗/吸收]
  11. OnCalculateResistAbsorb -- 伤害抵抗/吸收计算（命中前）

[阶段9: 命中]
  12. OnEffectHit            -- 效果命中处理（命中阶段）
  13. BeforeHit              -- 命中前回调（逐目标，含 missInfo）
  14. OnEffectHitTarget      -- 逐目标效果命中处理
  15. OnHit                  -- 伤害结算、光环触发前
  16. AfterHit               -- 目标所有处理完成后

[阶段10: 蓄力]
  17. OnEmpowerStageCompleted -- 蓄力法术每阶段完成
  18. OnEmpowerCompleted      -- 蓄力法术释放

[特殊: 驱散成功后]
  *. OnEffectSuccessfulDispel -- 效果成功驱散后触发
```

**HookList 成员与对应宏**:

| 序号 | HookList 成员 | 宏 | 虚函数 |
|------|--------------|-----|--------|
| 1 | (虚函数) | -- | `OnPrecast()` |
| 2 | `BeforeCast` | `SpellCastFn` | |
| 3 | `OnCheckCast` | `SpellCheckCastFn` | |
| 4a | `OnObjectAreaTargetSelect` | `SpellObjectAreaTargetSelectFn` | |
| 4b | `OnObjectTargetSelect` | `SpellObjectTargetSelectFn` | |
| 4c | `OnDestinationTargetSelect` | `SpellDestinationTargetSelectFn` | |
| 5 | `OnCast` | `SpellCastFn` | |
| 6 | `AfterCast` | `SpellCastFn` | |
| 7 | `OnEffectLaunch` | `SpellEffectFn` | |
| 8 | `OnCalcCritChance` | `SpellOnCalcCritChanceFn` | |
| 9 | `OnEffectLaunchTarget` | `SpellEffectFn` | |
| 10a | `CalcDamage` | `SpellCalcDamageFn` | |
| 10b | `CalcHealing` | `SpellCalcHealingFn` | |
| 11 | `OnCalculateResistAbsorb` | `SpellOnResistAbsorbCalculateFn` | |
| 12 | `OnEffectHit` | `SpellEffectFn` | |
| 13 | `BeforeHit` | `BeforeSpellHitFn` | |
| 14 | `OnEffectHitTarget` | `SpellEffectFn` | |
| 15 | `OnHit` | `SpellHitFn` | |
| 16 | `AfterHit` | `SpellHitFn` | |
| 17 | `OnEmpowerStageCompleted` | `SpellOnEmpowerStageCompletedFn` | |
| 18 | `OnEmpowerCompleted` | `SpellOnEmpowerCompletedFn` | |
| -- | `OnEffectSuccessfulDispel` | `SpellEffectFn` | |
| -- | (虚函数) | -- | `CalcCastTime(int32)` |

### 4.3 AuraScript Hook 分类

AuraScript 的 28 个 Hook 按功能分为以下类别:

**效果生命周期 Hook:**

| HookList 成员 | 宏 | 说明 |
|--------------|-----|------|
| `OnEffectApply` | `AuraEffectApplyFn` | 效果应用时（可阻止） |
| `AfterEffectApply` | `AuraEffectApplyFn` | 效果应用后 |
| `OnEffectRemove` | `AuraEffectRemoveFn` | 效果移除时（可阻止） |
| `AfterEffectRemove` | `AuraEffectRemoveFn` | 效果移除后 |

**周期效果 Hook:**

| HookList 成员 | 宏 | 说明 |
|--------------|-----|------|
| `OnEffectPeriodic` | `AuraEffectPeriodicFn` | 周期效果 tick |
| `OnEffectUpdatePeriodic` | `AuraEffectUpdatePeriodicFn` | 周期效果更新 |

**数值计算 Hook:**

| HookList 成员 | 宏 | 说明 |
|--------------|-----|------|
| `DoEffectCalcAmount` | `AuraEffectCalcAmountFn` | 效果数值计算 |
| `DoEffectCalcPeriodic` | `AuraEffectCalcPeriodicFn` | 周期数据计算 |
| `DoEffectCalcSpellMod` | `AuraEffectCalcSpellModFn` | 法术修正计算 |
| `DoEffectCalcCritChance` | `AuraEffectCalcCritChanceFn` | DoT/Hot 暴击率 |
| `DoEffectCalcDamageAndHealing` | `AuraEffectCalcDamageFn/HealingFn` | DoT/Hot 伤害/治疗 |

**吸收 Hook:**

| HookList 成员 | 宏 | 说明 |
|--------------|-----|------|
| `OnEffectAbsorb` | `AuraEffectAbsorbFn` | 伤害吸收 |
| `AfterEffectAbsorb` | (同上) | 吸收后 |
| `OnEffectAbsorbHeal` | `AuraEffectAbsorbHealFn` | 治疗吸收 |
| `AfterEffectAbsorbHeal` | (同上) | 治疗吸收后 |
| `OnEffectManaShield` | `AuraEffectManaShieldFn` | 法力盾吸收 |
| `AfterEffectManaShield` | (同上) | 法力盾吸收后 |
| `OnEffectSplit` | `AuraEffectSplitFn` | 分担伤害 |

**Proc Hook:**

| HookList 成员 | 宏 | 说明 |
|--------------|-----|------|
| `DoCheckProc` | `AuraCheckProcFn` | 全局 proc 检查 |
| `DoCheckEffectProc` | `AuraCheckEffectProcFn` | 效果级 proc 检查 |
| `DoPrepareProc` | `AuraProcFn` | proc 准备（可阻止充能/CD） |
| `OnProc` | `AuraProcFn` | proc 触发 |
| `AfterProc` | `AuraProcFn` | proc 触发后 |
| `OnEffectProc` | `AuraEffectProcFn` | 效果级 proc |
| `AfterEffectProc` | `AuraEffectProcFn` | 效果级 proc 后 |

**其他 Hook:**

| HookList 成员 | 宏 | 说明 |
|--------------|-----|------|
| `DoCheckAreaTarget` | `AuraCheckAreaTargetFn` | 区域光环目标检查 |
| `OnDispel` | `AuraDispelFn` | 驱散回调 |
| `AfterDispel` | `AuraDispelFn` | 驱散后回调 |
| `OnHeartbeat` | `AuraHeartbeatFn` | 单位心跳 |
| `OnEnterLeaveCombat` | `AuraEnterLeaveCombatFn` | 进出战斗 |

### 4.4 PreventDefault 机制

**SpellScript 中的阻止机制** (`SpellScript.h:720-721, 934-939`):

SpellScript 使用两个位掩码 (`m_hitPreventEffectMask`, `m_hitPreventDefaultEffectMask`) 来跟踪被阻止的效果:

```
void PreventHitEffect(SpellEffIndex effIndex);       // :934
    -- 阻止指定效果的全部执行（包括其他效果/hit 脚本）
    -- 不影响 aura/damage/heal 效果
    -- 已处理的效果不受影响
    -- 设置 m_hitPreventEffectMask 对应位

void PreventHitDefaultEffect(SpellEffIndex effIndex); // :939
    -- 阻止指定效果的默认执行，但脚本仍可运行
    -- 不影响 aura/damage/heal 效果
    -- 已处理的效果不受影响
    -- 设置 m_hitPreventDefaultEffectMask 对应位

void PreventHitDamage();        // :916 -- 等价于 SetHitDamage(0)
void PreventHitHeal();          // :920 -- 等价于 SetHitHeal(0)
void PreventHitAura();          // :928 -- 阻止在当前命中目标上应用光环
```

内部检查方法:
```cpp
bool _IsEffectPrevented(SpellEffIndex effIndex) const        // :720
    { return (m_hitPreventEffectMask & (1 << effIndex)) != 0; }

bool _IsDefaultEffectPrevented(SpellEffIndex effIndex) const // :721
    { return (m_hitPreventDefaultEffectMask & (1 << effIndex)) != 0; }
```

**阶段守卫方法** (`SpellScript.h:724-729`):
```
bool IsInCheckCastHook() const;           // :724
bool IsAfterTargetSelectionPhase() const;  // :725
bool IsInTargetHook() const;              // :726
bool IsInModifiableHook() const;          // :727
bool IsInHitPhase() const;               // :728
bool IsInEffectHook() const;             // :729
```

这些守卫方法用于在上下文访问接口中进行运行时检查，确保在错误阶段调用敏感方法时产生断言失败。

**AuraScript 中的阻止机制** (`SpellScript.h:1627, 1825`):

```
void PreventDefaultAction();  // :1825
    -- 阻止当前 Hook 的默认行为
    -- 仅在支持阻止的 Hook 中有效（如 OnEffectApply, DoCheckProc 等）

bool _IsDefaultActionPrevented() const;  // :1627
    -- 检查默认行为是否已被阻止
```

AuraScript 使用栈结构 (`ScriptStateStore` / `ScriptStateStack`, `:1633-1644`) 保存/恢复 `m_defaultActionPrevented` 标志，支持 Hook 嵌套调用时的状态隔离:

```
class ScriptStateStore
{
    AuraApplication const* _auraApplication;
    uint8 _currentScriptState;
    bool _defaultActionPrevented;
};
typedef std::stack<ScriptStateStore> ScriptStateStack;
```

---

## 5. 上下文访问接口 (Context Access API)

### 5.1 按阶段可用的方法分类

SpellScript 提供的上下文访问方法 (`SpellScript.h:854-963`) 根据法术处理阶段有不同的可用性限制:

**所有阶段可用** (`:855-860`):

| 方法 | 行号 | 返回类型 | 说明 |
|------|------|---------|------|
| `GetCaster()` | :855 | `Unit*` | 当前施法者 |
| `GetGObjCaster()` | :856 | `GameObject*` | 如果施法者是 GameObject |
| `GetOriginalCaster()` | :857 | `Unit*` | 原始施法者（触发链追溯） |
| `GetSpellInfo()` | :858 | `SpellInfo const*` | 法术原型数据 |
| `GetEffectInfo(effIndex)` | :859 | `SpellEffectInfo const&` | 指定效果的信息 |
| `GetSpellValue()` | :860 | `SpellValue const*` | 法术运行时数值 |
| `GetCastItem()` | :949 | `Item*` | 施法物品 |
| `GetTriggeringSpell()` | :955 | `SpellInfo const*` | 触发当前法术的法术 |
| `GetSpell()` | :924 | `Spell*` | Spell 对象指针 |

**施法准备完成后可用** (`:862-895`):

| 方法 | 行号 | 返回类型 | 说明 |
|------|------|---------|------|
| `GetExplTargetDest()` | :874 | `WorldLocation const*` | 显式目的地目标 |
| `SetExplTargetDest(loc)` | :876 | `void` | 设置显式目的地 |
| `GetExplTargetWorldObject()` | :879 | `WorldObject*` | 显式世界对象目标 |
| `GetExplTargetUnit()` | :882 | `Unit*` | 显式单位目标 |
| `GetExplTargetGObj()` | :885 | `GameObject*` | 显式 GameObject 目标 |
| `GetExplTargetItem()` | :888 | `Item*` | 显式物品目标 |
| `GetCastDifficulty()` | :963 | `Difficulty` | 触发法术的难度 |

> 参见 [01-spell-data.md](01-spell-data.md) 中关于 SpellInfo 效果目标 (ImplicitTarget) 的说明，以及 [02-spell-cast-flow.md](02-spell-cast-flow.md) 中关于目标解析阶段的描述。

**目标选择完成后可用** (`:890-895`):

| 方法 | 行号 | 返回类型 | 说明 |
|------|------|---------|------|
| `GetUnitTargetCountForEffect(effect)` | :891 | `int64` | 效果的单位目标数量 |
| `GetUnitTargetIndexForEffect(target, effect)` | :892 | `int32` | 目标在目标列表中的索引 |
| `GetGameObjectTargetCountForEffect(effect)` | :893 | `int64` | 效果的 GameObject 目标数量 |
| `GetItemTargetCountForEffect(effect)` | :894 | `int64` | 效果的物品目标数量 |
| `GetCorpseTargetCountForEffect(effect)` | :895 | `int64` | 效果的尸体目标数量 |

> 参见 [02-spell-cast-flow.md](02-spell-cast-flow.md) 中关于 Spell::SelectEffectTargets 和目标映射的说明。

**命中阶段可用** (`:897-946`):

| 方法 | 行号 | 返回类型 | 说明 |
|------|------|---------|------|
| `GetHitUnit()` | :899 | `Unit*` | 当前命中目标（Unit） |
| `GetHitCreature()` | :901 | `Creature*` | 当前命中目标（Creature） |
| `GetHitPlayer()` | :903 | `Player*` | 当前命中目标（Player） |
| `GetHitItem()` | :905 | `Item*` | 当前命中目标（Item） |
| `GetHitGObj()` | :907 | `GameObject*` | 当前命中目标（GameObject） |
| `GetHitCorpse()` | :909 | `Corpse*` | 当前命中目标（Corpse） |
| `GetHitDest()` | :911 | `WorldLocation*` | 当前命中目的地 |
| `GetHitDamage()` | :914 | `int32` | 命中伤害值 |
| `SetHitDamage(damage)` | :915 | `void` | 设置命中伤害 |
| `PreventHitDamage()` | :916 | `void` | 阻止命中伤害（设为 0） |
| `GetHitHeal()` | :919 | `int32` | 命中治疗值 |
| `SetHitHeal(heal)` | :920 | `void` | 设置命中治疗 |
| `PreventHitHeal()` | :921 | `void` | 阻止命中治疗（设为 0） |
| `IsHitCrit()` | :923 | `bool` | 是否暴击 |
| `GetHitAura(...)` | :926 | `Aura*` | 当前命中目标的光环 |
| `PreventHitAura()` | :928 | `void` | 阻止光环应用 |
| `PreventHitEffect(effIndex)` | :934 | `void` | 阻止效果执行 |
| `PreventHitDefaultEffect(effIndex)` | :939 | `void` | 阻止效果默认执行 |

> 参见 [03-spell-effects.md](03-spell-effects.md) 中关于 SpellEffectInfo 和效果处理器的说明，以及 [04-spell-targeting.md](04-spell-targeting.md) 中关于命中目标列表的说明。

**仅 EffectHandler 中可用** (`:941-946`):

| 方法 | 行号 | 返回类型 | 说明 |
|------|------|---------|------|
| `GetEffectInfo()` | :942 | `SpellEffectInfo const&` | 当前效果的 EffectInfo（无参数重载） |
| `GetEffectValue()` | :943 | `int32` | 当前效果值 |
| `SetEffectValue(value)` | :944 | `void` | 设置效果值 |
| `GetEffectVariance()` | :945 | `float` | 效果方差 |
| `SetEffectVariance(variance)` | :946 | `void` | 设置效果方差 |

**施法控制方法**:

| 方法 | 行号 | 返回类型 | 说明 |
|------|------|---------|------|
| `CreateItem(itemId, context)` | :952 | `void` | 创建物品 |
| `FinishCast(result, param1, param2)` | :958 | `void` | 提前结束施法 |
| `SetCustomCastResultMessage(result)` | :960 | `void` | 设置自定义施法失败消息 |

### 5.2 AuraScript 上下文访问接口

AuraScript 提供两类访问接口: 光环级 (`Aura` 代理) 和目标级 (`AuraApplication` 代理)。

**光环级访问 (始终可用)** (`:1827-1894`):

| 方法 | 行号 | 返回类型 | 说明 |
|------|------|---------|------|
| `GetSpellInfo()` | :1830 | `SpellInfo const*` | 光环法术原型 |
| `GetId()` | :1833 | `uint32` | 法术 ID |
| `GetCasterGUID()` | :1836 | `ObjectGuid` | 施法者 GUID |
| `GetCaster()` | :1838 | `Unit*` | 施法者 Unit |
| `GetGObjCaster()` | :1840 | `GameObject*` | 施法者 GameObject |
| `GetOwner()` | :1842 | `WorldObject*` | 光环所有者 |
| `GetUnitOwner()` | :1844 | `Unit*` | 光环所有者（Unit） |
| `GetDynobjOwner()` | :1846 | `DynamicObject*` | 光环所有者（DynObj） |
| `GetAura()` | :1851 | `Aura*` | Aura 对象 |
| `GetType()` | :1854 | `AuraObjectType` | 光环类型 |
| `Remove(removeMode)` | :1849 | `void` | 移除光环 |
| `GetDuration()` / `SetDuration()` | :1857-1858 | `int32` / `void` | 持续时间操控 |
| `GetMaxDuration()` / `SetMaxDuration()` | :1862-1863 | `int32` / `void` | 最大持续时间 |
| `GetCharges()` / `SetCharges()` | :1871-1872 | `uint8` / `void` | 充能数操控 |
| `GetStackAmount()` / `SetStackAmount()` | :1879-1880 | `uint8` / `void` | 层数操控 |
| `HasEffect(effIndex)` | :1889 | `bool` | 是否有指定效果 |
| `GetEffect(effIndex)` | :1891 | `AuraEffect*` | 获取效果指针 |
| `HasEffectType(type)` | :1894 | `bool` | 是否有指定光环类型 |

**目标级访问 (仅在 AuraApplication 可用的 Hook 中)** (`:1896-1904`):

| 方法 | 行号 | 返回类型 | 说明 |
|------|------|---------|------|
| `GetTarget()` | :1902 | `Unit*` | 当前处理的目标 |
| `GetTargetApplication()` | :1904 | `AuraApplication const*` | 当前目标的应用 |

> 参见 [01-spell-data.md](01-spell-data.md) 中关于 SpellInfo 光环类型 (AuraType) 的定义，以及 [03-spell-effects.md](03-spell-effects.md) 中关于 AuraEffect 生命周期的说明。

### 5.3 阶段检查与安全守卫

SpellScript 和 AuraScript 在执行 Hook 前后通过 `_PrepareScriptCall` / `_FinishScriptCall` 维护当前阶段状态:

- **SpellScript**: `_PrepareScriptCall(SpellScriptHookType)` 记录当前 Hook 类型，用于 `IsInCheckCastHook()`、`IsInHitPhase()` 等守卫方法
- **AuraScript**: `_PrepareScriptCall(AuraScriptHookType, AuraApplication*)` 同时保存当前 `AuraApplication` 和 `m_defaultActionPrevented` 状态到栈中，支持嵌套 Hook 调用的状态恢复

这种设计确保了:
1. 上下文访问方法在正确阶段被调用（错误阶段触发断言）
2. PreventDefaultAction 的作用域限制在当前 Hook 内
3. 多脚本场景下各脚本的 Hook 调用互不干扰
