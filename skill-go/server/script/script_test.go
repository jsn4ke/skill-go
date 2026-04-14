package script

import (
	"testing"
)

func TestScriptContext_RegisterAndFire(t *testing.T) {
	tests := []struct {
		name    string
		hook    string
		arg     interface{}
		wantHit bool
		wantArg interface{}
	}{
		{
			name:    "handler receives argument on Fire",
			hook:    "hook1",
			arg:     42,
			wantHit: true,
			wantArg: 42,
		},
		{
			name:    "handler receives string argument",
			hook:    "hook2",
			arg:     "hello",
			wantHit: true,
			wantArg: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewScriptContext()

			var received interface{}
			sc.Register(tt.hook, func(arg interface{}) {
				received = arg
			})

			hit := sc.Fire(tt.hook, tt.arg)

			if hit != tt.wantHit {
				t.Errorf("Fire(%q) hit = %v, want %v", tt.hook, hit, tt.wantHit)
			}
			if received != tt.wantArg {
				t.Errorf("handler received arg = %v, want %v", received, tt.wantArg)
			}
		})
	}
}

func TestScriptContext_PreventDefault(t *testing.T) {
	sc := NewScriptContext()

	sc.Register(HookOnCheckCast, func(interface{}) {
		sc.PreventDefault(HookOnCheckCast)
	})

	sc.Fire(HookOnCheckCast, nil)

	if !sc.IsPrevented(HookOnCheckCast) {
		t.Error("IsPrevented(HookOnCheckCast) = false, want true after handler calls PreventDefault")
	}
}

func TestScriptContext_ClearPrevented(t *testing.T) {
	sc := NewScriptContext()

	sc.PreventDefault(HookOnCheckCast)
	if !sc.IsPrevented(HookOnCheckCast) {
		t.Fatal("expected IsPrevented to be true after PreventDefault")
	}

	sc.ClearPrevented()
	if sc.IsPrevented(HookOnCheckCast) {
		t.Error("IsPrevented(HookOnCheckCast) = true after ClearPrevented, want false")
	}
}

func TestScriptContext_MultipleHandlers(t *testing.T) {
	sc := NewScriptContext()

	var callCount int
	sc.Register("multiHook", func(interface{}) {
		callCount++
	})
	sc.Register("multiHook", func(interface{}) {
		callCount++
	})

	hit := sc.Fire("multiHook", nil)

	if !hit {
		t.Error("Fire(multiHook) = false, want true")
	}
	if callCount != 2 {
		t.Errorf("handler called %d times, want 2", callCount)
	}
}

func TestScriptContext_FireNonExistent(t *testing.T) {
	sc := NewScriptContext()

	// Must not panic and must return false.
	hit := sc.Fire("does_not_exist", nil)
	if hit {
		t.Error("Fire on non-existent hook = true, want false")
	}
}

func TestSpellScript_HookRegistration(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(ss *SpellScript)
		fireHook string
		wantHit  bool
	}{
		{
			name: "OnCheckCast registers and fires",
			setup: func(ss *SpellScript) {
				ss.OnCheckCast(func(interface{}) {})
			},
			fireHook: HookOnCheckCast,
			wantHit:  true,
		},
		{
			name: "OnHit registers and fires",
			setup: func(ss *SpellScript) {
				ss.OnHit(func(interface{}) {})
			},
			fireHook: HookOnHit,
			wantHit:  true,
		},
		{
			name: "OnCast registers and fires",
			setup: func(ss *SpellScript) {
				ss.OnCast(func(interface{}) {})
			},
			fireHook: HookOnCast,
			wantHit:  true,
		},
		{
			name: "OnLaunch registers and fires",
			setup: func(ss *SpellScript) {
				ss.OnLaunch(func(interface{}) {})
			},
			fireHook: HookOnLaunch,
			wantHit:  true,
		},
		{
			name: "AfterHit registers and fires",
			setup: func(ss *SpellScript) {
				ss.AfterHit(func(interface{}) {})
			},
			fireHook: HookAfterHit,
			wantHit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := NewSpellScript()
			tt.setup(ss)

			hit := ss.Fire(tt.fireHook, nil)
			if hit != tt.wantHit {
				t.Errorf("Fire(%q) = %v, want %v", tt.fireHook, hit, tt.wantHit)
			}
		})
	}
}

func TestAuraScript_HookRegistration(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(as *AuraScript)
		fireHook string
		wantHit  bool
	}{
		{
			name: "OnApply registers and fires",
			setup: func(as *AuraScript) {
				as.OnApply(func(interface{}) {})
			},
			fireHook: HookOnAuraApply,
			wantHit:  true,
		},
		{
			name: "OnRemove registers and fires",
			setup: func(as *AuraScript) {
				as.OnRemove(func(interface{}) {})
			},
			fireHook: HookOnAuraRemove,
			wantHit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as := NewAuraScript()
			tt.setup(as)

			hit := as.Fire(tt.fireHook, nil)
			if hit != tt.wantHit {
				t.Errorf("Fire(%q) = %v, want %v", tt.fireHook, hit, tt.wantHit)
			}
		})
	}
}

func TestPhaseGuard(t *testing.T) {
	tests := []struct {
		name               string
		phase              Phase
		wantTargets        bool
		wantModifyHit      bool
		wantPreventDefault bool
	}{
		{
			name:               "PhasePrepare cannot access targets",
			phase:              PhasePrepare,
			wantTargets:        false,
			wantModifyHit:      false,
			wantPreventDefault: true,
		},
		{
			name:               "PhaseCast can access targets but not modify hit",
			phase:              PhaseCast,
			wantTargets:        true,
			wantModifyHit:      false,
			wantPreventDefault: false,
		},
		{
			name:               "PhaseLaunch can access targets but not modify hit",
			phase:              PhaseLaunch,
			wantTargets:        true,
			wantModifyHit:      false,
			wantPreventDefault: true,
		},
		{
			name:               "PhaseHit can access targets and modify hit",
			phase:              PhaseHit,
			wantTargets:        true,
			wantModifyHit:      true,
			wantPreventDefault: true,
		},
		{
			name:               "PhaseChannel can access targets but not modify hit",
			phase:              PhaseChannel,
			wantTargets:        true,
			wantModifyHit:      false,
			wantPreventDefault: false,
		},
		{
			name:               "PhaseNone defaults to allowing target access",
			phase:              PhaseNone,
			wantTargets:        true,
			wantModifyHit:      false,
			wantPreventDefault: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := NewPhaseGuard(tt.phase)

			if got := pg.CanAccessTargets(); got != tt.wantTargets {
				t.Errorf("CanAccessTargets() = %v, want %v", got, tt.wantTargets)
			}
			if got := pg.CanModifyHit(); got != tt.wantModifyHit {
				t.Errorf("CanModifyHit() = %v, want %v", got, tt.wantModifyHit)
			}
			if got := pg.CanPreventDefault(); got != tt.wantPreventDefault {
				t.Errorf("CanPreventDefault() = %v, want %v", got, tt.wantPreventDefault)
			}
		})
	}
}

func TestPhaseGuard_SetPhase(t *testing.T) {
	pg := NewPhaseGuard(PhasePrepare)

	if pg.CanAccessTargets() {
		t.Error("PhasePrepare should not allow target access")
	}

	pg.SetPhase(PhaseHit)

	if !pg.CanAccessTargets() {
		t.Error("PhaseHit should allow target access")
	}
	if !pg.CanModifyHit() {
		t.Error("PhaseHit should allow modify hit")
	}
}

func TestRegistry_SpellScript(t *testing.T) {
	tests := []struct {
		name      string
		spellID   uint32
		register  bool
		wantFound bool
	}{
		{
			name:      "registered spell returns non-nil script",
			spellID:   100,
			register:  true,
			wantFound: true,
		},
		{
			name:      "unregistered spell returns nil",
			spellID:   999,
			register:  false,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()

			if tt.register {
				r.RegisterSpellScript(tt.spellID, func(ss *SpellScript) {
					ss.OnCheckCast(func(interface{}) {})
				})
			}

			ss := r.GetSpellScript(tt.spellID)

			if tt.wantFound && ss == nil {
				t.Errorf("GetSpellScript(%d) = nil, want non-nil", tt.spellID)
			}
			if !tt.wantFound && ss != nil {
				t.Errorf("GetSpellScript(%d) = non-nil, want nil", tt.spellID)
			}
		})
	}
}

func TestRegistry_AuraScript(t *testing.T) {
	r := NewRegistry()

	r.RegisterAuraScript(200, func(as *AuraScript) {
		as.OnApply(func(interface{}) {})
	})

	as := r.GetAuraScript(200)
	if as == nil {
		t.Error("GetAuraScript(200) = nil, want non-nil")
	}

	missing := r.GetAuraScript(888)
	if missing != nil {
		t.Error("GetAuraScript(888) = non-nil, want nil")
	}
}

func TestRegistry_SpellScriptHooksWork(t *testing.T) {
	r := NewRegistry()

	var called bool
	r.RegisterSpellScript(100, func(ss *SpellScript) {
		ss.OnHit(func(interface{}) {
			called = true
		})
	})

	ss := r.GetSpellScript(100)
	if ss == nil {
		t.Fatal("GetSpellScript(100) returned nil")
	}

	ss.Fire(HookOnHit, nil)
	if !called {
		t.Error("OnHit handler was not called through registry-retrieved script")
	}
}
