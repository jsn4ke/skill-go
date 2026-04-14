// character.js — 3D character models, nameplates, HP/MP bars, movement
import * as THREE from 'three';
import { CSS2DObject } from 'three/addons/renderers/CSS2DRenderer.js';
import { addToScene, removeFromScene } from './scene.js';

const ROLE_COLORS = {
  'Mage': 0x4488ff,
  'Warrior': 0x8B6914,
  'Target Dummy': 0xcc3333,
  'Elite': 0xff8800,
  'Boss': 0xaa00ff,
};

function getRoleColor(name) {
  return ROLE_COLORS[name] || 0x888888;
}

function hpTier(pct) {
  if (pct < 25) return 'low';
  if (pct < 50) return 'medium';
  return 'high';
}

export function createCharacter(unitData) {
  const group = new THREE.Group();
  const color = getRoleColor(unitData.name);

  // Body — cylinder
  const bodyGeo = new THREE.CylinderGeometry(0.8, 0.8, 2.5, 12);
  const bodyMat = new THREE.MeshStandardMaterial({ color, roughness: 0.6, metalness: 0.3 });
  const body = new THREE.Mesh(bodyGeo, bodyMat);
  body.position.y = 1.25;
  body.castShadow = true;
  body.receiveShadow = true;
  group.add(body);

  // Head — sphere
  const headGeo = new THREE.SphereGeometry(0.5, 12, 12);
  const headMat = new THREE.MeshStandardMaterial({ color, roughness: 0.5, metalness: 0.2 });
  const head = new THREE.Mesh(headGeo, headMat);
  head.position.y = 3;
  head.castShadow = true;
  group.add(head);

  // Position
  const px = unitData.position?.X || 0;
  const pz = unitData.position?.Z || 0;
  group.position.set(px, 0, pz);

  // Store refs + unit data for HUD lookups
  group.userData = {
    guid: unitData.guid, name: unitData.name, bodyMat, headMat, body, head,
    targetPosition: null, isMoving: false,
    unitData: { ...unitData },
    speedMod: 1.0,
  };

  // Nameplate
  const nameDiv = document.createElement('div');
  nameDiv.className = 'char-nameplate';
  nameDiv.textContent = unitData.name;
  const nameLabel = new CSS2DObject(nameDiv);
  nameLabel.position.set(0, 3.8, 0);
  group.add(nameLabel);
  group.userData.nameLabel = nameLabel;

  // HP/MP bar container
  const barDiv = document.createElement('div');
  barDiv.className = 'char-bars';
  barDiv.innerHTML = `
    <div class="bar-row">
      <div class="bar-label">HP</div>
      <div class="bar-track"><div class="bar-fill bar-hp high" style="width:100%"></div></div>
      <div class="bar-value hp-val">${unitData.health}/${unitData.maxHealth}</div>
    </div>
    <div class="bar-row">
      <div class="bar-label">MP</div>
      <div class="bar-track"><div class="bar-fill bar-mp high" style="width:100%"></div></div>
      <div class="bar-value mp-val">${unitData.mana}/${unitData.maxMana}</div>
    </div>
  `;
  const barLabel = new CSS2DObject(barDiv);
  barLabel.position.set(0, 4.5, 0);
  group.add(barLabel);
  group.userData.barLabel = barLabel;

  // Level text
  const levelDiv = document.createElement('div');
  levelDiv.className = 'char-level';
  levelDiv.textContent = `Lv${unitData.level}`;
  const levelLabel = new CSS2DObject(levelDiv);
  levelLabel.position.set(0, -0.2, 0);
  group.add(levelLabel);
  group.userData.levelLabel = levelLabel;

  // Stun indicator (hidden by default)
  const stunDiv = document.createElement('div');
  stunDiv.className = 'char-stun-label';
  stunDiv.textContent = '眩晕';
  stunDiv.style.display = 'none';
  const stunLabel = new CSS2DObject(stunDiv);
  stunLabel.position.set(0, 5.2, 0);
  group.add(stunLabel);
  group.userData.stunLabel = stunLabel;

  addToScene(group);
  return group;
}

export function updateCharacter(group, unitData) {
  if (!group) return;
  const d = group.userData;
  d.unitData = { ...unitData };

  // Update HP bar — access DOM through barLabel.element
  const barEl = d.barLabel?.element;
  if (barEl) {
    const hpFill = barEl.querySelector('.bar-hp');
    const hpVal = barEl.querySelector('.hp-val');
    if (hpFill && hpVal) {
      const pct = unitData.maxHealth > 0 ? (unitData.health / unitData.maxHealth) * 100 : 0;
      hpFill.style.width = Math.max(0, pct) + '%';
      hpFill.className = 'bar-fill bar-hp ' + hpTier(pct);
      hpVal.textContent = `${unitData.health}/${unitData.maxHealth}`;
    }
    const mpFill = barEl.querySelector('.bar-mp');
    const mpVal = barEl.querySelector('.mp-val');
    if (mpFill && mpVal) {
      const isRage = (unitData.maxRage > 0);
      const current = isRage ? unitData.rage : unitData.mana;
      const max = isRage ? unitData.maxRage : unitData.maxMana;
      const pct = max > 0 ? (current / max) * 100 : 0;
      mpFill.style.width = Math.max(0, pct) + '%';
      mpFill.className = 'bar-fill ' + (isRage ? 'bar-rage ' : 'bar-mp ') + hpTier(pct);
      mpVal.textContent = `${current}/${max}`;
    }
  }

  // Dead state
  if (!unitData.alive) {
    d.bodyMat.color.set(0x444444);
    d.headMat.color.set(0x444444);
    group.rotation.z = Math.PI / 2;
    group.position.y = -0.5;
  } else {
    const color = getRoleColor(unitData.name);
    d.bodyMat.color.set(color);
    d.headMat.color.set(color);
    group.rotation.z = 0;
    group.position.y = 0;
  }

  // Update server position (if not moving, snap to server position)
  if (!d.isMoving && unitData.position) {
    group.position.x = unitData.position.X;
    group.position.z = unitData.position.Z || 0;
  }

  // Stun indicator — show if unit has debuff auras
  const stunEl = d.stunLabel?.element;
  if (stunEl) {
    const hasDebuff = unitData.auras?.some(a => a.auraType === 1) || false;
    stunEl.style.display = hasDebuff ? 'block' : 'none';
  }

  // Speed modifier — track and apply visual slow effect
  const speedMod = unitData.speedMod != null ? unitData.speedMod : 1.0;
  d.speedMod = speedMod;

  if (!unitData.alive) {
    // Don't override dead color
  } else if (speedMod < 1.0) {
    // Slowed: tint body blue-ish
    const slowColor = new THREE.Color(0x6688cc);
    const roleColor = new THREE.Color(getRoleColor(unitData.name));
    d.bodyMat.color.copy(roleColor).lerp(slowColor, 0.5);
    d.headMat.color.copy(roleColor).lerp(slowColor, 0.5);
  }
  // If speedMod returns to 1.0, the dead/alive branch above restores normal color
}

export function removeCharacter(group) {
  if (!group) return;
  // Remove CSS2D objects (nameplate, HP bars, etc.) — their DOM elements
  // persist in the CSS2DRenderer container even after the group is removed from the scene
  group.traverse(child => {
    if (child.isCSS2DObject && child.element && child.element.parentNode) {
      child.element.parentNode.removeChild(child.element);
    }
  });
  removeFromScene(group);
}

// ---- Movement ----

const MOVE_SPEED = 10; // units per second
const CHARGE_SPEED = 40; // charge sprint speed

// Set target position for smooth movement (speed defaults to MOVE_SPEED)
export function moveUnit(group, x, z, speed) {
  if (!group) return;
  group.userData.targetPosition = new THREE.Vector3(x, 0, z);
  group.userData.moveSpeed = speed || MOVE_SPEED;
  group.userData.isMoving = true;
}

// Update movement interpolation — call from animation loop
export function updateUnitMovement(group, dt) {
  const d = group.userData;
  if (!d.isMoving || !d.targetPosition) return;

  const target = d.targetPosition;
  const current = group.position;
  const dx = target.x - current.x;
  const dz = target.z - current.z;
  const dist = Math.sqrt(dx * dx + dz * dz);

  if (dist < 0.1) {
    // Snap to target
    current.x = target.x;
    current.z = target.z;
    d.targetPosition = null;
    d.isMoving = false;
    d.moveSpeed = null;
    return;
  }

  const step = (d.moveSpeed || MOVE_SPEED) * (d.speedMod || 1.0) * dt;
  const ratio = Math.min(step / dist, 1);
  current.x += dx * ratio;
  current.z += dz * ratio;
}

// Collect all raycastable meshes from a character group (body + head)
export function getCharacterMeshes(group) {
  const meshes = [];
  if (group.userData.body) meshes.push(group.userData.body);
  if (group.userData.head) meshes.push(group.userData.head);
  return meshes;
}
