package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"skill-go/server/aura"
	"skill-go/server/cooldown"
	"skill-go/server/effect"
	"skill-go/server/spell"
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// ---------------------------------------------------------------------------
// Data types for JSON serialization
// ---------------------------------------------------------------------------

// CastRequest is the JSON body for POST /api/cast.
type CastRequest struct {
	CasterGUID uint64   `json:"casterGuid"`
	SpellID    uint32   `json:"spellID"`
	TargetIDs  []uint64 `json:"targetIDs"`
	DestX      *float64 `json:"destX,omitempty"`
	DestZ      *float64 `json:"destZ,omitempty"`
}

// UnitJSON represents a unit's state for the API.
type UnitJSON struct {
	GUID        uint64             `json:"guid"`
	Name        string             `json:"name"`
	Health      int32              `json:"health"`
	MaxHealth   int32              `json:"maxHealth"`
	Mana        int32              `json:"mana"`
	MaxMana     int32              `json:"maxMana"`
	Rage        int32              `json:"rage"`
	MaxRage     int32              `json:"maxRage"`
	PowerType   int32              `json:"powerType"`
	Alive       bool               `json:"alive"`
	Level       uint8              `json:"level"`
	TeamID      uint32             `json:"teamId"`
	Armor       int32              `json:"armor"`
	Resistances map[string]float64 `json:"resistances"`
	Str         int32              `json:"str"`
	Agi         int32              `json:"agi"`
	Sta         int32              `json:"sta"`
	Int         int32              `json:"int"`
	Spi         int32              `json:"spi"`
	AttackPower int32              `json:"attackPower"`
	SpellPower  int32              `json:"spellPower"`
	CritMelee   float64            `json:"critMelee"`
	CritSpell   float64            `json:"critSpell"`
	HitMelee    float64            `json:"hitMelee"`
	HitSpell    float64            `json:"hitSpell"`
	Dodge       float64            `json:"dodge"`
	Parry       float64            `json:"parry"`
	Block       float64            `json:"block"`
	BlockValue  int32              `json:"blockValue"`
	MinWeapon   int32              `json:"minWeapon"`
	MaxWeapon   int32              `json:"maxWeapon"`
	Auras       []AuraJSON         `json:"auras"`
	Position    unit.Position      `json:"position"`
	SpeedMod    float64            `json:"speedMod"`
}

// SpellJSON represents a spell definition for the API.
type SpellJSON struct {
	ID              uint32             `json:"id"`
	Name            string             `json:"name"`
	SchoolMask      uint32             `json:"schoolMask"`
	SchoolName      string             `json:"schoolName"`
	CastTime        int32              `json:"castTime"`
	CD              int32              `json:"cooldown"`
	PowerCost       int32              `json:"powerCost"`
	MaxTargets      int                `json:"maxTargets"`
	CategoryCD      int32              `json:"categoryCD"`
	IsChanneled     bool               `json:"isChanneled"`
	ChannelDuration int32              `json:"channelDuration"`
	TickInterval    int32              `json:"tickInterval"`
	MissileSpeed    float64            `json:"missileSpeed"`
	Effects         []string           `json:"effects"`
	EffectsDetail   []EffectDetailJSON `json:"effectsDetail"`
}

// EffectDetailJSON provides full effect parameters for the config editor.
type EffectDetailJSON struct {
	EffectIndex   int32   `json:"effectIndex"`
	EffectType    string  `json:"effectType"`
	SchoolMask    uint32  `json:"schoolMask"`
	BasePoints    int32   `json:"basePoints"`
	Coef          float64 `json:"coef"`
	WeaponPercent float64 `json:"weaponPercent"`
	AuraDuration  int32   `json:"auraDuration"`
	AuraType      int32   `json:"auraType"`
	Radius        float64 `json:"radius"`
}

// TraceEventJSON represents a trace event for the API.
type TraceEventJSON struct {
	FlowID    uint64                 `json:"flowId"`
	Timestamp int64                  `json:"timestamp"`
	Span      string                 `json:"span"`
	Event     string                 `json:"event"`
	SpellID   uint32                 `json:"spellId"`
	SpellName string                 `json:"spellName"`
	Fields    map[string]interface{} `json:"fields"`
}

// CastResponse is the JSON response for POST /api/cast.
type CastResponse struct {
	Result          string           `json:"result"`
	Error           string           `json:"error,omitempty"`
	Units           []UnitJSON       `json:"units"`
	Events          []TraceEventJSON `json:"events"`
	ChannelDuration int32            `json:"channelDuration,omitempty"`
	DestX           float64          `json:"destX,omitempty"`
	DestZ           float64          `json:"destZ,omitempty"`
}

// AuraJSON represents an active aura for the API.
type AuraJSON struct {
	SpellID    uint32 `json:"spellID"`
	Name       string `json:"name"`
	Duration   int32  `json:"duration"`
	AuraType   int32  `json:"auraType"`
	Stacks     int32  `json:"stacks"`
	TimerStart int64  `json:"timerStart"`
}

// CastPrepareResponse is returned by /api/cast when the spell has a cast time.
type CastPrepareResponse struct {
	Result     string `json:"result"`
	CastTimeMs int32  `json:"castTimeMs"`
	SpellID    uint32 `json:"spellID"`
	SpellName  string `json:"spellName"`
	SchoolName string `json:"schoolName"`
}

// pendingCast holds a spell context between Prepare and Complete.
type pendingCast struct {
	ctx             *spell.SpellContext
	spellInfo       *spelldef.SpellInfo
	targetIDs       []uint64
	castTimeMs      int32
	pushbackTotalMs int32
	DestX           *float64
	DestZ           *float64
	Radius          float64
}

// ---------------------------------------------------------------------------
// JSON serialization helpers
// ---------------------------------------------------------------------------

func unitToJSON(u *unit.Unit, auraMgr *aura.AuraManager) UnitJSON {
	resistances := map[string]float64{
		"Fire":     u.GetResistance(spelldef.SchoolMaskFire),
		"Frost":    u.GetResistance(spelldef.SchoolMaskFrost),
		"Arcane":   u.GetResistance(spelldef.SchoolMaskArcane),
		"Nature":   u.GetResistance(spelldef.SchoolMaskNature),
		"Shadow":   u.GetResistance(spelldef.SchoolMaskShadow),
		"Holy":     u.GetResistance(spelldef.SchoolMaskHoly),
		"Physical": u.GetResistance(spelldef.SchoolMaskPhysical),
	}
	var auraList []AuraJSON
	if auraMgr != nil {
		for _, a := range auraMgr.Auras {
			timerStart := int64(0)
			if len(a.Applications) > 0 {
				timerStart = a.Applications[len(a.Applications)-1].TimerStart
			}
			auraList = append(auraList, AuraJSON{
				SpellID:    a.SpellID,
				Name:       a.SourceName,
				Duration:   a.Duration,
				AuraType:   int32(a.AuraType),
				Stacks:     a.StackAmount,
				TimerStart: timerStart,
			})
		}
	}
	return UnitJSON{
		GUID: u.GUID, Name: u.Name, Health: u.Health, MaxHealth: u.MaxHealth,
		Mana: u.Mana, MaxMana: u.MaxMana, Rage: u.Rage, MaxRage: u.MaxRage,
		PowerType: int32(u.PrimaryPowerType), Alive: u.Alive, Level: u.Level,
		TeamID: u.TeamID, Armor: u.Armor, Resistances: resistances,
		Str: u.Str, Agi: u.Agi, Sta: u.Sta, Int: u.Int, Spi: u.Spi,
		AttackPower: u.AttackPower, SpellPower: u.SpellPower,
		CritMelee: u.CritMelee, CritSpell: u.CritSpell,
		HitMelee: u.HitMelee, HitSpell: u.HitSpell,
		Dodge: u.Dodge, Parry: u.Parry, Block: u.Block, BlockValue: u.BlockValue,
		MinWeapon: u.MinWeaponDamage, MaxWeapon: u.MaxWeaponDamage,
		Auras: auraList, Position: u.Position, SpeedMod: u.SpeedMod,
	}
}

func spellToJSON(s *spelldef.SpellInfo) SpellJSON {
	effectNames := make([]string, len(s.Effects))
	effectDetails := make([]EffectDetailJSON, len(s.Effects))
	for i, e := range s.Effects {
		effectNames[i] = effectTypeName(e.EffectType)
		effectDetails[i] = EffectDetailJSON{
			EffectIndex:   int32(e.EffectIndex),
			EffectType:    effectTypeName(e.EffectType),
			SchoolMask:    uint32(e.SchoolMask),
			BasePoints:    e.BasePoints,
			Coef:          e.Coef,
			WeaponPercent: e.WeaponPercent,
			AuraDuration:  e.AuraDuration,
			AuraType:      e.AuraType,
			Radius:        e.Radius,
		}
	}
	return SpellJSON{
		ID: s.ID, Name: s.Name, SchoolMask: uint32(s.SchoolMask),
		SchoolName: schoolName(s.SchoolMask), CastTime: s.CastTime,
		CD: s.RecoveryTime, PowerCost: s.PowerCost,
		MaxTargets: s.MaxTargets, CategoryCD: s.CategoryRecoveryTime,
		IsChanneled: s.IsChanneled, ChannelDuration: s.ChannelDuration, TickInterval: s.TickInterval, MissileSpeed: s.MissileSpeed,
		Effects: effectNames, EffectsDetail: effectDetails,
	}
}

func schoolName(m spelldef.SchoolMask) string {
	names := map[spelldef.SchoolMask]string{
		spelldef.SchoolMaskFire:     "Fire",
		spelldef.SchoolMaskFrost:    "Frost",
		spelldef.SchoolMaskArcane:   "Arcane",
		spelldef.SchoolMaskNature:   "Nature",
		spelldef.SchoolMaskShadow:   "Shadow",
		spelldef.SchoolMaskHoly:     "Holy",
		spelldef.SchoolMaskPhysical: "Physical",
	}
	if n, ok := names[m]; ok {
		return n
	}
	return "Unknown"
}

func effectTypeName(t spelldef.SpellEffectType) string {
	names := map[spelldef.SpellEffectType]string{
		spelldef.SpellEffectNone:         "None",
		spelldef.SpellEffectSchoolDamage: "SchoolDamage",
		spelldef.SpellEffectHeal:         "Heal",
		spelldef.SpellEffectApplyAura:    "ApplyAura",
		spelldef.SpellEffectTriggerSpell: "TriggerSpell",
		spelldef.SpellEffectEnergize:     "Energize",
		spelldef.SpellEffectWeaponDamage: "WeaponDamage",
	}
	if n, ok := names[t]; ok {
		return n
	}
	return fmt.Sprintf("Effect(%d)", t)
}

func castResultName(r spelldef.CastResult) string {
	switch r {
	case spelldef.CastResultSuccess:
		return "success"
	case spelldef.CastResultFailed:
		return "failed"
	case spelldef.CastResultInterrupted:
		return "interrupted"
	default:
		return fmt.Sprintf("unknown(%d)", r)
	}
}

func castErrorName(e spelldef.CastError) string {
	names := map[spelldef.CastError]string{
		spelldef.CastErrNone:         "none",
		spelldef.CastErrNotReady:     "not_ready",
		spelldef.CastErrOutOfRange:   "out_of_range",
		spelldef.CastErrSilenced:     "silenced",
		spelldef.CastErrDisarmed:     "disarmed",
		spelldef.CastErrShapeshifted: "wrong_shapeshift",
		spelldef.CastErrNoItems:      "no_items",
		spelldef.CastErrWrongArea:    "wrong_area",
		spelldef.CastErrMounted:      "mounted",
		spelldef.CastErrNoMana:       "no_mana",
		spelldef.CastErrDead:         "caster_dead",
		spelldef.CastErrTargetDead:   "target_dead",
		spelldef.CastErrSchoolLocked: "school_locked",
		spelldef.CastErrNoCharges:    "no_charges",
		spelldef.CastErrOnGCD:        "on_gcd",
		spelldef.CastErrInterrupted:  "interrupted",
	}
	if n, ok := names[e]; ok {
		return n
	}
	return fmt.Sprintf("error(%d)", e)
}

func eventToJSON(e trace.FlowEvent) TraceEventJSON {
	return TraceEventJSON{
		FlowID:    e.FlowID,
		Timestamp: e.Timestamp.UnixMilli(),
		Span:      e.Span,
		Event:     e.Event,
		SpellID:   e.SpellID,
		SpellName: e.SpellName,
		Fields:    e.Fields,
	}
}

// ---------------------------------------------------------------------------
// simpleAuraProvider implements spell.AuraProvider.
// ---------------------------------------------------------------------------

type simpleAuraProvider struct {
	managers map[uint64]*aura.AuraManager
}

func (p *simpleAuraProvider) GetAuraManager(target interface{}) *aura.AuraManager {
	if u, ok := target.(*unit.Unit); ok {
		return p.managers[u.GUID]
	}
	return nil
}

func makeAuraHandler(provider *simpleAuraProvider) effect.AuraHandler {
	return func(ctx effect.CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
		if provider == nil {
			return
		}
		mgr := provider.GetAuraManager(target)
		if mgr == nil {
			return
		}
		a := &aura.Aura{
			SpellID:     ctx.GetSpellID(),
			SourceName:  ctx.GetSpellName(),
			CasterGUID:  ctx.Caster().GUID,
			Caster:      ctx.Caster(),
			AuraType:    aura.AuraType(eff.AuraType),
			Duration:    eff.AuraDuration,
			StackAmount: 1,
			Effects: []*aura.AuraEffect{
				{AuraType: aura.AuraType(eff.AuraType), BaseAmount: eff.BasePoints, MiscValue: eff.MiscValue, PeriodicTimer: eff.PeriodicTickInterval},
			},
		}
		mgr.ApplyAura(a, ctx.GetTrace(), ctx.GetSpellID(), ctx.GetSpellName())
	}
}

// tracingCooldownHistory wraps cooldown.SpellHistory with trace-aware methods.
type tracingCooldownHistory struct {
	*cooldown.SpellHistory
}

func (t *tracingCooldownHistory) TraceAddCooldown(spellID uint32, durationMs int32, category int32, tr *trace.Trace) {
	t.SpellHistory.TraceAddCooldown(spellID, durationMs, category, tr)
}

func (t *tracingCooldownHistory) TraceConsumeCharge(spellID uint32, tr *trace.Trace) bool {
	return t.SpellHistory.TraceConsumeCharge(spellID, tr)
}

func (t *tracingCooldownHistory) TraceStartGCD(category int32, durationMs int32, tr *trace.Trace) {
	t.SpellHistory.TraceStartGCD(category, durationMs, tr)
}

// ---------------------------------------------------------------------------
// Request types for unit and spell management
// ---------------------------------------------------------------------------

// AddUnitRequest is the JSON body for POST /api/units/add.
type AddUnitRequest struct {
	Name  string `json:"name"`
	Level uint8  `json:"level"`
}

// MoveUnitRequest is the JSON body for POST /api/units/move.
type MoveUnitRequest struct {
	GUID uint64  `json:"guid"`
	X    float64 `json:"x"`
	Z    float64 `json:"z"`
}

// UpdateUnitRequest is the JSON body for POST /api/units/update.
type UpdateUnitRequest struct {
	GUID  uint64 `json:"guid"`
	Level uint8  `json:"level"`
}

// CreateSpellRequest is the JSON body for POST /api/spells.
type CreateSpellRequest struct {
	Name                 string                `json:"name"`
	SchoolName           string                `json:"schoolName"`
	CastTime             int32                 `json:"castTime"`
	RecoveryTime         int32                 `json:"cooldown"`
	CategoryRecoveryTime int32                 `json:"categoryCD"`
	PowerCost            int32                 `json:"powerCost"`
	PowerType            int32                 `json:"powerType"`
	MaxTargets           int                   `json:"maxTargets"`
	Effects              []CreateEffectRequest `json:"effects"`
}

// CreateEffectRequest defines a single effect for spell creation.
type CreateEffectRequest struct {
	EffectType    string  `json:"effectType"`
	BasePoints    int32   `json:"basePoints"`
	Coef          float64 `json:"coef"`
	WeaponPercent float64 `json:"weaponPercent"`
	AuraDuration  int32   `json:"auraDuration"`
	AuraType      int32   `json:"auraType"`
}

func schoolMaskFromName(name string) spelldef.SchoolMask {
	switch name {
	case "Fire":
		return spelldef.SchoolMaskFire
	case "Frost":
		return spelldef.SchoolMaskFrost
	case "Arcane":
		return spelldef.SchoolMaskArcane
	case "Nature":
		return spelldef.SchoolMaskNature
	case "Shadow":
		return spelldef.SchoolMaskShadow
	case "Holy":
		return spelldef.SchoolMaskHoly
	case "Physical":
		return spelldef.SchoolMaskPhysical
	default:
		return spelldef.SchoolMaskFire
	}
}

func effectTypeFromName(name string) spelldef.SpellEffectType {
	switch name {
	case "SchoolDamage":
		return spelldef.SpellEffectSchoolDamage
	case "Heal":
		return spelldef.SpellEffectHeal
	case "ApplyAura":
		return spelldef.SpellEffectApplyAura
	case "TriggerSpell":
		return spelldef.SpellEffectTriggerSpell
	case "Energize":
		return spelldef.SpellEffectEnergize
	case "WeaponDamage":
		return spelldef.SpellEffectWeaponDamage
	default:
		return spelldef.SpellEffectNone
	}
}

// UpdateSpellRequest is the JSON body for PUT /api/spells/{id}.
type UpdateSpellRequest struct {
	Name                 string                `json:"name"`
	CastTime             int32                 `json:"castTime"`
	RecoveryTime         int32                 `json:"cooldown"`
	CategoryRecoveryTime int32                 `json:"categoryCD"`
	PowerCost            int32                 `json:"powerCost"`
	MaxTargets           int                   `json:"maxTargets"`
	Effects              []UpdateEffectRequest `json:"effects"`
}

// UpdateEffectRequest is a single effect update.
type UpdateEffectRequest struct {
	EffectIndex   int     `json:"effectIndex"`
	BasePoints    int32   `json:"basePoints"`
	Coef          float64 `json:"coef"`
	WeaponPercent float64 `json:"weaponPercent"`
	AuraDuration  int32   `json:"auraDuration"`
	AuraType      int32   `json:"auraType"`
}

// ---------------------------------------------------------------------------
// HTTP handlers — thin wrappers that send commands to GameLoop
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"marshal failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	w.Write(data)
}

func handleCast(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		var req CastRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		result := gl.Send(Command{Op: "cast", Payload: castPayload{
			CasterGUID: req.CasterGUID,
			SpellID:    req.SpellID,
			TargetIDs:  req.TargetIDs,
			DestX:      req.DestX,
			DestZ:      req.DestZ,
		}})

		if result.Err != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleCastComplete(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		result := gl.Send(Command{Op: "cast_complete"})

		if result.Err != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleCastCancel(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		result := gl.Send(Command{Op: "cast_cancel"})
		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleCastPushback(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		var req struct {
			PushbackMs int32
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		result := gl.Send(Command{Op: "cast_pushback", Payload: pushbackPayload{PushbackMs: req.PushbackMs}})
		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleUnits(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := gl.Send(Command{Op: "get_units"})
		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleAddUnit(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		var req AddUnitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		result := gl.Send(Command{Op: "add_unit", Payload: addUnitPayload{Name: req.Name, Level: req.Level}})

		if result.Err != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleRemoveUnit(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/units/")
		guid, err := strconv.ParseUint(path, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid GUID"})
			return
		}

		result := gl.Send(Command{Op: "remove_unit", Payload: removeUnitPayload{GUID: guid}})

		if result.Err != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleMoveUnit(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		var req MoveUnitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		result := gl.Send(Command{Op: "move_unit", Payload: moveUnitPayload{GUID: req.GUID, X: req.X, Z: req.Z}})

		if result.Err != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleUpdateUnit(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		var req UpdateUnitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		result := gl.Send(Command{Op: "update_unit", Payload: updateUnitPayload{GUID: req.GUID, Level: req.Level}})

		if result.Err != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleTraceStream(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		sub := gl.Hub().Subscribe()
		defer gl.Hub().Unsubscribe(sub)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-sub.Events():
				if !ok {
					return
				}
				data, _ := json.Marshal(eventToJSON(e))
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

func handleTraceHistory(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var flowID uint64
		if v := r.URL.Query().Get("flow_id"); v != "" {
			flowID, _ = strconv.ParseUint(v, 10, 64)
		}
		span := r.URL.Query().Get("span")
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}

		events := gl.Hub().Query(flowID, span, limit)
		eventsJSON := make([]TraceEventJSON, len(events))
		for i, e := range events {
			eventsJSON[i] = eventToJSON(e)
		}
		writeJSON(w, http.StatusOK, eventsJSON)
	}
}

func handleTrace(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("clear") == "true" {
			gl.Send(Command{Op: "trace_clear"})
		}

		events := gl.Hub().Query(0, "", 0)
		eventsJSON := make([]TraceEventJSON, len(events))
		for i, e := range events {
			eventsJSON[i] = eventToJSON(e)
		}
		writeJSON(w, http.StatusOK, eventsJSON)
	}
}

func handleSpells(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := gl.Send(Command{Op: "get_spells"})
		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleReset(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gl.Send(Command{Op: "reset"})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func handleCreateSpell(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		var req CreateSpellRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}

		result := gl.Send(Command{Op: "create_spell", Payload: createSpellPayload{Req: req}})

		if result.Err != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusCreated, result.Data)
	}
}

func handleDeleteSpell(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/spells/")
		id, err := strconv.ParseUint(path, 10, 32)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid spell ID"})
			return
		}

		result := gl.Send(Command{Op: "delete_spell", Payload: uint32(id)})

		if result.Err != "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleUpdateSpell(gl *GameLoop) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/spells/")
		id, err := strconv.ParseUint(path, 10, 32)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid spell ID"})
			return
		}

		var req UpdateSpellRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		result := gl.Send(Command{Op: "update_spell", Payload: updateSpellPayload{ID: uint32(id), Req: req}})

		if result.Err != "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": result.Err})
			return
		}

		writeJSON(w, http.StatusOK, result.Data)
	}
}

func handleSpellRoutes(gl *GameLoop) http.HandlerFunc {
	get := handleSpells(gl)
	post := handleCreateSpell(gl)
	putDel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			handleUpdateSpell(gl).ServeHTTP(w, r)
		} else if r.Method == http.MethodDelete {
			handleDeleteSpell(gl).ServeHTTP(w, r)
		} else {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			get.ServeHTTP(w, r)
		case http.MethodPost:
			post.ServeHTTP(w, r)
		default:
			putDel.ServeHTTP(w, r)
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleApiIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	routes := []map[string]string{
		{"method": "GET", "path": "/api", "description": "List all API routes (this endpoint)"},
		{"method": "GET", "path": "/api/docs", "description": "Spell system configuration reference"},
		{"method": "POST", "path": "/api/cast", "description": "Cast a spell (prepare phase for cast-time spells)"},
		{"method": "POST", "path": "/api/cast/complete", "description": "Complete a pending cast"},
		{"method": "POST", "path": "/api/cast/cancel", "description": "Cancel a pending cast"},
		{"method": "POST", "path": "/api/cast/pushback", "description": "Pushback a casting spell (interrupt if >100% cast time)"},
		{"method": "GET", "path": "/api/units", "description": "List all units with stats and auras"},
		{"method": "POST", "path": "/api/units/add", "description": "Spawn a new unit"},
		{"method": "PUT", "path": "/api/units/update", "description": "Update a unit (e.g. level change)"},
		{"method": "POST", "path": "/api/units/move", "description": "Move a unit to new position"},
		{"method": "DELETE", "path": "/api/units/{guid}", "description": "Remove a unit"},
		{"method": "GET", "path": "/api/trace", "description": "Get trace events for last cast"},
		{"method": "GET", "path": "/api/trace/stream", "description": "SSE stream of trace events"},
		{"method": "GET", "path": "/api/trace/history", "description": "Get full trace history"},
		{"method": "GET", "path": "/api/spells", "description": "List all spells"},
		{"method": "POST", "path": "/api/spells", "description": "Create a new spell"},
		{"method": "GET", "path": "/api/spells/{id}", "description": "Get spell details"},
		{"method": "PUT", "path": "/api/spells/{id}", "description": "Update a spell"},
		{"method": "DELETE", "path": "/api/spells/{id}", "description": "Delete a spell"},
		{"method": "POST", "path": "/api/reset", "description": "Reset session (restore units, clear cooldowns/auras)"},
	}
	writeJSON(w, http.StatusOK, routes)
}

func NewServer(addr string, gl *GameLoop) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/docs", handleDocs)
	mux.HandleFunc("/api", handleApiIndex)
	mux.HandleFunc("/api/cast", handleCast(gl))
	mux.HandleFunc("/api/cast/complete", handleCastComplete(gl))
	mux.HandleFunc("/api/cast/cancel", handleCastCancel(gl))
	mux.HandleFunc("/api/cast/pushback", handleCastPushback(gl))
	mux.HandleFunc("/api/units", handleUnits(gl))
	mux.HandleFunc("/api/units/add", handleAddUnit(gl))
	mux.HandleFunc("/api/units/update", handleUpdateUnit(gl))
	mux.HandleFunc("/api/units/move", handleMoveUnit(gl))
	mux.HandleFunc("/api/units/", handleRemoveUnit(gl))
	mux.HandleFunc("/api/trace", handleTrace(gl))
	mux.HandleFunc("/api/trace/stream", handleTraceStream(gl))
	mux.HandleFunc("/api/trace/history", handleTraceHistory(gl))
	mux.HandleFunc("/api/spells", handleSpellRoutes(gl))
	mux.HandleFunc("/api/spells/", handleSpellRoutes(gl))
	mux.HandleFunc("/api/reset", handleReset(gl))

	handler := corsMiddleware(mux)

	fs := http.FileServer(http.Dir("D:/goes/TrinityCore/arch/skill-go/server/web"))
	mux.Handle("/", fs)

	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}
