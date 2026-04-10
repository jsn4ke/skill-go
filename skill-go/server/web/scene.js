// scene.js — Three.js scene, camera, lighting, ground, render loop
import * as THREE from 'three';
import { CSS2DRenderer } from 'three/addons/renderers/CSS2DRenderer.js';

let scene, camera, renderer, labelRenderer;
const updatables = [];

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
  const grid = new THREE.GridHelper(80, 40, 0x333355, 0x222244);
  scene.add(grid);

  // Ground plane (semi-transparent floor)
  const floorGeo = new THREE.PlaneGeometry(80, 80);
  const floorMat = new THREE.MeshStandardMaterial({
    color: 0x111122,
    transparent: true,
    opacity: 0.5,
  });
  const floor = new THREE.Mesh(floorGeo, floorMat);
  floor.rotation.x = -Math.PI / 2;
  floor.position.set(20, -0.01, 0);
  floor.receiveShadow = true;
  scene.add(floor);

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
