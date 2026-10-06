import { KEY } from './physics.js';

// Usa e.code (posição física da tecla), então WASD funciona igual no ABNT2.
export class Input {
  constructor(el) {
    this.el = el;
    this.keys = new Set();
    this.fire = false;
    this.aim = false;
    this.trig = { melee: 0, nade: 0 }; // hora do toque (vale por 250 ms se a ação ainda não puder sair)
    this.sensMul = 1; // a mira com zoom reduz a sensibilidade
    this.onKey = null;
    this.typing = false; // chat aberto: teclado e mouse não controlam o boneco
    this.locked = false;
    this.yaw = 0;
    this.pitch = 0;
    this.sens = 0.0022;
    this.lookX = 0; // movimento do mouse acumulado no frame (balanço da arma)
    this.lookY = 0;
    this.onLockChange = null;

    addEventListener('keydown', (e) => {
      if (!this.locked || this.typing) return;
      if (e.code === 'Tab' || e.code === 'Space' || e.code.startsWith('Arrow')) e.preventDefault();
      if (!e.repeat) {
        if (e.code === 'KeyV' || e.code === 'KeyF') this.trig.melee = performance.now();
        if (e.code === 'KeyG') this.trig.nade = performance.now();
        this.onKey?.(e.code);
      }
      this.keys.add(e.code);
    });
    addEventListener('keyup', (e) => this.keys.delete(e.code));
    addEventListener('mousemove', (e) => {
      if (!this.locked || this.typing) return;
      this.yaw -= e.movementX * this.sens * this.sensMul;
      this.pitch -= e.movementY * this.sens * this.sensMul;
      this.clampAngles();
      this.lookX += e.movementX;
      this.lookY += e.movementY;
    });
    addEventListener('mousedown', (e) => {
      if (!this.locked || this.typing) return;
      if (e.button === 0) this.fire = true;
      if (e.button === 2) this.aim = true;
      if (e.button === 1) { e.preventDefault(); this.trig.melee = performance.now(); }
    });
    addEventListener('mouseup', (e) => {
      if (e.button === 0) this.fire = false;
      if (e.button === 2) this.aim = false;
    });
    addEventListener('contextmenu', (e) => e.preventDefault());
    addEventListener('blur', () => this.release());
    document.addEventListener('pointerlockchange', () => {
      this.locked = document.pointerLockElement === this.el;
      if (!this.locked) this.release();
      this.onLockChange?.(this.locked);
    });
  }

  release() {
    this.keys.clear();
    this.fire = false;
    this.aim = false;
  }

  setTyping(on) {
    this.typing = on;
    if (on) this.release();
  }

  armed(t) {
    return t > 0 && performance.now() - t < 250;
  }

  clampAngles() {
    this.pitch = Math.max(-1.55, Math.min(1.55, this.pitch));
    if (this.yaw > Math.PI) this.yaw -= Math.PI * 2;
    else if (this.yaw < -Math.PI) this.yaw += Math.PI * 2;
  }

  lock() {
    try {
      const p = this.el.requestPointerLock();
      p?.catch?.(() => {});
    } catch { /* o navegador recusou; o jogador clica de novo */ }
  }

  has(...codes) {
    return codes.some((c) => this.keys.has(c));
  }

  bits() {
    let k = 0;
    if (this.has('KeyW', 'ArrowUp')) k |= KEY.FORWARD;
    if (this.has('KeyS', 'ArrowDown')) k |= KEY.BACK;
    if (this.has('KeyA', 'ArrowLeft')) k |= KEY.LEFT;
    if (this.has('KeyD', 'ArrowRight')) k |= KEY.RIGHT;
    if (this.has('Space')) k |= KEY.JUMP;
    if (this.has('KeyR')) k |= KEY.RELOAD;
    if (this.fire) k |= KEY.FIRE;
    if (this.aim) k |= KEY.AIM;
    if (this.armed(this.trig.melee)) k |= KEY.MELEE;
    if (this.armed(this.trig.nade)) k |= KEY.NADE;
    return k;
  }

  consumeLook() {
    const l = { x: this.lookX, y: this.lookY };
    this.lookX = this.lookY = 0;
    return l;
  }
}
