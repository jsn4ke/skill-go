// vfx.js — Projectile effects, particles, floating damage numbers, hit flash
import * as THREE from 'three';
import { CSS2DObject } from 'three/addons/renderers/CSS2DRenderer.js';
import { addToScene, removeFromScene, addUpdatable, removeUpdatable, getScene } from './scene.js';

// ---- School colors ----
export const SCHOOL_COLORS = {
  'Fire': 0xff4400,
  'Frost': 0x44aaff,
  'Arcane': 0xcc44ff,
  'Shadow': 0x8844aa,
  'Nature': 0x44ff44,
  'Holy': 0xffcc00,
  'Physical': 0xcc8844,
};

// ---- Projectile ----
export function spawnProjectile(fromGroup, toGroup, schoolName) {
  const scene = getScene();
  const color = SCHOOL_COLORS[schoolName] || 0xff4400;

  const from = fromGroup.position.clone();
  from.y = 2.5;
  const to = toGroup.position.clone();
  to.y = 2.5;

  // Glowing sphere
  const geo = new THREE.SphereGeometry(0.4, 8, 8);
  const mat = new THREE.MeshBasicMaterial({ color });
  const sphere = new THREE.Mesh(geo, mat);
  sphere.position.copy(from);
  scene.add(sphere);

  // Point light
  const light = new THREE.PointLight(color, 2, 8);
  sphere.add(light);

  // Trail particles
  const trailParticles = [];
  const trailGroup = new THREE.Group();
  scene.add(trailGroup);

  const startTime = performance.now();
  const duration = 600;

  const obj = {
    update(dt) {
      const elapsed = performance.now() - startTime;
      const t = Math.min(elapsed / duration, 1);

      // Move along path (slight arc)
      sphere.position.lerpVectors(from, to, t);
      sphere.position.y += Math.sin(t * Math.PI) * 3;

      // Spawn trail particles
      if (Math.random() < 0.6) {
        const pGeo = new THREE.SphereGeometry(0.1, 4, 4);
        const pMat = new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.8 });
        const p = new THREE.Mesh(pGeo, pMat);
        p.position.copy(sphere.position);
        p.position.x += (Math.random() - 0.5) * 0.5;
        p.position.z += (Math.random() - 0.5) * 0.5;
        trailGroup.add(p);
        trailParticles.push({ mesh: p, life: 300, born: performance.now() });
      }

      // Fade trail particles
      for (let i = trailParticles.length - 1; i >= 0; i--) {
        const tp = trailParticles[i];
        const age = performance.now() - tp.born;
        if (age > tp.life) {
          trailGroup.remove(tp.mesh);
          tp.mesh.geometry.dispose();
          tp.mesh.material.dispose();
          trailParticles.splice(i, 1);
        } else {
          tp.mesh.material.opacity = 1 - age / tp.life;
          tp.mesh.scale.setScalar(1 - age / tp.life);
        }
      }

      if (t >= 1) {
        cleanup();
      }
    },
  };

  function cleanup() {
    removeUpdatable(obj);
    scene.remove(sphere);
    scene.remove(trailGroup);
    sphere.geometry.dispose();
    sphere.material.dispose();
    trailParticles.forEach(tp => {
      tp.mesh.geometry.dispose();
      tp.mesh.material.dispose();
    });
  }

  addUpdatable(obj);
  return obj;
}

// ---- Melee arc (Physical school) ----
export function spawnMeleeArc(targetGroup) {
  const scene = getScene();
  const pos = targetGroup.position.clone();
  pos.y = 2;

  const geo = new THREE.TorusGeometry(1.5, 0.08, 4, 20, Math.PI * 0.8);
  const mat = new THREE.MeshBasicMaterial({ color: 0xcc8844, transparent: true, opacity: 0.9 });
  const arc = new THREE.Mesh(geo, mat);
  arc.position.copy(pos);
  arc.rotation.y = Math.PI / 4;
  arc.rotation.x = Math.PI / 2;
  scene.add(arc);

  const startTime = performance.now();
  const duration = 400;

  const obj = {
    update() {
      const t = (performance.now() - startTime) / duration;
      if (t >= 1) {
        removeUpdatable(obj);
        scene.remove(arc);
        geo.dispose();
        mat.dispose();
        return;
      }
      arc.material.opacity = 0.9 * (1 - t);
      arc.scale.setScalar(1 + t * 0.5);
    },
  };

  addUpdatable(obj);
}

// CombatResult codes (must match Go spelldef.CombatResult iota)
const RESULT_HIT = 0, RESULT_MISS = 1, RESULT_CRIT = 2, RESULT_DODGE = 3;
const RESULT_PARRY = 4, RESULT_BLOCK = 5, RESULT_GLANCING = 6, RESULT_RESIST = 7, RESULT_FULL_RESIST = 8;

// ---- Floating damage number ----
export function spawnDamageNumber(targetGroup, value, result) {
  const scene = getScene();
  const text = String(value);

  const div = document.createElement('div');
  div.className = 'floating-number';

  let colorClass = 'hit-normal';
  let displayText = text;

  // result can be a numeric code (from API) or a string
  const code = typeof result === 'number' ? result : -1;

  if (code === RESULT_CRIT) {
    colorClass = 'hit-crit';
    displayText = text + ' CRIT!';
  } else if (code === RESULT_MISS) {
    colorClass = 'hit-miss';
    displayText = 'MISS';
  } else if (code === RESULT_DODGE) {
    colorClass = 'hit-dodge';
    displayText = 'DODGE';
  } else if (code === RESULT_PARRY) {
    colorClass = 'hit-parry';
    displayText = 'PARRY';
  } else if (code === RESULT_BLOCK) {
    colorClass = 'hit-block';
    displayText = text + ' BLOCK';
  } else if (code === RESULT_GLANCING) {
    colorClass = 'hit-normal';
    displayText = text + ' Glancing';
  } else if (code === RESULT_RESIST || code === RESULT_FULL_RESIST) {
    colorClass = 'hit-resist';
    displayText = code === RESULT_FULL_RESIST ? 'RESIST' : text + ' (partial)';
  }

  div.classList.add(colorClass);
  div.textContent = displayText;

  const label = new CSS2DObject(div);
  const pos = targetGroup.position.clone();
  pos.y = 4;
  pos.x += (Math.random() - 0.5) * 2;
  label.position.copy(pos);
  targetGroup.add(label);

  const startTime = performance.now();
  const duration = 1500;

  const obj = {
    update() {
      const elapsed = performance.now() - startTime;
      const t = elapsed / duration;
      if (t >= 1) {
        removeUpdatable(obj);
        targetGroup.remove(label);
        return;
      }
      label.position.y = 4 + t * 3;
      label.position.x += (Math.random() - 0.5) * 0.05;
      div.style.opacity = 1 - t;
    },
  };

  addUpdatable(obj);
}

// ---- Floating heal number ----
export function spawnHealNumber(targetGroup, value) {
  const div = document.createElement('div');
  div.className = 'floating-number hit-heal';
  div.textContent = '+' + Math.abs(Math.round(value));

  const label = new CSS2DObject(div);
  const pos = targetGroup.position.clone();
  pos.y = 4;
  pos.x += (Math.random() - 0.5) * 2;
  label.position.copy(pos);
  targetGroup.add(label);

  const startTime = performance.now();
  const duration = 1500;

  const obj = {
    update() {
      const t = (performance.now() - startTime) / duration;
      if (t >= 1) {
        removeUpdatable(obj);
        targetGroup.remove(label);
        return;
      }
      label.position.y = 4 + t * 3;
      div.style.opacity = 1 - t;
    },
  };

  addUpdatable(obj);
}

// ---- Hit flash ----
export function flashHit(targetGroup) {
  const d = targetGroup.userData;
  if (!d.bodyMat) return;

  const origColor = d.bodyMat.color.getHex();
  const origEmissive = d.bodyMat.emissive?.getHex() || 0;

  d.bodyMat.emissive = new THREE.Color(0xff0000);
  d.bodyMat.emissiveIntensity = 0.5;

  setTimeout(() => {
    d.bodyMat.emissive = new THREE.Color(origEmissive);
    d.bodyMat.emissiveIntensity = 0;
  }, 200);
}

// ---- Heal beam ----
export function spawnHealBeam(fromGroup, toGroup) {
  const scene = getScene();
  const from = fromGroup.position.clone();
  from.y = 2;
  const to = toGroup.position.clone();
  to.y = 2;

  const points = [from, to];
  const geo = new THREE.BufferGeometry().setFromPoints(points);
  const mat = new THREE.LineBasicMaterial({ color: 0x44ff44, transparent: true, opacity: 0.8, linewidth: 2 });
  const line = new THREE.Line(geo, mat);
  scene.add(line);

  // Also add glowing cylinder along the beam
  const dir = to.clone().sub(from);
  const len = dir.length();
  const mid = from.clone().add(to).multiplyScalar(0.5);
  const cylGeo = new THREE.CylinderGeometry(0.15, 0.15, len, 6);
  const cylMat = new THREE.MeshBasicMaterial({ color: 0x44ff44, transparent: true, opacity: 0.4 });
  const cyl = new THREE.Mesh(cylGeo, cylMat);
  cyl.position.copy(mid);
  cyl.lookAt(to);
  cyl.rotateX(Math.PI / 2);
  scene.add(cyl);

  const startTime = performance.now();
  const duration = 800;

  const obj = {
    update() {
      const t = (performance.now() - startTime) / duration;
      if (t >= 1) {
        removeUpdatable(obj);
        scene.remove(line);
        scene.remove(cyl);
        geo.dispose();
        mat.dispose();
        cylGeo.dispose();
        cylMat.dispose();
        return;
      }
      mat.opacity = 0.8 * (1 - t);
      cylMat.opacity = 0.4 * (1 - t);
    },
  };

  addUpdatable(obj);
}

// ---- Aura ring ----
export function spawnAuraRing(targetGroup, colorHex) {
  const pos = targetGroup.position.clone();
  pos.y = 0.05;

  const geo = new THREE.RingGeometry(1, 1.3, 32);
  const mat = new THREE.MeshBasicMaterial({
    color: colorHex || 0x44ff44,
    transparent: true,
    opacity: 0.6,
    side: THREE.DoubleSide,
  });
  const ring = new THREE.Mesh(geo, mat);
  ring.position.copy(pos);
  ring.rotation.x = -Math.PI / 2;
  addToScene(ring);

  // Store ref so we can clean up on reset
  if (!targetGroup.userData.auraRings) targetGroup.userData.auraRings = [];
  targetGroup.userData.auraRings.push(ring);

  return ring;
}

// ---- Cast glow ----
export function spawnCastGlow(casterGroup, schoolColor) {
  const pos = casterGroup.position.clone();
  pos.y = 2;

  const geo = new THREE.SphereGeometry(1.5, 16, 16);
  const mat = new THREE.MeshBasicMaterial({
    color: schoolColor || 0x4488ff,
    transparent: true,
    opacity: 0.3,
  });
  const glow = new THREE.Mesh(geo, mat);
  glow.position.copy(pos);
  addToScene(glow);

  const startTime = performance.now();
  const duration = 600;

  const obj = {
    update() {
      const t = (performance.now() - startTime) / duration;
      if (t >= 1) {
        removeUpdatable(obj);
        removeFromScene(glow);
        geo.dispose();
        mat.dispose();
        return;
      }
      mat.opacity = 0.3 * (1 - t);
      glow.scale.setScalar(1 + t * 0.5);
    },
  };

  addUpdatable(obj);
}

// ---- Clear all VFX from a target's aura rings ----
export function clearAuraRings(targetGroup) {
  if (!targetGroup.userData.auraRings) return;
  for (const ring of targetGroup.userData.auraRings) {
    removeFromScene(ring);
    ring.geometry.dispose();
    ring.material.dispose();
  }
  targetGroup.userData.auraRings = [];
}
