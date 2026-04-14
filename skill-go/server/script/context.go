package script

import (
	"sync"

	"skill-go/server/trace"
)

// Handler is a type-erased callback function.
type Handler func(interface{})

// ScriptContext holds hooks and state for a spell or aura script instance.
type ScriptContext struct {
	hooks     map[string][]Handler
	mu        sync.Mutex
	prevented map[string]bool // which hooks have been prevented
	trace     *trace.Trace    // optional trace for logging
	spellID   uint32
	spellName string
}

// NewScriptContext creates an empty ScriptContext.
func NewScriptContext() *ScriptContext {
	return &ScriptContext{
		hooks:     make(map[string][]Handler),
		prevented: make(map[string]bool),
	}
}

// SetTrace sets the trace for this script context.
func (sc *ScriptContext) SetTrace(t *trace.Trace, spellID uint32, spellName string) {
	sc.trace = t
	sc.spellID = spellID
	sc.spellName = spellName
}

// Register adds a handler for a hook name.
func (sc *ScriptContext) Register(hookName string, handler Handler) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.hooks[hookName] = append(sc.hooks[hookName], handler)
}

// Fire invokes all handlers registered for a hook name.
// Returns true if any handler was called.
func (sc *ScriptContext) Fire(hookName string, arg interface{}) bool {
	sc.mu.Lock()
	handlers := sc.hooks[hookName]
	sc.mu.Unlock()

	if len(handlers) == 0 {
		return false
	}

	for _, h := range handlers {
		h(arg)
	}

	if sc.trace != nil {
		sc.trace.Event(trace.SpanScript, "hook_fired", sc.spellID, sc.spellName, map[string]interface{}{
			"hook":         hookName,
			"handlerCount": len(handlers),
		})
	}

	return true
}

// PreventDefault marks a hook as "prevented", signaling the system
// to skip its default behavior.
func (sc *ScriptContext) PreventDefault(hookName string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.prevented[hookName] = true
}

// IsPrevented checks if a hook's default behavior was prevented.
func (sc *ScriptContext) IsPrevented(hookName string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.prevented[hookName]
}

// ClearPrevented resets the prevented state (called between phases).
func (sc *ScriptContext) ClearPrevented() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.prevented = make(map[string]bool)
}

// HasHooks returns true if any hooks are registered.
func (sc *ScriptContext) HasHooks() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return len(sc.hooks) > 0
}
