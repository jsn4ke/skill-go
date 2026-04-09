package targeting

import (
	"math"

	"skill-go/server/trace"
	"skill-go/server/unit"
)

// UnitProvider is an interface for getting all units in the world/scene.
// In a real server this would query the map/scene manager.
type UnitProvider interface {
	GetAllUnits() []*unit.Unit
}

// Select executes the full target selection pipeline:
// algorithm → validation filter → script interception → final result.
func Select(ctx *SelectionContext, world UnitProvider, t *trace.Trace, spellID uint32, spellName string) []*unit.Unit {
	var candidates []*unit.Unit

	switch ctx.Descriptor.Category {
	case SelectSelf:
		candidates = []*unit.Unit{ctx.Caster}

	case SelectSingle:
		if len(ctx.ExplicitTargets) > 0 {
			candidates = []*unit.Unit{ctx.ExplicitTargets[0]}
		}

	case SelectFriendly:
		candidates = selectNearby(ctx, world, func(u *unit.Unit) bool {
			return u.GUID != ctx.Caster.GUID
		})

	case SelectEnemy:
		candidates = selectNearby(ctx, world, func(u *unit.Unit) bool {
			return u.GUID != ctx.Caster.GUID
		})

	case SelectArea:
		candidates = selectArea(ctx, world)

	case SelectCone:
		candidates = selectCone(ctx, world)

	case SelectChain:
		candidates = selectChain(ctx, world)
	case SelectLine:
		candidates = selectLine(ctx, world)

	case SelectTrajectory:
		candidates = selectTrajectory(ctx, world)
	}

	// Apply validation filter
	candidates = applyValidation(candidates, ctx.Descriptor.Validation)

	// Apply script interception (FilterUnit)
	if filters, ok := ctx.Filters[FilterUnit]; ok {
		for _, f := range filters {
			candidates = f(candidates)
		}
	}

	if t != nil {
		t.Event(trace.SpanTargeting, "selected", spellID, spellName, map[string]interface{}{
			"category":    ctx.Descriptor.Category.String(),
			"count":       len(candidates),
		})
	}

	return candidates
}

// --- Selection algorithms ---

// selectNearby selects units within radius of the reference point.
func selectNearby(ctx *SelectionContext, world UnitProvider, extraFilter func(*unit.Unit) bool) []*unit.Unit {
	origin := getOrigin(ctx)
	radius := ctx.Descriptor.Dir.Radius
	if radius <= 0 {
		radius = 100
	}

	var result []*unit.Unit
	for _, u := range world.GetAllUnits() {
		if u.GUID == ctx.Caster.GUID {
			continue
		}
		if extraFilter != nil && !extraFilter(u) {
			continue
		}
		if distance(origin, u.Position) <= radius {
			result = append(result, u)
		}
	}
	return result
}

// selectArea selects all units within a circle.
func selectArea(ctx *SelectionContext, world UnitProvider) []*unit.Unit {
	origin := getOrigin(ctx)
	radius := ctx.Descriptor.Dir.Radius
	if radius <= 0 {
		radius = 10
	}

	var result []*unit.Unit
	for _, u := range world.GetAllUnits() {
		if distance(origin, u.Position) <= radius {
			result = append(result, u)
		}
	}

	// Apply area filters
	if filters, ok := ctx.Filters[FilterArea]; ok {
		for _, f := range filters {
			result = f(result)
		}
	}

	return result
}

// selectCone selects units within a cone in front of the caster.
func selectCone(ctx *SelectionContext, world UnitProvider) []*unit.Unit {
	origin := getOrigin(ctx)
	radius := ctx.Descriptor.Dir.Radius
	if radius <= 0 {
		radius = 15
	}
	halfAngle := ctx.Descriptor.Dir.ConeAngle
	if halfAngle <= 0 {
		halfAngle = math.Pi / 4 // 45 degrees default
	}

	// Assume caster faces along +Y axis for simplicity
	facingX := 0.0
	facingY := 1.0

	var result []*unit.Unit
	for _, u := range world.GetAllUnits() {
		dist := distance(origin, u.Position)
		if dist > radius || dist == 0 {
			continue
		}

		dx := u.Position.X - origin.X
		dy := u.Position.Y - origin.Y
		angle := math.Abs(math.Atan2(dx*facingY-dy*facingX, dx*facingX+dy*facingY))

		if angle <= halfAngle {
			result = append(result, u)
		}
	}

	return result
}

// selectChain bounces between nearby targets (e.g., Chain Lightning).
func selectChain(ctx *SelectionContext, world UnitProvider) []*unit.Unit {
	if len(ctx.ExplicitTargets) == 0 {
		return nil
	}

	maxBounces := 5
	if ctx.Descriptor.Validation.MaxTargets > 0 {
		maxBounces = ctx.Descriptor.Validation.MaxTargets
	}
	bounceRange := ctx.Descriptor.Dir.Radius
	if bounceRange <= 0 {
		bounceRange = 10
	}

	// Start with explicit target
	result := []*unit.Unit{ctx.ExplicitTargets[0]}
	visited := make(map[uint64]bool)
	visited[ctx.ExplicitTargets[0].GUID] = true
	visited[ctx.Caster.GUID] = true

	allUnits := world.GetAllUnits()

	for i := 1; i < maxBounces; i++ {
		lastTarget := result[len(result)-1]
		var nearest *unit.Unit
		nearestDist := bounceRange

		// Prefer most damaged target
		for _, u := range allUnits {
			if visited[u.GUID] {
				continue
			}
			dist := lastTarget.DistanceTo(u)
			if dist <= nearestDist {
				// Prefer lower health targets (chain prefers wounded)
				if nearest == nil || u.Health < nearest.Health {
					nearest = u
					nearestDist = dist
				}
			}
		}

		if nearest == nil {
			break
		}

		visited[nearest.GUID] = true
		result = append(result, nearest)
	}

	return result
}

// selectLine selects units in a rectangular area in front of the caster.
func selectLine(ctx *SelectionContext, world UnitProvider) []*unit.Unit {
	origin := getOrigin(ctx)
	length := ctx.Descriptor.Dir.Length
	if length <= 0 {
		length = 20
	}
	halfWidth := ctx.Descriptor.Dir.Width / 2.0
	if halfWidth <= 0 {
		halfWidth = 3
	}

	// Assume facing +Y
	facingX, facingY := 0.0, 1.0
	normalX, normalY := -facingY, facingX

	var result []*unit.Unit
	for _, u := range world.GetAllUnits() {
		if u.GUID == ctx.Caster.GUID {
			continue
		}
		dx := u.Position.X - origin.X
		dy := u.Position.Y - origin.Y
		along := dx*facingX + dy*facingY
		perp := dx*normalX + dy*normalY

		if along >= 0 && along <= length && perp >= -halfWidth && perp <= halfWidth {
			result = append(result, u)
		}
	}
	return result
}

// selectTrajectory selects units near the line between start and end points.
func selectTrajectory(ctx *SelectionContext, world UnitProvider) []*unit.Unit {
	var start, end unit.Position
	switch ctx.Descriptor.Reference {
	case RefTarget:
		start = ctx.Caster.Position
		if len(ctx.ExplicitTargets) > 0 {
			end = ctx.ExplicitTargets[0].Position
		} else {
			end = start
		}
	case RefPosition:
		start = ctx.Caster.Position
		end = ctx.OriginPos
	default:
		start = ctx.Caster.Position
		if len(ctx.ExplicitTargets) > 0 {
			end = ctx.ExplicitTargets[0].Position
		} else {
			end = start
		}
	}

	halfWidth := ctx.Descriptor.Dir.Width / 2.0
	if halfWidth <= 0 {
		halfWidth = 3
	}

	lineLen := distance(start, end)
	if lineLen == 0 {
		return nil
	}

	dirX := (end.X - start.X) / lineLen
	dirY := (end.Y - start.Y) / lineLen
	normalX, normalY := -dirY, dirX

	var result []*unit.Unit
	for _, u := range world.GetAllUnits() {
		if u.GUID == ctx.Caster.GUID {
			continue
		}
		dx := u.Position.X - start.X
		dy := u.Position.Y - start.Y
		along := dx*dirX + dy*dirY
		perp := dx*normalX + dy*normalY

		if along >= 0 && along <= lineLen && perp >= -halfWidth && perp <= halfWidth {
			result = append(result, u)
		}
	}
	return result
}

// --- Helpers ---

func getOrigin(ctx *SelectionContext) unit.Position {
	switch ctx.Descriptor.Reference {
	case RefTarget:
		if len(ctx.ExplicitTargets) > 0 {
			return ctx.ExplicitTargets[0].Position
		}
		return ctx.Caster.Position
	case RefPosition:
		return ctx.OriginPos
	default:
		return ctx.Caster.Position
	}
}

func distance(a, b unit.Position) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return sqrt(dx*dx + dy*dy + dz*dz)
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := 1.0
	for i := 0; i < 20; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

func applyValidation(targets []*unit.Unit, rule ValidationRule) []*unit.Unit {
	var result []*unit.Unit
	for _, u := range targets {
		if rule.AliveOnly && !u.IsAlive() {
			continue
		}
		if rule.DeadOnly && u.IsAlive() {
			continue
		}
		passed := true
		for _, cond := range rule.Conditions {
			if !cond(u) {
				passed = false
				break
			}
		}
		if passed {
			result = append(result, u)
		}
	}

	if rule.MaxTargets > 0 && len(result) > rule.MaxTargets {
		result = result[:rule.MaxTargets]
	}

	return result
}
