import * as THREE from 'three';

// Cores das equipes: 0 neutro, 1 vermelho, 2 azul
export const TEAM_COLORS = [0xf3ead8, 0xe8453c, 0x2f7bff];
export const TEAM_NAMES = ['', 'vermelha', 'azul'];

const basic = (color, opacity, additive = false) => new THREE.MeshBasicMaterial({
  color, transparent: true, opacity, depthWrite: false, side: THREE.DoubleSide,
  blending: additive ? THREE.AdditiveBlending : THREE.NormalBlending,
});

// Desenha no mundo os objetivos que chegam no snapshot: pontos de captura,
// colina e bandeiras. Cada frame só reposiciona e recolore.
export class Objectives {
  constructor(scene) {
    this.scene = scene;
    this.zones = new Map();
    this.flags = [null, null, null];
    this.time = 0;
  }

  makeZone(r) {
    const g = new THREE.Group();
    const flat = (geo, mat) => { const m = new THREE.Mesh(geo.rotateX(-Math.PI / 2), mat); g.add(m); return m; };
    const ring = flat(new THREE.RingGeometry(r - 0.18, r, 56), basic(0xffffff, 0.9));
    const fill = flat(new THREE.CircleGeometry(r, 56), basic(0xffffff, 0.1));
    const beam = new THREE.Mesh(new THREE.CylinderGeometry(0.3, 0.3, 46, 10, 1, true), basic(0xffffff, 0.22, true));
    beam.position.y = 23;
    g.add(beam);
    const arc = flat(new THREE.RingGeometry(r - 0.5, r - 0.24, 56), basic(0xffffff, 0.9));
    arc.position.y = 0.01;
    this.scene.add(g);
    return { g, ring, fill, beam, arc, r, prog: null };
  }

  zone(key, cp) {
    let z = this.zones.get(key);
    if (z && z.r !== cp.r) { this.drop(key); z = null; }
    if (!z) { z = this.makeZone(cp.r); this.zones.set(key, z); }
    z.used = true;
    z.g.position.set(cp.x, cp.y + 0.03, cp.z);
    const owner = new THREE.Color(TEAM_COLORS[cp.o]);
    const pulse = cp.c ? 0.5 + 0.5 * Math.sin(this.time * 10) : 1;
    z.ring.material.color.copy(cp.c ? new THREE.Color(0xffd27a) : owner);
    z.ring.material.opacity = 0.55 + 0.4 * pulse;
    z.fill.material.color.copy(owner);
    z.beam.material.color.copy(owner);
    // arco de progresso: só enquanto alguém está capturando
    const p = Math.abs(cp.p), show = p > 0.01 && p < 0.999;
    z.arc.visible = show;
    if (show && (z.prog === null || Math.abs(z.prog - cp.p) > 0.01)) {
      z.prog = cp.p;
      z.arc.geometry.dispose();
      z.arc.geometry = new THREE.RingGeometry(cp.r - 0.5, cp.r - 0.24, 56, 1, Math.PI / 2, p * Math.PI * 2).rotateX(-Math.PI / 2);
      z.arc.material.color.set(TEAM_COLORS[cp.p < 0 ? 1 : 2]);
    }
  }

  drop(key) {
    const z = this.zones.get(key);
    if (!z) return;
    this.scene.remove(z.g);
    z.g.traverse((o) => { o.geometry?.dispose(); o.material?.dispose(); });
    this.zones.delete(key);
  }

  makeFlag(team) {
    const color = TEAM_COLORS[team];
    const std = (c) => new THREE.MeshStandardMaterial({ color: c, flatShading: true, roughness: 0.8 });
    const flag = new THREE.Group();
    const pole = new THREE.Mesh(new THREE.CylinderGeometry(0.045, 0.045, 2.5, 6), std(0xd9d9d9));
    pole.position.y = 1.25;
    const cloth = new THREE.Mesh(new THREE.BoxGeometry(0.95, 0.6, 0.04, 4, 1, 1), std(color));
    cloth.geometry.translate(0.5, 0, 0);
    cloth.position.set(0.02, 2.15, 0);
    const tip = new THREE.Mesh(new THREE.OctahedronGeometry(0.09), std(0xf3d36b));
    tip.position.y = 2.55;
    flag.add(pole, cloth, tip);
    flag.traverse((o) => { o.castShadow = true; });
    const pad = new THREE.Group();
    const disc = new THREE.Mesh(new THREE.CylinderGeometry(1.3, 1.45, 0.14, 14), std(color));
    disc.position.y = 0.07;
    disc.receiveShadow = true;
    const ring = new THREE.Mesh(new THREE.RingGeometry(1.55, 1.75, 40).rotateX(-Math.PI / 2), basic(color, 0.7));
    ring.position.y = 0.03;
    pad.add(disc, ring);
    this.scene.add(flag, pad);
    return { flag, pad, cloth, base: null };
  }

  // carrierPos(id) → onde desenhar o boneco que carrega (ou null se sou eu)
  flagUpdate(f, carrierPos) {
    let v = this.flags[f.t];
    if (!v) v = this.flags[f.t] = this.makeFlag(f.t);
    v.used = true;
    if (f.s === 'base' && !v.base) v.base = new THREE.Vector3(f.x, f.y, f.z);
    if (v.base) v.pad.position.copy(v.base);
    v.pad.visible = !!v.base;
    v.cloth.rotation.y = Math.sin(this.time * 2.3 + f.t) * 0.35;
    if (f.s === 'carried') {
      const c = carrierPos(f.c);
      v.flag.visible = !!c;
      if (c) { // nas costas de quem carrega
        v.flag.position.set(c.x, c.y + 0.2, c.z);
        v.flag.scale.setScalar(0.75);
      }
    } else {
      v.flag.visible = true;
      v.flag.position.set(f.x, f.y, f.z);
      v.flag.scale.setScalar(1);
    }
  }

  update(dt, mt, carrierPos) {
    this.time += dt;
    for (const z of this.zones.values()) z.used = false;
    this.flags.forEach((v) => { if (v) v.used = false; });
    if (mt?.pts) mt.pts.forEach((cp) => this.zone(`p${cp.n}`, cp));
    if (mt?.hill) this.zone('hill', mt.hill);
    if (mt?.fl) mt.fl.forEach((f) => this.flagUpdate(f, carrierPos));
    for (const [k, z] of this.zones) if (!z.used) this.drop(k);
    this.flags.forEach((v, t) => {
      if (v && !v.used) {
        this.scene.remove(v.flag, v.pad);
        v.flag.traverse((o) => { o.geometry?.dispose(); o.material?.dispose(); });
        v.pad.traverse((o) => { o.geometry?.dispose(); o.material?.dispose(); });
        this.flags[t] = null;
      }
    });
  }
}
