// Todos os sons são sintetizados na hora: ruído branco filtrado + osciladores
// com envelopes. Nenhum arquivo de áudio. Sons de outros jogadores passam por
// um PannerNode (HRTF), então dá pra ouvir de onde vem o tiro ou o passo.

function setPos(node, x, y, z) {
  if (node.positionX) {
    node.positionX.value = x;
    node.positionY.value = y;
    node.positionZ.value = z;
  } else node.setPosition(x, y, z);
}

export class Sfx {
  constructor() {
    this.ctx = null;
  }

  // Precisa ser chamado num clique (o navegador só libera áudio com gesto do usuário)
  init() {
    if (this.ctx) {
      this.ctx.resume();
      return;
    }
    const AC = window.AudioContext || window.webkitAudioContext;
    if (!AC) return;
    const ctx = (this.ctx = new AC());

    this.master = ctx.createGain();
    this.master.gain.value = 0.8;
    const comp = ctx.createDynamicsCompressor();
    comp.threshold.value = -14;
    comp.ratio.value = 4;
    this.master.connect(comp).connect(ctx.destination);

    // 2 s de ruído branco, reaproveitado por quase todos os sons
    const len = ctx.sampleRate * 2;
    this.noiseBuf = ctx.createBuffer(1, len, ctx.sampleRate);
    const d = this.noiseBuf.getChannelData(0);
    for (let i = 0; i < len; i++) d[i] = Math.random() * 2 - 1;

    this.startWind();
  }

  // ---------------------------------------------------------------- blocos básicos

  out(pos) {
    if (!pos) return this.master;
    const p = this.ctx.createPanner();
    p.panningModel = 'HRTF';
    p.distanceModel = 'inverse';
    p.refDistance = 3;
    p.maxDistance = 120;
    p.rolloffFactor = 1.1;
    setPos(p, pos.x, pos.y, pos.z);
    p.connect(this.master);
    return p;
  }

  gain(dest, t, peak, attack, decay) {
    const g = this.ctx.createGain();
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(peak, t + attack);
    g.gain.exponentialRampToValueAtTime(0.0001, t + attack + decay);
    g.connect(dest);
    return g;
  }

  filter(type, freq, q, dest) {
    const f = this.ctx.createBiquadFilter();
    f.type = type;
    f.frequency.value = freq;
    f.Q.value = q;
    f.connect(dest);
    return f;
  }

  noise(t, dur, dest) {
    const src = this.ctx.createBufferSource();
    src.buffer = this.noiseBuf;
    src.connect(dest);
    src.start(t, Math.random() * 1.4, dur);
  }

  tone(type, t, dur, f0, f1, dest) {
    const o = this.ctx.createOscillator();
    o.type = type;
    o.frequency.setValueAtTime(f0, t);
    if (f1 !== f0) o.frequency.exponentialRampToValueAtTime(f1, t + dur);
    o.connect(dest);
    o.start(t);
    o.stop(t + dur + 0.05);
  }

  // Ouvido do jogador: segue a câmera (só o yaw, pra orientação ficar estável)
  setListener(x, y, z, yaw) {
    if (!this.ctx) return;
    const l = this.ctx.listener, fx = -Math.sin(yaw), fz = -Math.cos(yaw);
    if (l.positionX) {
      l.positionX.value = x; l.positionY.value = y; l.positionZ.value = z;
      l.forwardX.value = fx; l.forwardY.value = 0; l.forwardZ.value = fz;
      l.upX.value = 0; l.upY.value = 1; l.upZ.value = 0;
    } else {
      l.setPosition(x, y, z);
      l.setOrientation(fx, 0, fz, 0, 1, 0);
    }
  }

  // ---------------------------------------------------------------- sons do jogo

  // Tiro: cada arma tem um timbre. 0 fuzil, 1 sniper, 2 escopeta
  shot(pos, vol = 1, kind = 0) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, out = this.out(pos);
    const cfg = [
      { crack: 7000, end: 450, len: 0.16, thump: 160, tail: 0.38, tailVol: 0.16 },
      { crack: 9000, end: 300, len: 0.3, thump: 120, tail: 1.1, tailVol: 0.3 },
      { crack: 4500, end: 220, len: 0.26, thump: 90, tail: 0.6, tailVol: 0.24 },
    ][kind] ?? { crack: 7000, end: 450, len: 0.16, thump: 160, tail: 0.38, tailVol: 0.16 };
    const lp = this.filter('lowpass', cfg.crack, 0.7, this.gain(out, t, 0.9 * vol, 0.002, cfg.len));
    lp.frequency.setValueAtTime(cfg.crack, t);
    lp.frequency.exponentialRampToValueAtTime(cfg.end, t + cfg.len * 0.9);
    this.noise(t, cfg.len + 0.05, lp);
    this.tone('sine', t, cfg.len * 0.8, cfg.thump, 38, this.gain(out, t, (kind === 0 ? 0.9 : 1.1) * vol, 0.002, cfg.len * 0.75));
    this.noise(t + 0.02, cfg.tail + 0.1, this.filter('bandpass', kind === 1 ? 600 : 900, 0.8, this.gain(out, t + 0.02, cfg.tailVol * vol, 0.03, cfg.tail)));
    if (kind === 2) this.click(t + 0.42, 900, 0.35 * vol, out), this.click(t + 0.58, 1300, 0.35 * vol, out);   // bomba
    if (kind === 1) this.click(t + 0.5, 1600, 0.3 * vol, out), this.click(t + 0.72, 2200, 0.3 * vol, out);    // ferrolho
  }

  knife(pos) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, out = this.out(pos);
    const bp = this.filter('bandpass', 1800, 2, this.gain(out, t, 0.22, 0.02, 0.16));
    bp.frequency.setValueAtTime(1200, t);
    bp.frequency.exponentialRampToValueAtTime(3200, t + 0.16);
    this.noise(t, 0.2, bp);
  }

  knifeHit() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    this.tone('sine', t, 0.1, 140, 60, this.gain(this.master, t, 0.5, 0.002, 0.09));
    this.noise(t, 0.08, this.filter('lowpass', 1400, 1, this.gain(this.master, t, 0.35, 0.002, 0.07)));
  }

  nadeThrow() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    this.click(t, 2600, 0.25, this.master); // pino
    const bp = this.filter('bandpass', 600, 1.4, this.gain(this.master, t + 0.12, 0.16, 0.03, 0.2));
    bp.frequency.setValueAtTime(600, t + 0.12);
    bp.frequency.exponentialRampToValueAtTime(1800, t + 0.32);
    this.noise(t + 0.12, 0.25, bp);
  }

  nadeBounce(pos, k = 1) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, out = this.out(pos);
    this.tone('triangle', t, 0.08, 1900 + Math.random() * 400, 1500, this.gain(out, t, 0.18 * Math.min(1, k), 0.001, 0.07));
    this.click(t, 3000, 0.12 * Math.min(1, k), out);
  }

  boom(pos) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, out = this.out(pos);
    this.tone('sine', t, 0.7, 90, 28, this.gain(out, t, 1.4, 0.004, 0.65));
    const lp = this.filter('lowpass', 2400, 0.7, this.gain(out, t, 1.3, 0.003, 0.9));
    lp.frequency.setValueAtTime(2400, t);
    lp.frequency.exponentialRampToValueAtTime(180, t + 0.9);
    this.noise(t, 1.0, lp);
    this.noise(t + 0.05, 1.6, this.filter('lowpass', 500, 0.6, this.gain(this.master, t + 0.05, 0.25, 0.08, 1.4)));
  }

  chatBlip() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    this.tone('sine', t, 0.08, 880, 1320, this.gain(this.master, t, 0.08, 0.004, 0.07));
  }

  roundStart() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    [330, 440, 660].forEach((f, i) => {
      const tt = t + i * 0.12;
      this.tone('square', tt, 0.16, f, f, this.filter('lowpass', 1800, 1, this.gain(this.master, tt, 0.12, 0.005, 0.15)));
    });
    this.tone('sawtooth', t + 0.36, 0.7, 220, 220, this.filter('lowpass', 900, 1, this.gain(this.master, t + 0.36, 0.14, 0.04, 0.6)));
  }

  victory() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    [523, 659, 784, 1047].forEach((f, i) => {
      const tt = t + i * 0.11;
      this.tone('triangle', tt, 0.5, f, f, this.gain(this.master, tt, 0.18, 0.005, 0.45));
    });
    [523, 659, 784].forEach((f) => this.tone('sine', t + 0.5, 1.2, f, f, this.gain(this.master, t + 0.5, 0.07, 0.05, 1.1)));
  }

  // Objetivo: duas notas subindo (bom para você) ou descendo (ruim)
  objective(good) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, notes = good ? [587, 880] : [523, 349];
    notes.forEach((f, i) => {
      const tt = t + i * 0.12;
      this.tone('triangle', tt, 0.22, f, f, this.gain(this.master, tt, 0.16, 0.005, 0.2));
    });
  }

  scope() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    this.click(t, 1500, 0.15, this.master);
  }

  step(pos, vol = 0.2) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, out = this.out(pos);
    this.noise(t, 0.09, this.filter('lowpass', 450 + Math.random() * 500, 1.4, this.gain(out, t, vol, 0.004, 0.07)));
    this.tone('sine', t, 0.06, 95 + Math.random() * 25, 60, this.gain(out, t, vol * 0.5, 0.002, 0.05));
  }

  jump() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    const bp = this.filter('bandpass', 500, 1.2, this.gain(this.master, t, 0.12, 0.02, 0.14));
    bp.frequency.setValueAtTime(500, t);
    bp.frequency.exponentialRampToValueAtTime(1600, t + 0.15);
    this.noise(t, 0.18, bp);
  }

  land(intensity = 1) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, v = Math.min(1, intensity);
    this.tone('sine', t, 0.12, 110, 45, this.gain(this.master, t, 0.5 * v, 0.003, 0.1));
    this.noise(t, 0.12, this.filter('lowpass', 380, 1, this.gain(this.master, t, 0.3 * v, 0.003, 0.1)));
  }

  hit(head) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    this.tone('square', t, 0.05, head ? 2300 : 1500, head ? 2300 : 1500,
      this.filter('lowpass', 4000, 1, this.gain(this.master, t, 0.16, 0.001, 0.05)));
    if (head) this.tone('sine', t + 0.03, 0.3, 3100, 3000, this.gain(this.master, t + 0.03, 0.18, 0.002, 0.28));
  }

  hurt() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    this.tone('sawtooth', t, 0.2, 150, 70, this.filter('lowpass', 900, 1, this.gain(this.master, t, 0.32, 0.005, 0.18)));
  }

  kill() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    [660, 880, 1320].forEach((f, i) => {
      const tt = t + 0.05 + i * 0.065;
      this.tone('triangle', tt, 0.14, f, f, this.gain(this.master, tt, 0.2, 0.004, 0.13));
    });
  }

  death(pos) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, out = this.out(pos);
    this.tone('sawtooth', t, 0.9, 420, 45, this.filter('lowpass', 1200, 1, this.gain(out, t, 0.28, 0.01, 0.9)));
    this.noise(t, 0.6, this.filter('lowpass', 300, 1, this.gain(out, t, 0.25, 0.01, 0.55)));
  }

  click(t, freq, vol, out) {
    this.noise(t, 0.04, this.filter('bandpass', freq, 6, this.gain(out, t, vol, 0.001, 0.035)));
    this.tone('square', t, 0.02, freq / 2, freq / 2, this.gain(out, t, vol * 0.3, 0.001, 0.02));
  }

  reload(pos, vol = 0.5, kind = 0) {
    if (!this.ctx) return;
    const t = this.ctx.currentTime, out = this.out(pos);
    if (kind === 2) { // cartucho por cartucho
      for (let i = 0; i < 6; i++) this.click(t + 0.2 + i * 0.27, 1100 + (i % 2) * 300, vol, out);
      return;
    }
    const k = kind === 1 ? 1.5 : 1;
    this.click(t + 0.12 * k, 1800, vol, out);  // pente sai
    this.click(t + 0.95 * k, 1200, vol, out);  // pente entra
    this.click(t + 1.35 * k, 2400, vol, out);  // ferrolho
  }

  spawn() {
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    [523, 659, 784, 1047].forEach((f, i) => {
      const tt = t + i * 0.05;
      this.tone('sine', tt, 0.35, f, f * 1.01, this.gain(this.master, tt, 0.09, 0.01, 0.32));
    });
  }

  // Vento de fundo: ruído em loop com um passa-baixa "respirando" devagar
  startWind() {
    const ctx = this.ctx;
    const src = ctx.createBufferSource();
    src.buffer = this.noiseBuf;
    src.loop = true;
    const g = ctx.createGain();
    g.gain.value = 0.045;
    g.connect(this.master);
    const lp = this.filter('lowpass', 420, 0.6, g);
    const lfo = ctx.createOscillator();
    lfo.frequency.value = 0.07;
    const depth = ctx.createGain();
    depth.gain.value = 260;
    lfo.connect(depth).connect(lp.frequency);
    src.connect(lp);
    src.start();
    lfo.start();
  }
}
