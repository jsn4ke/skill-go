package api

import (
	"net/http"
)

// ---------------------------------------------------------------------------
// /api/docs — spell system configuration reference
// ---------------------------------------------------------------------------

// docSection represents one section of the config reference.
type docSection struct {
	Title  string     `json:"title"`
	Fields []docField `json:"fields"`
}

// docField documents a single configuration field.
type docField struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Values      interface{} `json:"values,omitempty"`
}

func handleDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	sections := []docSection{
		{
			Title: "schoolMask",
			Fields: []docField{
				{Name: "SchoolMaskNone", Type: "uint32 (bitfield)", Description: "无属性（值为 0）", Values: 0},
				{Name: "SchoolMaskFire", Type: "uint32 (bitfield)", Description: "火焰系法术", Values: 1},
				{Name: "SchoolMaskFrost", Type: "uint32 (bitfield)", Description: "冰霜系法术", Values: 2},
				{Name: "SchoolMaskArcane", Type: "uint32 (bitfield)", Description: "奥术系法术", Values: 4},
				{Name: "SchoolMaskNature", Type: "uint32 (bitfield)", Description: "自然系法术", Values: 8},
				{Name: "SchoolMaskShadow", Type: "uint32 (bitfield)", Description: "暗影系法术", Values: 16},
				{Name: "SchoolMaskHoly", Type: "uint32 (bitfield)", Description: "神圣系法术", Values: 32},
				{Name: "SchoolMaskPhysical", Type: "uint32 (bitfield)", Description: "物理攻击", Values: 64},
			},
		},
		{
			Title: "combatResult",
			Fields: []docField{
				{Name: "CombatResultHit", Type: "int (enum)", Description: "普通命中", Values: 0},
				{Name: "CombatResultMiss", Type: "int (enum)", Description: "未命中（攻击落空）", Values: 1},
				{Name: "CombatResultCrit", Type: "int (enum)", Description: "暴击（伤害 x2）", Values: 2},
				{Name: "CombatResultDodge", Type: "int (enum)", Description: "闪避（目标躲开攻击）", Values: 3},
				{Name: "CombatResultParry", Type: "int (enum)", Description: "招架（目标格挡并减免）", Values: 4},
				{Name: "CombatResultBlock", Type: "int (enum)", Description: "格挡（通过盾牌减伤）", Values: 5},
				{Name: "CombatResultGlancing", Type: "int (enum)", Description: "擦过（PvE 中对高等级目标的减伤）", Values: 6},
				{Name: "CombatResultResist", Type: "int (enum)", Description: "部分抵抗（法术伤害减免）", Values: 7},
				{Name: "CombatResultFullResist", Type: "int (enum)", Description: "完全抵抗（法术伤害归零）", Values: 8},
			},
		},
		{
			Title: "castResult",
			Fields: []docField{
				{Name: "CastResultSuccess", Type: "int (enum)", Description: "施法成功", Values: 0},
				{Name: "CastResultFailed", Type: "int (enum)", Description: "施法失败（被各种条件阻止）", Values: 1},
				{Name: "CastResultInterrupted", Type: "int (enum)", Description: "施法被打断", Values: 2},
			},
		},
		{
			Title: "castError",
			Fields: []docField{
				{Name: "CastErrNone", Type: "int (enum)", Description: "无错误", Values: 0},
				{Name: "CastErrNotReady", Type: "int (enum)", Description: "技能冷却中（CD 未结束）", Values: 1},
				{Name: "CastErrOutOfRange", Type: "int (enum)", Description: "目标超出射程", Values: 2},
				{Name: "CastErrSilenced", Type: "int (enum)", Description: "被沉默（无法施法）", Values: 3},
				{Name: "CastErrDisarmed", Type: "int (enum)", Description: "被缴械（无法武器攻击）", Values: 4},
				{Name: "CastErrShapeshifted", Type: "int (enum)", Description: "形态不正确（需要特定变形形态）", Values: 5},
				{Name: "CastErrNoItems", Type: "int (enum)", Description: "缺少所需物品（如施法材料）", Values: 6},
				{Name: "CastErrWrongArea", Type: "int (enum)", Description: "当前区域不允许施放", Values: 7},
				{Name: "CastErrMounted", Type: "int (enum)", Description: "骑乘状态无法施法", Values: 8},
				{Name: "CastErrNoMana", Type: "int (enum)", Description: "法力值不足", Values: 9},
				{Name: "CastErrDead", Type: "int (enum)", Description: "施法者已死亡", Values: 10},
				{Name: "CastErrTargetDead", Type: "int (enum)", Description: "目标已死亡", Values: 11},
				{Name: "CastErrSchoolLocked", Type: "int (enum)", Description: "该法术系被锁定（如法术反制）", Values: 12},
				{Name: "CastErrNoCharges", Type: "int (enum)", Description: "充能次数已用完", Values: 13},
				{Name: "CastErrOnGCD", Type: "int (enum)", Description: "公共冷却中", Values: 14},
				{Name: "CastErrInterrupted", Type: "int (enum)", Description: "读条被打断", Values: 15},
			},
		},
		{
			Title: "spellInfo",
			Fields: []docField{
				{Name: "ID", Type: "uint32", Description: "法术唯一标识（自动分配）", Values: "42833"},
				{Name: "Name", Type: "string", Description: "法术名称", Values: "Fireball"},
				{Name: "SchoolMask", Type: "SchoolMask", Description: "法术所属魔法学派（位掩码）", Values: "见 schoolMask 节"},
				{Name: "RecoveryTime", Type: "int32", Description: "冷却时间（毫秒），0 = 无冷却", Values: "0, 6000, 120000"},
				{Name: "CategoryRecoveryTime", Type: "int32", Description: "分类冷却时间（毫秒），同分类技能共享 CD", Values: "0"},
				{Name: "CastTime", Type: "int32", Description: "基础施法时间（毫秒），0 = 瞬发", Values: "0, 1500, 3500"},
				{Name: "RangeMin", Type: "float64", Description: "最小射程", Values: "0"},
				{Name: "RangeMax", Type: "float64", Description: "最大射程", Values: "0, 30, 40"},
				{Name: "MaxTargets", Type: "int", Description: "最大目标数量", Values: "1, 3, 0(无限)"},
				{Name: "PowerCost", Type: "int32", Description: "法力消耗", Values: "0, 400"},
				{Name: "Effects", Type: "[]SpellEffectInfo", Description: "法术效果列表（最多多个效果）", Values: "见 spellEffectInfo 节"},
				{Name: "IsAutoRepeat", Type: "bool", Description: "是否为自动重复施法（如自动射击）", Values: "false"},
				{Name: "PreventionType", Type: "PreventionType", Description: "可被何种方式打断（沉默/安抚）", Values: "0=None, 1=Silence, 2=Pacify"},
				{Name: "MissileSpeed", Type: "float64", Description: "弹道飞行速度（码/秒，0=无弹道即时命中）", Values: "0, 14"},
				{Name: "IsChanneled", Type: "bool", Description: "是否为引导法术（如暴风雪）", Values: "false"},
				{Name: "ChannelDuration", Type: "int32", Description: "引导总时长（毫秒）", Values: "8000"},
				{Name: "TickInterval", Type: "int32", Description: "引导期间每次跳动的间隔（毫秒）", Values: "1000, 2000"},
				{Name: "IsEmpower", Type: "bool", Description: "是否为蓄力法术（Dragonflight 机制）", Values: "false"},
				{Name: "EmpowerStages", Type: "[]int32", Description: "蓄力各阶段的阈值（毫秒）", Values: "[1000, 2000, 3000]"},
				{Name: "EmpowerMinTime", Type: "int32", Description: "最低蓄力时间（毫秒）", Values: "500"},
				{Name: "RequiresShapeshiftMask", Type: "uint32", Description: "需要特定变形形态（位掩码）", Values: "0"},
				{Name: "RequiredAuraState", Type: "uint32", Description: "需要施法者身上有特定光环状态", Values: "0"},
				{Name: "RequiredAreaID", Type: "int32", Description: "限定区域 ID", Values: "0"},
				{Name: "MaxCharges", Type: "int32", Description: "充能次数上限，>0 表示充能技能", Values: "0, 2, 3"},
				{Name: "ChargeRecoveryTime", Type: "int32", Description: "恢复一次充能的时间（毫秒）", Values: "0, 15000"},
				{Name: "RecoveryCategory", Type: "int32", Description: "冷却分类，同分类共享 CD", Values: "0"},
				{Name: "Reflectable", Type: "bool", Description: "是否可被法术反射", Values: "false"},
			},
		},
		{
			Title: "spellEffectInfo",
			Fields: []docField{
				{Name: "EffectIndex", Type: "int", Description: "效果序号（从 0 开始）", Values: "0, 1, 2"},
				{Name: "EffectType", Type: "SpellEffectType", Description: "效果类型", Values: "见 effectType 节"},
				{Name: "SchoolMask", Type: "SchoolMask", Description: "该效果使用的魔法学派", Values: "见 schoolMask 节"},
				{Name: "BasePoints", Type: "int32", Description: "基础数值（伤害/治疗量）", Values: "888"},
				{Name: "Coef", Type: "float64", Description: "法术强度系数（SP 加成倍率）", Values: "0.0, 0.5, 1.0, 1.428"},
				{Name: "TargetA", Type: "TargetReference", Description: "目标选择器 A（主目标）", Values: "见 targetReference 节"},
				{Name: "TargetB", Type: "TargetReference", Description: "目标选择器 B（次目标/区域）", Values: "见 targetReference 节"},
				{Name: "TriggerSpellID", Type: "uint32", Description: "触发的法术 ID（EffectType=TriggerSpell 时使用）", Values: "0"},
				{Name: "EnergizeType", Type: "PowerType", Description: "恢复的资源类型（EffectType=Energize 时使用）", Values: "0=Mana, 1=Rage, 2=Energy"},
				{Name: "EnergizeAmount", Type: "int32", Description: "恢复的资源量", Values: "100"},
				{Name: "WeaponPercent", Type: "float64", Description: "武器伤害百分比（EffectType=WeaponDamage 时使用）", Values: "1.0, 2.0"},
				{Name: "AuraType", Type: "int32", Description: "光环类型（EffectType=ApplyAura 时使用）", Values: "见 auraType 节"},
				{Name: "AuraDuration", Type: "int32", Description: "光环持续时间（毫秒），0 = 永久", Values: "0, 8000, 30000"},
			},
		},
		{
			Title: "effectType",
			Fields: []docField{
				{Name: "SpellEffectNone", Type: "int (enum)", Description: "无效果（占位）", Values: 0},
				{Name: "SpellEffectSchoolDamage", Type: "int (enum)", Description: "指定学派的伤害（如火焰伤害、冰霜伤害）", Values: 1},
				{Name: "SpellEffectHeal", Type: "int (enum)", Description: "治疗目标", Values: 2},
				{Name: "SpellEffectApplyAura", Type: "int (enum)", Description: "为目标施加光环（Buff/Debuff）", Values: 3},
				{Name: "SpellEffectTriggerSpell", Type: "int (enum)", Description: "触发另一个法术（用于多段效果）", Values: 4},
				{Name: "SpellEffectEnergize", Type: "int (enum)", Description: "恢复资源（法力/怒气/能量）", Values: 5},
				{Name: "SpellEffectWeaponDamage", Type: "int (enum)", Description: "基于武器伤害的攻击", Values: 6},
			},
		},
		{
			Title: "auraType",
			Fields: []docField{
				{Name: "AuraTypeBuff", Type: "int (enum)", Description: "增益光环（有益效果，如力量祝福）", Values: 0},
				{Name: "AuraTypeDebuff", Type: "int (enum)", Description: "减益光环（有害效果，如燃烧 DoT）", Values: 1},
				{Name: "AuraTypePassive", Type: "int (enum)", Description: "被动光环（始终生效，如光环效果）", Values: 2},
				{Name: "AuraTypeProc", Type: "int (enum)", Description: "触发型光环（满足条件时触发效果）", Values: 3},
			},
		},
		{
			Title: "aura",
			Fields: []docField{
				{Name: "SpellID", Type: "uint32", Description: "关联的法术 ID", Values: "42833"},
				{Name: "SourceName", Type: "string", Description: "创建此光环的法术名称", Values: "Fireball"},
				{Name: "CasterGUID", Type: "uint64", Description: "施法者的 GUID", Values: "1"},
				{Name: "AuraType", Type: "AuraType", Description: "光环分类（Buff/Debuff/Passive/Proc）", Values: "见 auraType 节"},
				{Name: "MaxCharges", Type: "int32", Description: "最大充能次数（0 = 不限）", Values: "0"},
				{Name: "Charges", Type: "int32", Description: "当前剩余充能次数", Values: "0"},
				{Name: "Duration", Type: "int32", Description: "持续时间（毫秒），0 = 永久", Values: "0, 8000, 300000"},
				{Name: "StackAmount", Type: "int32", Description: "当前层数", Values: "1, 2, 5"},
				{Name: "MaxStack", Type: "int32", Description: "最大可叠层数", Values: "1, 3, 5"},
				{Name: "ProcChance", Type: "float64", Description: "触发概率（0-100），0 = 禁用", Values: "0, 50, 100"},
				{Name: "PPM", Type: "float64", Description: "每分钟触发次数（Procs Per Minute），0 = 不限制", Values: "0, 2.5, 5"},
				{Name: "ProcCharges", Type: "int32", Description: "最大触发次数后光环消失，0 = 不限", Values: "0, 1, 3"},
				{Name: "RemainingProcs", Type: "int32", Description: "剩余可触发次数", Values: "0"},
				{Name: "Effects", Type: "[]*AuraEffect", Description: "光环效果列表", Values: "见 auraEffect 节"},
				{Name: "Applications", Type: "[]*AuraApplication", Description: "光环应用记录（追踪每个目标的施加时间）", Values: "见 auraApplication 节"},
			},
		},
		{
			Title: "auraEffect",
			Fields: []docField{
				{Name: "AuraType", Type: "AuraType", Description: "效果的光环类型", Values: "见 auraType 节"},
				{Name: "SpellID", Type: "uint32", Description: "关联法术 ID", Values: "42833"},
				{Name: "EffectIndex", Type: "int", Description: "对应 SpellEffectInfo 的序号", Values: "0, 1"},
				{Name: "AuraName", Type: "string", Description: "效果名称", Values: "DoT"},
				{Name: "BaseAmount", Type: "int32", Description: "基础数值（伤害/治疗/属性加成）", Values: "29"},
				{Name: "MiscValue", Type: "int32", Description: "附加参数（用途随效果类型变化）", Values: "0"},
				{Name: "PeriodicTimer", Type: "int32", Description: "周期触发间隔（毫秒），0 = 非周期性", Values: "0, 2000, 3000"},
			},
		},
		{
			Title: "auraApplication",
			Fields: []docField{
				{Name: "Target", Type: "*unit.Unit", Description: "光环施加的目标单位", Values: "目标 Unit 指针"},
				{Name: "BaseAmount", Type: "int32", Description: "施加时记录的基础数值", Values: "29"},
				{Name: "RemoveMode", Type: "RemoveMode", Description: "光环移除原因（0=默认, 1=过期, 2=取消, 3=驱散, 4=死亡）", Values: "0, 1, 2, 3, 4"},
				{Name: "NeedClientUpdate", Type: "bool", Description: "是否需要向客户端同步更新", Values: "false, true"},
				{Name: "TimerStart", Type: "int64", Description: "施加时间戳（Unix 毫秒），用于计算剩余时间", Values: "1712345678900"},
			},
		},
		{
			Title: "cooldown",
			Fields: []docField{
				{Name: "RecoveryTime", Type: "int32", Description: "法术冷却时间（毫秒），施法后触发", Values: "0, 6000"},
				{Name: "CategoryRecoveryTime", Type: "int32", Description: "分类共享 CD（毫秒），同 RecoveryCategory 的技能共享", Values: "0, 1500"},
				{Name: "RecoveryCategory", Type: "int32", Description: "冷却分类 ID，0 = 无分类", Values: "0, 133"},
				{Name: "MaxCharges", Type: "int32", Description: "充能上限，>0 启用充能机制", Values: "0, 2, 3"},
				{Name: "ChargeRecoveryTime", Type: "int32", Description: "每次充能恢复时间（毫秒）", Values: "0, 15000, 40000"},
				{Name: "GCD", Type: "int32", Description: "公共冷却时间（毫秒），默认 1500ms", Values: "1500"},
				{Name: "SchoolLockout", Type: "int32", Description: "学派锁定时间（毫秒），被法术反制后锁定整个学派", Values: "4000, 6000"},
				{Name: "IsReady()", Type: "func", Description: "检查法术是否可用（CD/充能/学派锁定全部通过）", Values: "bool"},
			},
		},
		{
			Title: "targetReference",
			Fields: []docField{
				{Name: "TargetType (Type)", Type: "TargetType", Description: "目标对象类型", Values: "0=Unit, 1=GameObject, 2=Item, 3=Corpse"},
				{Name: "TargetReferenceType (Reference)", Type: "TargetReferenceType", Description: "目标的参照基准", Values: "0=Caster(施法者自身), 1=Target(当前目标), 2=Dest(目标位置)"},
			},
		},
		{
			Title: "castingFlow",
			Fields: []docField{
				{Name: "POST /api/cast", Type: "API", Description: "第一阶段：准备施法。瞬发法术直接完成；有读条的法术返回 preparing + castTimeMs", Values: ""},
				{Name: "POST /api/cast/complete", Type: "API", Description: "第二阶段：完成施法。在客户端 castTimeMs 计时结束后调用", Values: ""},
				{Name: "POST /api/cast/cancel", Type: "API", Description: "取消正在读条的法术（如移动中断）", Values: ""},
				{Name: "preparing", Type: "response.result", Description: "CastTime > 0 时的返回状态，表示法术正在蓄力", Values: ""},
				{Name: "castTimeMs", Type: "int32", Description: "客户端需要等待的读条时间（毫秒）", Values: "3500"},
				{Name: "success", Type: "response.result", Description: "施法成功，返回更新的单位状态和事件", Values: ""},
				{Name: "failed", Type: "response.result", Description: "施法失败，附带 error 字段说明原因", Values: ""},
				{Name: "cancelled", Type: "response.result", Description: "施法被取消", Values: ""},
			},
		},
		{
			Title: "fireballExample",
			Fields: []docField{
				{Name: "Spell ID", Type: "uint32", Description: "暴雪官方 Fireball Rank 13", Values: 42833},
				{Name: "CastTime", Type: "int32", Description: "读条 3.5 秒", Values: 3500},
				{Name: "RecoveryTime", Type: "int32", Description: "无冷却", Values: 0},
				{Name: "PowerCost", Type: "int32", Description: "消耗 400 法力（约 19% 基础法力）", Values: 400},
				{Name: "MaxTargets", Type: "int", Description: "单体目标", Values: 1},
				{Name: "Effect[0] Type", Type: "SpellEffectType", Description: "火焰伤害", Values: "SchoolDamage"},
				{Name: "Effect[0] SchoolMask", Type: "SchoolMask", Description: "火焰系", Values: "Fire"},
				{Name: "Effect[0] BasePoints", Type: "int32", Description: "基础伤害 888", Values: 888},
				{Name: "Effect[0] Coef", Type: "float64", Description: "SP 系数 100%", Values: 1.0},
				{Name: "Effect[1] Type", Type: "SpellEffectType", Description: "施加光环", Values: "ApplyAura"},
				{Name: "Effect[1] AuraType", Type: "int32", Description: "减益（DoT）", Values: "Debuff"},
				{Name: "Effect[1] AuraDuration", Type: "int32", Description: "持续 8 秒", Values: 8000},
			},
		},
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"title":    "skill-go Spell System Configuration Reference",
		"version":  "1.0",
		"sections": sections,
	})
}
