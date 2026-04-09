package script

import (
	"log"
	"sync"
)

// Handler is a type-erased callback function.
type Handler func(interface{})

// ScriptContext holds hooks and state for a spell or aura script instance.
type ScriptContext struct {
	hooks    map[string][]Handler
	mu       sync.Mutex
	prevented map[string]bool // which hooks have been prevented
}

// NewScriptContext creates an empty ScriptContext.
func NewScriptContext() *ScriptContext {
	return &ScriptContext{
		hooks:     make(map[string][]Handler),
		prevented: make(map[string]bool),
	}
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

	log.Printf("[Script] fired hook %s (%d handlers)", hookName, len(handlers))
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
