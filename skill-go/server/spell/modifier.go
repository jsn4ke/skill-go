package spell

// CastTimeModifier modifies the base cast time of a spell.
type CastTimeModifier interface {
	Modify(baseMs int32) int32
	Type() string
}

// HasteModifier reduces cast time by a percentage.
// final = base / (1 + hastePercent/100)
type HasteModifier struct {
	HastePercent float64 // e.g., 50 = 50% haste
}

func (m HasteModifier) Modify(baseMs int32) int32 {
	if m.HastePercent <= 0 {
		return baseMs
	}
	result := float64(baseMs) / (1.0 + m.HastePercent/100.0)
	return int32(result)
}

func (m HasteModifier) Type() string { return "haste" }

// FlatModifier adds or subtracts a fixed amount of ms.
// final = base + flatMs
type FlatModifier struct {
	FlatMs int32 // positive = increase, negative = decrease
}

func (m FlatModifier) Modify(baseMs int32) int32 {
	return baseMs + m.FlatMs
}

func (m FlatModifier) Type() string { return "flat" }

// ModifierChain applies a sequence of modifiers in order.
type ModifierChain []CastTimeModifier

// Apply runs all modifiers in sequence.
func (chain ModifierChain) Apply(baseMs int32) int32 {
	result := baseMs
	for _, m := range chain {
		result = m.Modify(result)
	}
	// Minimum cast time of 0
	if result < 0 {
		result = 0
	}
	return result
}
