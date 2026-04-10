package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"math"
	"strconv"

	"skill-go/server/aura"
	"skill-go/server/cooldown"
	"skill-go/server/effect"
	"skill-go/server/script"
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
	SpellID   uint32   `json:"spellID"`
	TargetIDs []uint64 `json:"targetIDs"`
}

// UnitJSON represents a unit's state for the API.
type UnitJSON struct {
	GUID        uint64              `json:"guid"`
	Name        string              `json:"name"`
	Health      int32               `json:"health"`
	MaxHealth   int32               `json:"maxHealth"`
	Mana        int32               `json:"mana"`
	MaxMana     int32               `json:"maxMana"`
	Alive       bool                `json:"alive"`
	Level       uint8               `json:"level"`
	TeamID      uint32              `json:"teamId"`
	Armor       int32               `json:"armor"`
	Resistances map[string]float64  `json:"resistances"`
	Str         int32               `json:"str"`
	Agi         int32               `json:"agi"`
	Sta         int32               `json:"sta"`
	Int         int32               `json:"int"`
	Spi         int32               `json:"spi"`
	AttackPower int32               `json:"attackPower"`
	SpellPower  int32               `json:"spellPower"`
	CritMelee   float64             `json:"critMelee"`
	CritSpell   float64             `json:"critSpell"`
	HitMelee    float64             `json:"hitMelee"`
	HitSpell    float64             `json:"hitSpell"`
	Dodge       float64             `json:"dodge"`
	Parry       float64             `json:"parry"`
	Block       float64             `json:"block"`
	BlockValue  int32               `json:"blockValue"`
	MinWeapon   int32               `json:"minWeapon"`
	MaxWeapon   int32               `json:"maxWeapon"`
	Auras       []string            `json:"auras"`
	Position    unit.Position       `json:"position"`
}

// SpellJSON represents a spell definition for the API.
type SpellJSON struct {
	ID         uint32   `json:"id"`
	Name       string   `json:"name"`
	SchoolMask uint32   `json:"schoolMask"`
	SchoolName string   `json:"schoolName"`
	CastTime   int32    `json:"castTime"`
	CD         int32    `json:"cooldown"`
	PowerCost  int32    `json:"powerCost"`
	MaxTargets int      `json:"maxTargets"`
	CategoryCD int32    `json:"categoryCD"`
	Effects    []string `json:"effects"`
	EffectsDetail []EffectDetailJSON `json:"effectsDetail"`
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
}

// TraceEventJSON represents a trace event for the API.
type TraceEventJSON struct {
	FlowID    uint64                  `json:"flowId"`
	Timestamp int64                   `json:"timestamp"`
	Span      string                  `json:"span"`
	Event     string                  `json:"event"`
	SpellID   uint32                  `json:"spellId"`
	SpellName string                  `json:"spellName"`
	Fields    map[string]interface{}   `json:"fields"`
}

// CastResponse is the JSON response for POST /api/cast.
type CastResponse struct {
	Result   string          `json:"result"`
	Error    string          `json:"error,omitempty"`
	Units    []UnitJSON     `json:"units"`
	Events   []TraceEventJSON `json:"events"`
}

// ---------------------------------------------------------------------------
// GameState — single session state
// ---------------------------------------------------------------------------

// GameState holds the entire game session state.
type GameState struct {
	mu           sync.RWMutex
	Caster       *unit.Unit
	Targets      []*unit.Unit
	AllUnits     []*unit.Unit
	History      *cooldown.SpellHistory
	AuraManagers map[uint64]*aura.AuraManager
	Recorder     *trace.FlowRecorder
	Store        *effect.Store
	Registry     *script.Registry
	SpellBook    []*spelldef.SpellInfo
	Tr           *trace.Trace
	Hub          *trace.StreamHub
	FileSink     *trace.FileSink
}

// NewGameState creates a game session with predefined units and spells.
// Optional FileSink will be wired into all trace paths for file logging.
func NewGameState(fileSink *trace.FileSink) *GameState {
	mage := unit.NewUnit(1, "Mage", 5000, 20000)
	mage.SetLevel(60)
	mage.SpellPower = 500
	mage.SetWeaponDamage(30, 30)
	mage.HitSpell = 100.0
	mage.CritSpell = 20.0
	mage.Position = unit.Position{X: 0, Y: 0, Z: 0}

	warrior := unit.NewUnit(2, "Warrior", 10000, 5000)
	warrior.SetLevel(60)
	warrior.Armor = 3000
	warrior.Block = 15.0
	warrior.BlockValue = 200
	warrior.Dodge = 5.0
	warrior.Parry = 10.0
	warrior.SetWeaponDamage(80, 120)
	warrior.HitMelee = 100.0
	warrior.Position = unit.Position{X: 20, Y: 0, Z: 0}

	target := unit.NewUnit(3, "Target Dummy", 15000, 0)
	target.SetLevel(63)
	target.Armor = 5000
	target.SetResistance(spelldef.SchoolMaskFire, 100)
	target.Position = unit.Position{X: 40, Y: 0, Z: 0}

	allUnits := []*unit.Unit{mage, warrior, target}

	targetAuraMgr := aura.NewAuraManager(target)
	warriorAuraMgr := aura.NewAuraManager(warrior)
	auraManagers := map[uint64]*aura.AuraManager{
		target.GUID:  targetAuraMgr,
		warrior.GUID: warriorAuraMgr,
	}

	recorder := trace.NewFlowRecorder()
	hub := trace.NewStreamHub(10000)
	streamSink := trace.NewStreamSink(hub)

	// Build sink list for trace creation
	var sinks []trace.TraceSink
	sinks = append(sinks, recorder, streamSink)
	if fileSink != nil {
		sinks = append(sinks, fileSink)
	}
	tr := trace.NewTraceWithSinks(sinks...)

	store := effect.NewStore()
	registry := script.NewRegistry()
	history := cooldown.NewSpellHistory()

	auraProvider := &simpleAuraProvider{managers: auraManagers}

	gs := &GameState{
		Caster:       mage,
		Targets:      []*unit.Unit{warrior, target},
		AllUnits:     allUnits,
		History:      history,
		AuraManagers: auraManagers,
		Recorder:     recorder,
		Store:        store,
		Registry:     registry,
		Tr:           tr,
		Hub:          hub,
		FileSink:     fileSink,
	}

	effect.RegisterExtended(store, makeAuraHandler(auraProvider), nil)
	gs.initSpellBook()
	return gs
}

func (gs *GameState) initSpellBook() {
	gs.SpellBook = []*spelldef.SpellInfo{
		{
			ID:         1001,
			Name:       "Fireball",
			SchoolMask: spelldef.SchoolMaskFire,
			CastTime:   0,
			RecoveryTime: 6000,
			PowerCost:  200,
			MaxTargets: 1,
			Effects: []spelldef.SpellEffectInfo{
				{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, SchoolMask: spelldef.SchoolMaskFire, BasePoints: 500, Coef: 1.0},
			},
		},
		{
			ID:         1002,
			Name:       "Frostbolt",
			SchoolMask: spelldef.SchoolMaskFrost,
			CastTime:   0,
			RecoveryTime: 3000,
			PowerCost:  150,
			MaxTargets: 1,
			Effects: []spelldef.SpellEffectInfo{
				{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, SchoolMask: spelldef.SchoolMaskFrost, BasePoints: 300, Coef: 0.8},
			},
		},
		{
			ID:         1003,
			Name:       "Arcane Intellect",
			SchoolMask: spelldef.SchoolMaskArcane,
			CastTime:   0,
			RecoveryTime: 0,
			PowerCost:  0,
			MaxTargets: 1,
			Effects: []spelldef.SpellEffectInfo{
				{EffectIndex: 0, EffectType: spelldef.SpellEffectApplyAura, AuraType: int32(aura.AuraTypeBuff), AuraDuration: 30000},
			},
		},
		{
			ID:         1004,
			Name:       "Heal",
			SchoolMask: spelldef.SchoolMaskHoly,
			CastTime:   0,
			RecoveryTime: 2000,
			PowerCost:  300,
			MaxTargets: 1,
			Effects: []spelldef.SpellEffectInfo{
				{EffectIndex: 0, EffectType: spelldef.SpellEffectHeal, BasePoints: 800},
			},
		},
		{
			ID:         1005,
			Name:       "Heroic Strike",
			SchoolMask: spelldef.SchoolMaskPhysical,
			CastTime:   0,
			RecoveryTime: 4000,
			PowerCost: 0,
			MaxTargets: 1,
			Effects: []spelldef.SpellEffectInfo{
				{EffectIndex: 0, EffectType: spelldef.SpellEffectWeaponDamage, BasePoints: 50, WeaponPercent: 1.0},
			},
		},
		{
			ID:         1006,
			Name:       "Whirlwind",
			SchoolMask: spelldef.SchoolMaskPhysical,
			CastTime:   0,
			RecoveryTime: 10000,
			PowerCost: 0,
			MaxTargets: 5,
			Effects: []spelldef.SpellEffectInfo{
				{EffectIndex: 0, EffectType: spelldef.SpellEffectWeaponDamage, BasePoints: 80, WeaponPercent: 1.0},
			},
		},
		{
			ID:         1007,
			Name:       "Mana Shield",
			SchoolMask: spelldef.SchoolMaskArcane,
			CastTime:   0,
			RecoveryTime: 12000,
			PowerCost: 500,
			MaxTargets: 1,
			Effects: []spelldef.SpellEffectInfo{
				{EffectIndex: 0, EffectType: spelldef.SpellEffectApplyAura, AuraType: int32(aura.AuraTypeBuff), AuraDuration: 60000},
			},
		},
		{
			ID:         1008,
			Name:       "Shadow Bolt",
			SchoolMask: spelldef.SchoolMaskShadow,
			CastTime:   0,
			RecoveryTime: 4000,
			PowerCost:  250,
			MaxTargets: 1,
			Effects: []spelldef.SpellEffectInfo{
				{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, SchoolMask: spelldef.SchoolMaskShadow, BasePoints: 400, Coef: 0.9},
			},
		},
	}
}

func (gs *GameState) Reset() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Restore units
	gs.Caster.Health = gs.Caster.MaxHealth
	gs.Caster.Mana = gs.Caster.MaxMana
	gs.Caster.Alive = true
	for _, u := range gs.AllUnits {
		u.Health = u.MaxHealth
		u.Mana = u.MaxMana
		u.Alive = true
	}

	// Clear cooldowns
	gs.History = cooldown.NewSpellHistory()

	// Clear auras
	for _, mgr := range gs.AuraManagers {
		for _, a := range mgr.Auras {
			mgr.RemoveAura(a, aura.RemoveModeDefault, nil, 0, "")
		}
	}

	// Clear trace
	gs.Recorder.Reset()
	gs.Hub.Clear()
	gs.Hub.ClearSubscribers()

	var sinks []trace.TraceSink
	sinks = append(sinks, gs.Recorder, trace.NewStreamSink(gs.Hub))
	if gs.FileSink != nil {
		sinks = append(sinks, gs.FileSink)
	}
	gs.Tr = trace.NewTraceWithSinks(sinks...)
}

func (gs *GameState) FindSpell(id uint32) *spelldef.SpellInfo {
	for _, s := range gs.SpellBook {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func (gs *GameState) FindUnit(guid uint64) *unit.Unit {
	for _, u := range gs.AllUnits {
		if u.GUID == guid {
			return u
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// JSON serialization helpers
// ---------------------------------------------------------------------------

func unitToJSON(u *unit.Unit, auraMgr *aura.AuraManager) UnitJSON {
	resistances := map[string]float64{
		"Fire":     u.GetResistance(spelldef.SchoolMaskFire),
		"Frost":    u.GetResistance(spelldef.SchoolMaskFrost),
		"Arcane":  u.GetResistance(spelldef.SchoolMaskArcane),
		"Nature":   u.GetResistance(spelldef.SchoolMaskNature),
		"Shadow":   u.GetResistance(spelldef.SchoolMaskShadow),
		"Holy":     u.GetResistance(spelldef.SchoolMaskHoly),
		"Physical": u.GetResistance(spelldef.SchoolMaskPhysical),
	}
	var auraNames []string
	if auraMgr != nil {
		for _, a := range auraMgr.Auras {
			auraNames = append(auraNames, fmt.Sprintf("Spell#%d", a.SpellID))
		}
	}
	return UnitJSON{
		GUID: u.GUID, Name: u.Name, Health: u.Health, MaxHealth: u.MaxHealth,
		Mana: u.Mana, MaxMana: u.MaxMana, Alive: u.Alive, Level: u.Level,
		TeamID: u.TeamID, Armor: u.Armor, Resistances: resistances,
		Str: u.Str, Agi: u.Agi, Sta: u.Sta, Int: u.Int, Spi: u.Spi,
		AttackPower: u.AttackPower, SpellPower: u.SpellPower,
		CritMelee: u.CritMelee, CritSpell: u.CritSpell,
		HitMelee: u.HitMelee, HitSpell: u.HitSpell,
		Dodge: u.Dodge, Parry: u.Parry, Block: u.Block, BlockValue: u.BlockValue,
		MinWeapon: u.MinWeaponDamage, MaxWeapon: u.MaxWeaponDamage,
		Auras: auraNames, Position: u.Position,
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
		}
	}
	return SpellJSON{
		ID: s.ID, Name: s.Name, SchoolMask: uint32(s.SchoolMask),
		SchoolName: schoolName(s.SchoolMask), CastTime: s.CastTime,
		CD: s.RecoveryTime, PowerCost: s.PowerCost,
		MaxTargets: s.MaxTargets, CategoryCD: s.CategoryRecoveryTime,
		Effects: effectNames, EffectsDetail: effectDetails,
	}
}

func schoolName(m spelldef.SchoolMask) string {
	names := map[spelldef.SchoolMask]string{
		spelldef.SchoolMaskFire:    "Fire",
		spelldef.SchoolMaskFrost:   "Frost",
		spelldef.SchoolMaskArcane:  "Arcane",
		spelldef.SchoolMaskNature:  "Nature",
		spelldef.SchoolMaskShadow:  "Shadow",
		spelldef.SchoolMaskHoly:    "Holy",
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
			SpellID:    uint32(eff.EffectIndex) + 9000,
			CasterGUID: ctx.Caster().GUID,
			Caster:     ctx.Caster(),
			AuraType:   aura.AuraType(eff.AuraType),
			Duration:   eff.AuraDuration,
			StackAmount: 1,
			Effects: []*aura.AuraEffect{
				{AuraType: aura.AuraType(eff.AuraType), BaseAmount: eff.BasePoints},
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

// nextGUID is a counter for generating unique GUIDs for new units.
var nextGUID uint64 = 100

func (gs *GameState) nextUnitGUID() uint64 {
	nextGUID++
	return nextGUID
}

// addUnit creates a new unit and adds it to the game state.
func (gs *GameState) addUnit(name string, level uint8) *unit.Unit {
	if name == "" {
		name = "Unknown"
	}
	if level == 0 {
		level = 60
	}

	// Level-scaled stats (similar to TrinityCore formulas)
	lvl := float64(level)
	maxHP := int32(100 + lvl*50)            // base HP scaling
	maxMana := int32(50 + lvl*20)           // base mana scaling
	armor := int32(lvl * 30)                // armor scaling
	spellPower := int32(lvl * 5)            // minimal SP for enemies

	u := unit.NewUnit(gs.nextUnitGUID(), name, maxHP, maxMana)
	u.SetLevel(level)
	u.Armor = armor
	u.SpellPower = spellPower

	// Default position: spread along X axis with random Z offset
	offsetX := 25.0 + float64(len(gs.AllUnits))*5 + math.Round(float64(len(gs.AllUnits)%3)*3)
	offsetZ := float64(((len(gs.AllUnits)*7+3)%11) - 5) * 1.5
	u.Position = unit.Position{X: offsetX, Y: 0, Z: offsetZ}

	// Create aura manager
	auraMgr := aura.NewAuraManager(u)
	gs.AuraManagers[u.GUID] = auraMgr

	// Add to state
	gs.AllUnits = append(gs.AllUnits, u)
	gs.Targets = append(gs.Targets, u)

	return u
}

// removeUnit removes a unit by GUID. Returns error if trying to remove caster.
func (gs *GameState) removeUnit(guid uint64) error {
	if guid == gs.Caster.GUID {
		return fmt.Errorf("cannot remove caster")
	}

	// Remove from AllUnits
	found := false
	var newAll []*unit.Unit
	for _, u := range gs.AllUnits {
		if u.GUID == guid {
			found = true
			continue
		}
		newAll = append(newAll, u)
	}
	if !found {
		return fmt.Errorf("unit not found")
	}
	gs.AllUnits = newAll

	// Remove from Targets
	var newTargets []*unit.Unit
	for _, u := range gs.Targets {
		if u.GUID != guid {
			newTargets = append(newTargets, u)
		}
	}
	gs.Targets = newTargets

	// Clean up aura manager
	if mgr, ok := gs.AuraManagers[guid]; ok {
		for _, a := range mgr.Auras {
			mgr.RemoveAura(a, aura.RemoveModeDefault, nil, 0, "")
		}
		delete(gs.AuraManagers, guid)
	}

	return nil
}

func (gs *GameState) moveUnit(guid uint64, x, z float64) error {
	u := gs.FindUnit(guid)
	if u == nil {
		return fmt.Errorf("unit not found")
	}
	u.Position = unit.Position{X: x, Y: 0, Z: z}
	return nil
}

func (gs *GameState) updateUnitLevel(guid uint64, level uint8) error {
	if guid == gs.Caster.GUID {
		return fmt.Errorf("cannot modify caster")
	}
	u := gs.FindUnit(guid)
	if u == nil {
		return fmt.Errorf("unit not found")
	}
	if level == 0 {
		level = 60
	}
	u.SetLevel(level)
	lvl := float64(level)
	u.MaxHealth = int32(100 + lvl*50)
	u.Health = u.MaxHealth
	u.MaxMana = int32(50 + lvl*20)
	u.Mana = u.MaxMana
	u.Armor = int32(lvl * 30)
	u.SpellPower = int32(lvl * 5)
	return nil
}

func unitListJSON(gs *GameState) []UnitJSON {
	unitsJSON := make([]UnitJSON, len(gs.AllUnits))
	for i, u := range gs.AllUnits {
		auraMgr := gs.AuraManagers[u.GUID]
		unitsJSON[i] = unitToJSON(u, auraMgr)
	}
	return unitsJSON
}

func handleAddUnit(gs *GameState) http.HandlerFunc {
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

		gs.mu.Lock()
		defer gs.mu.Unlock()

		_ = gs.addUnit(req.Name, req.Level)

		writeJSON(w, http.StatusOK, unitListJSON(gs))
	}
}

func handleRemoveUnit(gs *GameState) http.HandlerFunc {
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

		gs.mu.Lock()
		defer gs.mu.Unlock()

		if err := gs.removeUnit(guid); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, unitListJSON(gs))
	}
}

func handleMoveUnit(gs *GameState) http.HandlerFunc {
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

		gs.mu.Lock()
		defer gs.mu.Unlock()

		if err := gs.moveUnit(req.GUID, req.X, req.Z); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, unitListJSON(gs))
	}
}

func handleUpdateUnit(gs *GameState) http.HandlerFunc {
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

		gs.mu.Lock()
		defer gs.mu.Unlock()

		if err := gs.updateUnitLevel(req.GUID, req.Level); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, unitListJSON(gs))
	}
}

// ---------------------------------------------------------------------------
// HTTP handlers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func handleCast(gs *GameState) http.HandlerFunc {
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

		gs.mu.Lock()
		defer gs.mu.Unlock()

		spellInfo := gs.FindSpell(req.SpellID)
		if spellInfo == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown spell ID %d", req.SpellID)})
			return
		}

		// Resolve target units
		var targets []*unit.Unit
		for _, guid := range req.TargetIDs {
			u := gs.FindUnit(guid)
			if u != nil {
				targets = append(targets, u)
			}
		}
		if len(targets) == 0 {
			targets = gs.Targets // default to all targets
		}

		// Create spell context
		var castSinks []trace.TraceSink
		castSinks = append(castSinks, gs.Recorder, trace.NewStreamSink(gs.Hub))
		if gs.FileSink != nil {
			castSinks = append(castSinks, gs.FileSink)
		}
		castTrace := trace.NewTraceWithSinks(castSinks...)
		ctx := spell.New(spellInfo.ID, spellInfo, gs.Caster, targets)
		ctx.EffectStore = gs.Store
		ctx.HistoryProvider = gs.History
		ctx.CooldownProvider = &tracingCooldownHistory{SpellHistory: gs.History}
		ctx.AuraProvider = &simpleAuraProvider{managers: gs.AuraManagers}
		ctx.ScriptRegistry = gs.Registry
		ctx.Trace = castTrace

		// Cast
		result := ctx.Prepare()

		// Collect events from this cast
		castEvents := gs.Recorder.Events()

		// Build response
		unitsJSON := make([]UnitJSON, len(gs.AllUnits))
		for i, u := range gs.AllUnits {
			auraMgr := gs.AuraManagers[u.GUID]
			unitsJSON[i] = unitToJSON(u, auraMgr)
		}

		eventsJSON := make([]TraceEventJSON, len(castEvents))
		for i, e := range castEvents {
			eventsJSON[i] = eventToJSON(e)
		}

		resp := CastResponse{
			Result: castResultName(result),
			Units:  unitsJSON,
			Events: eventsJSON,
		}
		if result != spelldef.CastResultSuccess {
			resp.Error = castErrorName(ctx.LastCastErr)
		}

		writeJSON(w, http.StatusOK, resp)

		// Clear trace events so next cast only returns fresh events
		gs.Recorder.Reset()
	}
}

func handleUnits(gs *GameState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs.mu.RLock()
		defer gs.mu.RUnlock()

		unitsJSON := make([]UnitJSON, len(gs.AllUnits))
		for i, u := range gs.AllUnits {
			auraMgr := gs.AuraManagers[u.GUID]
			unitsJSON[i] = unitToJSON(u, auraMgr)
		}
		writeJSON(w, http.StatusOK, unitsJSON)
	}
}


func handleTraceStream(gs *GameState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		sub := gs.Hub.Subscribe()
		defer gs.Hub.Unsubscribe(sub)

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

func handleTraceHistory(gs *GameState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs.mu.RLock()
		defer gs.mu.RUnlock()

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

		events := gs.Hub.Query(flowID, span, limit)
		eventsJSON := make([]TraceEventJSON, len(events))
		for i, e := range events {
			eventsJSON[i] = eventToJSON(e)
		}
		writeJSON(w, http.StatusOK, eventsJSON)
	}
}

func handleTrace(gs *GameState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs.mu.Lock()
		defer gs.mu.Unlock()

		if r.URL.Query().Get("clear") == "true" {
			gs.Recorder.Reset()
			gs.Hub.Clear()
		}

		// Return from ring buffer for full session history
		events := gs.Hub.Query(0, "", 0)
		eventsJSON := make([]TraceEventJSON, len(events))
		for i, e := range events {
			eventsJSON[i] = eventToJSON(e)
		}
		writeJSON(w, http.StatusOK, eventsJSON)
	}
}

func handleSpells(gs *GameState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs.mu.RLock()
		defer gs.mu.RUnlock()

		spellsJSON := make([]SpellJSON, len(gs.SpellBook))
		for i, s := range gs.SpellBook {
			spellsJSON[i] = spellToJSON(s)
		}
		writeJSON(w, http.StatusOK, spellsJSON)
	}
}

func handleReset(gs *GameState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gs.Reset()
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// UpdateSpellRequest is the JSON body for PUT /api/spells/{id}.
type UpdateSpellRequest struct {
	Name           string  `json:"name"`
	CastTime       int32   `json:"castTime"`
	RecoveryTime   int32   `json:"cooldown"`
	CategoryRecoveryTime int32 `json:"categoryCD"`
	PowerCost      int32   `json:"powerCost"`
	MaxTargets     int      `json:"maxTargets"`
	Effects        []UpdateEffectRequest `json:"effects"`
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

func handleUpdateSpell(gs *GameState) http.HandlerFunc {
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

		gs.mu.Lock()
		defer gs.mu.Unlock()

		spell := gs.FindSpell(uint32(id))
		if spell == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("spell %d not found", id)})
			return
		}

		// Update top-level fields
		if req.Name != "" {
			spell.Name = req.Name
		}
		spell.CastTime = req.CastTime
		spell.RecoveryTime = req.RecoveryTime
		spell.CategoryRecoveryTime = req.CategoryRecoveryTime
		spell.PowerCost = req.PowerCost
		spell.MaxTargets = req.MaxTargets

		// Update effects
		for _, ue := range req.Effects {
			if ue.EffectIndex < 0 || ue.EffectIndex >= len(spell.Effects) {
				continue
			}
			eff := &spell.Effects[ue.EffectIndex]
			eff.BasePoints = ue.BasePoints
			eff.Coef = ue.Coef
			eff.WeaponPercent = ue.WeaponPercent
			eff.AuraDuration = ue.AuraDuration
			eff.AuraType = ue.AuraType
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// corsMiddleware adds CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// NewServer creates and returns a configured HTTP server.
func NewServer(addr string, gs *GameState) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/cast", handleCast(gs))
	mux.HandleFunc("/api/units", handleUnits(gs))
	mux.HandleFunc("/api/units/add", handleAddUnit(gs))
	mux.HandleFunc("/api/units/update", handleUpdateUnit(gs))
	mux.HandleFunc("/api/units/move", handleMoveUnit(gs))
	mux.HandleFunc("/api/units/", handleRemoveUnit(gs)) // matches /api/units/{guid} for DELETE
	mux.HandleFunc("/api/trace", handleTrace(gs))
	mux.HandleFunc("/api/trace/stream", handleTraceStream(gs))
	mux.HandleFunc("/api/trace/history", handleTraceHistory(gs))
	mux.HandleFunc("/api/spells", handleSpells(gs))
	mux.HandleFunc("/api/spells/", handleUpdateSpell(gs))
	mux.HandleFunc("/api/reset", handleReset(gs))

	handler := corsMiddleware(mux)

	// Static file server for web/
	fs := http.FileServer(http.Dir(filepath.Join("server", "web")))
	mux.Handle("/", fs)

	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}
