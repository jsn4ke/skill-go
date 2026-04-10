// timeline.js — Maps trace events to 3D animations with proper sequencing
import { spawnProjectile, spawnMeleeArc, spawnDamageNumber, spawnHealNumber, spawnHealBeam, flashHit, spawnCastGlow, spawnAuraRing, SCHOOL_COLORS } from './vfx.js';

// CombatResult enum from Go (must match spelldef.CombatResult)
const RESULT = {
  HIT: 0, MISS: 1, CRIT: 2, DODGE: 3, PARRY: 4, BLOCK: 5, GLANCING: 6, RESIST: 7, FULL_RESIST: 8,
};

// School mask to name mapping
const SCHOOL_MASK_TO_NAME = {
  1: 'Fire', 2: 'Frost', 4: 'Arcane', 16: 'Shadow', 32: 'Nature', 64: 'Holy',
};

// Callbacks set by app.js
let onCooldownStart = null;

export function setOnCooldownStart(fn) {
  onCooldownStart = fn;
}

export function processEvents(events, characters, casterGUID) {
  const casterGroup = characters.find(c => c.userData.guid == casterGUID);

  let lastSpellID = 0;
  let lastSchoolMask = 0;

  for (const e of events) {
    const span = e.span || '';
    const event = e.event || '';
    const fields = e.fields || {};

    // Track current spell context
    if (span === 'spell') {
      lastSpellID = e.spellId || lastSpellID;
    }

    // Cast preparation glow
    if (span === 'spell' && event === 'prepare') {
      if (casterGroup) {
        const spell = spellMap_entry(e.spellId);
        const color = spell ? (SCHOOL_COLORS[spell.schoolName] || 0x4488ff) : 0x4488ff;
        spawnCastGlow(casterGroup, color);
      }
      continue;
    }

    // Cooldown added → notify HUD
    if (span === 'cooldown' && event === 'add_cooldown') {
      if (onCooldownStart) {
        const spellID = e.spellId || lastSpellID;
        const duration = fields.duration_ms || 0;
        onCooldownStart(spellID, duration);
      }
      continue;
    }

    // Effect launch → spawn projectile / melee arc
    if (span === 'effect_launch' && event === 'launch') {
      const effectType = fields.effectType;
      if (effectType === 6) {
        // Weapon damage — melee arc on all targets (we don't know target yet)
        // Will spawn on hit events instead
      }
      continue;
    }

    // Track school mask from launch events
    if (span === 'effect_launch' && event === 'school_damage_launch') {
      lastSchoolMask = fields.school || 0;
      continue;
    }

    // Effect hit — school damage (also spawn projectile since we now know target)
    if (span === 'effect_hit' && event === 'school_damage_hit') {
      const targetGroup = findCharacterByName(characters, fields.target);
      if (targetGroup && casterGroup) {
        const delay = 800;
        const damage = fields.damage;
        const resultCode = fields.result;
        const schoolName = SCHOOL_MASK_TO_NAME[fields.school] || 'Fire';

        // Spawn projectile (arrives at target when damage number appears)
        setTimeout(() => spawnProjectile(casterGroup, targetGroup, schoolName), 0);

        if (damage != null) {
          setTimeout(() => {
            flashHit(targetGroup);
            spawnDamageNumber(targetGroup, Math.round(damage), resultCode);
          }, delay);
        }
      }
      continue;
    }

    // Effect hit — weapon damage hit
    if (span === 'effect_hit' && event === 'weapon_damage_hit') {
      const targetGroup = findCharacterByName(characters, fields.target);
      if (targetGroup) {
        const delay = 200; // melee is instant
        const damage = fields.totalDamage;
        const resultCode = fields.result;

        if (damage != null) {
          setTimeout(() => {
            spawnMeleeArc(targetGroup);
            flashHit(targetGroup);
            spawnDamageNumber(targetGroup, Math.round(damage), resultCode);
          }, delay);
        }
      }
      continue;
    }

    // Effect hit — weapon damage miss (dodge/parry/block)
    if (span === 'effect_hit' && event === 'weapon_damage_miss') {
      const targetGroup = findCharacterByName(characters, fields.target);
      if (targetGroup) {
        const delay = 200;
        const resultCode = fields.result;

        setTimeout(() => {
          spawnMeleeArc(targetGroup);
          spawnDamageNumber(targetGroup, 0, resultCode);
        }, delay);
      }
      continue;
    }

    // Effect hit — heal
    if (span === 'effect_hit' && event === 'heal_hit') {
      const targetGroup = findCharacterByName(characters, fields.target);
      if (targetGroup && casterGroup) {
        const delay = 400;
        const amount = fields.amount;
        if (amount != null) {
          setTimeout(() => {
            spawnHealBeam(casterGroup, targetGroup);
            spawnHealNumber(targetGroup, amount);
          }, delay);
        }
      }
      continue;
    }

    // Aura applied
    if (span === 'aura' && (event === 'applied' || event === 'refreshed')) {
      // Only show ring on first apply, not refresh
      if (event === 'applied') {
        const targetGroup = findCharacterByName(characters, fields.target);
        if (targetGroup) {
          const schoolName = SCHOOL_MASK_TO_NAME[lastSchoolMask] || 'Arcane';
          const color = SCHOOL_COLORS[schoolName] || 0x44ff44;
          setTimeout(() => spawnAuraRing(targetGroup, color), 500);
        }
      }
      continue;
    }
  }
}

// Cache for spell info keyed by ID
let _spellCache = null;
let _spellCacheCharacters = null;

function spellMap_entry(spellID) {
  // Access the global spellMap from app.js
  return window.__spellMap ? window.__spellMap[spellID] : null;
}

function findCharacterByName(characters, name) {
  if (!name) return null;
  return characters.find(c => c.userData.name === name);
}

// Re-export for backward compatibility
export { RESULT };
