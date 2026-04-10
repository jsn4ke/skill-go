// character.js — 3D character models, nameplates, HP/MP bars
import * as THREE from 'three';
import { CSS2DObject } from 'three/addons/renderers/CSS2DRenderer.js';
import { addToScene, removeFromScene } from './scene.js';

const ROLE_COLORS = {
  'Mage': 0x4488ff,
  'Warrior': 0x8B6914,
  'Target Dummy': 0xcc3333,
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
  const pz = unitData.position?.Y || 0;
  group.position.set(px, 0, pz);

  // Store refs
  group.userData = { guid: unitData.guid, name: unitData.name, bodyMat, headMat, body, head };

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

  addToScene(group);
  return group;
}

export function updateCharacter(group, unitData) {
  if (!group) return;
  const d = group.userData;

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
      const pct = unitData.maxMana > 0 ? (unitData.mana / unitData.maxMana) * 100 : 0;
      mpFill.style.width = Math.max(0, pct) + '%';
      mpFill.className = 'bar-fill bar-mp ' + hpTier(pct);
      mpVal.textContent = `${unitData.mana}/${unitData.maxMana}`;
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
}

export function removeCharacter(group) {
  if (!group) return;
  removeFromScene(group);
}
