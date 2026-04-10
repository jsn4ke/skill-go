// app.js — Entry point: API calls, scene init, HUD interaction
import { initScene } from './scene.js';
import { createCharacter, updateCharacter, removeCharacter } from './character.js';
import { clearAuraRings } from './vfx.js';
import { processEvents, setOnCooldownStart } from './timeline.js';

window.addEventListener('error', (e) => {
  document.getElementById('canvas-container').innerHTML =
    `<div style="color:red;padding:20px;font-family:monospace;white-space:pre-wrap">${e.message}\n${e.filename}:${e.lineno}</div>`;
});

window.addEventListener('unhandledrejection', (e) => {
  console.error('Unhandled rejection:', e.reason);
});

// ---- State ----
let characters = [];
let spells = [];
let spellMap = {};
const cooldownTimers = {};

// CombatResult enum (must match Go spelldef.CombatResult)
const RESULT_HIT = 0, RESULT_MISS = 1, RESULT_CRIT = 2;

// ---- Stats ----
let stats = { casts: 0, damage: 0, healing: 0, crits: 0, misses: 0 };

// ---- API ----
async function apiGet(path) {
  const resp = await fetch(path);
  if (!resp.ok) throw new Error(`GET ${path}: ${resp.status}`);
  return resp.json();
}

async function apiPost(path, body) {
  const resp = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!resp.ok) throw new Error(`POST ${path}: ${resp.status}`);
  return resp.json();
}

// ---- Init ----
async function init() {
  initScene();

  // Set cooldown callback for timeline
  setOnCooldownStart((spellID, duration) => {
    startCooldown(spellID, duration);
  });

  try {
    const [loadedSpells, units] = await Promise.all([
      apiGet('/api/spells'),
      apiGet('/api/units'),
    ]);
    spells = loadedSpells;
    spellMap = {};
    for (const s of spells) spellMap[s.id] = s;
    window.__spellMap = spellMap; // expose for timeline.js
    renderActionBar(spells);
    createAllCharacters(units);
  } catch (err) {
    console.error('Init failed:', err);
  }
}

// ---- Characters ----
function createAllCharacters(units) {
  // Remove old
  for (const c of characters) removeCharacter(c);
  characters = [];

  for (const u of units) {
    const group = createCharacter(u);
    characters.push(group);
  }
}

function updateAllCharacters(units) {
  for (const u of units) {
    const group = characters.find(c => c.userData.guid == u.guid);
    if (group) updateCharacter(group, u);
  }
}

// ---- Spell cast ----
async function castSpell(spellID) {
  try {
    const result = await apiPost('/api/cast', { spellID, targetIDs: [] });

    if (result.result === 'success') {
      updateAllCharacters(result.units || []);
      processEvents(result.events || [], characters, 1); // caster GUID = 1
      updateStats(result.events || []);
    } else {
      // Cast failed (cooldown, etc) — still update for any partial changes
      if (result.units) updateAllCharacters(result.units);
      // Show error briefly
      console.log('Cast failed:', result.error);
    }
  } catch (err) {
    console.error('Cast error:', err);
  }
}

// ---- Action bar ----
function renderActionBar(spells) {
  const container = document.getElementById('spell-buttons');
  container.innerHTML = '';

  const schoolColorMap = {
    'Fire': '#ff4400',
    'Frost': '#44aaff',
    'Arcane': '#cc44ff',
    'Shadow': '#8844aa',
    'Nature': '#44ff44',
    'Holy': '#ffcc00',
    'Physical': '#cc8844',
  };

  for (const sp of spells) {
    const btn = document.createElement('button');
    btn.className = 'spell-btn';
    btn.id = `spell-btn-${sp.id}`;
    btn.title = `${sp.name} (${sp.schoolName}) — CD: ${sp.cd}ms, Cost: ${sp.powerCost}`;

    const dot = document.createElement('span');
    dot.className = 'school-dot';
    dot.style.backgroundColor = schoolColorMap[sp.schoolName] || '#888';

    const name = document.createElement('span');
    name.className = 'spell-name';
    name.textContent = sp.name;

    btn.appendChild(dot);
    btn.appendChild(name);
    btn.addEventListener('click', () => castSpell(sp.id));
    container.appendChild(btn);
  }
}

// ---- Cooldown ----
function startCooldown(spellID, durationMs) {
  const btn = document.getElementById(`spell-btn-${spellID}`);
  if (!btn || durationMs <= 0) return;

  btn.disabled = true;

  // Remove old overlay if exists
  let overlay = btn.querySelector('.cd-overlay');
  let cdText = btn.querySelector('.cd-text');
  if (!overlay) {
    overlay = document.createElement('div');
    overlay.className = 'cd-overlay';
    btn.appendChild(overlay);
  }
  if (!cdText) {
    cdText = document.createElement('span');
    cdText.className = 'cd-text';
    btn.appendChild(cdText);
  }

  const startTime = performance.now();
  const duration = durationMs;

  if (cooldownTimers[spellID]) clearInterval(cooldownTimers[spellID]);

  cooldownTimers[spellID] = setInterval(() => {
    const elapsed = performance.now() - startTime;
    const remaining = duration - elapsed;

    if (remaining <= 0) {
      clearInterval(cooldownTimers[spellID]);
      delete cooldownTimers[spellID];
      btn.disabled = false;
      overlay.remove();
      cdText.remove();
      return;
    }

    const pct = ((duration - remaining) / duration) * 100;
    overlay.style.setProperty('--cd-pct', pct + '%');

    const secs = Math.ceil(remaining / 1000);
    cdText.textContent = secs;
  }, 50);
}

function clearAllCooldowns() {
  for (const id in cooldownTimers) {
    clearInterval(cooldownTimers[id]);
    delete cooldownTimers[id];
  }
  document.querySelectorAll('.spell-btn').forEach(btn => {
    btn.disabled = false;
    btn.querySelector('.cd-overlay')?.remove();
    btn.querySelector('.cd-text')?.remove();
  });
}

// ---- Stats ----
function updateStats(events) {
  for (const e of events) {
    if (!e.fields) continue;
    const fields = e.fields;
    const span = e.span || '';
    const event = e.event || '';

    // School damage hits: {damage, result}
    if (span === 'effect_hit' && event === 'school_damage_hit') {
      if (fields.damage != null) {
        stats.casts++;
        stats.damage += Math.round(fields.damage);
        const result = typeof fields.result === 'number' ? fields.result : -1;
        if (result === RESULT_CRIT) stats.crits++;
        if (result === RESULT_MISS) stats.misses++;
      }
    }

    // Weapon damage hits: {totalDamage, result}
    if (span === 'effect_hit' && event === 'weapon_damage_hit') {
      if (fields.totalDamage != null) {
        stats.casts++;
        stats.damage += Math.round(fields.totalDamage);
        const result = typeof fields.result === 'number' ? fields.result : -1;
        if (result === RESULT_CRIT) stats.crits++;
        if (result === RESULT_MISS) stats.misses++;
      }
    }

    // Weapon misses (dodge/parry/block)
    if (span === 'effect_hit' && event === 'weapon_damage_miss') {
      stats.casts++;
      stats.misses++;
    }

    // Heal hits: {amount}
    if (span === 'effect_hit' && event === 'heal_hit') {
      if (fields.amount != null) {
        stats.healing += Math.round(fields.amount);
      }
    }
  }

  const critRate = stats.casts > 0 ? ((stats.crits / stats.casts) * 100).toFixed(1) : '0';
  document.getElementById('stat-casts').textContent = stats.casts;
  document.getElementById('stat-damage').textContent = stats.damage;
  document.getElementById('stat-healing').textContent = stats.healing;
  document.getElementById('stat-crits').textContent = stats.crits;
  document.getElementById('stat-misses').textContent = stats.misses;
  document.getElementById('stat-critrate').textContent = critRate + '%';
}

function resetStats() {
  stats = { casts: 0, damage: 0, healing: 0, crits: 0, misses: 0 };
  document.getElementById('stat-casts').textContent = '0';
  document.getElementById('stat-damage').textContent = '0';
  document.getElementById('stat-healing').textContent = '0';
  document.getElementById('stat-crits').textContent = '0';
  document.getElementById('stat-misses').textContent = '0';
  document.getElementById('stat-critrate').textContent = '0%';
}

// ---- Reset ----
async function resetSession() {
  try {
    await apiPost('/api/reset', null);
    clearAllCooldowns();
    resetStats();

    // Clear aura rings
    for (const c of characters) clearAuraRings(c);

    // Reload units to restore HP/MP
    const units = await apiGet('/api/units');
    updateAllCharacters(units);
  } catch (err) {
    console.error('Reset error:', err);
  }
}

// Expose to HTML onclick
window.__resetSession = resetSession;

// ---- Start ----
init();
