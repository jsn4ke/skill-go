package script

// --- SpellScript ---

// SpellSetupFunc is called when a spell script is loaded. It receives
// a SpellContext to register hooks on.
type SpellSetupFunc func(sc *SpellScript)

// SpellScript wraps a ScriptContext with spell-specific helper methods.
type SpellScript struct {
	*ScriptContext
}

// NewSpellScript creates a new SpellScript.
func NewSpellScript() *SpellScript {
	return &SpellScript{
		ScriptContext: NewScriptContext(),
	}
}

// OnCheckCast registers a handler for cast validation.
// Handler signature: func(info interface{}) where info is the SpellInfo.
func (ss *SpellScript) OnCheckCast(fn func(interface{})) {
	ss.Register(HookOnCheckCast, fn)
}

// OnCast registers a handler for when the spell is cast.
func (ss *SpellScript) OnCast(fn func(interface{})) {
	ss.Register(HookOnCast, fn)
}

// BeforeCast registers a handler before the spell casts.
func (ss *SpellScript) BeforeCast(fn func(interface{})) {
	ss.Register(HookBeforeCast, fn)
}

// AfterCast registers a handler after the spell completes.
func (ss *SpellScript) AfterCast(fn func(interface{})) {
	ss.Register(HookAfterCast, fn)
}

// OnLaunch registers a handler for the launch phase.
func (ss *SpellScript) OnLaunch(fn func(interface{})) {
	ss.Register(HookOnLaunch, fn)
}

// OnHit registers a handler for when the spell hits a target.
func (ss *SpellScript) OnHit(fn func(interface{})) {
	ss.Register(HookOnHit, fn)
}

// AfterHit registers a handler after the hit phase.
func (ss *SpellScript) AfterHit(fn func(interface{})) {
	ss.Register(HookAfterHit, fn)
}

// OnTargetSelect registers a handler for target selection interception.
func (ss *SpellScript) OnTargetSelect(fn func(interface{})) {
	ss.Register(HookOnTargetSelect, fn)
}

// OnChannelStart registers a handler for when channeling begins.
func (ss *SpellScript) OnChannelStart(fn func(interface{})) {
	ss.Register(HookOnChannelStart, fn)
}

// OnChannelTick registers a handler for each channel tick.
func (ss *SpellScript) OnChannelTick(fn func(interface{})) {
	ss.Register(HookOnChannelTick, fn)
}

// OnChannelEnd registers a handler for when channeling ends.
func (ss *SpellScript) OnChannelEnd(fn func(interface{})) {
	ss.Register(HookOnChannelEnd, fn)
}

// OnEffectLaunch registers a handler for effect launch phase.
func (ss *SpellScript) OnEffectLaunch(fn func(interface{})) {
	ss.Register(HookOnEffectLaunch, fn)
}

// OnEffectHit registers a handler for effect hit phase.
func (ss *SpellScript) OnEffectHit(fn func(interface{})) {
	ss.Register(HookOnEffectHit, fn)
}

// OnCalcDamage registers a handler for damage calculation.
func (ss *SpellScript) OnCalcDamage(fn func(interface{})) {
	ss.Register(HookOnCalcDamage, fn)
}

// OnCalcHeal registers a handler for heal calculation.
func (ss *SpellScript) OnCalcHeal(fn func(interface{})) {
	ss.Register(HookOnCalcHeal, fn)
}

// PreventHitEffect prevents the hit effect from executing.
func (ss *SpellScript) PreventHitEffect() {
	ss.PreventDefault(HookOnEffectHit)
}
