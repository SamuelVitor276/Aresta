import * as THREE from 'three';

const MAX_PARTICLES = 600;

// Partículas são cubinhos num único InstancedMesh (1 chamada de desenho
// para todas). Ao morrer, o boneco "estilhaça" em cubos da cor dele.
export class Effects {
  constructor(scene) {
    this.scene = scene;
    this.mesh = new THREE.InstancedMesh(
      new THREE.BoxGeometry(1, 1, 1),
      new THREE.MeshLambertMaterial({ color: 0xffffff, flatShading: true }),
      MAX_PARTICLES,
    );
    this.mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    this.mesh.frustumCulled = false;
    scene.add(this.mesh);

    this.parts = [];
    const zero = new THREE.Matrix4().makeScale(0, 0, 0), white = new THREE.Color(1, 1, 1);
    for (let i = 0; i < MAX_PARTICLES; i++) {
      this.parts.push({ life: 0, max: 1, x: 0, y: 0, z: 0, vx: 0, vy: 0, vz: 0, size: 0, rx: 0, ry: 0, spin: 0, floor: 0, grav: 1 });
      this.mesh.setMatrixAt(i, zero);
      this.mesh.setColorAt(i, white);
    }
    this.next = 0;
    this.transients = []; // rastros e clarões que somem sozinhos
    this.unitBox = new THREE.BoxGeometry(1, 1, 1);
    this.flashGeo = new THREE.OctahedronGeometry(0.12);
    this.boomGeo = new THREE.IcosahedronGeometry(1, 0);

    this._m = new THREE.Matrix4();
    this._q = new THREE.Quaternion();
    this._e = new THREE.Euler();
    this._p = new THREE.Vector3();
    this._s = new THREE.Vector3();
    this._c = new THREE.Color();
  }

  burst(pos, color, count, speed, size, life, { up = 2, floor = 0, grav = 1 } = {}) {
    for (let n = 0; n < count; n++) {
      const i = this.next;
      this.next = (this.next + 1) % MAX_PARTICLES;
      const p = this.parts[i];
      const a = Math.random() * Math.PI * 2, s = speed * (0.4 + Math.random() * 0.8);
      p.x = pos.x; p.y = pos.y; p.z = pos.z;
      p.vx = Math.cos(a) * s;
      p.vz = Math.sin(a) * s;
      p.vy = up + Math.random() * speed * 0.8;
      p.size = size * (0.6 + Math.random() * 0.8);
      p.life = p.max = life * (0.7 + Math.random() * 0.6);
      p.rx = Math.random() * 6;
      p.ry = Math.random() * 6;
      p.spin = (Math.random() - 0.5) * 14;
      p.floor = floor;
      p.grav = grav;
      this._c.set(color).offsetHSL(0, 0, (Math.random() - 0.5) * 0.2);
      this.mesh.setColorAt(i, this._c);
    }
    this.mesh.instanceColor.needsUpdate = true;
  }

  impact(pos) {
    const floor = Math.max(0, pos.y - 1.5);
    this.burst(pos, 0xffd28a, 5, 3, 0.05, 0.3, { up: 1.5, floor });
    this.burst(pos, 0xb9a48c, 4, 1.5, 0.09, 0.5, { up: 1, floor });
  }

  blood(pos, color, floor = 0) {
    this.burst(pos, color, 7, 3, 0.08, 0.45, { up: 2, floor });
    this.burst(pos, 0xffffff, 3, 4, 0.04, 0.2, { up: 2, floor });
  }

  explode(pos, color) {
    const c = { x: pos.x, y: pos.y + 1, z: pos.z };
    this.burst(c, color, 26, 5.5, 0.2, 1.8, { up: 4, floor: pos.y });
    this.burst(c, 0x3b4256, 10, 4, 0.16, 1.6, { up: 3, floor: pos.y });
  }

  tracer(from, to, color = 0xffe3a3) {
    const dx = to.x - from.x, dy = to.y - from.y, dz = to.z - from.z;
    const len = Math.sqrt(dx * dx + dy * dy + dz * dz);
    if (len < 0.5) return;
    const m = new THREE.Mesh(this.unitBox, new THREE.MeshBasicMaterial({
      color, transparent: true, opacity: 0.85, blending: THREE.AdditiveBlending, depthWrite: false,
    }));
    m.scale.set(0.03, 0.03, len);
    m.position.set(from.x + dx / 2, from.y + dy / 2, from.z + dz / 2);
    m.lookAt(to.x, to.y, to.z);
    this.scene.add(m);
    this.transients.push({ obj: m, life: 0.08, max: 0.08 });
  }

  flash(pos) {
    const m = new THREE.Mesh(this.flashGeo, new THREE.MeshBasicMaterial({
      color: 0xffd27a, transparent: true, opacity: 1, blending: THREE.AdditiveBlending, depthWrite: false,
    }));
    m.position.set(pos.x, pos.y, pos.z);
    m.rotation.set(Math.random() * 3, Math.random() * 3, 0);
    this.scene.add(m);
    this.transients.push({ obj: m, life: 0.06, max: 0.06 });
  }

  // Explosão de granada: clarão, bola de fogo, estilhaços e fumaça que sobe
  explosion(pos) {
    const c = { x: pos.x, y: pos.y + 0.3, z: pos.z };
    this.burst(c, 0xffb347, 22, 7, 0.22, 0.7, { up: 5, floor: Math.max(0, pos.y - 0.1), grav: 0.6 });
    this.burst(c, 0xff6a2b, 14, 5, 0.3, 0.5, { up: 4, floor: Math.max(0, pos.y - 0.1), grav: 0.4 });
    this.burst(c, 0x3a3530, 16, 9, 0.12, 1.4, { up: 6, floor: Math.max(0, pos.y - 0.1) });
    const fire = new THREE.Mesh(this.boomGeo, new THREE.MeshBasicMaterial({
      color: 0xffd08a, transparent: true, opacity: 1, blending: THREE.AdditiveBlending, depthWrite: false,
    }));
    fire.position.set(c.x, c.y, c.z);
    this.scene.add(fire);
    this.transients.push({ obj: fire, life: 0.28, max: 0.28, grow: 4.2 });
    for (let i = 0; i < 7; i++) {
      const puff = new THREE.Mesh(this.boomGeo, new THREE.MeshLambertMaterial({
        color: 0x8c8278, transparent: true, opacity: 0.75, flatShading: true, depthWrite: false,
      }));
      puff.position.set(c.x + (Math.random() - 0.5) * 2, c.y + Math.random() * 0.8, c.z + (Math.random() - 0.5) * 2);
      puff.scale.setScalar(0.6);
      this.scene.add(puff);
      this.transients.push({ obj: puff, life: 2.2 + Math.random(), max: 3, grow: 2.4, rise: 0.9 + Math.random() * 0.6, smoke: true });
    }
  }

  update(dt) {
    let dirty = false;
    for (let i = 0; i < MAX_PARTICLES; i++) {
      const p = this.parts[i];
      if (p.life <= 0) continue;
      dirty = true;
      p.life -= dt;
      if (p.life <= 0) {
        this.mesh.setMatrixAt(i, this._m.makeScale(0, 0, 0));
        continue;
      }
      p.vy -= 18 * p.grav * dt;
      p.x += p.vx * dt;
      p.y += p.vy * dt;
      p.z += p.vz * dt;
      const half = p.size / 2;
      if (p.y < p.floor + half) { // quica no chão e perde energia
        p.y = p.floor + half;
        p.vy *= -0.35;
        p.vx *= 0.6;
        p.vz *= 0.6;
        p.spin *= 0.6;
      }
      p.rx += p.spin * dt;
      p.ry += p.spin * dt * 0.7;
      const k = Math.min(1, p.life / (p.max * 0.35)); // encolhe no final
      this._m.compose(this._p.set(p.x, p.y, p.z), this._q.setFromEuler(this._e.set(p.rx, p.ry, 0)), this._s.setScalar(p.size * k));
      this.mesh.setMatrixAt(i, this._m);
    }
    if (dirty) this.mesh.instanceMatrix.needsUpdate = true;

    for (let i = this.transients.length - 1; i >= 0; i--) {
      const t = this.transients[i];
      t.life -= dt;
      if (t.life <= 0) {
        this.scene.remove(t.obj);
        t.obj.material.dispose();
        this.transients.splice(i, 1);
      } else {
        const k = t.life / t.max;
        t.obj.material.opacity = t.smoke ? Math.min(0.7, k * 1.2) : k * 0.9;
        if (t.grow) t.obj.scale.setScalar(t.smoke ? 0.6 + (1 - k) * t.grow : 0.3 + (1 - k) * t.grow);
        if (t.rise) t.obj.position.y += t.rise * dt;
      }
    }
  }
}
