import * as THREE from 'three';

// Cores bem distintas entre si e contra a areia/terracota do mapa
const PLAYER_COLORS = [
  0x2f7bff, 0xe93bb0, 0x12c2d6, 0x8c5bff, 0x7dd93a,
  0xf0433a, 0x0fa889, 0xff6fa5, 0x4b56f0, 0xb6e02e,
];

export function playerColor(id) {
  return new THREE.Color(PLAYER_COLORS[(Math.max(1, id) - 1) % PLAYER_COLORS.length]);
}

const mat = (color, extra = {}) =>
  new THREE.MeshStandardMaterial({ color, flatShading: true, roughness: 0.85, metalness: 0.05, ...extra });

function part(geo, material, x, y, z, parent) {
  const m = new THREE.Mesh(geo, material);
  m.position.set(x, y, z);
  m.castShadow = true;
  parent.add(m);
  return m;
}

const B = (w, h, d) => new THREE.BoxGeometry(w, h, d);
const C = (r, len, seg = 6) => new THREE.CylinderGeometry(r, r, len, seg).rotateX(Math.PI / 2);

function nameSprite(text, color) {
  const canvas = document.createElement('canvas');
  canvas.width = 256;
  canvas.height = 64;
  const ctx = canvas.getContext('2d');
  ctx.font = '700 34px system-ui, sans-serif';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.lineWidth = 7;
  ctx.strokeStyle = 'rgba(20, 24, 40, 0.75)';
  ctx.strokeText(text, 128, 34);
  ctx.fillStyle = `#${color.getHexString()}`;
  ctx.fillText(text, 128, 34);
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true, depthWrite: false }));
  sprite.scale.set(1.6, 0.4, 1);
  sprite.position.y = 2.2;
  return sprite;
}

// ------------------------------------------------------------ armas (geometria)
// Cada arma é montada apontando para −Z, com a empunhadura perto da origem.
// Devolve { group, muzzle (z da boca do cano), mag, pump, bolt }.

function buildGun(kind, accent, scale = 1) {
  const g = new THREE.Group();
  const metal = mat(0x5b606c, { roughness: 0.55, metalness: 0.15 });
  const dark = mat(0x2f333c, { roughness: 0.6, metalness: 0.1 });
  const wood = mat(0x8f6a45, { roughness: 0.8 });
  const tan = mat(0x8f7a5e, { roughness: 0.8 });
  const acc = mat(accent, { roughness: 0.6 });
  const out = { group: g };
  if (kind === 1) { // sniper: longa, luneta e ferrolho
    part(B(0.08, 0.1, 0.5), dark, 0, 0, 0, g);
    part(C(0.016, 0.62), metal, 0, 0.01, -0.55, g);
    part(C(0.03, 0.08), dark, 0, 0.01, -0.86, g);              // freio de boca
    part(C(0.032, 0.28, 8), dark, 0, 0.1, -0.02, g);           // luneta
    part(C(0.042, 0.05, 8), metal, 0, 0.1, -0.17, g);
    part(C(0.042, 0.05, 8), metal, 0, 0.1, 0.13, g);
    part(B(0.025, 0.05, 0.03), dark, 0, 0.06, -0.06, g);
    part(B(0.025, 0.05, 0.03), dark, 0, 0.06, 0.06, g);
    part(B(0.07, 0.13, 0.32), wood, 0, -0.03, 0.36, g);        // coronha
    part(B(0.07, 0.05, 0.3), wood, 0, -0.03, -0.28, g);        // telha
    const grip = part(B(0.045, 0.12, 0.06), wood, 0, -0.1, 0.14, g);
    grip.rotation.x = -0.3;
    out.mag = part(B(0.05, 0.08, 0.1), acc, 0, -0.08, -0.04, g);
    out.bolt = part(B(0.03, 0.03, 0.09), metal, 0.06, 0.03, 0.08, g);
    part(B(0.081, 0.022, 0.2), acc, 0, 0.035, 0.2, g);
    out.muzzle = -0.9;
  } else if (kind === 2) { // escopeta: cano grosso, tubo e bomba
    part(B(0.09, 0.11, 0.36), dark, 0, 0, 0, g);
    part(C(0.026, 0.55), metal, 0, 0.02, -0.45, g);
    part(C(0.02, 0.45), dark, 0, -0.035, -0.4, g);             // tubo do carregador
    out.pump = part(B(0.075, 0.07, 0.18), wood, 0, -0.035, -0.32, g);
    part(B(0.075, 0.12, 0.3), wood, 0, -0.03, 0.32, g);
    const grip = part(B(0.045, 0.12, 0.06), wood, 0, -0.1, 0.14, g);
    grip.rotation.x = -0.3;
    part(B(0.02, 0.025, 0.02), metal, 0, 0.065, -0.68, g);     // massa de mira
    part(B(0.092, 0.022, 0.16), acc, 0, 0.035, 0.05, g);
    out.mag = out.pump;
    out.muzzle = -0.73;
  } else { // fuzil
    part(B(0.09, 0.11, 0.46), metal, 0, 0, 0, g);
    part(B(0.075, 0.075, 0.26), dark, 0, -0.005, -0.34, g);
    part(C(0.017, 0.24), dark, 0, 0.005, -0.57, g);
    out.mag = part(B(0.05, 0.17, 0.09), acc, 0, -0.12, -0.07, g);
    out.mag.rotation.x = 0.18;
    part(B(0.07, 0.1, 0.22), tan, 0, -0.02, 0.33, g);
    const grip = part(B(0.045, 0.12, 0.06), tan, 0, -0.1, 0.12, g);
    grip.rotation.x = -0.3;
    part(B(0.035, 0.05, 0.1), dark, 0, 0.08, -0.04, g);
    part(B(0.012, 0.045, 0.02), dark, 0, 0.06, -0.44, g);
    part(B(0.092, 0.022, 0.2), acc, 0, 0.035, 0.06, g);
    out.muzzle = -0.74;
  }
  g.scale.setScalar(scale);
  out.muzzle *= scale;
  return out;
}

function buildKnife() {
  const g = new THREE.Group();
  part(B(0.03, 0.04, 0.12), mat(0x2b2f38), 0, 0, 0.06, g);                       // cabo
  part(B(0.06, 0.012, 0.012), mat(0x5b606c, { metalness: 0.4 }), 0, 0, 0, g);   // guarda
  const blade = new THREE.Mesh(new THREE.ConeGeometry(0.022, 0.2, 4).rotateX(-Math.PI / 2),
    mat(0xdfe3ea, { roughness: 0.25, metalness: 0.6 }));
  blade.scale.set(1, 0.35, 1);
  blade.position.z = -0.1;
  g.add(blade);
  return g;
}

export function buildNade(scale = 1) {
  const g = new THREE.Group();
  part(new THREE.IcosahedronGeometry(0.05, 0), mat(0x4b5a3a), 0, 0, 0, g);
  part(B(0.03, 0.025, 0.03), mat(0x3a3f4a), 0, 0.05, 0, g);
  part(new THREE.TorusGeometry(0.014, 0.004, 4, 8), mat(0xc9ccd2, { metalness: 0.5 }), 0.025, 0.06, 0, g);
  g.scale.setScalar(scale);
  return g;
}

// ------------------------------------------------------------ boneco (outros jogadores)

// Pés em y = 0, olhando para −Z. As medidas batem com as hitboxes do
// servidor: corpo até 1,45 m, cabeça até ~1,85 m.
export class PlayerModel {
  constructor(color, name) {
    this.color = color;
    this.phase = 0;
    this.swing = 0; // animação da faca / arremesso
    this.root = new THREE.Group();
    this.body = new THREE.Group();
    this.root.add(this.body);

    const main = mat(color), dark = mat(color.clone().multiplyScalar(0.55));
    const suit = mat(0x3b4256), boot = mat(0x23262f);
    const visor = mat(0x10141f, { emissive: color, emissiveIntensity: 0.9, roughness: 0.3 });

    this.legs = [-0.16, 0.16].map((x) => {
      const pivot = new THREE.Group();
      pivot.position.set(x, 0.85, 0);
      this.body.add(pivot);
      part(B(0.24, 0.72, 0.26), suit, 0, -0.36, 0, pivot);
      part(B(0.27, 0.16, 0.34), boot, 0, -0.77, -0.03, pivot);
      return pivot;
    });

    part(B(0.62, 0.62, 0.34), main, 0, 1.16, 0, this.body);
    part(B(0.5, 0.3, 0.06), dark, 0, 1.24, -0.18, this.body);
    part(B(0.42, 0.44, 0.16), dark, 0, 1.18, 0.24, this.body);
    part(B(0.66, 0.1, 0.38), suit, 0, 0.88, 0, this.body);

    this.head = new THREE.Group();
    this.head.position.set(0, 1.47, 0);
    this.body.add(this.head);
    part(B(0.36, 0.36, 0.36), main, 0, 0.18, 0, this.head);
    part(B(0.3, 0.11, 0.05), visor, 0, 0.2, -0.18, this.head);
    part(B(0.05, 0.14, 0.14), dark, 0.2, 0.17, 0.02, this.head); // fone (dentro da hitbox)

    this.arms = new THREE.Group();
    this.arms.position.set(0, 1.36, 0);
    this.body.add(this.arms);
    const armR = part(B(0.15, 0.15, 0.5), main, 0.27, -0.04, -0.2, this.arms);
    armR.rotation.y = 0.25;
    this.armL = part(B(0.15, 0.15, 0.55), main, -0.19, -0.06, -0.3, this.arms);
    this.armL.rotation.y = -0.55;

    this.guns = [0, 1, 2].map((k) => {
      const gun = buildGun(k, color, 1.1);
      gun.group.position.set(0.1, -0.06, -0.3);
      gun.group.visible = false;
      this.arms.add(gun.group);
      return gun;
    });
    this.muzzle = new THREE.Object3D();
    this.arms.add(this.muzzle);
    this.setWeapon(0);
    this.root.add(nameSprite(name, color));
  }

  setWeapon(kind) {
    if (kind === this.kind) return;
    this.kind = kind;
    this.guns.forEach((g, k) => { g.group.visible = k === kind; });
    this.muzzle.position.set(0.1, -0.04, -0.3 + this.guns[kind].muzzle);
  }

  strike() { this.swing = 1; }

  animate(dt, speed, pitch) {
    const s = Math.min(1, speed / 7.5);
    this.phase += dt * (3 + speed * 1.3);
    const swing = Math.sin(this.phase) * 0.75 * s;
    this.legs[0].rotation.x = swing;
    this.legs[1].rotation.x = -swing;
    this.body.position.y = Math.abs(Math.sin(this.phase)) * 0.05 * s;
    this.head.rotation.x = pitch * 0.7;
    this.arms.rotation.x = pitch;
    this.swing = Math.max(0, this.swing - dt * 4);
    this.armL.rotation.x = -Math.sin(this.swing * Math.PI) * 1.4;
  }

  muzzleWorld(target) {
    return this.muzzle.getWorldPosition(target);
  }

  dispose() {
    this.root.traverse((o) => {
      o.geometry?.dispose();
      if (o.material) {
        o.material.map?.dispose();
        o.material.dispose();
      }
    });
  }
}

// ------------------------------------------------------------ primeira pessoa

// Posição de repouso e de mira (ADS) de cada arma
const POSE = [
  { hip: [0.2, -0.19, -0.62], ads: [0, -0.115, -0.5] },
  { hip: [0.2, -0.2, -0.6], ads: [0, -0.2, -0.4] },
  { hip: [0.2, -0.2, -0.58], ads: [0.0, -0.145, -0.5] },
];

export class ViewModel {
  constructor(accent) {
    this.root = new THREE.Group();
    this.holder = new THREE.Group();
    this.root.add(this.holder);
    this.recoil = 0;
    this.flashT = 0;
    this.bobT = 0;
    this.bob = 0;
    this.reload = 0;
    this.ads = 0;
    this.knifeT = 0;
    this.throwT = 0;
    this.pumpT = 0;
    this.sway = new THREE.Vector2();

    this.guns = [0, 1, 2].map((k) => {
      const gun = buildGun(k, accent, 0.85);
      gun.group.visible = false;
      this.holder.add(gun.group);
      return gun;
    });
    const glove = mat(0x4a5060);
    this.handR = part(B(0.07, 0.07, 0.1), glove, 0.005, -0.11, 0.11, this.holder);
    this.knife = buildKnife();
    this.knife.visible = false;
    this.root.add(this.knife);
    this.nade = buildNade(1);
    this.nade.visible = false;
    this.root.add(this.nade);

    const flashMat = new THREE.MeshBasicMaterial({ color: 0xffd27a, transparent: true, opacity: 0.95, blending: THREE.AdditiveBlending, depthWrite: false });
    this.flash = new THREE.Group();
    const f1 = new THREE.Mesh(new THREE.OctahedronGeometry(0.06), flashMat);
    f1.scale.set(1, 1, 2.6);
    const f2 = new THREE.Mesh(new THREE.OctahedronGeometry(0.09), flashMat);
    f2.scale.set(1.6, 1.6, 0.5);
    this.flash.add(f1, f2);
    this.flash.visible = false;
    this.holder.add(this.flash);
    this.root.traverse((o) => { o.castShadow = false; });
    this.setWeapon(0);
  }

  setWeapon(kind) {
    if (kind === this.kind) return;
    this.kind = kind;
    this.guns.forEach((g, k) => { g.group.visible = k === kind; });
    this.flash.position.set(0, 0.005, this.guns[kind].muzzle - 0.02);
  }

  kick() {
    this.recoil = this.kind === 0 ? 1 : 1.6;
    this.flashT = 0.05;
    this.flash.rotation.z = Math.random() * Math.PI;
    if (this.kind !== 0) this.pumpT = 1; // bomba / ferrolho depois do tiro
  }

  stab() { this.knifeT = 1; }
  toss() { this.throwT = 1; }

  update(dt, { moving, onGround, reloading, look, ads }) {
    const walking = moving && onGround ? 1 : 0;
    this.bob += (walking - this.bob) * Math.min(1, dt * 8);
    this.bobT += dt * 9.5 * this.bob;
    const bobK = 1 - ads * 0.85;
    const bx = Math.sin(this.bobT) * 0.012 * this.bob * bobK;
    const by = Math.abs(Math.cos(this.bobT)) * 0.016 * this.bob * bobK;

    const tx = Math.max(-0.04, Math.min(0.04, -look.x * 0.0006)) * bobK;
    const ty = Math.max(-0.04, Math.min(0.04, look.y * 0.0006)) * bobK;
    this.sway.x += (tx - this.sway.x) * Math.min(1, dt * 12);
    this.sway.y += (ty - this.sway.y) * Math.min(1, dt * 12);

    this.recoil = Math.max(0, this.recoil - dt * 8);
    const r = this.recoil * this.recoil;
    this.reload += ((reloading ? 1 : 0) - this.reload) * Math.min(1, dt * 10);
    this.ads = ads;
    this.knifeT = Math.max(0, this.knifeT - dt / 0.35);
    this.throwT = Math.max(0, this.throwT - dt / 0.4);
    this.pumpT = Math.max(0, this.pumpT - dt / 0.5);
    const busy = Math.max(Math.sin(this.knifeT * Math.PI), Math.sin(this.throwT * Math.PI));

    const pose = POSE[this.kind], gun = this.guns[this.kind];
    const lerp = (a, b) => a + (b - a) * ads;
    this.holder.position.set(
      lerp(pose.hip[0], pose.ads[0]) + bx + this.sway.x,
      lerp(pose.hip[1], pose.ads[1]) + by + this.sway.y - this.reload * 0.12 - busy * 0.18,
      lerp(pose.hip[2], pose.ads[2]) + r * 0.07,
    );
    this.holder.rotation.set(r * 0.1 - this.reload * 0.55 - busy * 0.4, this.sway.x * 2, this.reload * 0.45);
    if (gun.mag && this.kind === 0) gun.mag.position.y = -0.12 - this.reload * 0.06;
    const pump = Math.sin(Math.min(1, (1 - this.pumpT) * 1.25) * Math.PI) * (this.pumpT > 0 ? 1 : 0);
    if (this.kind === 2) gun.pump.position.z = -0.32 + pump * 0.08;
    if (this.kind === 1) gun.bolt.position.z = 0.08 + pump * 0.07;

    // Faca: corte da direita para a esquerda
    this.knife.visible = this.knifeT > 0;
    if (this.knife.visible) {
      const k = 1 - this.knifeT;
      this.knife.position.set(0.25 - k * 0.45, -0.18 + Math.sin(k * Math.PI) * 0.08, -0.45);
      this.knife.rotation.set(-0.2, 0.9 - k * 1.8, -0.6 + k * 0.5);
    }
    // Granada: sai da mão esquerda e vai para frente
    this.nade.visible = this.throwT > 0.3;
    if (this.nade.visible) {
      const k = 1 - this.throwT;
      this.nade.position.set(-0.18 + k * 0.1, -0.2 + k * 0.25, -0.4 - k * 0.5);
    }

    this.flashT -= dt;
    this.flash.visible = this.flashT > 0;
  }
}
