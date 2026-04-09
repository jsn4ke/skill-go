# 02 - 法术目标选择系统 (Spell Target Selection System)

## 1. 概述 (Overview)

TrinityCore 的法术目标选择系统负责在法术施放阶段确定所有受影响的目标。该系统是整个法术管线中最复杂的子系统之一，它需要同时处理来自客户端的显式目标（玩家点击/选中的目标）和来自 DBC 数据的隐式目标（法术效果自身定义的目标选取规则）。

系统的核心设计理念是**声明式目标描述**：每个法术效果通过 `TargetA` 和 `TargetB` 两个字段声明目标选取方式，运行时引擎根据这些声明式描述自动完成目标选取，无需为每个法术编写特定的目标选取逻辑。DBSpellEffect.dbc 中的原始枚举值被映射到一个五维参数空间，从而将上百种目标类型统一到少数几个标准化的选取算法中。

### 关键设计原则

- **显式与隐式分离**：客户端提供的目标与 DBC 定义的目标规则独立处理，先解析显式目标，再通过隐式规则扩展目标列表。
- **五维参数化**：每个目标类型通过 `ObjectType`、`ReferenceType`、`SelectionCategory`、`CheckType`、`DirectionType` 五个维度完整描述。
- **脚本可拦截**：所有目标选取路径都提供脚本钩子，允许服务器端脚本修改或替换目标列表。

### 主要文件位置

| 文件 | 职责 |
|------|------|
| `src/server/game/Spells/SpellInfo.h` | 目标类型枚举定义、`SpellImplicitTargetInfo` 类 |
| `src/server/game/Spells/SpellInfo.cpp:245+` | 五维静态数据数组 `_data` 的初始化 |
| `src/server/game/Spells/SpellDefines.h:309-450` | `SpellCastTargetFlags`、`SpellCastTargets` 定义 |
| `src/server/game/Spells/Spell.cpp:619-827` | 显式目标初始化与选择 |
| `src/server/game/Spells/Spell.cpp:718-812` | `SelectSpellTargets()` 主调度函数 |
| `src/server/game/Spells/Spell.cpp:894-1012` | `SelectEffectImplicitTargets()` 隐式目标分派 |
| `src/server/game/Spells/Spell.h:1009-1084` | 四种目标检查器（Area/Cone/Line/Traj） |
| `src/server/game/Spells/Spell.h:1088-1116` | `TargetPriorityRule` 排序规则 |

---

## 2. 核心数据结构 (Core Data Structures)

### 2.1 SpellImplicitTargetInfo 五维系统

`SpellImplicitTargetInfo` 是目标选择系统的核心数据结构，定义于 `src/server/game/Spells/SpellInfo.h:174-207`。它将 DBSpellEffect.dbc 中的每个目标枚举值映射到一个五维参数空间。

```cpp
// src/server/game/Spells/SpellInfo.h:174-207
class TC_GAME_API SpellImplicitTargetInfo
{
    Targets _target;
    struct StaticData
    {
        SpellTargetObjectTypes ObjectType;     // 返回的对象类型
        SpellTargetReferenceTypes ReferenceType; // 参考系对象
        SpellTargetSelectionCategories SelectionCategory; // 选取算法分类
        SpellTargetCheckTypes SelectionCheckType;   // 选取过滤条件
        SpellTargetDirectionTypes DirectionType;    // 方向（锥形/目标点）
    };
    static std::array<StaticData, TOTAL_SPELL_TARGETS> _data;
};
```

静态数组 `_data` 在 `src/server/game/Spells/SpellInfo.cpp:245` 处以聚合初始化方式填充，每个条目对应一个 `Targets` 枚举值。构造函数（`SpellInfo.cpp:73-76`）仅将传入的枚举值存储，所有查询都通过 `_data` 数组间接访问。

#### 维度一：SelectionCategory（选择类别）

定义于 `src/server/game/Spells/SpellInfo.h:41-51`，决定使用哪个选取算法。

```
enum SpellTargetSelectionCategories
{
    TARGET_SELECT_CATEGORY_NYI,       // 未实现
    TARGET_SELECT_CATEGORY_DEFAULT,   // 默认：对象/目标点选取
    TARGET_SELECT_CATEGORY_CHANNEL,   // 引导法术目标
    TARGET_SELECT_CATEGORY_NEARBY,    // 附近单一目标
    TARGET_SELECT_CATEGORY_CONE,      // 锥形范围
    TARGET_SELECT_CATEGORY_AREA,      // 面积范围
    TARGET_SELECT_CATEGORY_TRAJ,      // 抛物线弹道
    TARGET_SELECT_CATEGORY_LINE       // 直线范围
};
```

#### 维度二：ReferenceType（参考类型）

定义于 `src/server/game/Spells/SpellInfo.h:53-61`，确定目标选取的参考位置。

```
enum SpellTargetReferenceTypes
{
    TARGET_REFERENCE_TYPE_NONE,   // 无参考
    TARGET_REFERENCE_TYPE_CASTER, // 以施法者为参考
    TARGET_REFERENCE_TYPE_TARGET, // 以显式目标为参考
    TARGET_REFERENCE_TYPE_LAST,   // 以最后添加的目标为参考
    TARGET_REFERENCE_TYPE_SRC,    // 以源位置为参考
    TARGET_REFERENCE_TYPE_DEST    // 以目标位置为参考
};
```

#### 维度三：ObjectType（对象类型）

定义于 `src/server/game/Spells/SpellInfo.h:63-77`，确定目标选取返回的世界对象类型。

```
enum SpellTargetObjectTypes : uint8
{
    TARGET_OBJECT_TYPE_NONE = 0,
    TARGET_OBJECT_TYPE_SRC,          // 源位置（设置 m_targets 的 src）
    TARGET_OBJECT_TYPE_DEST,         // 目标位置（设置 m_targets 的 dst）
    TARGET_OBJECT_TYPE_UNIT,         // 单位
    TARGET_OBJECT_TYPE_UNIT_AND_DEST, // 单位 + 同时设置目标位置
    TARGET_OBJECT_TYPE_GOBJ,         // 游戏对象
    TARGET_OBJECT_TYPE_GOBJ_ITEM,    // 游戏对象 + 物品
    TARGET_OBJECT_TYPE_ITEM,         // 物品
    TARGET_OBJECT_TYPE_CORPSE,       // 尸体
    TARGET_OBJECT_TYPE_CORPSE_ENEMY, // 敌方尸体（仅用于效果目标类型）
    TARGET_OBJECT_TYPE_CORPSE_ALLY   // 友方尸体（仅用于效果目标类型）
};
```

#### 维度四：CheckType（检查类型）

定义于 `src/server/game/Spells/SpellInfo.h:79-90`，确定目标过滤条件。

```
enum SpellTargetCheckTypes : uint8
{
    TARGET_CHECK_DEFAULT,    // 默认检查
    TARGET_CHECK_ENTRY,      // 按 Creature/GameObject Entry 过滤
    TARGET_CHECK_ENEMY,      // 仅敌方
    TARGET_CHECK_ALLY,       // 仅友方
    TARGET_CHECK_PARTY,      // 仅队伍成员
    TARGET_CHECK_RAID,       // 仅团队副本成员
    TARGET_CHECK_RAID_CLASS, // 团队中指定职业
    TARGET_CHECK_PASSENGER,  // 载具乘客
    TARGET_CHECK_SUMMONED    // 召唤物
};
```

#### 维度五：DirectionType（方向类型）

定义于 `src/server/game/Spells/SpellInfo.h:92-105`，用于锥形范围和目标点偏移的方向计算。

```
enum SpellTargetDirectionTypes
{
    TARGET_DIR_NONE,         // 无方向
    TARGET_DIR_FRONT,        // 正前方 (0)
    TARGET_DIR_BACK,         // 正后方 (PI)
    TARGET_DIR_RIGHT,        // 右侧 (-PI/2)
    TARGET_DIR_LEFT,         // 左侧 (PI/2)
    TARGET_DIR_FRONT_RIGHT,  // 右前方 (-PI/4)
    TARGET_DIR_BACK_RIGHT,   // 右后方 (-3PI/4)
    TARGET_DIR_BACK_LEFT,    // 左后方 (3PI/4)
    TARGET_DIR_FRONT_LEFT,   // 左前方 (PI/4)
    TARGET_DIR_RANDOM,       // 随机方向
    TARGET_DIR_ENTRY         // 由目标条件决定
};
```

方向角度的换算逻辑在 `SpellImplicitTargetInfo::CalcDirectionAngle()` 中实现（`SpellInfo.cpp:108-133`）。

### 2.2 SpellCastTargets

定义于 `src/server/game/Spells/SpellDefines.h:362-450`，代表一次法术施放的完整目标集合，包含施法器收集的所有目标信息。

```
+---------------------------------------------------+
|                  SpellCastTargets                  |
+---------------------------------------------------+
| m_targetMask: uint32                               |
|   -- 位掩码，由 SpellCastTargetFlags 组合而成     |
|                                                   |
| 对象目标:                                         |
|   m_objectTarget / m_objectTargetGUID              |
|   m_itemTarget / m_itemTargetGUID                  |
|                                                   |
| 位置目标:                                         |
|   m_src: SpellDestination (源位置)                 |
|   m_dst: SpellDestination (目标位置)               |
|                                                   |
| 弹道参数:                                         |
|   m_pitch: float (仰角)                           |
|   m_speed: float (飞行速度)                       |
|                                                   |
| 其他:                                             |
|   m_strTarget: string (字符串目标)                 |
|   m_itemTargetEntry: uint32                        |
+---------------------------------------------------+
```

关键方法：
- `GetObjectTarget()` / `GetUnitTarget()` / `GetGOTarget()` -- 获取对象目标
- `GetSrc()` / `GetDst()` -- 获取源/目标位置
- `SetUnitTarget()` / `SetGOTarget()` / `SetDst()` -- 设置目标
- `HasTraj()` -- 是否为弹道法术（`m_speed != 0`）

### 2.3 SpellCastTargetFlags

定义于 `src/server/game/Spells/SpellDefines.h:309-344`，是用于客户端-服务器通信的位标志，描述法术施放请求中包含的目标类型。

```
TARGET_FLAG_NONE            = 0x00000000
TARGET_FLAG_UNIT            = 0x00000002  // 单位目标 (pguid)
TARGET_FLAG_UNIT_RAID       = 0x00000004  // 团队成员验证
TARGET_FLAG_UNIT_PARTY      = 0x00000008  // 队伍成员验证
TARGET_FLAG_ITEM            = 0x00000010  // 物品目标 (pguid)
TARGET_FLAG_SOURCE_LOCATION = 0x00000020  // 源位置 (pguid + 3 float)
TARGET_FLAG_DEST_LOCATION   = 0x00000040  // 目标位置 (pguid + 3 float)
TARGET_FLAG_UNIT_ENEMY      = 0x00000080  // 敌方单位验证
TARGET_FLAG_UNIT_ALLY       = 0x00000100  // 友方单位验证
TARGET_FLAG_CORPSE_ENEMY    = 0x00000200  // 敌方尸体 (pguid)
TARGET_FLAG_UNIT_DEAD       = 0x00000400  // 死亡单位验证
TARGET_FLAG_GAMEOBJECT      = 0x00000800  // 游戏对象 (pguid)
TARGET_FLAG_TRADE_ITEM      = 0x00000010  // 交易物品
TARGET_FLAG_STRING          = 0x00002000  // 字符串
TARGET_FLAG_CORPSE_ALLY     = 0x00008000  // 友方尸体 (pguid)
TARGET_FLAG_UNIT_MINIPET    = 0x00010000  // 非战斗宠物验证
TARGET_FLAG_GLYPH_SLOT      = 0x00020000  // 雕文槽位
TARGET_FLAG_DEST_TARGET     = 0x00040000  // 目标位置目标

// 组合掩码:
TARGET_FLAG_UNIT_MASK    = UNIT | RAID | PARTY | ENEMY | ALLY | DEAD | MINIPET | PASSENGER
TARGET_FLAG_GAMEOBJECT_MASK = GAMEOBJECT | GAMEOBJECT_ITEM
TARGET_FLAG_CORPSE_MASK  = CORPSE_ALLY | CORPSE_ENEMY
TARGET_FLAG_ITEM_MASK    = TRADE_ITEM | ITEM | GAMEOBJECT_ITEM
```

`SpellImplicitTargetInfo::GetExplicitTargetMask()`（`SpellInfo.cpp:140+`）将 ObjectType 映射为 SpellCastTargetFlags，用于构建发给客户端的 `SMSG_SPELL_GO` 等消息包中的目标掩码。

---

## 3. 执行流程 (Execution Flow)

### 3.1 总体流程

目标选择的入口是 `Spell::SelectSpellTargets()`（`Spell.cpp:718-812`），在法术状态机进入 `SPELL_STATE_PREPARING` 阶段之后、正式施放之前调用。

```
玩家施放法术
      |
      v
Spell::prepare() [ Spell.cpp:594 ]
      |
      v
Spell::InitExplicitTargets() [ Spell.cpp:619-687 ]
      |  -- 解析客户端发来的目标数据
      |  -- 修正/补充缺失的目标
      v
Spell::SelectSpellTargets() [ Spell.cpp:718-812 ]
      |
      +---> SelectExplicitTargets() [ Spell.cpp:689-717 ]
      |       |  -- 处理目标重定向（如接地图腾）
      |       v
      +---> FOR EACH SpellEffectInfo in m_spellInfo->GetEffects()
      |       |
      |       +---> SelectEffectImplicitTargets(TargetA) [ Spell.cpp:894-1012 ]
      |       +---> SelectEffectImplicitTargets(TargetB) [ Spell.cpp:894-1012 ]
      |       +---> SelectEffectTypeImplicitTargets() [ effect-based targets ]
      |       +---> AddDestTarget() [ if has dst ]
      |       +---> 验证: SPELL_ATTR1_REQUIRE_ALL_TARGETS
      |       +---> 验证: 引导法术必须有目标
      |       v
      +---> 验证: SPELL_ATTR2_FAIL_ON_ALL_TARGETS_IMMUNE
              |
              v
      目标选择完成，进入施放阶段
```

### 3.2 显式目标初始化（InitExplicitTargets）

`Spell::InitExplicitTargets()`（`Spell.cpp:619-687`）负责从客户端发来的 `SpellCastTargets` 中解析和修正目标：

1. **复制目标**：直接将客户端发来的 `targets` 赋值给 `m_targets`
2. **计算需要的目标掩码**：通过 `m_spellInfo->GetExplicitTargetMask()` 获取该法术声明的目标需求
3. **验证对象目标**：如果客户端发来的对象目标（Unit/GameObject/Corpse）与法术需求不匹配，则移除
4. **自动选择单位目标**：
   - 玩家施法器：尝试使用当前选中的目标
   - NPC 施法器：尝试使用当前攻击目标
   - 兜底：若法术需要友方目标（RAID/PARTY/ALLY），使用自身
5. **设置目标位置**：若法术需要 `DEST_LOCATION` 但未设置，使用对象目标或施法者位置
6. **设置源位置**：若法术需要 `SOURCE_LOCATION` 但未设置，使用施法者位置

### 3.3 显式目标重定向（SelectExplicitTargets）

`Spell::SelectExplicitTargets()`（`Spell.cpp:689-717`）处理目标重定向机制：

```
m_targets.GetUnitTarget()
      |
      v
法术是否有敌方/敌意目标需求?
      |
  YES +---> 法术伤害类型?
      |       |
      |       +---> MAGIC  -> GetMagicHitRedirectTarget()
      |       +---> MELEE   -> GetMeleeHitRedirectTarget()
      |       +---> RANGED  -> GetMeleeHitRedirectTarget()
      |       |
      |       v
      |   redirect != target?  --->  SetUnitTarget(redirect)
      |
  NO  ---> 无处理
```

典型的重定向场景：玩家对术士施放有害法术，但术士身旁有接地图腾（Grounding Totem），法术目标被重定向到图腾上。

### 3.4 隐式目标分派（SelectEffectImplicitTargets）

`Spell::SelectEffectImplicitTargets()`（`Spell.cpp:894-1012`）是隐式目标选择的核心调度函数。它首先通过 `SelectionCategory` 确定哪些效果可以合并处理（相同目标类型/条件/半径的效果共享同一次区域搜索），然后根据类别分派到具体算法：

```
SelectEffectImplicitTargets(targetType)
      |
      v
targetType.GetTarget() == 0?  --YES--> return
      |
      NO
      |
      v
targetType.GetSelectionCategory()
      |
      +---> CATEGORY_CHANNEL  ---> SelectImplicitChannelTargets()
      |
      +---> CATEGORY_NEARBY   ---> SelectImplicitNearbyTargets()
      |                           + SelectImplicitChainTargets()
      |
      +---> CATEGORY_CONE     ---> SelectImplicitConeTargets()
      |
      +---> CATEGORY_AREA     ---> SelectImplicitAreaTargets()
      |
      +---> CATEGORY_TRAJ     ---> CheckDst()
      |                           + SelectImplicitTrajTargets()
      |
      +---> CATEGORY_LINE     ---> SelectImplicitLineTargets()
      |
      +---> CATEGORY_DEFAULT
      |       |
      |       +---> OBJECT_TYPE_SRC   ---> m_targets.SetSrc(*m_caster)
      |       +---> OBJECT_TYPE_DEST  ---> 分三种 ReferenceType:
      |       |       +---> CASTER  ---> SelectImplicitCasterDestTargets()
      |       |       +---> TARGET  ---> SelectImplicitTargetDestTargets()
      |       |       +---> DEST    ---> SelectImplicitDestDestTargets()
      |       +---> OTHER           ---> 分两种 ReferenceType:
      |               +---> CASTER  ---> SelectImplicitCasterObjectTargets()
      |               +---> TARGET  ---> SelectImplicitTargetObjectTargets()
      |
      +---> CATEGORY_NYI      ---> LOG_DEBUG, 不处理
```

---

## 4. 关键算法与分支 (Key Algorithms & Branches)

### 4.1 SelectImplicitNearbyTargets（附近目标选取）

位置：`Spell.cpp:1075-1257`

此函数处理 `TARGET_SELECT_CATEGORY_NEARBY` 类别。它从施法者附近选取**单个**目标（最常见于治疗/增益/单体伤害法术）。

**核心逻辑**：

1. **范围计算**：根据 `CheckType` 选择不同的距离范围
   - `ENEMY`：使用 `GetMaxRange(false)`（敌对范围）
   - `ALLY/PARTY/RAID/RAID_CLASS`：使用 `GetMaxRange(true)`（友方范围）
   - `ENTRY/DEFAULT`：根据法术正负性选择范围

2. **TARGET_CHECK_ENTRY 紧急处理**：当 `CheckType == TARGET_CHECK_ENTRY` 但缺少条件列表时，回退到特殊逻辑：
   - `GOBJ` + `RequiresSpellFocus`：使用焦点对象
   - `DEST` + `RequiresSpellFocus`：使用焦点对象位置
   - `DEST` + `TARGET_DEST_NEARBY_ENTRY_OR_DB`：尝试 SpellTargetPosition DB，失败则使用施法者 + 随机半径

3. **目标搜索**：调用 `SearchNearbyTarget()` 在指定范围内搜索符合条件的目标

4. **脚本钩子**：调用 `CallScriptObjectAreaTargetSelectHandlers()` 后，将目标添加到目标列表

5. **链式目标**：最后调用 `SelectImplicitChainTargets()` 处理可能存在的链式跳跃效果

**最终分支**根据 `ObjectType` 决定如何添加目标：
- `UNIT`：`AddUnitTarget()`
- `GOBJ`：`AddGOTarget()`
- `GOBJ_ITEM`：添加游戏对象 + 物品
- `ITEM`：`AddItemTarget()`
- `CORPSE`：`AddCorpseTarget()`
- `DEST` / `UNIT_AND_DEST`：设置目标位置

### 4.2 SelectImplicitConeTargets（锥形范围选取）

位置：`Spell.cpp:1259-1311`

处理 `TARGET_SELECT_CATEGORY_CONE` 类别，用于前方锥形 AOE（如战士的雷霆一击、法师的冰锥术）。

**算法**：

1. **锥角计算**：从 `SpellInfo::ConeAngle` 获取，`TARGET_UNIT_CONE_180_DEG_ENEMY` 默认为 180 度
2. **创建锥形检查器**：`WorldObjectSpellConeTargetCheck`（`Spell.h:1055-1064`）
   - 参数：`coneSrc`（施法者位置）、`coneAngle`（锥角弧度）、`lineWidth`（宽度，取 SpellInfo::Width 或施法者 CombatReach）、`radius`（半径）
3. **区域搜索**：使用 `WorldObjectListSearcher` + `SearchTargets` 在以施法者为圆心、`radius + EXTRA_CELL_SEARCH_RADIUS` 为搜索半径的区域内查找
4. **脚本钩子**：`CallScriptObjectAreaTargetSelectHandlers()`
5. **数量限制**：若 `MaxAffectedTargets > 0`，随机裁剪目标列表
6. **添加目标**：遍历结果列表，根据类型调用 `AddUnitTarget()` / `AddGOTarget()` / `AddCorpseTarget()`

### 4.3 SelectImplicitAreaTargets（面积范围选取）

位置：`Spell.cpp:1312-1449`

处理 `TARGET_SELECT_CATEGORY_AREA` 类别，用于所有 AOE 法术（暴风雪、神圣新星、群体治疗等）。这是最复杂的选取算法，因为需要处理多种参考系。

**参考系解析**：

```
ReferenceType        参考对象(referer)    搜索中心(center)
---------------------------------------------------------
CASTER               m_caster             m_caster
TARGET               m_targets.GetUnitTarget()  同 referer
LAST                 最后添加的目标       同 referer
SRC                  m_caster             m_targets.GetSrcPos()
DEST                 m_caster             m_targets.GetDstPos()
```

`TARGET_REFERENCE_TYPE_LAST` 的特殊处理：遍历 `m_UniqueTargetInfo`（逆序），找到当前效果最后一个被添加的目标单位。

**特殊目标类型**：

| 目标枚举 | 处理方式 |
|----------|----------|
| `TARGET_UNIT_CASTER_AND_PASSENGERS` | 施法者 + 载具所有乘客 |
| `TARGET_UNIT_TARGET_ALLY_OR_RAID` | 若目标与施法者在同一团队则做区域搜索，否则仅添加目标自身 |
| `TARGET_UNIT_CASTER_AND_SUMMONS` | 施法者 + 区域搜索 |
| `TARGET_UNIT_AREA_THREAT_LIST` | 施法者仇恨列表中的所有单位 |
| `TARGET_UNIT_AREA_TAP_LIST` | 生物的拾取名单中的所有玩家 |
| 其他 | 标准 `SearchAreaTargets()` |

**`SearchAreaTargets()`**（`Spell.cpp:2194-2206`）的内部实现：
1. 通过 `GetSearcherTypeMask()` 确定搜索的容器类型掩码（Grid/World）
2. 创建 `WorldObjectSpellAreaTargetCheck`（`Spell.h:1043-1053`）
3. 使用 `WorldObjectListSearcher` 执行区域搜索
4. 搜索半径为 `range + EXTRA_CELL_SEARCH_RADIUS`（额外格子边界余量）

**数量限制与排序**：
- `TARGET_UNIT_SRC_AREA_FURTHEST_ENEMY`：按距离降序排序，取最远的 N 个
- 其他：若 `MaxAffectedTargets > 0`，随机裁剪

**UNIT_AND_DEST 联合处理**：当 `ObjectType == TARGET_OBJECT_TYPE_UNIT_AND_DEST` 时，在添加目标后还会调用 `CallScriptDestinationTargetSelectHandlers()` 修改目标位置。

### 4.4 SelectImplicitChainTargets（链式目标选取）

位置：`Spell.cpp:1819-1851`

链式法术（如治疗链、闪电链）在选取到初始目标后，通过此函数扩展到额外的跳跃目标。

**核心参数**：

1. **ChainTargets**：来自 `SpellEffectInfo::ChainTargets`，可通过 `SpellModOp::ChainTargets` 修改
2. **跳跃半径（jumpRadius）**：根据 `DmgClass` 决定：
   - `RANGED`：7.5 码（多重射击）
   - `MELEE`：5.0 码（顺劈斩、横扫）
   - `NONE/MAGIC`：10.0 码（链式闪电），12.5 码（治疗链）

**搜索半径计算**：

```
SPELL_ATTR2_CHAIN_FROM_CASTER?  --YES--> GetMinMaxRange(false).second
ChainFromInitialTarget?         --YES--> jumpRadius
其他                           ------> jumpRadius * chainTargets
```

**LOS 检查**：链式目标需要视线检查（除非有忽略 LOS 属性）。LOS 检查的参考位置取决于 `SPELL_ATTR2_CHAIN_FROM_CASTER` 和 `ChainFromInitialTarget` 属性：
- `CHAIN_FROM_CASTER`：始终从施法者检查 LOS
- `ChainFromInitialTarget`：始终从初始目标检查 LOS
- 默认：从前一个链式目标检查 LOS

链式目标的实际搜索通过 `SearchChainTargets()`（`Spell.cpp:2208+`）实现。

### 4.5 SelectImplicitTrajTargets（抛物线弹道目标选取）

位置：`Spell.cpp:1865-1950`

处理 `TARGET_SELECT_CATEGORY_TRAJ`，用于抛射物法术（如投石、箭雨）。此函数不选取目标单位，而是**调整弹道的目标落点**，使弹道在碰到第一个有效碰撞体时停止。

**算法**：

1. **前置检查**：必须已有弹道数据（`HasTraj()`），且水平距离不为零
2. **参数计算**：
   - `b = tan(pitch)`（弹道斜率）
   - `a = (dz - dist2d * b) / dist2d^2`（抛物线方程参数，限制 `a <= 0`）
3. **碰撞检测**：遍历弹道路径上的所有世界对象：
   - 按距离排序
   - 对每个目标计算水平偏离距离和弹道高度差
   - 若 `|dz - height| > size + b/2 + TRAJECTORY_MISSILE_SIZE`，则未命中
   - 找到第一个命中点后，将 `m_dst` 修改到该碰撞位置

4. **脚本钩子**：通过 `CallScriptDestinationTargetSelectHandlers()` 允许脚本修改落点

### 4.6 SelectImplicitLineTargets（直线范围选取）

位置：`Spell.cpp:1951-2010`

处理 `TARGET_SELECT_CATEGORY_LINE`，用于直线 AOE（如战士的英勇打击、法师的霜火之箭直线版本）。

**算法**：

1. **确定终点参考**：根据 `ReferenceType` 选择终点
   - `SRC`：使用 `m_targets.GetSrcPos()`
   - `DEST`：使用 `m_targets.GetDstPos()`
   - `CASTER`：使用 `m_caster`
   - `TARGET`：使用 `m_targets.GetUnitTarget()`

2. **创建检查器**：`WorldObjectSpellLineTargetCheck`（`Spell.h:1076-1084`）
   - 参数：起点（施法者）、终点、`lineWidth`（取 `SpellInfo::Width` 或施法者 CombatReach）、`radius`

3. **区域搜索**：在 `radius` 范围内搜索
4. **脚本钩子**：`CallScriptObjectAreaTargetSelectHandlers()`
5. **数量限制**：若有 `MaxAffectedTargets` 限制，按距离排序后截断

### 4.7 SelectImplicitChannelTargets（引导法术目标）

位置：`Spell.cpp:1014-1073`

引导法术的目标从当前正在引导的法术中继承。通过 `m_originalCaster->GetCurrentSpell(CURRENT_CHANNELED_SPELL)` 获取引导中的法术对象，然后从中提取目标。

### 4.8 四种 WorldObjectSpellTargetCheck 检查器

所有区域/锥形/直线/弹道搜索都基于统一的检查器继承体系：

```
WorldObjectSpellTargetCheck [Spell.h:1009-1025]  基类
  |
  +-- WorldObjectSpellNearbyTargetCheck [Spell.h:1027-1035]
  |     附近目标检查（单一目标选取）
  |
  +-- WorldObjectSpellAreaTargetCheck [Spell.h:1043-1053]
  |     |  面积目标检查
  |     +-- WorldObjectSpellConeTargetCheck [Spell.h:1055-1064]
  |     |     锥形检查（增加锥角和线宽参数）
  |     |
  |     +-- WorldObjectSpellLineTargetCheck [Spell.h:1076-1084]
  |           直线检查（增加终点和线宽参数）
  |
  +-- WorldObjectSpellTrajTargetCheck [Spell.h:1066-1074]
        弹道检查（增加位置和范围参数）
```

基类 `WorldObjectSpellTargetCheck`（`Spell.h:1009-1025`）的构造函数接受以下参数：
- `caster`：施法者
- `referer`：参考对象
- `spellInfo`：法术信息
- `selectionType`：选择检查类型
- `condList`：条件容器
- `objectType`：目标对象类型

检查器的 `operator()` 执行核心过滤逻辑，包括：阵营检查、存活检查、条件列表匹配等。

### 4.9 CheckEffectTarget 变体

定义于 `Spell.cpp:8216-8324`，有三个重载版本：

| 重载 | 位置 | 检查内容 |
|------|------|----------|
| `CheckEffectTarget(Unit const*, SpellEffectInfo const*, Position const*)` | `Spell.cpp:8216-8302` | 控制效果（魅惑/ possessed）、LOS 检查、特殊效果（剥皮） |
| `CheckEffectTarget(GameObject const*, SpellEffectInfo const*)` | `Spell.cpp:8304-8319` | GO 类型检查（仅可破坏建筑） |
| `CheckEffectTarget(Item const*, SpellEffectInfo const*)` | `Spell.cpp:8321-8324` | 始终返回 true |

Unit 版本是最复杂的，包含：
- **控制效果限制**：`SPELL_AURA_MOD_POSSESS` / `SPELL_AURA_MOD_CHARM` 等不能对有载具、坐骑、已有魅惑者、等级过高的目标使用
- **LOS 忽略层级**：`SPELL_ATTR2_IGNORE_LINE_OF_SIGHT` > 触发法术的 Aura LOS > GO 的 RequireLOS > 默认需要 LOS
- **特殊效果处理**：`SPELL_EFFECT_SKIN_PLAYER_CORPSE` 需要尸体匹配和剥皮标记检查

### 4.10 TargetA + TargetB 组合逻辑

每个 `SpellEffectInfo` 有 `TargetA` 和 `TargetB` 两个目标（`SpellInfo.h:228-229`）。在 `SelectSpellTargets()` 中，两者按顺序独立处理：

```
FOR EACH effect:
    SelectEffectImplicitTargets(effect, effect.TargetA, TargetA, mask)
    SelectEffectImplicitTargets(effect, effect.TargetB, TargetB, mask)
```

**效果合并优化**：在 `SelectEffectImplicitTargets()`（`Spell.cpp:900-933`）中，对于 `NEARBY`/`CONE`/`AREA`/`LINE` 类别的效果，系统会检查后续效果是否可以合并处理。合并条件（全部满足）：
- TargetA 和 TargetB 的目标类型相同
- 隐式目标条件列表相同
- TargetA 半径和 TargetB 半径相同
- `PlayersOnly` 属性相同
- 脚本检查通过（`CheckScriptEffectImplicitTargets()`）

合并后的 `effectMask` 包含多个效果的位，使得一次区域搜索的结果可以同时应用于多个效果。

**典型的 TargetA + TargetB 组合模式**：

| TargetA | TargetB | 含义 |
|---------|---------|------|
| 单位目标 | DEST | 对目标施加效果 + 在目标位置产生效果（如 AOE 标记） |
| AREA_A | AREA_B | 两层区域效果共享搜索结果 |
| 单体 | CHAIN | 先选定一个目标，再链式跳跃到额外目标 |

### 4.11 TargetPriorityRule 排序

定义于 `Spell.h:1088-1116`，是一个泛型目标优先级规则。它支持接受 `WorldObject*`、`Unit*` 或 `Player*` 的谓词函数，自动处理类型检查。

```cpp
// Spell.h:1090-1106
template <typename Func>
TargetPriorityRule(Func&& func) : Rule([func]<typename T = WorldObject>(T* target) {
    if constexpr (invocable_r<Func, bool, WorldObject*>)
        return std::invoke(func, target);
    else if constexpr (invocable_r<Func, bool, Unit*>)
        return target->IsUnit() && std::invoke(func, target->ToUnit());
    else if constexpr (invocable_r<Func, bool, Player*>)
        return target->IsPlayer() && std::invoke(func, target->ToPlayer());
})
```

通过 `SortTargetsWithPriorityRules()` 对目标列表排序，最多支持 31 条规则。这为脚本系统提供了灵活的目标优先级自定义能力。

---

## 5. 扩展点与脚本集成 (Extension Points & Script Integration)

### 5.1 三大脚本拦截钩子

TrinityCore 在目标选择流程中提供了三个脚本钩子，全部定义于 `Spell.h:952-954`：

```cpp
// Spell.h:952
void CallScriptObjectAreaTargetSelectHandlers(
    std::list<WorldObject*>& targets,    // [可修改] 目标列表
    SpellEffIndex effIndex,              // 效果索引
    SpellImplicitTargetInfo const& targetType);  // 目标类型信息

// Spell.h:953
void CallScriptObjectTargetSelectHandlers(
    WorldObject*& target,                // [可修改] 单一目标指针
    SpellEffIndex effIndex,
    SpellImplicitTargetInfo const& targetType);

// Spell.h:954
void CallScriptDestinationTargetSelectHandlers(
    SpellDestination& target,            // [可修改] 目标位置
    SpellEffIndex effIndex,
    SpellImplicitTargetInfo const& targetType);
```

#### 钩子一：OnObjectAreaTargetSelect

**调用位置**（6 处）：
- `SelectImplicitConeTargets()` -- `Spell.cpp:1291`（锥形搜索后）
- `SelectImplicitAreaTargets()` -- `Spell.cpp:1423`（面积搜索后）
- `SelectImplicitLineTargets()` -- `Spell.cpp:1985`（直线搜索后）
- `SelectImplicitChainTargets()` -- `Spell.cpp:1838`（链式搜索后）
- 其他需要列表级修改的场景

**用途**：脚本可以修改整个目标列表（添加/移除/排序目标）。适用于需要自定义 AOE 目标选取逻辑的场景。

#### 钩子二：OnObjectTargetSelect

**调用位置**（多处）：
- `SelectImplicitNearbyTargets()` -- `Spell.cpp:1194`（附近目标搜索后）
- `SelectImplicitCasterObjectTargets()` 中的隐式施法者对象选取
- `SelectImplicitTargetObjectTargets()` 中的隐式目标对象选取

**用途**：脚本可以替换单一目标指针。适用于需要自定义单体目标选取的场景，例如让某个法术总是选取血量最低的队友。

**注意**：如果脚本将 `target` 设为 `nullptr`，法术将以 `SPELL_FAILED_BAD_IMPLICIT_TARGETS` 失败（`Spell.cpp:1196-1201`）。

#### 钩子三：OnDestinationTargetSelect

**调用位置**（多处）：
- `SelectImplicitCasterDestTargets()` 中的目标位置选取
- `SelectImplicitTargetDestTargets()` 中的目标位置选取
- `SelectImplicitDestDestTargets()` 中的目标位置选取
- `SelectImplicitTrajTargets()` -- `Spell.cpp:1946`（弹道落点调整后）
- `SelectImplicitAreaTargets()` 中的 UNIT_AND_DEST 处理 -- `Spell.cpp:1418`

**用途**：脚本可以修改法术的目标落点坐标。适用于需要动态调整 AOE 位置的场景。

### 5.2 脚本注册方式

在 `SpellScript` 中，这三个钩子通过以下宏注册：

```cpp
// 在 OnObjectAreaTargetSelect 中修改 AOE 目标列表
class my_spell_script : public SpellScript
{
    void FilterTargets(std::list<WorldObject*>& targets)
    {
        // 移除 BOSS 级别的目标
        targets.remove_if([](WorldObject* obj) {
            if (Unit* u = obj->ToUnit())
                return u->IsCreature() && u->ToCreature()->isWorldBoss();
            return false;
        });
    }

    void Register() override
    {
        OnObjectAreaTargetSelect += SpellObjectAreaTargetSelectFn(
            my_spell_script::FilterTargets, EFFECT_0, TARGET_UNIT_DEST_AREA_ENEMY);
    }
};
```

### 5.3 与其他系统的关联

#### 与法术状态机的关联

目标选择发生在 `SPELL_STATE_PREPARING` 状态的 `Spell::prepare()` 中。目标选择完成后，法术进入 `SPELL_STATE_CASTING`（立即施放法术）或进入准备阶段（有施放时间的法术）。详见 [01-spell-state-machine.md](01-spell-state-machine.md)。

**关键接口**：
- `Spell::prepare()` 调用 `InitExplicitTargets()` + `SelectSpellTargets()`
- `Spell::finish(SPELL_FAILED_*)` 在目标选取失败时中止法术
- `Spell::cast()` 在目标选取成功后执行效果

#### 与脚本系统的关联

目标选择脚本钩子是整个 `SpellScript` / `AuraScript` 框架的一部分。完整的脚本事件生命周期详见 [05-spell-scripting.md](05-spell-scripting.md)。

**脚本事件执行顺序**：
1. `OnObjectTargetSelect` -- 单体目标选取时
2. `OnObjectAreaTargetSelect` -- 区域目标选取时
3. `OnDestinationTargetSelect` -- 目标位置选取时
4. `OnCast` -- 法术开始施放时
5. `BeforeHit` / `OnHit` / `AfterHit` -- 命中时

#### 与条件系统（Condition）的关联

`TARGET_CHECK_ENTRY` 类型的目标选取依赖于 `ConditionContainer`（`SpellEffectInfo::ImplicitTargetConditions`）。条件系统提供比 CheckType 更细粒度的过滤：
- 按 CreatureEntry / GameObjectEntry 过滤
- 按 Aura 状态过滤
- 按 区域/地图/Phase 过滤
- 自定义 Script 条件

当 `TARGET_CHECK_ENTRY` 缺少条件列表时，系统会尝试回退到特殊逻辑（见 4.1 节的紧急处理）。

#### 与伤害计算系统的关联

链式法术（`SelectImplicitChainTargets`）会设置 `m_damageMultipliers` 和 `m_applyMultiplierMask`，这些值在后续伤害计算阶段用于递减链式伤害系数。每次跳跃的伤害为前一次的 `ChainAmplitude` 比例。

#### 与视觉/网络系统的关联

`SelectSpellTargets()` 中通过 `GetTargetFlagMask()` 和 `SetTargetFlag()` 设置的标志位最终被写入 `SMSG_SPELL_GO` / `MSG_SPELL_START` 等网络包，客户端根据这些标志渲染法术效果动画。
