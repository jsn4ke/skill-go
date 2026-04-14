package script

// ScriptHook names for SpellScript.
const (
	HookOnCheckCast      = "OnCheckCast"
	HookBeforeCast       = "BeforeCast"
	HookOnCast           = "OnCast"
	HookAfterCast        = "AfterCast"
	HookOnLaunch         = "OnLaunch"
	HookOnHit            = "OnHit"
	HookAfterHit         = "AfterHit"
	HookOnTargetSelect   = "OnTargetSelect"
	HookOnChannelStart   = "OnChannelStart"
	HookOnChannelTick    = "OnChannelTick"
	HookOnChannelEnd     = "OnChannelEnd"
	HookOnEmpowerStart   = "OnEmpowerStart"
	HookOnEmpowerEnd     = "OnEmpowerEnd"
	HookOnEffectLaunch   = "OnEffectLaunch"
	HookOnEffectHit      = "OnEffectHit"
	HookOnCalcDamage     = "OnCalcDamage"
	HookOnCalcHeal       = "OnCalcHeal"
	HookPreventHitEffect = "PreventHitEffect"
)

// ScriptHook names for AuraScript.
const (
	HookOnAuraApply        = "OnApply"
	HookOnAuraRemove       = "OnRemove"
	HookOnAuraPeriodicTick = "OnPeriodicTick"
	HookOnAuraAbsorb       = "OnAbsorb"
	HookOnAuraProc         = "OnProc"
	HookOnAuraDispel       = "OnDispel"
	HookAfterAuraProc      = "AfterProc"
	HookOnAuraCheckProc    = "OnCheckProc"
	HookOnAuraPrepareProc  = "OnPrepareProc"
)

// AllSpellHooks lists all spell hook names.
var AllSpellHooks = []string{
	HookOnCheckCast, HookBeforeCast, HookOnCast, HookAfterCast,
	HookOnLaunch, HookOnHit, HookAfterHit, HookOnTargetSelect,
	HookOnChannelStart, HookOnChannelTick, HookOnChannelEnd,
	HookOnEmpowerStart, HookOnEmpowerEnd,
	HookOnEffectLaunch, HookOnEffectHit,
	HookOnCalcDamage, HookOnCalcHeal,
	HookPreventHitEffect,
}

// AllAuraHooks lists all aura hook names.
var AllAuraHooks = []string{
	HookOnAuraApply, HookOnAuraRemove, HookOnAuraPeriodicTick,
	HookOnAuraAbsorb, HookOnAuraProc, HookOnAuraDispel, HookAfterAuraProc,
	HookOnAuraCheckProc, HookOnAuraPrepareProc,
}
