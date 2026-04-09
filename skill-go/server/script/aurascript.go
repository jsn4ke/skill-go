package script

// --- AuraScript ---

// AuraSetupFunc is called when an aura script is loaded.
type AuraSetupFunc func(as *AuraScript)

// AuraScript wraps a ScriptContext with aura-specific helper methods.
type AuraScript struct {
	*ScriptContext
}

// NewAuraScript creates a new AuraScript.
func NewAuraScript() *AuraScript {
	return &AuraScript{
		ScriptContext: NewScriptContext(),
	}
}

// OnApply registers a handler for when the aura is applied.
func (as *AuraScript) OnApply(fn func(interface{})) {
	as.Register(HookOnAuraApply, fn)
}

// OnRemove registers a handler for when the aura is removed.
func (as *AuraScript) OnRemove(fn func(interface{})) {
	as.Register(HookOnAuraRemove, fn)
}

// OnPeriodicTick registers a handler for each periodic tick.
func (as *AuraScript) OnPeriodicTick(fn func(interface{})) {
	as.Register(HookOnAuraPeriodicTick, fn)
}

// OnAbsorb registers a handler for absorb events.
func (as *AuraScript) OnAbsorb(fn func(interface{})) {
	as.Register(HookOnAuraAbsorb, fn)
}

// OnProc registers a handler for proc events.
func (as *AuraScript) OnProc(fn func(interface{})) {
	as.Register(HookOnAuraProc, fn)
}

// AfterProc registers a handler after a proc completes.
func (as *AuraScript) AfterProc(fn func(interface{})) {
	as.Register(HookAfterAuraProc, fn)
}

// OnDispel registers a handler for dispel events.
func (as *AuraScript) OnDispel(fn func(interface{})) {
	as.Register(HookOnAuraDispel, fn)
}

// OnCheckProc registers a handler for proc eligibility check.
func (as *AuraScript) OnCheckProc(fn func(interface{})) {
	as.Register(HookOnAuraCheckProc, fn)
}

// PreventDefaultAction prevents the aura's default action.
func (as *AuraScript) PreventDefaultAction() {
	as.PreventDefault(HookOnAuraApply)
}
