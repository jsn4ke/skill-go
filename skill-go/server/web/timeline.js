// timeline.js — Maps trace events to 3D animations with proper sequencing
import { spawnProjectile, spawnMeleeArc, spawnDamageNumber, spawnHealNumber, spawnHealBeam, flashHit, spawnCastGlow, spawnAuraRing, SCHOOL_COLORS } from './vfx.js';
import { moveUnit } from './character.js';

const CHARGE_SPEED = 40;

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

export function processEvents(events, characters, casterGUID, selectedTargetGUID) {
  const casterGroup = characters.find(c => c.userData.guid == casterGUID);

  let lastSpellID = 0;
  let lastSchoolMask = 0;
  let lastEffectType = 0;

  // Determine which target name to show VFX for
  const targetFilter = selectedTargetGUID
    ? characters.find(c => c.userData.guid == selectedTargetGUID)
    : null;

  for (const e of events) {
    const span = e.span || '';
    const event = e.event || '';
    const fields = e.fields || {};

    try {
      // Track current spell context
      if (span === 'spell') {
        lastSpellID = e.spellId || lastSpellID;
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

      // Effect launch — spawn projectile / melee arc
      if (span === 'effect_launch' && event === 'launch') {
        lastEffectType = fields.effectType || 0;
        continue;
      }

      // Track school mask from launch events
      if (span === 'effect_launch' && event === 'school_damage_launch') {
        lastSchoolMask = fields.school || 0;
        continue;
      }

      // Effect hit — school damage
      if (span === 'effect_hit' && event === 'school_damage_hit') {
        const targetName = fields.target;
        const targetGroup = findCharacterByName(characters, targetName);

        // Filter: if a target is selected, only show VFX for that target
        if (targetFilter && targetGroup && targetGroup !== targetFilter) continue;

        if (targetGroup && casterGroup) {
          const delay = 800;
          const damage = fields.damage;
          const resultCode = fields.result;
          const schoolName = SCHOOL_MASK_TO_NAME[fields.school] || 'Fire';

          // Skip projectile for channeled AoE spells (e.g. Blizzard spell=10).
          // For these spells, damage comes from the area effect, not a traveling projectile.
          // Spawning a projectile causes visual desync since HP is updated immediately
          // via SSE but the projectile takes 600ms+ to arrive.
          const isChanneledAoE = lastSpellID === 10; // Blizzard

          if (!isChanneledAoE) {
            spawnProjectile(casterGroup, targetGroup, schoolName);
          }

          if (damage != null) {
            setTimeout(() => {
              flashHit(targetGroup);
              spawnDamageNumber(targetGroup, Math.round(damage), resultCode);
            }, isChanneledAoE ? 0 : delay);
          }
        } else {
          console.warn('[timeline] school_damage_hit: targetGroup or casterGroup not found. targetName=', targetName, 'casterGroup=', !!casterGroup);
        }
        continue;
      }

      // Effect hit — weapon damage hit
      if (span === 'effect_hit' && event === 'weapon_damage_hit') {
        const targetGroup = findCharacterByName(characters, fields.target);

        if (targetFilter && targetGroup && targetGroup !== targetFilter) continue;

        if (targetGroup) {
          const delay = 200;
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

      // Effect hit — weapon damage miss
      if (span === 'effect_hit' && event === 'weapon_damage_miss') {
        const targetGroup = findCharacterByName(characters, fields.target);

        if (targetFilter && targetGroup && targetGroup !== targetFilter) continue;

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

        if (targetFilter && targetGroup && targetGroup !== targetFilter) continue;

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
      if (span === 'aura' && event === 'applied') {
        console.log('[DEBUG] aura.applied event, fields:', fields, 'chars:', characters.length);
        const targetGroup = findCharacterByName(characters, fields.target);
        console.log('[DEBUG] findCharacterByName("Target Dummy"):', targetGroup ? targetGroup.userData.name : 'NOT FOUND');
        if (targetGroup) {
          const schoolName = SCHOOL_MASK_TO_NAME[lastSchoolMask] || 'Arcane';
          const color = SCHOOL_COLORS[schoolName] || 0x44ff44;
          setTimeout(() => spawnAuraRing(targetGroup, color), 500);
        }
        continue;
      }

      // Charge teleport — smooth sprint instead of instant snap
      if (span === 'effect_hit' && event === 'charge_teleport') {
        const casterName = fields.caster;
        const casterChar = findCharacterByName(characters, casterName);
        if (casterChar && fields.newPos) {
          moveUnit(casterChar, fields.newPos.x, fields.newPos.z, CHARGE_SPEED);
        }
        continue;
      }
    } catch (err) {
      console.error('[timeline] error processing event:', span, event, err);
    }
  }
}

function spellMap_entry(spellID) {
  return window.__spellMap ? window.__spellMap[spellID] : null;
}

function findCharacterByName(characters, name) {
  if (!name) return null;
  return characters.find(c => c.userData.name === name);
}

export { RESULT };
