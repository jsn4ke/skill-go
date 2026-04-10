// config.js — Spell Config Editor page logic

let spells = []; // current spell data from server
const effectTypes = ['SchoolDamage', 'Heal', 'ApplyAura', 'TriggerSpell', 'Energize', 'WeaponDamage'];
const auraTypes = [
  { value: 0, label: 'None' },
  { value: 1, label: 'Buff' },
  { value: 2, label: 'Debuff' },
  { value: 3, label: 'Passive' },
  { value: 4, label: 'Proc' },
];

document.addEventListener('DOMContentLoaded', () => {
  loadSpells();
  addCreateEffect(); // start with one empty effect row
});

// ---- API ----
async function loadSpells() {
  try {
    const resp = await fetch('/api/spells');
    spells = await resp.json();
    render();
  } catch (e) {
    showFeedback('Failed to load spells: ' + e.message, 'error');
  }
}

async function applySpell(id) {
  const card = document.querySelector(`.spell-card[data-id="${id}"]`);
  if (!card) return;

  const data = buildRequest(id, card);

  try {
    const resp = await fetch(`/api/spells/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (resp.ok) {
      showFeedback(`Spell #${id} updated`, 'success');
    } else {
      const err = await resp.json();
      showFeedback(err.error || 'Update failed', 'error');
    }
  } catch (e) {
    showFeedback('Request failed: ' + e.message, 'error');
  }
}

// ---- Build request from form ----
function buildRequest(id, card) {
  const data = {
    name: card.querySelector('.field-name').value,
    castTime: parseInt(card.querySelector('.field-castTime').value) || 0,
    cooldown: parseInt(card.querySelector('.field-cd').value) || 0,
    categoryCD: parseInt(card.querySelector('.field-catCD').value) || 0,
    powerCost: parseInt(card.querySelector('.field-power').value) || 0,
    maxTargets: parseInt(card.querySelector('.field-maxTargets').value) || 1,
  };

  const effectBlocks = card.querySelectorAll('.effect-block');
  data.effects = [];
  effectBlocks.forEach((block, i) => {
    const bp = block.querySelector('.eff-bp');
    const coef = block.querySelector('.eff-coef');
    const wp = block.querySelector('.eff-wp');
    const ad = block.querySelector('.eff-ad');
    const at = block.querySelector('.eff-at');
    if (bp) {
      data.effects.push({
        effectIndex: i,
        basePoints: parseInt(bp.value) || 0,
        coef: coef ? parseFloat(coef.value) || 0 : 0,
        weaponPercent: wp ? parseFloat(wp.value) || 0 : 0,
        auraDuration: ad ? parseInt(ad.value) || 0 : 0,
        auraType: at ? parseInt(at.value) || 0 : 0,
      });
    }
  });

  return data;
}

// ---- Rendering ----
function render() {
  const container = document.getElementById('spell-list');
  container.innerHTML = '';

  for (const spell of spells) {
    const card = document.createElement('div');
    card.className = 'spell-card';
    card.dataset.id = spell.id;

    const effectsSummary = spell.effectsDetail
      ? spell.effectsDetail.map(e => e.effectType).join(', ')
      : spell.effects.join(', ');

    card.innerHTML = `
      <div class="spell-header">
        <div>
          <span class="spell-name">${escHtml(spell.name)}</span>
          <span class="spell-id">#${spell.id}</span>
          <span class="school-badge ${escAttr(spell.schoolName)}">${escHtml(spell.schoolName)}</span>
        </div>
        <div>
          <span class="spell-summary">CD: ${spell.cooldown}ms | Mana: ${spell.powerCost}</span>
          <button class="btn-delete" data-id="${spell.id}" title="Delete spell">&#10005;</button>
          <span class="spell-toggle">&#9654;</span>
        </div>
      </div>
      <div class="spell-body">
        <div class="form-row">
          <label>Name</label>
          <input type="text" class="field-name" value="${escAttr(spell.name)}">
        </div>
        <div class="form-row">
          <label>Cast Time</label>
          <input type="number" class="field-castTime" value="${spell.castTime}">
          <span class="unit">ms</span>
        </div>
        <div class="form-row">
          <label>Cooldown</label>
          <input type="number" class="field-cd" value="${spell.cooldown}">
          <span class="unit">ms</span>
        </div>
        <div class="form-row">
          <label>Category CD</label>
          <input type="number" class="field-catCD" value="${spell.categoryCD}">
          <span class="unit">ms</span>
        </div>
        <div class="form-row">
          <label>Power Cost (Mana)</label>
          <input type="number" class="field-power" value="${spell.powerCost}">
        </div>
        <div class="form-row">
          <label>Max Targets</label>
          <input type="number" class="field-maxTargets" value="${spell.maxTargets}">
        </div>
        ${renderEffects(spell)}
        <div class="apply-row">
          <button class="btn-apply">Apply</button>
        </div>
      </div>
    `;

    // Toggle expand
    card.querySelector('.spell-header').addEventListener('click', (e) => {
      if (e.target.closest('.btn-delete')) return;
      const body = card.querySelector('.spell-body');
      const toggle = card.querySelector('.spell-toggle');
      body.classList.toggle('expanded');
      toggle.classList.toggle('expanded');
    });

    // Delete button
    card.querySelector('.btn-delete').addEventListener('click', (e) => {
      e.stopPropagation();
      deleteSpell(spell.id, card);
    });

    // Apply button
    card.querySelector('.btn-apply').addEventListener('click', () => {
      applySpell(spell.id);
    });

    container.appendChild(card);
  }
}

function renderEffects(spell) {
  if (!spell.effectsDetail || spell.effectsDetail.length === 0) return '';

  let html = '<div class="effect-section"><h3>Effects</h3>';

  for (const eff of spell.effectsDetail) {
    html += `
      <div class="effect-block">
        <div class="effect-type">#${eff.effectIndex} ${escHtml(eff.effectType)}</div>
        <div class="form-row">
          <label>Base Points</label>
          <input type="number" class="eff-bp" value="${eff.basePoints}">
        </div>
        ${eff.coef !== 0 ? `
        <div class="form-row">
          <label>Coefficient</label>
          <input type="number" step="0.1" class="eff-coef" value="${eff.coef}">
        </div>
        ` : ''}
        ${eff.weaponPercent !== 0 ? `
        <div class="form-row">
          <label>Weapon %</label>
          <input type="number" step="0.1" class="eff-wp" value="${eff.weaponPercent}">
        </div>
        ` : ''}
        ${eff.auraDuration !== 0 ? `
        <div class="form-row">
          <label>Aura Duration</label>
          <input type="number" class="eff-ad" value="${eff.auraDuration}">
          <span class="unit">ms</span>
        </div>
        ` : ''}
        ${eff.auraType !== 0 ? `
        <div class="form-row">
          <label>Aura Type</label>
          <input type="number" class="eff-at" value="${eff.auraType}">
        </div>
        ` : ''}
      </div>
    `;
  }

  html += '</div>';
  return html;
}

// ---- Feedback ----
function showFeedback(msg, type) {
  const container = document.getElementById('feedback');
  const el = document.createElement('div');
  el.className = 'feedback-msg ' + type;
  el.textContent = msg;
  container.appendChild(el);
  setTimeout(() => el.remove(), 2500);
}

// ---- Helpers ----
function escHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function escAttr(str) {
  if (!str) return '';
  return str.replace(/[^a-zA-Z0-9_]/g, '_');
}

// ---- Create Spell ----
function addCreateEffect() {
  const list = document.getElementById('create-effect-list');
  const idx = list.children.length;
  const row = document.createElement('div');
  row.className = 'effect-block';
  row.dataset.idx = idx;
  row.innerHTML = `
    <div class="create-effect-header">
      <span class="effect-type">#${idx} Effect</span>
      <button class="btn-remove-effect" onclick="this.closest('.effect-block').remove(); reindexCreateEffects();" title="Remove">&#10005;</button>
    </div>
    <div class="form-row">
      <label>Effect Type</label>
      <select class="eff-type">
        ${effectTypes.map(t => `<option value="${t}">${t}</option>`).join('')}
      </select>
    </div>
    <div class="form-row">
      <label>Base Points</label>
      <input type="number" class="eff-bp" value="0">
    </div>
    <div class="form-row">
      <label>Coefficient</label>
      <input type="number" step="0.1" class="eff-coef" value="0">
    </div>
    <div class="form-row">
      <label>Weapon %</label>
      <input type="number" step="0.1" class="eff-wp" value="0">
    </div>
    <div class="form-row">
      <label>Aura Duration</label>
      <input type="number" class="eff-ad" value="0">
      <span class="unit">ms</span>
    </div>
    <div class="form-row">
      <label>Aura Type</label>
      <select class="eff-at">
        ${auraTypes.map(a => `<option value="${a.value}">${a.label}</option>`).join('')}
      </select>
    </div>
  `;
  list.appendChild(row);
}

function reindexCreateEffects() {
  document.querySelectorAll('#create-effect-list .effect-block').forEach((block, i) => {
    block.dataset.idx = i;
    block.querySelector('.effect-type').textContent = `#${i} Effect`;
  });
}

async function createSpell() {
  const name = document.getElementById('create-name').value.trim();
  if (!name) {
    showFeedback('Spell name is required', 'error');
    return;
  }

  const effects = [];
  document.querySelectorAll('#create-effect-list .effect-block').forEach((block, i) => {
    effects.push({
      effectType: block.querySelector('.eff-type').value,
      basePoints: parseInt(block.querySelector('.eff-bp').value) || 0,
      coef: parseFloat(block.querySelector('.eff-coef').value) || 0,
      weaponPercent: parseFloat(block.querySelector('.eff-wp').value) || 0,
      auraDuration: parseInt(block.querySelector('.eff-ad').value) || 0,
      auraType: parseInt(block.querySelector('.eff-at').value) || 0,
    });
  });

  const data = {
    name,
    schoolName: document.getElementById('create-school').value,
    castTime: parseInt(document.getElementById('create-castTime').value) || 0,
    cooldown: parseInt(document.getElementById('create-cd').value) || 0,
    categoryCD: parseInt(document.getElementById('create-catCD').value) || 0,
    powerCost: parseInt(document.getElementById('create-power').value) || 0,
    maxTargets: parseInt(document.getElementById('create-maxTargets').value) || 1,
    effects,
  };

  try {
    const resp = await fetch('/api/spells', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (resp.ok || resp.status === 201) {
      showFeedback(`Spell "${name}" created`, 'success');
      document.getElementById('create-name').value = '';
      document.getElementById('create-effect-list').innerHTML = '';
      addCreateEffect();
      loadSpells();
    } else {
      const err = await resp.json();
      showFeedback(err.error || 'Create failed', 'error');
    }
  } catch (e) {
    showFeedback('Request failed: ' + e.message, 'error');
  }
}

// ---- Delete Spell ----
async function deleteSpell(id, cardEl) {
  if (!confirm(`Delete spell #${id}? This cannot be undone.`)) return;

  try {
    const resp = await fetch(`/api/spells/${id}`, { method: 'DELETE' });
    if (resp.ok) {
      cardEl.remove();
      showFeedback(`Spell #${id} deleted`, 'success');
    } else {
      const err = await resp.json();
      showFeedback(err.error || 'Delete failed', 'error');
    }
  } catch (e) {
    showFeedback('Request failed: ' + e.message, 'error');
  }
}
