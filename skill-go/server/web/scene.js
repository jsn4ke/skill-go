// scene.js — Three.js scene, camera, lighting, ground, render loop, raycasting
import * as THREE from 'three';
import { CSS2DRenderer } from 'three/addons/renderers/CSS2DRenderer.js';

let scene, camera, renderer, labelRenderer;
let groundPlane; // invisible plane for ground-click detection
let gridHelper, floorMesh; // theme-updatable objects
const updatables = [];
const raycaster = new THREE.Raycaster();
const mouse = new THREE.Vector2();

export function initScene() {
  // Scene
  scene = new THREE.Scene();
  scene.background = new THREE.Color(0x1a1a2e);
  scene.fog = new THREE.FogExp2(0x1a1a2e, 0.008);

  // Orthographic camera — fixed 45° elevation, 30° azimuth, distance=40
  const aspect = window.innerWidth / window.innerHeight;
  const d = 40;
  camera = new THREE.OrthographicCamera(
    -d * aspect / 2, d * aspect / 2,
    d / 2, -d / 2,
    0.1, 200
  );
  const phi = THREE.MathUtils.degToRad(45);
  const theta = THREE.MathUtils.degToRad(30);
  camera.position.set(
    d * Math.sin(phi) * Math.cos(theta),
    d * Math.cos(phi),
    d * Math.sin(phi) * Math.sin(theta)
  );
  camera.lookAt(20, 0, 0); // center of the combat area

  // WebGL renderer
  renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setSize(window.innerWidth, window.innerHeight);
  renderer.setPixelRatio(window.devicePixelRatio);
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = THREE.PCFSoftShadowMap;
  document.getElementById('canvas-container').appendChild(renderer.domElement);

  // CSS2D renderer for labels and floating numbers
  labelRenderer = new CSS2DRenderer();
  labelRenderer.setSize(window.innerWidth, window.innerHeight);
  labelRenderer.domElement.style.position = 'absolute';
  labelRenderer.domElement.style.top = '0';
  labelRenderer.domElement.style.left = '0';
  labelRenderer.domElement.style.pointerEvents = 'none';
  document.getElementById('canvas-container').appendChild(labelRenderer.domElement);

  // Lighting
  const ambient = new THREE.AmbientLight(0x404060, 0.6);
  scene.add(ambient);

  const dirLight = new THREE.DirectionalLight(0xffffff, 0.8);
  dirLight.position.set(30, 50, 20);
  dirLight.castShadow = true;
  dirLight.shadow.mapSize.width = 1024;
  dirLight.shadow.mapSize.height = 1024;
  dirLight.shadow.camera.near = 1;
  dirLight.shadow.camera.far = 120;
  dirLight.shadow.camera.left = -50;
  dirLight.shadow.camera.right = 50;
  dirLight.shadow.camera.top = 50;
  dirLight.shadow.camera.bottom = -50;
  scene.add(dirLight);

  // Ground grid
  gridHelper = new THREE.GridHelper(80, 40, 0x333355, 0x222244);
  scene.add(gridHelper);

  // Ground plane (semi-transparent floor) — also used for raycasting
  const floorGeo = new THREE.PlaneGeometry(80, 80);
  const floorMat = new THREE.MeshStandardMaterial({
    color: 0x111122,
    transparent: true,
    opacity: 0.5,
  });
  floorMesh = new THREE.Mesh(floorGeo, floorMat);
  floorMesh.rotation.x = -Math.PI / 2;
  floorMesh.position.set(20, -0.01, 0);
  floorMesh.receiveShadow = true;
  scene.add(floorMesh);

  // Invisible ground plane for raycasting (larger area, always at Y=0)
  const groundGeo = new THREE.PlaneGeometry(200, 200);
  const groundMat = new THREE.MeshBasicMaterial({ visible: false });
  groundPlane = new THREE.Mesh(groundGeo, groundMat);
  groundPlane.rotation.x = -Math.PI / 2;
  groundPlane.position.set(0, 0, 0);
  scene.add(groundPlane);

  // Resize handler
  window.addEventListener('resize', onResize);

  // Start render loop
  animate();
}

function onResize() {
  const aspect = window.innerWidth / window.innerHeight;
  const d = 40;
  camera.left = -d * aspect / 2;
  camera.right = d * aspect / 2;
  camera.top = d / 2;
  camera.bottom = -d / 2;
  camera.updateProjectionMatrix();
  renderer.setSize(window.innerWidth, window.innerHeight);
  labelRenderer.setSize(window.innerWidth, window.innerHeight);
}

function animate() {
  requestAnimationFrame(animate);
  const dt = 1 / 60; // approximate
  for (const u of updatables) {
    u.update(dt);
  }
  renderer.render(scene, camera);
  labelRenderer.render(scene, camera);
}

export function getScene() { return scene; }
export function getCamera() { return camera; }
export function getGroundPlane() { return groundPlane; }
export function getGridAndFloor() { return { gridHelper, floorMesh }; }

export function addUpdatable(obj) {
  const idx = updatables.indexOf(obj);
  if (idx === -1) updatables.push(obj);
}

export function removeUpdatable(obj) {
  const idx = updatables.indexOf(obj);
  if (idx !== -1) updatables.splice(idx, 1);
}

export function addToScene(obj) {
  scene.add(obj);
}

export function removeFromScene(obj) {
  scene.remove(obj);
}

// ---- Raycasting ----

// Set mouse coordinates from a click event (NDC: -1 to 1)
export function setMouseFromEvent(event) {
  const rect = renderer.domElement.getBoundingClientRect();
  mouse.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
  mouse.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
}

// Raycast against character meshes. Returns { group, point } or null.
// characterMeshes: flat array of all THREE.Mesh objects from character groups
export function raycastCharacters(characterMeshes) {
  raycaster.setFromCamera(mouse, camera);
  const intersects = raycaster.intersectObjects(characterMeshes, false);
  if (intersects.length === 0) return null;

  // Walk up to find the character Group (parent of body/head)
  const hit = intersects[0];
  let obj = hit.object;
  while (obj.parent && !obj.userData.guid) {
    obj = obj.parent;
  }
  if (!obj.userData.guid) return null;

  return { group: obj, point: hit.point };
}

// Raycast against ground plane. Returns world point {x, z} or null.
export function raycastGround() {
  raycaster.setFromCamera(mouse, camera);
  const intersects = raycaster.intersectObject(groundPlane, false);
  if (intersects.length === 0) return null;

  const p = intersects[0].point;
  return { x: p.x, z: p.z };
}
