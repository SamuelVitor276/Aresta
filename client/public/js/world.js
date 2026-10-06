import * as THREE from 'three';

// Tudo aqui é gerado por código. A "cara" low poly vem de duas coisas:
// normais por face (flat shading) e uma cor levemente diferente em cada
// triângulo. Os objetos estáticos são fundidos em poucas malhas (Batch),
// então o mapa inteiro custa poucas chamadas de desenho.

export const COLORS = {
  skyTop: 0x4f6fb3, skyHorizon: 0xffcf9e, skyBottom: 0xe6a57c, fog: 0xf1c39c, sun: 0xfff2d6,
  sand: [0xe3c191, 0xdcb684, 0xe8c99c], dirt: 0xc49a6c,
  grass: [0x9fae62, 0x8ea457, 0xaab56e], rock: 0xa98f7a, rockLight: 0xc9b6a2, snow: 0xfbf5ec,
  wall: 0xc9744f, wallCap: 0xe9a57c,
  platform: 0x6f7f9e, platformRim: 0xa3b0cb, stair: 0x8593b0,
  crate: 0xbd6d31, crateFrame: 0x7a4120,
  pillar: 0xe0b483, pillarBand: 0xb4804f,
  cover: 0x4f9a8f, coverCap: 0x93cdbf,
  trunk: 0x7a5236, pine: [0x3f7a4a, 0x4a8a55, 0x356b40], leaf: [0x7fae4f, 0x93bd5c, 0x6f9e45],
  mountain: [0x8c86a6, 0x7f7a9c, 0x9a92ad], cloud: 0xfff6ee,
};

// Cores por tipo de bloco da vila (v escolhe a variação)
const KIND = {
  rampart: [0xc98f62, 0xb97c52],
  house: [0xefe3cf, 0xe8b97f, 0xd98e6b, 0xa9c4c9, 0xe7d3a1],
  roof: [0xd9ccb6, 0xcf9f68, 0xc47c5c, 0x93aeb4, 0xd2bd8c],
  stone: [0xe6dccb, 0xcdbfa6, 0xf1ead9],
  stair: [0xbcaa8e, 0x6f7680],
  ruin: [0xcdb79b, 0xb9a283],
  rubble: [0xb5a38c, 0x9c8a75],
  metal: [0x8d969e, 0x6f7880],
  rail: [0x3a3f48], post: [0x3a3a40], lamp: [0x3a3a40],
  container: [0xb8402f, 0x2f6fa6, 0x3f8a5a, 0xd98a2b, 0x7d8590],
  car: [0x3f7fb5, 0xc9a227, 0x6b4a3a], carcab: [0x2c3440, 0x2c3440, 0x3a2e28],
  sandbag: [0xb9a275], pew: [0x8a5a3c], wood: [0x9c6b45, 0x7d5536], shelf: [0x6f7680],
  stall: [0x9c6b45, 0x8a5a3c, 0xa4744c], awning: [0xc0392b, 0x2e86ab, 0xe0a526],
  planter: [0xb07a52], trunk: [0x8b6b4a, 0x6e5a44], yard: [0xe2d2b6, 0xd8c39f],
  basin: [0xd9cdb8], belfry: [0xc47c5c, 0xb86f4f], bed: [0x6b4f36], tank: [0x9aa3ab],
};

const ZONE = {
  paving: { tile: 1.5, colors: [0xd9c4a3, 0xcdb795, 0xe2cfb0] },
  cobble: { tile: 0.8, colors: [0xbfa684, 0xb09676, 0xc9b291] },
  concrete: { tile: 3.0, colors: [0xb8b2a6, 0xaea89c, 0xc2bcb0] },
  grass: { tile: 1.2, colors: [0x9fae62, 0x8ea457, 0xaab56e] },
  dirt: { tile: 1.4, colors: [0xc49a6c, 0xb88e62, 0xcfa679] },
};

// ------------------------------------------------------------ aleatoriedade

export function mulberry32(a) {
  return function () {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function hash3(x, y, z) {
  let h = Math.imul(x | 0, 374761393) ^ Math.imul(y | 0, 668265263) ^ Math.imul(z | 0, 2147483647) ^ 0x5bd1e995;
  h = Math.imul(h ^ (h >>> 13), 1274126177);
  return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
}

function valueNoise(x, z) {
  const ix = Math.floor(x), iz = Math.floor(z), fx = x - ix, fz = z - iz;
  const sx = fx * fx * (3 - 2 * fx), sz = fz * fz * (3 - 2 * fz);
  const a = hash3(ix, iz, 0), b = hash3(ix + 1, iz, 0), c = hash3(ix, iz + 1, 0), d = hash3(ix + 1, iz + 1, 0);
  return a + (b - a) * sx + (c - a) * sz + (a - b - c + d) * sx * sz;
}

function fbm(x, z) {
  let v = 0, amp = 0.5, f = 1;
  for (let o = 0; o < 4; o++) {
    v += amp * valueNoise(x * f, z * f);
    f *= 2;
    amp *= 0.5;
  }
  return v / 0.9375;
}

// Chão plano dentro da arena; morros crescem à medida que se afasta dela
export function terrainHeight(x, z, hx, hz = hx) {
  const edge = Math.max(Math.abs(x) - hx, Math.abs(z) - hz) - 4;
  if (edge <= 0) return 0;
  const n = fbm(x * 0.025 + 7.3, z * 0.025 - 3.1);
  const ramp = Math.min(1, edge / 30);
  return ramp * (3 + n * n * 30) + Math.min(edge, 6) * 0.15;
}

// Desloca vértices com base na própria posição: vértices duplicados na mesma
// posição andam juntos, então a malha não racha.
function jitter(geo, amount, seed = 0) {
  const p = geo.attributes.position;
  for (let i = 0; i < p.count; i++) {
    const x = p.getX(i), y = p.getY(i), z = p.getZ(i);
    const kx = Math.round(x * 97), ky = Math.round(y * 97), kz = Math.round(z * 97);
    p.setXYZ(i,
      x + (hash3(kx + seed, ky, kz) - 0.5) * amount,
      y + (hash3(kx, ky + seed, kz + 7) - 0.5) * amount,
      z + (hash3(kx + 3, ky, kz + seed) - 0.5) * amount);
  }
  return geo;
}

const _q = new THREE.Quaternion(), _e = new THREE.Euler(), _v = new THREE.Vector3(), _s = new THREE.Vector3();
const _fwd = new THREE.Vector3(), _focus = new THREE.Vector3();
function M(x, y, z, rx = 0, ry = 0, rz = 0, sx = 1, sy = 1, sz = 1) {
  return new THREE.Matrix4().compose(_v.set(x, y, z), _q.setFromEuler(_e.set(rx, ry, rz)), _s.set(sx, sy, sz));
}

// Junta várias geometrias numa só, com uma cor por triângulo
class Batch {
  constructor(rnd) {
    this.rnd = rnd;
    this.pos = [];
    this.nor = [];
    this.col = [];
  }

  add(geo, color, matrix, jit = 0.05) {
    const g = geo.index ? geo.toNonIndexed() : geo.clone();
    if (matrix) g.applyMatrix4(matrix);
    g.computeVertexNormals();
    const p = g.attributes.position, n = g.attributes.normal, c = new THREE.Color();
    for (let i = 0; i < p.count; i++) {
      if (i % 3 === 0) {
        c.set(Array.isArray(color) ? color[Math.floor(this.rnd() * color.length)] : color);
        if (jit) c.offsetHSL(0, 0, (this.rnd() - 0.5) * jit);
      }
      this.pos.push(p.getX(i), p.getY(i), p.getZ(i));
      this.nor.push(n.getX(i), n.getY(i), n.getZ(i));
      this.col.push(c.r, c.g, c.b);
    }
    g.dispose();
    geo.dispose();
  }

  mesh(material) {
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(this.pos, 3));
    g.setAttribute('normal', new THREE.Float32BufferAttribute(this.nor, 3));
    g.setAttribute('color', new THREE.Float32BufferAttribute(this.col, 3));
    g.computeBoundingSphere();
    return new THREE.Mesh(g, material);
  }
}

function disposeMesh(m) {
  if (!m) return;
  m.parent?.remove(m);
  m.geometry.dispose();
}

// ------------------------------------------------------------ mundo

export class World {
  constructor(scene) {
    this.scene = scene;
    this.half = 0;
    this.material = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.95, metalness: 0 });
    this.clouds = [];
    this.buildSkyAndLights();
    this.buildTerrain(52, 42);
    this.buildClouds();
  }

  buildSkyAndLights() {
    const scene = this.scene;
    scene.fog = new THREE.Fog(COLORS.fog, 90, 460);

    // Domo do céu com gradiente nos vértices. Ele acompanha a câmera.
    this.sky = new THREE.Group();
    const skyGeo = new THREE.SphereGeometry(400, 32, 16);
    const pos = skyGeo.attributes.position, colors = [], c = new THREE.Color();
    const top = new THREE.Color(COLORS.skyTop), hor = new THREE.Color(COLORS.skyHorizon), bot = new THREE.Color(COLORS.skyBottom);
    for (let i = 0; i < pos.count; i++) {
      const y = pos.getY(i) / 400;
      if (y > 0) c.copy(hor).lerp(top, Math.pow(y, 0.55));
      else c.copy(hor).lerp(bot, Math.min(1, -y * 4));
      colors.push(c.r, c.g, c.b);
    }
    skyGeo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    this.sky.add(new THREE.Mesh(skyGeo, new THREE.MeshBasicMaterial({ vertexColors: true, side: THREE.BackSide, fog: false, depthWrite: false })));

    // Sol baixo de fim de tarde: sombras compridas
    const sunDir = new THREE.Vector3(-0.62, 0.48, -0.36).normalize();
    const disc = new THREE.Mesh(new THREE.IcosahedronGeometry(15, 1), new THREE.MeshBasicMaterial({ color: COLORS.sun, fog: false }));
    disc.position.copy(sunDir).multiplyScalar(360);
    this.sky.add(disc);
    this.sky.renderOrder = -1;
    scene.add(this.sky);

    scene.add(new THREE.HemisphereLight(0xbcd3ff, 0xc08a5a, 1.25));
    const sun = new THREE.DirectionalLight(0xffdcb0, 2.8);
    sun.position.copy(sunDir).multiplyScalar(100);
    sun.castShadow = true;
    sun.shadow.mapSize.set(2048, 2048);
    const sc = sun.shadow.camera;
    sc.left = sc.bottom = -52;
    sc.right = sc.top = 52;
    sc.near = 1;
    sc.far = 220;
    sun.shadow.bias = -0.0004;
    sun.shadow.normalBias = 0.03;
    scene.add(sun, sun.target);
    this.sun = sun;
    this.sunDir = sunDir;
  }

  // Terreno + montanhas + árvores + pedras (tudo fora da arena)
  buildTerrain(hx, hz) {
    this.hx = hx;
    this.hz = hz;
    disposeMesh(this.terrain);
    disposeMesh(this.decor);
    const rnd = mulberry32(20261005);

    // --- terreno
    const geo = new THREE.PlaneGeometry(400, 400, 100, 100).rotateX(-Math.PI / 2);
    const p0 = geo.attributes.position;
    for (let i = 0; i < p0.count; i++) p0.setY(i, terrainHeight(p0.getX(i), p0.getZ(i), hx, hz));
    const g = geo.toNonIndexed();
    geo.dispose();
    g.computeVertexNormals();
    const p = g.attributes.position, n = g.attributes.normal;
    const col = new Float32Array(p.count * 3), c = new THREE.Color();
    for (let i = 0; i < p.count; i += 3) {
      const cx = (p.getX(i) + p.getX(i + 1) + p.getX(i + 2)) / 3;
      const cy = (p.getY(i) + p.getY(i + 1) + p.getY(i + 2)) / 3;
      const cz = (p.getZ(i) + p.getZ(i + 1) + p.getZ(i + 2)) / 3;
      const edge = Math.max(Math.abs(cx) - hx, Math.abs(cz) - hz);
      const slope = 1 - n.getY(i);
      if (edge < 0.5) {
        // piso da arena: manchas grandes de areia + variação por triângulo
        const patch = hash3(Math.floor(cx / 6), Math.floor(cz / 6), 11);
        c.set(COLORS.sand[Math.floor(patch * 3)]).offsetHSL(0, 0, (rnd() - 0.5) * 0.035);
      } else if (slope > 0.42) c.set(COLORS.rock).offsetHSL(0, 0, (rnd() - 0.5) * 0.06);
      else if (cy > 24) c.set(COLORS.snow);
      else if (cy > 13) c.set(COLORS.rockLight).offsetHSL(0, 0, (rnd() - 0.5) * 0.05);
      else if (edge < 10) c.set(COLORS.dirt).offsetHSL(0, 0, (rnd() - 0.5) * 0.06);
      else c.set(COLORS.grass[Math.floor(rnd() * 3)]).offsetHSL(0, 0, (rnd() - 0.5) * 0.05);
      for (let k = 0; k < 3; k++) col.set([c.r, c.g, c.b], (i + k) * 3);
    }
    g.setAttribute('color', new THREE.BufferAttribute(col, 3));
    this.terrain = new THREE.Mesh(g, this.material);
    this.terrain.receiveShadow = true;
    this.scene.add(this.terrain);

    // --- decoração fundida numa malha só
    const b = new Batch(rnd);

    // Montanhas no horizonte
    for (let i = 0; i < 20; i++) {
      const a = (i / 20) * Math.PI * 2 + rnd() * 0.2, r = 250 + rnd() * 60;
      const h = 60 + rnd() * 70, rad = 45 + rnd() * 30;
      const cone = jitter(new THREE.ConeGeometry(rad, h, 7, 4), 9, i);
      b.add(cone, COLORS.mountain, M(Math.cos(a) * r, h / 2 - 4, Math.sin(a) * r, 0, rnd() * 6), 0.04);
      if (h > 95) {
        const cap = jitter(new THREE.ConeGeometry(rad * 0.3, h * 0.3, 7, 1), 2, i + 50);
        b.add(cap, COLORS.snow, M(Math.cos(a) * r, h - 4 - h * 0.15 + 0.5, Math.sin(a) * r, 0, rnd() * 6), 0.02);
      }
    }

    // Árvores e pedras ao redor da arena
    const spot = (minEdge) => {
      for (let t = 0; t < 30; t++) {
        const x = (rnd() * 2 - 1) * 175, z = (rnd() * 2 - 1) * 175;
        if (Math.max(Math.abs(x) - hx, Math.abs(z) - hz) > minEdge) return [x, terrainHeight(x, z, hx, hz), z];
      }
      return null;
    };
    for (let i = 0; i < 150; i++) {
      const s = spot(7);
      if (!s || s[1] > 20) continue;
      const [x, y, z] = s, k = 0.8 + rnd() * 0.7, ry = rnd() * 6;
      b.add(new THREE.CylinderGeometry(0.22, 0.32, 1.8, 5), COLORS.trunk, M(x, y + 0.9 * k, z, 0, ry, 0, k, k, k));
      if (rnd() < 0.6) { // pinheiro
        [[2.2, 3, 2.7], [1.7, 2.6, 4.3], [1.1, 2.2, 5.7]].forEach(([r, h, yy], j) =>
          b.add(jitter(new THREE.ConeGeometry(r, h, 6, 1), 0.35, i * 3 + j), COLORS.pine, M(x, y + yy * k, z, 0, ry + j, 0, k, k, k), 0.06));
      } else { // árvore redonda
        b.add(jitter(new THREE.IcosahedronGeometry(1.9, 0), 0.6, i), COLORS.leaf, M(x, y + 3.3 * k, z, 0, ry, 0, k, k * 0.9, k), 0.07);
      }
    }
    for (let i = 0; i < 80; i++) {
      const s = spot(3);
      if (!s) continue;
      const k = 0.5 + rnd() * 1.8;
      b.add(jitter(new THREE.DodecahedronGeometry(1, 0), 0.4, i), COLORS.rock,
        M(s[0], s[1] + 0.2 * k, s[2], rnd() * 3, rnd() * 3, rnd() * 3, k * (1 + rnd()), k * 0.7, k), 0.08);
    }

    this.decor = b.mesh(this.material);
    this.decor.castShadow = true;
    this.decor.receiveShadow = true;
    this.scene.add(this.decor);
  }

  buildClouds() {
    const mat = new THREE.MeshLambertMaterial({ color: COLORS.cloud, flatShading: true, emissive: 0x6b4f45, emissiveIntensity: 0.35 });
    const rnd = mulberry32(77);
    for (let i = 0; i < 16; i++) {
      const b = new Batch(rnd);
      const parts = 3 + Math.floor(rnd() * 3);
      for (let j = 0; j < parts; j++) {
        const r = 6 + rnd() * 6;
        b.add(jitter(new THREE.IcosahedronGeometry(r, 1), 2, i * 10 + j), COLORS.cloud,
          M((j - parts / 2) * 7 + rnd() * 3, rnd() * 3, rnd() * 6 - 3, 0, 0, 0, 1, 0.55, 1), 0);
      }
      const m = b.mesh(mat);
      m.position.set((rnd() * 2 - 1) * 260, 60 + rnd() * 35, (rnd() * 2 - 1) * 260);
      m.userData.speed = 1.5 + rnd() * 2;
      this.clouds.push(m);
      this.scene.add(m);
    }
  }

  // Blocos do mapa (vindos do servidor), com detalhes só visuais
  buildArena(map) {
    disposeMesh(this.arena);
    disposeMesh(this.ground);
    const hx = map.hx ?? 30, hz = map.hz ?? 30;
    if (hx !== this.hx || hz !== this.hz) this.buildTerrain(hx, hz);
    const rnd = mulberry32(1337);
    const b = new Batch(rnd);
    const seg = (v) => Math.max(1, Math.round(v / 2.5));
    const box = (w, h, d, x, y, z, color, jit = 0.05, subdivide = true) =>
      b.add(new THREE.BoxGeometry(w, h, d, subdivide ? seg(w) : 1, subdivide ? seg(h) : 1, subdivide ? seg(d) : 1), color, M(x, y, z), jit);
    const pick = (k, v) => { const list = KIND[k]; return list ? list[(v ?? 0) % list.length] : 0x999999; };

    for (const blk of map.boxes) {
      const [x0, y0, z0] = blk.min, [x1, y1, z1] = blk.max;
      const w = x1 - x0, h = y1 - y0, d = z1 - z0, cx = (x0 + x1) / 2, cy = (y0 + y1) / 2, cz = (z0 + z1) / 2;
      const k = blk.k, v = blk.v ?? 0;
      switch (k) {
        // ---- arena antiga
        case 'wall':
          box(w, h, d, cx, cy, cz, COLORS.wall, 0.07);
          box(w + 0.1, 0.35, d + 0.1, cx, y1 - 0.025, cz, COLORS.wallCap, 0.04, false);
          break;
        case 'platform':
          box(w, h, d, cx, cy, cz, COLORS.platform, 0.05);
          break;
        case 'stair':
          box(w, h, d, cx, cy, cz, map.boxes.length > 200 ? pick('stair', v) : COLORS.stair, 0.05, false);
          break;
        case 'crate': {
          box(w, h, d, cx, cy, cz, COLORS.crate, 0.06, false);
          const t = Math.min(0.1, w * 0.12), o = 0.02;
          for (const [ex, ez] of [[x0, z0], [x1, z0], [x0, z1], [x1, z1]]) {
            box(t, h + 2 * o, t, ex + (ex === x0 ? t / 2 - o : o - t / 2), cy, ez + (ez === z0 ? t / 2 - o : o - t / 2), COLORS.crateFrame, 0.04, false);
          }
          for (const yy of [y0 + t / 2 - o, y1 - t / 2 + o]) {
            box(w + 2 * o, t, t, cx, yy, z0 + t / 2 - o, COLORS.crateFrame, 0.04, false);
            box(w + 2 * o, t, t, cx, yy, z1 - t / 2 + o, COLORS.crateFrame, 0.04, false);
            box(t, t, d + 2 * o, x0 + t / 2 - o, yy, cz, COLORS.crateFrame, 0.04, false);
            box(t, t, d + 2 * o, x1 - t / 2 + o, yy, cz, COLORS.crateFrame, 0.04, false);
          }
          break;
        }
        case 'pillar':
          box(w, h, d, cx, cy, cz, COLORS.pillar, 0.06);
          box(w + 0.08, 0.45, d + 0.08, cx, y0 + 0.225, cz, COLORS.pillarBand, 0.04, false);
          box(w + 0.16, 0.3, d + 0.16, cx, y1 - 0.13, cz, COLORS.pillarBand, 0.04, false);
          break;
        case 'cover':
          box(w, h, d, cx, cy, cz, COLORS.cover, 0.06);
          box(w + 0.06, 0.12, d + 0.06, cx, y1 - 0.04, cz, COLORS.coverCap, 0.03, false);
          break;
        // ---- vila
        case 'rampart': {
          box(w, h, d, cx, cy, cz, pick(k, v), 0.06);
          if (h >= 9) { // ameias no alto da muralha (fora de alcance)
            const along = w > d, len = along ? w : d;
            for (let a = 1; a < len - 0.5; a += 2.2) {
              const px = along ? x0 + a : cx, pz = along ? cz : z0 + a;
              box(along ? 1.1 : w, 1.0, along ? d : 1.1, px, y1 + 0.5, pz, pick(k, 1), 0.04, false);
            }
          }
          break;
        }
        case 'container': { // nervuras: faixas alternando o tom
          const base = new THREE.Color(pick(k, v)), dark = base.clone().multiplyScalar(0.8);
          const along = d > w, len = along ? d : w, n = Math.max(2, Math.round(len / 0.35));
          for (let i = 0; i < n; i++) {
            const a0 = (along ? z0 : x0) + (len * i) / n, a1 = a0 + len / n;
            const c = i % 2 ? dark : base;
            if (along) box(w, h, a1 - a0, cx, cy, (a0 + a1) / 2, c, 0.02, false);
            else box(a1 - a0, h, d, (a0 + a1) / 2, cy, cz, c, 0.02, false);
          }
          break;
        }
        case 'awning': { // listras
          const along = w > d, len = along ? w : d, n = Math.max(3, Math.round(len / 0.45));
          for (let i = 0; i < n; i++) {
            const a0 = (along ? x0 : z0) + (len * i) / n, a1 = a0 + len / n, c = i % 2 ? 0xf3ead8 : pick(k, v);
            if (along) box(a1 - a0, h, d, (a0 + a1) / 2, cy, cz, c, 0.02, false);
            else box(w, h, a1 - a0, cx, cy, (a0 + a1) / 2, c, 0.02, false);
          }
          break;
        }
        case 'trunk':
          box(w, h, d, cx, cy, cz, pick(k, v), 0.08, false);
          if (v === 0) { // palmeira: folhas em leque
            for (let i = 0; i < 7; i++) {
              const a = (i / 7) * Math.PI * 2;
              b.add(new THREE.ConeGeometry(0.35, 2.6, 4).rotateX(Math.PI / 2), COLORS.leaf,
                M(cx + Math.sin(a) * 1.1, y1 - 0.25, cz + Math.cos(a) * 1.1, 0.45, a, 0, 1, 0.25, 1), 0.08);
            }
          } else { // oliveira: copa arredondada acima da cabeça
            b.add(jitter(new THREE.IcosahedronGeometry(1.5, 0), 0.5, Math.round(cx * 7 + cz)), COLORS.leaf, M(cx, y1 + 0.8, cz, 0, cx, 0, 1.2, 0.8, 1.2), 0.07);
          }
          break;
        case 'lamp':
          box(w, h, d, cx, cy, cz, pick(k, v), 0.02, false);
          box(0.5, 0.35, 0.5, cx, y1 + 0.1, cz, 0xffe6a8, 0.02, false);
          break;
        case 'car':
          box(w, h - 0.25, d, cx, cy + 0.12, cz, pick(k, v), 0.05, false);
          for (const [sx, sz] of [[-1, -1], [1, -1], [-1, 1], [1, 1]]) {
            const along = d > w;
            const wx = along ? cx + sx * (w / 2 - 0.05) : cx + sx * (w / 2 - 0.75);
            const wz = along ? cz + sz * (d / 2 - 0.75) : cz + sz * (d / 2 - 0.05);
            b.add(new THREE.CylinderGeometry(0.34, 0.34, 0.22, 8), 0x1f2228, along ? M(wx, 0.34, wz, 0, 0, Math.PI / 2) : M(wx, 0.34, wz, Math.PI / 2, 0, 0), 0.02);
          }
          break;
        case 'basin':
          box(w, h, d, cx, cy, cz, pick(k, v), 0.05, false);
          box(w - 0.6, 0.02, d - 0.6, cx, y1 + 0.005, cz, 0x5fa8c9, 0.04, false);
          break;
        case 'belfry':
          box(w, h, d, cx, cy, cz, pick(k, v), 0.05, false);
          b.add(new THREE.ConeGeometry(Math.max(w, d) * 0.72, 3.2, 4).rotateY(Math.PI / 4), pick(k, 0), M(cx, y1 + 1.6, cz), 0.06);
          b.add(new THREE.CylinderGeometry(0.35, 0.6, 0.9, 8), 0xb08d3e, M(cx, y0 - 0.5, cz), 0.04); // sino
          break;
        default:
          box(w, h, d, cx, cy, cz, pick(k, v), 0.05);
      }
    }
    this.arena = b.mesh(this.material);
    this.arena.castShadow = true;
    this.arena.receiveShadow = true;
    this.scene.add(this.arena);

    // Pisos por região (lajotas, paralelepípedos, concreto, grama, terra)
    const g = new Batch(rnd);
    (map.zones ?? []).forEach((zn, zi) => {
      const style = ZONE[zn.k];
      if (!style) return;
      const nx = Math.max(1, Math.round((zn.x1 - zn.x0) / style.tile)), nz = Math.max(1, Math.round((zn.z1 - zn.z0) / style.tile));
      const tw = (zn.x1 - zn.x0) / nx, td = (zn.z1 - zn.z0) / nz;
      for (let i = 0; i < nx; i++) {
        for (let j = 0; j < nz; j++) {
          const px = zn.x0 + (i + 0.5) * tw, pz = zn.z0 + (j + 0.5) * td;
          g.add(new THREE.PlaneGeometry(tw * 0.97, td * 0.97).rotateX(-Math.PI / 2), style.colors, M(px, 0.012 + zi * 0.002, pz), 0.05);
        }
      }
    });
    this.ground = g.mesh(this.material);
    this.ground.receiveShadow = true;
    this.scene.add(this.ground);
  }

  update(dt, camera) {
    this.sky.position.copy(camera.position);
    // A sombra cobre ~100 m em volta do que a câmera olha (fica nítida)
    camera.getWorldDirection(_fwd);
    _focus.copy(camera.position).addScaledVector(_fwd, 28);
    _focus.x = Math.round(_focus.x / 2) * 2;
    _focus.z = Math.round(_focus.z / 2) * 2;
    _focus.y = 0;
    this.sun.target.position.copy(_focus);
    this.sun.position.copy(_focus).addScaledVector(this.sunDir, 100);
    for (const c of this.clouds) {
      c.position.x += c.userData.speed * dt;
      if (c.position.x > 280) c.position.x = -280;
    }
  }
}
