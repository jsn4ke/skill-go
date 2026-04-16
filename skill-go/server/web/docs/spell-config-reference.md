# skill-go Spell System Configuration Reference

> Source: TrinityCore spell system replica

---

## schoolMask

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **SchoolMaskNone** | `uint32 (bitfield)` | 无属性（值为 0） | `0` |
| **SchoolMaskFire** | `uint32 (bitfield)` | 火焰系法术 | `1` |
| **SchoolMaskFrost** | `uint32 (bitfield)` | 冰霜系法术 | `2` |
| **SchoolMaskArcane** | `uint32 (bitfield)` | 奥术系法术 | `4` |
| **SchoolMaskNature** | `uint32 (bitfield)` | 自然系法术 | `8` |
| **SchoolMaskShadow** | `uint32 (bitfield)` | 暗影系法术 | `16` |
| **SchoolMaskHoly** | `uint32 (bitfield)` | 神圣系法术 | `32` |
| **SchoolMaskPhysical** | `uint32 (bitfield)` | 物理攻击 | `64` |

## combatResult

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **CombatResultHit** | `int (enum)` | 普通命中 | `0` |
| **CombatResultMiss** | `int (enum)` | 未命中（攻击落空） | `1` |
| **CombatResultCrit** | `int (enum)` | 暴击（伤害 x2） | `2` |
| **CombatResultDodge** | `int (enum)` | 闪避（目标躲开攻击） | `3` |
| **CombatResultParry** | `int (enum)` | 招架（目标格挡并减免） | `4` |
| **CombatResultBlock** | `int (enum)` | 格挡（通过盾牌减伤） | `5` |
| **CombatResultGlancing** | `int (enum)` | 擦过（PvE 中对高等级目标的减伤） | `6` |
| **CombatResultResist** | `int (enum)` | 部分抵抗（法术伤害减免） | `7` |
| **CombatResultFullResist** | `int (enum)` | 完全抵抗（法术伤害归零） | `8` |

## castResult

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **CastResultSuccess** | `int (enum)` | 施法成功 | `0` |
| **CastResultFailed** | `int (enum)` | 施法失败（被各种条件阻止） | `1` |
| **CastResultInterrupted** | `int (enum)` | 施法被打断 | `2` |

## castError

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **CastErrNone** | `int (enum)` | 无错误 | `0` |
| **CastErrNotReady** | `int (enum)` | 技能冷却中（CD 未结束） | `1` |
| **CastErrOutOfRange** | `int (enum)` | 目标超出射程 | `2` |
| **CastErrSilenced** | `int (enum)` | 被沉默（无法施法） | `3` |
| **CastErrDisarmed** | `int (enum)` | 被缴械（无法武器攻击） | `4` |
| **CastErrShapeshifted** | `int (enum)` | 形态不正确（需要特定变形形态） | `5` |
| **CastErrNoItems** | `int (enum)` | 缺少所需物品（如施法材料） | `6` |
| **CastErrWrongArea** | `int (enum)` | 当前区域不允许施放 | `7` |
| **CastErrMounted** | `int (enum)` | 骑乘状态无法施法 | `8` |
| **CastErrNoMana** | `int (enum)` | 法力值不足 | `9` |
| **CastErrDead** | `int (enum)` | 施法者已死亡 | `10` |
| **CastErrTargetDead** | `int (enum)` | 目标已死亡 | `11` |
| **CastErrSchoolLocked** | `int (enum)` | 该法术系被锁定（如法术反制） | `12` |
| **CastErrNoCharges** | `int (enum)` | 充能次数已用完 | `13` |
| **CastErrOnGCD** | `int (enum)` | 公共冷却中 | `14` |
| **CastErrInterrupted** | `int (enum)` | 读条被打断 | `15` |

## spellInfo

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **ID** | `uint32` | 法术唯一标识（自动分配） | `42833` |
| **Name** | `string` | 法术名称 | `Fireball` |
| **SchoolMask** | `SchoolMask` | 法术所属魔法学派（位掩码） | `见 schoolMask 节` |
| **RecoveryTime** | `int32` | 冷却时间（毫秒），0 = 无冷却 | `0, 6000, 120000` |
| **CategoryRecoveryTime** | `int32` | 分类冷却时间（毫秒），同分类技能共享 CD | `0` |
| **CastTime** | `int32` | 基础施法时间（毫秒），0 = 瞬发 | `0, 1500, 3500` |
| **RangeMin** | `float64` | 最小射程 | `0` |
| **RangeMax** | `float64` | 最大射程 | `0, 30, 40` |
| **MaxTargets** | `int` | 最大目标数量 | `1, 3, 0(无限)` |
| **PowerCost** | `int32` | 法力消耗 | `0, 400` |
| **Effects** | `[]SpellEffectInfo` | 法术效果列表（最多多个效果） | `见 spellEffectInfo 节` |
| **IsAutoRepeat** | `bool` | 是否为自动重复施法（如自动射击） | `false` |
| **PreventionType** | `PreventionType` | 可被何种方式打断（沉默/安抚） | `0=None, 1=Silence, 2=Pacify` |
| **MissileSpeed** | `float64` | 弹道飞行速度（码/秒，0=无弹道即时命中） | `0, 14` |
| **IsChanneled** | `bool` | 是否为引导法术（如暴风雪） | `false` |
| **ChannelDuration** | `int32` | 引导总时长（毫秒） | `8000` |
| **TickInterval** | `int32` | 引导期间每次跳动的间隔（毫秒） | `1000, 2000` |
| **IsEmpower** | `bool` | 是否为蓄力法术（Dragonflight 机制） | `false` |
| **EmpowerStages** | `[]int32` | 蓄力各阶段的阈值（毫秒） | `[1000, 2000, 3000]` |
| **EmpowerMinTime** | `int32` | 最低蓄力时间（毫秒） | `500` |
| **RequiresShapeshiftMask** | `uint32` | 需要特定变形形态（位掩码） | `0` |
| **RequiredAuraState** | `uint32` | 需要施法者身上有特定光环状态 | `0` |
| **RequiredAreaID** | `int32` | 限定区域 ID | `0` |
| **MaxCharges** | `int32` | 充能次数上限，>0 表示充能技能 | `0, 2, 3` |
| **ChargeRecoveryTime** | `int32` | 恢复一次充能的时间（毫秒） | `0, 15000` |
| **RecoveryCategory** | `int32` | 冷却分类，同分类共享 CD | `0` |
| **Reflectable** | `bool` | 是否可被法术反射 | `false` |

## spellEffectInfo

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **EffectIndex** | `int` | 效果序号（从 0 开始） | `0, 1, 2` |
| **EffectType** | `SpellEffectType` | 效果类型 | `见 effectType 节` |
| **SchoolMask** | `SchoolMask` | 该效果使用的魔法学派 | `见 schoolMask 节` |
| **BasePoints** | `int32` | 基础数值（伤害/治疗量） | `888` |
| **Coef** | `float64` | 法术强度系数（SP 加成倍率） | `0.0, 0.5, 1.0, 1.428` |
| **TargetA** | `TargetReference` | 目标选择器 A（主目标） | `见 targetReference 节` |
| **TargetB** | `TargetReference` | 目标选择器 B（次目标/区域） | `见 targetReference 节` |
| **TriggerSpellID** | `uint32` | 触发的法术 ID（EffectType=TriggerSpell 时使用） | `0` |
| **EnergizeType** | `PowerType` | 恢复的资源类型（EffectType=Energize 时使用） | `0=Mana, 1=Rage, 2=Energy` |
| **EnergizeAmount** | `int32` | 恢复的资源量 | `100` |
| **WeaponPercent** | `float64` | 武器伤害百分比（EffectType=WeaponDamage 时使用） | `1.0, 2.0` |
| **AuraType** | `int32` | 光环类型（EffectType=ApplyAura 时使用） | `见 auraType 节` |
| **AuraDuration** | `int32` | 光环持续时间（毫秒），0 = 永久 | `0, 8000, 30000` |

## effectType

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **SpellEffectNone** | `int (enum)` | 无效果（占位） | `0` |
| **SpellEffectSchoolDamage** | `int (enum)` | 指定学派的伤害（如火焰伤害、冰霜伤害） | `1` |
| **SpellEffectHeal** | `int (enum)` | 治疗目标 | `2` |
| **SpellEffectApplyAura** | `int (enum)` | 为目标施加光环（Buff/Debuff） | `3` |
| **SpellEffectTriggerSpell** | `int (enum)` | 触发另一个法术（用于多段效果） | `4` |
| **SpellEffectEnergize** | `int (enum)` | 恢复资源（法力/怒气/能量） | `5` |
| **SpellEffectWeaponDamage** | `int (enum)` | 基于武器伤害的攻击 | `6` |

## auraType

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **AuraTypeBuff** | `int (enum)` | 增益光环（有益效果，如力量祝福） | `0` |
| **AuraTypeDebuff** | `int (enum)` | 减益光环（有害效果，如燃烧 DoT） | `1` |
| **AuraTypePassive** | `int (enum)` | 被动光环（始终生效，如光环效果） | `2` |
| **AuraTypeProc** | `int (enum)` | 触发型光环（满足条件时触发效果） | `3` |

## aura

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **SpellID** | `uint32` | 关联的法术 ID | `42833` |
| **SourceName** | `string` | 创建此光环的法术名称 | `Fireball` |
| **CasterGUID** | `uint64` | 施法者的 GUID | `1` |
| **AuraType** | `AuraType` | 光环分类（Buff/Debuff/Passive/Proc） | `见 auraType 节` |
| **MaxCharges** | `int32` | 最大充能次数（0 = 不限） | `0` |
| **Charges** | `int32` | 当前剩余充能次数 | `0` |
| **Duration** | `int32` | 持续时间（毫秒），0 = 永久 | `0, 8000, 300000` |
| **StackAmount** | `int32` | 当前层数 | `1, 2, 5` |
| **MaxStack** | `int32` | 最大可叠层数 | `1, 3, 5` |
| **ProcChance** | `float64` | 触发概率（0-100），0 = 禁用 | `0, 50, 100` |
| **PPM** | `float64` | 每分钟触发次数（Procs Per Minute），0 = 不限制 | `0, 2.5, 5` |
| **ProcCharges** | `int32` | 最大触发次数后光环消失，0 = 不限 | `0, 1, 3` |
| **RemainingProcs** | `int32` | 剩余可触发次数 | `0` |
| **Effects** | `[]*AuraEffect` | 光环效果列表 | `见 auraEffect 节` |
| **Applications** | `[]*AuraApplication` | 光环应用记录（追踪每个目标的施加时间） | `见 auraApplication 节` |

## auraEffect

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **AuraType** | `AuraType` | 效果的光环类型 | `见 auraType 节` |
| **SpellID** | `uint32` | 关联法术 ID | `42833` |
| **EffectIndex** | `int` | 对应 SpellEffectInfo 的序号 | `0, 1` |
| **AuraName** | `string` | 效果名称 | `DoT` |
| **BaseAmount** | `int32` | 基础数值（伤害/治疗/属性加成） | `29` |
| **MiscValue** | `int32` | 附加参数（用途随效果类型变化） | `0` |
| **PeriodicTimer** | `int32` | 周期触发间隔（毫秒），0 = 非周期性 | `0, 2000, 3000` |

## auraApplication

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **Target** | `*unit.Unit` | 光环施加的目标单位 | `目标 Unit 指针` |
| **BaseAmount** | `int32` | 施加时记录的基础数值 | `29` |
| **RemoveMode** | `RemoveMode` | 光环移除原因（0=默认, 1=过期, 2=取消, 3=驱散, 4=死亡） | `0, 1, 2, 3, 4` |
| **NeedClientUpdate** | `bool` | 是否需要向客户端同步更新 | `false, true` |
| **TimerStart** | `int64` | 施加时间戳（Unix 毫秒），用于计算剩余时间 | `1712345678900` |

## cooldown

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **RecoveryTime** | `int32` | 法术冷却时间（毫秒），施法后触发 | `0, 6000` |
| **CategoryRecoveryTime** | `int32` | 分类共享 CD（毫秒），同 RecoveryCategory 的技能共享 | `0, 1500` |
| **RecoveryCategory** | `int32` | 冷却分类 ID，0 = 无分类 | `0, 133` |
| **MaxCharges** | `int32` | 充能上限，>0 启用充能机制 | `0, 2, 3` |
| **ChargeRecoveryTime** | `int32` | 每次充能恢复时间（毫秒） | `0, 15000, 40000` |
| **GCD** | `int32` | 公共冷却时间（毫秒），默认 1500ms | `1500` |
| **SchoolLockout** | `int32` | 学派锁定时间（毫秒），被法术反制后锁定整个学派 | `4000, 6000` |
| **IsReady()** | `func` | 检查法术是否可用（CD/充能/学派锁定全部通过） | `bool` |

## targetReference

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **TargetType (Type)** | `TargetType` | 目标对象类型 | `0=Unit, 1=GameObject, 2=Item, 3=Corpse` |
| **TargetReferenceType (Reference)** | `TargetReferenceType` | 目标的参照基准 | `0=Caster(施法者自身), 1=Target(当前目标), 2=Dest(目标位置)` |

## castingFlow

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **POST /api/cast** | `API` | 第一阶段：准备施法。瞬发法术直接完成；有读条的法术返回 preparing + castTimeMs | `` |
| **POST /api/cast/complete** | `API` | 第二阶段：完成施法。在客户端 castTimeMs 计时结束后调用 | `` |
| **POST /api/cast/cancel** | `API` | 取消正在读条的法术（如移动中断） | `` |
| **preparing** | `response.result` | CastTime > 0 时的返回状态，表示法术正在蓄力 | `` |
| **castTimeMs** | `int32` | 客户端需要等待的读条时间（毫秒） | `3500` |
| **success** | `response.result` | 施法成功，返回更新的单位状态和事件 | `` |
| **failed** | `response.result` | 施法失败，附带 error 字段说明原因 | `` |
| **cancelled** | `response.result` | 施法被取消 | `` |

## fireballExample

| Name | Type | Description | Values |
|------|------|-------------|--------|
| **Spell ID** | `uint32` | 暴雪官方 Fireball Rank 13 | `42833` |
| **CastTime** | `int32` | 读条 3.5 秒 | `3500` |
| **RecoveryTime** | `int32` | 无冷却 | `0` |
| **PowerCost** | `int32` | 消耗 400 法力（约 19% 基础法力） | `400` |
| **MaxTargets** | `int` | 单体目标 | `1` |
| **Effect[0] Type** | `SpellEffectType` | 火焰伤害 | `SchoolDamage` |
| **Effect[0] SchoolMask** | `SchoolMask` | 火焰系 | `Fire` |
| **Effect[0] BasePoints** | `int32` | 基础伤害 888 | `888` |
| **Effect[0] Coef** | `float64` | SP 系数 100% | `1` |
| **Effect[1] Type** | `SpellEffectType` | 施加光环 | `ApplyAura` |
| **Effect[1] AuraType** | `int32` | 减益（DoT） | `Debuff` |
| **Effect[1] AuraDuration** | `int32` | 持续 8 秒 | `8000` |
