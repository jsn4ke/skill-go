// trace.js — Spell Trace Viewer page logic

const API_BASE = ''; // same origin
let allEvents = [];     // flat array of all events
let flowMap = new Map(); // flowId -> { events[], spellName, collapsed }
let activeSpan = 'all';
let eventSource = null;

// ---- Init ----
document.addEventListener('DOMContentLoaded', () => {
  loadHistory();
  connectSSE();
  bindToolbar();
  bindSpanFilters();
});

// ---- API calls ----
async function loadHistory() {
  try {
    const resp = await fetch(API_BASE + '/api/trace/history');
    const events = await resp.json();
    appendEvents(events);
    render();
  } catch (e) {
    console.error('Failed to load history:', e);
  }
}

async function clearTrace() {
  try {
    await fetch(API_BASE + '/api/trace?clear=true');
    allEvents = [];
    flowMap.clear();
    render();
  } catch (e) {
    console.error('Failed to clear:', e);
  }
}

// ---- SSE ----
function connectSSE() {
  if (eventSource) eventSource.close();
  setConnStatus(false);

  eventSource = new EventSource(API_BASE + '/api/trace/stream');

  eventSource.onopen = () => {
    setConnStatus(true);
  };

  eventSource.onmessage = (msg) => {
    try {
      const event = JSON.parse(msg.data);
      appendEvents([event]);
      render();
    } catch (e) {
      // ignore parse errors
    }
  };

  eventSource.onerror = () => {
    setConnStatus(false);
    eventSource.close();
    // auto-reconnect after 3s
    setTimeout(connectSSE, 3000);
  };
}

function setConnStatus(connected) {
  const dot = document.getElementById('conn-status');
  dot.className = 'conn-dot ' + (connected ? 'connected' : 'disconnected');
  dot.title = connected ? 'Connected (SSE)' : 'Disconnected - retrying...';
}

// ---- Data management ----
function appendEvents(events) {
  for (const e of events) {
    allEvents.push(e);
    const fid = e.flowId;
    if (!flowMap.has(fid)) {
      flowMap.set(fid, { events: [], spellName: e.spellName || '', collapsed: false });
    }
    flowMap.get(fid).events.push(e);
    if (e.spellName) flowMap.get(fid).spellName = e.spellName;
  }
  // limit to 5000 DOM events
  if (allEvents.length > 5000) {
    const trimCount = allEvents.length - 5000;
    allEvents.splice(0, trimCount);
    rebuildFlowMap();
  }
}

function rebuildFlowMap() {
  flowMap.clear();
  for (const e of allEvents) {
    const fid = e.flowId;
    if (!flowMap.has(fid)) {
      flowMap.set(fid, { events: [], spellName: e.spellName || '', collapsed: false });
    }
    flowMap.get(fid).events.push(e);
    if (e.spellName) flowMap.get(fid).spellName = e.spellName;
  }
}

// ---- Rendering ----
function render() {
  const tbody = document.getElementById('table-body');
  tbody.innerHTML = '';

  let shownCount = 0;

  for (const [flowId, group] of flowMap) {
    // filter events in this flow
    const filtered = activeSpan === 'all'
      ? group.events
      : group.events.filter(e => e.span === activeSpan);

    if (filtered.length === 0) continue;

    // flow header
    const header = document.createElement('div');
    header.className = 'flow-header';
    header.dataset.flowId = flowId;
    header.innerHTML = `
      <span class="flow-toggle">${group.collapsed ? '&#9654;' : '&#9660;'}</span>
      <span class="flow-label">Flow #${flowId} — ${escHtml(group.spellName || 'unknown')}</span>
      <span class="flow-count">(${filtered.length} events)</span>
    `;
    header.addEventListener('click', () => {
      group.collapsed = !group.collapsed;
      render();
    });
    tbody.appendChild(header);

    // event rows
    const container = document.createElement('div');
    container.className = 'flow-events' + (group.collapsed ? ' collapsed' : '');

    for (const e of filtered) {
      const row = document.createElement('div');
      row.className = 'table-row';
      row.innerHTML = `
        <span class="col-time">${formatTime(e.timestamp)}</span>
        <span class="col-span"><span class="span-badge ${escAttr(e.span)}">${escHtml(e.span)}</span></span>
        <span class="col-event">${escHtml(e.event)}</span>
        <span class="col-spell">${e.spellId ? e.spellId + ' (' + escHtml(e.spellName || '') + ')' : ''}</span>
        <span class="col-fields">${formatFields(e.fields)}</span>
      `;
      container.appendChild(row);
      shownCount++;
    }

    tbody.appendChild(container);
  }

  // empty state
  if (shownCount === 0) {
    tbody.innerHTML = '<div id="empty-state">No trace events. Cast a spell to see traces here.</div>';
  }

  // stats
  document.getElementById('stat-flows').textContent = 'Flows: ' + flowMap.size;
  document.getElementById('stat-events').textContent = 'Events: ' + allEvents.length;
  document.getElementById('stat-shown').textContent = 'Shown: ' + shownCount;
}

// ---- Format helpers ----
function formatTime(ts) {
  if (!ts) return '—';
  const d = new Date(ts);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  const ms = String(d.getMilliseconds()).padStart(3, '0');
  return `${hh}:${mm}:${ss}.${ms}`;
}

function formatFields(fields) {
  if (!fields || Object.keys(fields).length === 0) return '';

  const parts = [];
  for (const [key, val] of Object.entries(fields)) {
    if (val !== null && typeof val === 'object') {
      // nested object — collapsible
      const id = 'f_' + Math.random().toString(36).slice(2, 8);
      const summary = formatValue(val);
      parts.push(
        `<span class="field-key">${escHtml(key)}</span>=` +
        `<span class="field-collapsible" data-target="${id}" title="click to expand">` +
        `${escHtml(summary)}&#9662;</span>` +
        `<span class="field-nested" id="${id}">${formatNested(val)}</span>`
      );
    } else {
      parts.push(`<span class="field-key">${escHtml(key)}</span>=<span class="field-value">${escHtml(String(val))}</span>`);
    }
  }
  return parts.join('<span class="field-sep"> | </span>');
}

function formatValue(val) {
  if (Array.isArray(val)) return `[${val.length} items]`;
  if (typeof val === 'object' && val !== null) return `{${Object.keys(val).length} keys}`;
  return String(val);
}

function formatNested(obj, depth) {
  depth = depth || 0;
  if (depth > 3) return '...';
  const parts = [];
  for (const [key, val] of Object.entries(obj)) {
    if (typeof val === 'object' && val !== null) {
      parts.push(`<span class="field-key">${escHtml(key)}</span>: ${formatNested(val, depth + 1)}`);
    } else {
      parts.push(`<span class="field-key">${escHtml(key)}</span>: <span class="field-value">${escHtml(String(val))}</span>`);
    }
  }
  return parts.join('<span class="field-sep"> | </span>');
}

function escHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function escAttr(str) {
  if (!str) return '';
  return str.replace(/[^a-zA-Z0-9_]/g, '_');
}

// ---- Event delegation for collapsible fields ----
document.addEventListener('click', (e) => {
  const collapsible = e.target.closest('.field-collapsible');
  if (!collapsible) return;
  const targetId = collapsible.dataset.target;
  const nested = document.getElementById(targetId);
  if (nested) {
    nested.classList.toggle('expanded');
    collapsible.innerHTML = nested.classList.contains('expanded')
      ? escHtml(collapsible.textContent.replace(/\u25BC.*$/, '')) + '&#9652;'
      : escHtml(collapsible.textContent.replace(/\u25B7.*$/, '')) + '&#9662;';
  }
});

// ---- Toolbar ----
function bindToolbar() {
  document.getElementById('btn-refresh').addEventListener('click', () => {
    allEvents = [];
    flowMap.clear();
    loadHistory();
  });
  document.getElementById('btn-clear').addEventListener('click', () => {
    clearTrace();
  });
}

// ---- Span filters ----
function bindSpanFilters() {
  document.getElementById('span-filters').addEventListener('click', (e) => {
    const btn = e.target.closest('.span-btn');
    if (!btn) return;
    const span = btn.dataset.span;

    // update active state
    document.querySelectorAll('.span-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');

    activeSpan = span;
    render();
  });
}
