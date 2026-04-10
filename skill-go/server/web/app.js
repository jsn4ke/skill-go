// app.js — Entry point: targeting, movement, HUD, spell log
import * as THREE from 'three';
import { initScene, setMouseFromEvent, raycastCharacters, raycastGround, addUpdatable, getScene, getGridAndFloor } from './scene.js';
import { createCharacter, updateCharacter, removeCharacter, moveUnit, updateUnitMovement, getCharacterMeshes } from './character.js';
import { clearAuraRings, createSelectionRing, updateSelectionRingPosition, removeSelectionRing } from './vfx.js';
import { processEvents, setOnCooldownStart } from './timeline.js';

window.addEventListener('error', (e) => {
  document.getElementById('canvas-container').innerHTML =
    `<div style="color:red;padding:20px;font-family:monospace;white-space:pre-wrap">${e.message}\n${e.filename}:${e.lineno}</div>`;
});

// ---- State ----
let characters = [];
let spells = [];
let spellMap = {};
const cooldownTimers = {};
let selectedTargetGUID = null;
let targetingSpellID = null;
const CASTER_GUID = 1;
let spellLogCounter = 0;
let serverLogCounter = 0;
const RESULT_CRIT = 2, RESULT_MISS = 1;
let stats = { casts: 0, damage: 0, healing: 0, crits: 0, misses: 0 };

// ---- API ----
async function apiGet(path) {
  const resp = await fetch(path);
  if (!resp.ok) throw new Error(`GET ${path}: ${resp.status}`);
  return resp.json();
}
async function apiPost(path, body) {
  const resp = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: body ? JSON.stringify(body) : undefined });
  if (!resp.ok) throw new Error(`POST ${path}: ${resp.status}`);
  return resp.json();
}
async function apiDelete(path) {
  const resp = await fetch(path, { method: 'DELETE' });
  if (!resp.ok) throw new Error(`DELETE ${path}: ${resp.status}`);
  return resp.json();
}

// ---- Draggable & Collapsible Panels ----
function initPanel(panelId) {
  const panel = document.getElementById(panelId);
  if (!panel) return;
  const titleBar = panel.querySelector('.panel-title-bar');
  const body = panel.querySelector('.panel-body');
  const collapseBtn = panel.querySelector('.panel-collapse-btn');
  if (!titleBar || !body) return;

  // Collapse toggle
  let collapsed = false;
  const toggleCollapse = () => {
    collapsed = !collapsed;
    body.style.display = collapsed ? 'none' : '';
    panel.classList.toggle('collapsed', collapsed);
    if (collapseBtn) collapseBtn.textContent = collapsed ? '+' : '\u2212';
  };
  if (collapseBtn) collapseBtn.addEventListener('click', (e) => { e.stopPropagation(); toggleCollapse(); });
  const titleText = titleBar.querySelector('.panel-title-text');
  if (titleText) titleText.addEventListener('click', (e) => { e.stopPropagation(); toggleCollapse(); });

  // Drag
  let isDragging = false, offsetX = 0, offsetY = 0;
  titleBar.addEventListener('mousedown', (e) => {
    if (e.target.closest('.panel-collapse-btn')) return;
    isDragging = true;
    panel.classList.add('dragging');
    const rect = panel.getBoundingClientRect();
    offsetX = e.clientX - rect.left;
    offsetY = e.clientY - rect.top;
    e.preventDefault();
  });
  document.addEventListener('mousemove', (e) => {
    if (!isDragging) return;
    let newLeft = e.clientX - offsetX;
    let newTop = e.clientY - offsetY;
    // Viewport clamping
    const maxLeft = window.innerWidth - panel.offsetWidth;
    const maxTop = window.innerHeight - panel.offsetHeight;
    newLeft = Math.max(0, Math.min(newLeft, maxLeft));
    newTop = Math.max(0, Math.min(newTop, maxTop));
    panel.style.left = newLeft + 'px';
    panel.style.top = newTop + 'px';
    panel.style.right = 'auto';
    panel.style.bottom = 'auto';
  });
  document.addEventListener('mouseup', () => {
    if (!isDragging) return;
    isDragging = false;
    panel.classList.remove('dragging');
  });
}

// ---- Init ----
async function init() {
  initScene();
  setOnCooldownStart((spellID, duration) => startCooldown(spellID, duration));
  addUpdatable({ update(dt) { for (const c of characters) updateUnitMovement(c, dt); updateSelectionRingPosition(); } });

  // Initialize draggable/collapsible panels
  initPanel('spell-log');
  initPanel('self-panel');
  initPanel('spawn-panel');
  initPanel('enemy-panel');
  initPanel('server-log-panel');

  const canvas = document.querySelector('#canvas-container canvas');
  canvas.addEventListener('click', onCanvasClick);

  try {
    const [loadedSpells, units] = await Promise.all([apiGet('/api/spells'), apiGet('/api/units')]);
    spells = loadedSpells;
    spellMap = {};
    for (const s of spells) spellMap[s.id] = s;
    window.__spellMap = spellMap;
    renderActionBar(spells);
    createAllCharacters(units);
    // Remove dead non-caster units (server returns them but they shouldn't be visible)
    for (const u of units) {
      if (!u.alive && u.guid !== CASTER_GUID) {
        const g = characters.find(c => c.userData.guid == u.guid);
        if (g) { clearAuraRings(g); removeCharacter(g); characters.splice(characters.indexOf(g), 1); }
      }
    }
    updateSelfPanel(units.find(u => u.guid == CASTER_GUID));
    updateEnemyDropdown(units);
  } catch (err) {
    console.error('Init failed:', err);
  }
}

// ---- Characters ----
function createAllCharacters(units) {
  for (const c of characters) removeCharacter(c);
  characters = [];
  for (const u of units) { characters.push(createCharacter(u)); }
}

function updateAllCharacters(units) {
  for (const u of units) {
    const g = characters.find(c => c.userData.guid == u.guid);
    if (g) updateCharacter(g, u);
  }
}

function reconcileUnits(apiUnits) {
  const apiGUIDs = new Set(apiUnits.map(u => String(u.guid)));
  // Only create alive units — skip dead non-caster units
  for (const u of apiUnits) {
    if (!u.alive && u.guid !== CASTER_GUID) continue;
    if (!characters.find(c => String(c.userData.guid) === String(u.guid))) characters.push(createCharacter(u));
  }
  for (const c of [...characters]) { if (!apiGUIDs.has(String(c.userData.guid))) { clearAuraRings(c); removeCharacter(c); characters.splice(characters.indexOf(c), 1); if (selectedTargetGUID == c.userData.guid) deselectTarget(); } }
  // Remove dead units that somehow remain in characters array
  for (const c of [...characters]) {
    if (c.userData.unitData && !c.userData.unitData.alive && c.userData.guid !== CASTER_GUID) {
      clearAuraRings(c);
      removeCharacter(c);
      characters.splice(characters.indexOf(c), 1);
    }
  }
  updateAllCharacters(apiUnits);
}

// ---- Targeting Mode ----
function enterTargetingMode(spellID) {
  targetingSpellID = spellID;
  const spell = spellMap[spellID];
  document.getElementById('targeting-spell').textContent = spell ? spell.name : 'Spell';
  document.getElementById('targeting-indicator').classList.remove('hidden');
  document.querySelector('#canvas-container canvas').style.cursor = 'crosshair';
  document.querySelectorAll('.spell-btn').forEach(b => b.classList.remove('active'));
  const btn = document.getElementById(`spell-btn-${spellID}`);
  if (btn) btn.classList.add('active');
}

function exitTargetingMode() {
  targetingSpellID = null;
  document.getElementById('targeting-indicator').classList.add('hidden');
  document.querySelector('#canvas-container canvas').style.cursor = '';
  document.querySelectorAll('.spell-btn').forEach(b => b.classList.remove('active'));
}

// ---- Selection ----
function hideTargetUI() {
  selectedTargetGUID = null;
  const ei = document.getElementById('enemy-info');
  if (ei) { ei.hidden = true; ei.style.display = 'none'; ei.classList.add('hidden'); }
  const tl = document.getElementById('target-lock');
  if (tl) { tl.hidden = true; tl.style.display = 'none'; tl.classList.add('hidden'); }
  const es = document.getElementById('enemy-select');
  if (es) es.value = '';
  try { removeSelectionRing(); } catch(e) {}
}

function showTargetUI() {
  const ei = document.getElementById('enemy-info');
  if (ei) { ei.hidden = false; ei.style.display = ''; ei.classList.remove('hidden'); }
  const tl = document.getElementById('target-lock');
  if (tl) { tl.hidden = false; tl.style.display = ''; tl.classList.remove('hidden'); }
}

function selectTarget(guid) {
  selectedTargetGUID = guid;
  const group = characters.find(c => c.userData.guid == guid);
  if (!group) return;
  const isEnemy = guid !== CASTER_GUID;
  createSelectionRing(group, isEnemy);
  const data = group.userData.unitData;
  updateTargetLock(data);
  if (isEnemy) {
    document.getElementById('enemy-select').value = String(guid);
    updateEnemyInfo(data);
  } else {
    document.getElementById('enemy-select').value = '';
    hideTargetUI();
  }
}

function deselectTarget() {
  hideTargetUI();
}

function updateTargetLock(data) {
  if (!data || !data.alive) { hideTargetUI(); return; }
  const tl = document.getElementById('target-lock');
  if (tl) { tl.hidden = false; tl.style.display = ''; tl.classList.remove('hidden'); }
  document.getElementById('target-lock-name').textContent = data.name || 'Unknown';
  document.getElementById('target-lock-level').textContent = `Lv${data.level || 0}`;
  const hp = data.health ?? 0;
  const maxHp = data.maxHealth ?? 1;
  const hpPct = maxHp > 0 ? (hp / maxHp) * 100 : 0;
  document.getElementById('target-lock-hp-bar').style.width = Math.max(0, hpPct) + '%';
  document.getElementById('target-lock-hp-bar').className = 'target-lock-hp-fill hp-' + hpTier(hpPct);
  document.getElementById('target-lock-hp-text').textContent = `${hp}/${maxHp}`;

  // Portrait icon based on unit type
  const icon = document.getElementById('target-lock-portrait-icon');
  const name = (data.name || '').toLowerCase();
  if (name.includes('boss')) { icon.textContent = 'skull'; icon.style.color = '#aa00ff'; }
  else if (name.includes('elite')) { icon.textContent = 'elite'; icon.style.color = '#ff8800'; }
  else if (name.includes('mage')) { icon.textContent = 'mage'; icon.style.color = '#44aaff'; }
  else { icon.textContent = 'target'; icon.style.color = '#e94560'; }
}

// ---- Canvas Click ----
function onCanvasClick(event) {
  setMouseFromEvent(event);
  const allMeshes = [];
  for (const c of characters) allMeshes.push(...getCharacterMeshes(c));
  const charHit = raycastCharacters(allMeshes);

  // Priority 1: targeting mode → cast at clicked character
  if (targetingSpellID) {
    if (charHit) {
      const guid = charHit.group.userData.guid;
      selectTarget(guid);
      const spellID = targetingSpellID;
      exitTargetingMode();
      castSpell(spellID, guid);
      return;
    }
    exitTargetingMode();
    return;
  }

  // Priority 2: character click → select (including caster)
  if (charHit) {
    const guid = charHit.group.userData.guid;
    // Don't select dead/removed units
    const data = charHit.group.userData.unitData;
    if (data && !data.alive) return;
    if (charHit.group.userData.removed) return;
    selectTarget(guid);
    return;
  }

  // Priority 3: ground click → move selected target
  const groundHit = raycastGround();
  if (groundHit && selectedTargetGUID) {
    const group = characters.find(c => c.userData.guid == selectedTargetGUID);
    if (group) {
      moveUnit(group, groundHit.x, groundHit.z);
      apiPost('/api/units/move', { guid: selectedTargetGUID, x: groundHit.x, z: groundHit.z }).catch(() => {});
    }
    return;
  }
}

// ---- Spell Cast ----
async function castSpell(spellID, targetGUID) {
  try {
    const targetIDs = targetGUID ? [targetGUID] : [];
    const result = await apiPost('/api/cast', { spellID, targetIDs });
    if (result.units) reconcileUnits(result.units);

    // Handle dead targets FIRST — deselect before updating any UI
    let targetDied = false;
    if (result.units) {
      for (const u of result.units) {
        if (!u.alive && u.guid !== CASTER_GUID) {
          if (selectedTargetGUID != null && u.guid == selectedTargetGUID) {
            targetDied = true;
            deselectTarget();
          }
        }
      }
    }

    if (result.result === 'success') {
      processEvents(result.events || [], characters, CASTER_GUID, targetGUID);
      updateStats(result.events || []);
      updateSelfPanel(result.units?.find(u => u.guid == CASTER_GUID));
      if (!targetDied && targetGUID) {
        const targetData = result.units?.find(u => u.guid == targetGUID);
        if (targetData) { updateEnemyInfo(targetData); updateTargetLock(targetData); }
      }
      addSpellLogEntry(result.events || [], spellMap[spellID]);
      addServerLogEntry(result.events || [], spellMap[spellID]);
    } else {
      console.log('Cast failed:', result.error);
    }

    // Auto-remove dead units — immediately, no delay
    if (result.units) {
      for (const u of result.units) {
        if (!u.alive && u.guid !== CASTER_GUID) {
          const group = characters.find(c => c.userData.guid == u.guid);
          if (group && !group.userData.removed) {
            group.userData.removed = true;
            // Remove from dropdown
            const opt = document.querySelector(`#enemy-select option[value="${u.guid}"]`);
            if (opt) opt.remove();
            // Remove from scene immediately
            clearAuraRings(group);
            removeCharacter(group);
            const idx = characters.indexOf(group);
            if (idx >= 0) characters.splice(idx, 1);
          }
        }
      }
    }
  } catch (err) { console.error('Cast error:', err); }
}

// ---- Self Panel ----
function updateSelfPanel(u) {
  if (!u) return;
  document.getElementById('self-name').textContent = u.name;
  document.getElementById('self-level').textContent = `Lv${u.level}`;
  const hpPct = u.maxHealth > 0 ? (u.health / u.maxHealth) * 100 : 0;
  document.getElementById('self-hp-bar').style.width = Math.max(0, hpPct) + '%';
  document.getElementById('self-hp-bar').className = 'panel-bar-fill hp-' + hpTier(hpPct);
  document.getElementById('self-hp-val').textContent = `${u.health}/${u.maxHealth}`;
  const mpPct = u.maxMana > 0 ? (u.mana / u.maxMana) * 100 : 0;
  document.getElementById('self-mp-bar').style.width = Math.max(0, mpPct) + '%';
  document.getElementById('self-mp-bar').className = 'panel-bar-fill mp-' + hpTier(mpPct);
  document.getElementById('self-mp-val').textContent = `${u.mana}/${u.maxMana}`;
  document.getElementById('self-sp').textContent = u.spellPower;
  document.getElementById('self-crit').textContent = u.critSpell + '%';
  document.getElementById('self-hit').textContent = u.hitSpell + '%';
}

// ---- Enemy Panel ----
function updateEnemyDropdown(units) {
  const select = document.getElementById('enemy-select');
  select.innerHTML = '<option value="">-- Select Target --</option>';
  for (const u of units) {
    if (u.guid == CASTER_GUID || !u.alive) continue;
    const opt = document.createElement('option');
    opt.value = String(u.guid);
    opt.textContent = `${u.name} (Lv${u.level})`;
    select.appendChild(opt);
  }
  if (selectedTargetGUID) select.value = String(selectedTargetGUID);
}

function updateEnemyInfo(data) {
  if (!data || !data.alive) { hideTargetUI(); return; }
  const ei = document.getElementById('enemy-info');
  if (ei) { ei.hidden = false; ei.style.display = ''; ei.classList.remove('hidden'); }
  document.getElementById('enemy-name').textContent = data.name || 'Unknown';
  document.getElementById('enemy-level').textContent = `Lv${data.level || 0}`;
  const hp = data.health ?? 0;
  const maxHp = data.maxHealth ?? 1;
  const hpPct = maxHp > 0 ? (hp / maxHp) * 100 : 0;
  document.getElementById('enemy-hp-bar').style.width = Math.max(0, hpPct) + '%';
  document.getElementById('enemy-hp-bar').className = 'panel-bar-fill hp-' + hpTier(hpPct);
  document.getElementById('enemy-hp-val').textContent = `${hp}/${maxHp}`;
  document.getElementById('enemy-armor').textContent = data.armor || 0;
  const r = data.resistances || {};
  document.getElementById('enemy-fire').textContent = Math.round(r.Fire || 0);
  document.getElementById('enemy-frost').textContent = Math.round(r.Frost || 0);
  document.getElementById('enemy-shadow').textContent = Math.round(r.Shadow || 0);
}

function hpTier(pct) { return pct < 25 ? 'low' : pct < 50 ? 'medium' : 'high'; }

// ---- Spell Log (client-side visual) ----
function addSpellLogEntry(events, spell) {
  spellLogCounter++;
  const container = document.getElementById('spell-log-entries');

  let targetName = '?';
  let totalDmg = 0, isCrit = false, isMiss = false, isHeal = false, healAmount = 0;

  for (const e of events) {
    const f = e.fields || {};
    if (e.span === 'effect_hit') {
      if (e.event === 'school_damage_hit' || e.event === 'weapon_damage_hit') {
        targetName = f.target || targetName;
        const dmg = f.damage || f.totalDamage || 0;
        totalDmg += dmg;
        if (f.result === 2) isCrit = true;
      }
      if (e.event === 'weapon_damage_miss') { targetName = f.target || targetName; isMiss = true; }
      if (e.event === 'heal_hit') { targetName = f.target || targetName; healAmount += f.amount || 0; isHeal = true; }
    }
  }

  const entry = document.createElement('div');
  entry.className = 'log-entry';

  let resultClass = '', resultText = '';
  if (isMiss) { resultClass = 'log-miss'; resultText = 'MISS'; }
  else if (isCrit) { resultClass = 'log-crit'; resultText = `${totalDmg} CRIT`; }
  else if (isHeal) { resultClass = 'log-heal'; resultText = `+${healAmount}`; }
  else { resultText = `${totalDmg}`; }

  entry.innerHTML = `
    <div class="log-header ${resultClass}">
      <span><span class="log-spell">${spell ? spell.name : '?'}</span> &rarr; ${targetName}</span>
      <span><span class="log-result">${resultText}</span> <span class="log-toggle">&uarr;</span></span>
    </div>
    <div class="log-details">${formatLogDetails(events)}</div>
  `;

  entry.querySelector('.log-header').addEventListener('click', () => entry.classList.toggle('open'));
  container.prepend(entry);

  while (container.children.length > 20) container.lastChild.remove();
}

function formatLogDetails(events) {
  return events
    .filter(e => e.span === 'combat' || e.span === 'effect_hit')
    .map(e => {
      const f = e.fields || {};
      const parts = [];
      if (e.event === 'spell_roll' && f.result) parts.push(`roll: ${f.result} (${f.roll})`);
      if (e.event === 'melee_roll' && f.result != null) {
        const names = { 0: 'hit', 1: 'miss', 2: 'crit', 3: 'dodge', 4: 'parry', 5: 'block', 6: 'glancing' };
        parts.push(`melee: ${names[f.result] || f.result}`);
      }
      if (e.event === 'damage_calc') parts.push(`dmg: ${Math.round(f.finalDamage || 0)}`);
      if (e.event === 'resist_reduction') parts.push(`resist: -${Math.round((f.avgReduction || 0) * 100)}%`);
      if (e.event === 'armor_mitigation') parts.push(`armor: -${Math.round((f.reduction || 0) * 100)}%`);
      return parts.join(' | ');
    })
    .filter(Boolean)
    .join('\n');
}

// ---- Server Spell Trace Log ----
function addServerLogEntry(events, spell) {
  if (!events || events.length === 0) return;
  serverLogCounter++;
  const container = document.getElementById('server-log-entries');

  // Cast header
  const header = document.createElement('div');
  header.className = 'slog-entry';
  header.setAttribute('data-span', 'spell');
  header.innerHTML = `<div class="slog-header"><span class="slog-event" style="color:#cc44ff;font-weight:bold">#${serverLogCounter} ${spell ? spell.name : '?'}</span><span class="slog-span">spell</span></div>`;
  container.prepend(header);

  // Each event as a line
  for (const e of events) {
    const entry = document.createElement('div');
    entry.className = 'slog-entry';
    entry.setAttribute('data-span', e.span || '');
    const fields = e.fields || {};
    const fieldParts = Object.entries(fields)
      .filter(([k]) => k !== 'target')
      .map(([k, v]) => `${k}=${typeof v === 'number' ? Math.round(v) : v}`);
    const targetName = fields.target ? ` [${fields.target}]` : '';
    entry.innerHTML = `
      <div class="slog-header">
        <span class="slog-event">${e.event || '?'}${targetName}</span>
        <span class="slog-span">${e.span || ''}</span>
      </div>
      ${fieldParts.length ? `<div class="slog-fields">${fieldParts.join(', ')}</div>` : ''}
    `;
    container.prepend(entry);
  }

  while (container.children.length > 100) container.lastChild.remove();
}

// ---- Action Bar ----
const SCHOOL_ICONS = {
  Fire: `<svg viewBox="0 0 32 32" width="36" height="36"><path d="M16 2c0 8-8 12-8 18a8 8 0 0016 0c0-6-8-10-8-18z" fill="#ff4400"/><path d="M16 10c0 4-4 6-4 9a4 4 0 008 0c0-3-4-5-4-9z" fill="#ffaa00"/><path d="M16 16c0 2-2 3-2 4.5a2 2 0 004 0c0-1.5-2-2.5-2-4.5z" fill="#ffe066"/></svg>`,
  Frost: `<svg viewBox="0 0 32 32" width="36" height="36"><line x1="16" y1="2" x2="16" y2="30" stroke="#44aaff" stroke-width="2"/><line x1="2" y1="16" x2="30" y2="16" stroke="#44aaff" stroke-width="2"/><line x1="6" y1="6" x2="26" y2="26" stroke="#88ccff" stroke-width="1.5"/><line x1="26" y1="6" x2="6" y2="26" stroke="#88ccff" stroke-width="1.5"/><circle cx="16" cy="16" r="3" fill="#aaddff"/><circle cx="16" cy="4" r="2" fill="#88ccff"/><circle cx="16" cy="28" r="2" fill="#88ccff"/><circle cx="4" cy="16" r="2" fill="#88ccff"/><circle cx="28" cy="16" r="2" fill="#88ccff"/></svg>`,
  Arcane: `<svg viewBox="0 0 32 32" width="36" height="36"><polygon points="16,2 20,12 30,12 22,18 25,28 16,22 7,28 10,18 2,12 12,12" fill="#cc44ff" opacity="0.8"/><circle cx="16" cy="16" r="4" fill="#ee88ff"/></svg>`,
  Shadow: `<svg viewBox="0 0 32 32" width="36" height="36"><ellipse cx="16" cy="18" rx="10" ry="12" fill="#8844aa" opacity="0.8"/><circle cx="12" cy="14" r="3" fill="#aa66cc"/><circle cx="20" cy="14" r="3" fill="#aa66cc"/><circle cx="12" cy="14" r="1.5" fill="#220033"/><circle cx="20" cy="14" r="1.5" fill="#220033"/><path d="M12 24 Q16 28 20 24" stroke="#6633aa" stroke-width="1.5" fill="none"/></svg>`,
  Nature: `<svg viewBox="0 0 32 32" width="36" height="36"><path d="M16 28 Q16 16 8 10 Q12 16 10 20 Q16 14 22 8 Q20 14 22 18 Q26 12 24 6 Q28 14 24 22 Q20 18 16 28z" fill="#44cc44"/><path d="M16 28 Q16 20 20 14" stroke="#88ff88" stroke-width="1" fill="none"/></svg>`,
  Holy: `<svg viewBox="0 0 32 32" width="36" height="36"><circle cx="16" cy="16" r="12" fill="none" stroke="#ffcc00" stroke-width="2"/><circle cx="16" cy="16" r="8" fill="#ffcc00" opacity="0.3"/><circle cx="16" cy="16" r="4" fill="#ffee88" opacity="0.6"/><line x1="16" y1="2" x2="16" y2="30" stroke="#ffcc00" stroke-width="1" opacity="0.5"/><line x1="2" y1="16" x2="30" y2="16" stroke="#ffcc00" stroke-width="1" opacity="0.5"/></svg>`,
  Physical: `<svg viewBox="0 0 32 32" width="36" height="36"><rect x="6" y="8" width="20" height="4" rx="1" fill="#cc8844" transform="rotate(-30 16 16)"/><rect x="6" y="8" width="20" height="4" rx="1" fill="#cc8844" transform="rotate(30 16 16)"/><rect x="6" y="20" width="20" height="4" rx="1" fill="#cc8844"/></svg>`,
};

function renderActionBar(spells) {
  const container = document.getElementById('spell-buttons');
  container.innerHTML = '';

  for (const sp of spells) {
    const btn = document.createElement('button');
    btn.className = 'spell-btn' + (sp.id !== 1001 ? ' greyed' : '');
    btn.id = `spell-btn-${sp.id}`;
    btn.title = `${sp.name} (${sp.schoolName})`;
    if (sp.id !== 1001) btn.disabled = true;

    // Spell icon
    const iconWrap = document.createElement('div');
    iconWrap.className = 'spell-icon';
    iconWrap.innerHTML = SCHOOL_ICONS[sp.schoolName] || SCHOOL_ICONS.Physical;
    btn.appendChild(iconWrap);

    const name = document.createElement('span');
    name.className = 'spell-name';
    name.textContent = sp.name;
    btn.appendChild(name);

    btn.addEventListener('click', () => {
      if (sp.id !== 1001) return;
      if (selectedTargetGUID) {
        castSpell(sp.id, selectedTargetGUID);
      } else {
        enterTargetingMode(sp.id);
      }
    });
    container.appendChild(btn);
  }
}

// ---- Cooldown ----
function startCooldown(spellID, durationMs) {
  const btn = document.getElementById(`spell-btn-${spellID}`);
  if (!btn || durationMs <= 0) return;
  btn.disabled = true;
  let overlay = btn.querySelector('.cd-overlay');
  let cdText = btn.querySelector('.cd-text');
  if (!overlay) { overlay = document.createElement('div'); overlay.className = 'cd-overlay'; btn.appendChild(overlay); }
  if (!cdText) { cdText = document.createElement('span'); cdText.className = 'cd-text'; btn.appendChild(cdText); }
  const startTime = performance.now();
  const duration = durationMs;
  if (cooldownTimers[spellID]) clearInterval(cooldownTimers[spellID]);
  cooldownTimers[spellID] = setInterval(() => {
    const remaining = duration - (performance.now() - startTime);
    if (remaining <= 0) { clearInterval(cooldownTimers[spellID]); delete cooldownTimers[spellID]; btn.disabled = (spellID !== 1001); overlay.remove(); cdText.remove(); return; }
    overlay.style.setProperty('--cd-pct', ((duration - remaining) / duration * 100) + '%');
    cdText.textContent = Math.ceil(remaining / 1000);
  }, 50);
}

function clearAllCooldowns() {
  for (const id in cooldownTimers) { clearInterval(cooldownTimers[id]); delete cooldownTimers[id]; }
  document.querySelectorAll('.spell-btn').forEach(btn => { if (btn.id !== 'spell-btn-1001') btn.disabled = true; else btn.disabled = false; btn.querySelector('.cd-overlay')?.remove(); btn.querySelector('.cd-text')?.remove(); });
}

// ---- Stats ----
function updateStats(events) {
  for (const e of events) {
    if (!e.fields) continue;
    const f = e.fields, s = e.span || '', ev = e.event || '';
    if (s === 'effect_hit' && ev === 'school_damage_hit' && f.damage != null) { stats.casts++; stats.damage += Math.round(f.damage); if (f.result === RESULT_CRIT) stats.crits++; if (f.result === RESULT_MISS) stats.misses++; }
    if (s === 'effect_hit' && ev === 'weapon_damage_hit' && f.totalDamage != null) { stats.casts++; stats.damage += Math.round(f.totalDamage); if (f.result === RESULT_CRIT) stats.crits++; if (f.result === RESULT_MISS) stats.misses++; }
    if (s === 'effect_hit' && ev === 'weapon_damage_miss') { stats.casts++; stats.misses++; }
    if (s === 'effect_hit' && ev === 'heal_hit' && f.amount != null) { stats.healing += Math.round(f.amount); }
  }
  const cr = stats.casts > 0 ? ((stats.crits / stats.casts) * 100).toFixed(1) : '0';
  document.getElementById('stat-casts').textContent = stats.casts;
  document.getElementById('stat-damage').textContent = stats.damage;
  document.getElementById('stat-healing').textContent = stats.healing;
  document.getElementById('stat-crits').textContent = stats.crits;
  document.getElementById('stat-critrate').textContent = cr + '%';
}
function resetStats() {
  stats = { casts: 0, damage: 0, healing: 0, crits: 0, misses: 0 };
  ['stat-casts','stat-damage','stat-healing','stat-crits','stat-critrate'].forEach(id => document.getElementById(id).textContent = id.includes('rate') ? '0%' : '0');
}

// ---- Spawn / Remove ----
async function spawnUnit(name) {
  try {
    const units = await apiPost('/api/units/add', { name });
    reconcileUnits(units);
    updateEnemyDropdown(units);
  } catch (err) { console.error('Spawn error:', err); }
}

async function removeSelectedTarget() {
  if (!selectedTargetGUID) return;
  const guid = selectedTargetGUID;
  try {
    const units = await apiDelete(`/api/units/${guid}`);
    reconcileUnits(units);
    updateEnemyDropdown(units);
    // Force-hide target UI AFTER reconcile — safety net
    hideTargetUI();
  } catch (err) { console.error('Remove error:', err); }
}

async function adjustLevel(delta) {
  if (!selectedTargetGUID) return;
  const group = characters.find(c => c.userData.guid == selectedTargetGUID);
  if (!group) return;
  const newLevel = Math.max(1, Math.min(83, (group.userData.level || 60) + delta));
  try {
    const units = await apiPost('/api/units/update', { guid: selectedTargetGUID, level: newLevel });
    reconcileUnits(units);
    updateEnemyDropdown(units);
    if (String(selectedTargetGUID) === document.getElementById('enemy-select').value) {
      const updated = units.find(u => u.guid == selectedTargetGUID);
      if (updated) { updateEnemyInfo(updated); updateTargetLock(updated); }
    }
  } catch (err) { console.error('Update error:', err); }
}

// ---- Reset ----
async function resetSession() {
  try {
    await apiPost('/api/reset', null);
    clearAllCooldowns();
    resetStats();
    deselectTarget();
    exitTargetingMode();
    spellLogCounter = 0;
    serverLogCounter = 0;
    document.getElementById('spell-log-entries').innerHTML = '';
    document.getElementById('server-log-entries').innerHTML = '';
    for (const c of characters) clearAuraRings(c);
    const units = await apiGet('/api/units');
    createAllCharacters(units);
    updateSelfPanel(units.find(u => u.guid == CASTER_GUID));
    updateEnemyDropdown(units);
  } catch (err) { console.error('Reset error:', err); }
}

// ---- Expose ----
window.__resetSession = resetSession;
window.__spawnUnit = spawnUnit;
window.__removeSelectedTarget = removeSelectedTarget;
window.__adjustLevel = adjustLevel;
window.__cancelTargeting = exitTargetingMode;
window.__deselectTarget = deselectTarget;
window.__toggleTheme = toggleTheme;

// ---- Theme ----
let isLightTheme = false;
function toggleTheme() {
  isLightTheme = !isLightTheme;
  document.body.classList.toggle('light-theme', isLightTheme);
  const btn = document.getElementById('btn-theme');
  if (btn) btn.textContent = isLightTheme ? 'Dark' : 'Light';
  updateSceneTheme();
}

function updateSceneTheme() {
  const scene = getScene();
  if (!scene) return;
  const style = getComputedStyle(document.body);
  scene.background = new THREE.Color(style.getPropertyValue('--scene-bg').trim());
  scene.fog = new THREE.FogExp2(
    style.getPropertyValue('--scene-fog').trim(),
    parseFloat(style.getPropertyValue('--scene-fog-density').trim())
  );
  // Update grid and floor colors
  const { gridHelper, floorMesh } = getGridAndFloor();
  if (gridHelper) {
    const gridColor1 = style.getPropertyValue('--grid-primary').trim();
    const gridColor2 = style.getPropertyValue('--grid-secondary').trim();
    gridHelper.material.color = new THREE.Color(gridColor1);
    // GridHelper creates two materials — update center line too
    if (Array.isArray(gridHelper.material) && gridHelper.material.length > 1) {
      gridHelper.material[0].color = new THREE.Color(gridColor1);
      gridHelper.material[1].color = new THREE.Color(gridColor2);
    }
  }
  if (floorMesh) {
    const floorColor = style.getPropertyValue('--floor-color').trim();
    floorMesh.material.color = new THREE.Color(floorColor);
  }
}

// Enemy dropdown change handler
document.getElementById('enemy-select').addEventListener('change', (e) => {
  const val = e.target.value;
  if (val) selectTarget(Number(val));
  else deselectTarget();
});

// Keyboard: Escape cancels targeting
document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && targetingSpellID) exitTargetingMode(); });

// Safety: periodically verify target UI consistency
setInterval(() => {
  if (!selectedTargetGUID) {
    const ei = document.getElementById('enemy-info');
    if (ei && !ei.hidden && ei.style.display !== 'none') {
      hideTargetUI();
    }
  }
}, 500);

// ---- Start ----
init();
