// Física, armas e granadas.
//
// ESTE ARQUIVO É ESPELHO DE server/physics.go.
// O cliente roda a mesma simulação do servidor para prever o próprio
// movimento, a munição e as granadas. Mudou algo aqui? Mude lá também.

export const TICK_RATE = 60;
export const DT = 1 / TICK_RATE;

export const KEY = {
  FORWARD: 1, BACK: 2, LEFT: 4, RIGHT: 8, JUMP: 16, FIRE: 32, RELOAD: 64,
  AIM: 128, MELEE: 256, NADE: 512,
};

export const P = {
  halfW: 0.4, height: 1.8, eye: 1.6, headBase: 1.45, headHalfW: 0.25,
  speed: 7.5, groundAccel: 14, airAccel: 2.5, gravity: 22, jump: 8, step: 0.55,
};

export const W_AR = 0, W_SNIPER = 1, W_SHOTGUN = 2, W_KNIFE = 3, W_NADE = 4;

export const WEAPONS = [
  { name: 'Fuzil', mag: 30, fireInterval: 6, reload: 96, pellets: 1,
    spread: 0.018, aimSpread: 0.004, moveSpread: 0.02, range: 200, moveMul: 1, aimMoveMul: 0.7 },
  { name: 'Sniper', mag: 5, fireInterval: 72, reload: 150, pellets: 1,
    spread: 0.08, aimSpread: 0, moveSpread: 0.05, range: 300, moveMul: 0.93, aimMoveMul: 0.5 },
  { name: 'Escopeta', mag: 6, fireInterval: 48, reload: 120, pellets: 9,
    spread: 0.075, aimSpread: 0.055, moveSpread: 0.01, range: 45, moveMul: 1.05, aimMoveMul: 0.85 },
];

export const AIM_TIME = 12;
export const KNIFE = { range: 2.3, cooldown: 33, busy: 21 };
export const NADE = {
  count: 2, cooldown: 45, busy: 24, fuse: 150, speed: 15, up: 3.5, gravity: 18,
  bounce: 0.35, friction: 0.75, roll: 0.9, half: 0.09, radius: 7,
};

export const A_NONE = 0, A_FIRE = 1, A_RELOAD = 2, A_MELEE = 3, A_NADE = 4;

const EPS = 1e-4;

export function newState(x = 0, y = 0, z = 0) {
  return { x, y, z, vx: 0, vy: 0, vz: 0, onGround: false };
}

export function newWeapon(kind = W_AR) {
  if (!(kind >= 0 && kind < WEAPONS.length)) kind = W_AR;
  return { kind, ammo: WEAPONS[kind].mag, nextFire: 0, reloadEnd: 0, busy: 0,
    nextMelee: 0, nextNade: 0, nades: NADE.count, aimSince: 0 };
}

// Converte o estado da arma vindo do servidor (nomes curtos do JSON)
export function weaponFromServer(y, w) {
  w.kind = y.wk; w.ammo = y.am; w.nextFire = y.nf; w.reloadEnd = y.re; w.busy = y.bu;
  w.nextMelee = y.nm; w.nextNade = y.nn; w.nades = y.ng; w.aimSince = y.as;
  return w;
}

function overlaps(x, y, z, b) {
  return x + P.halfW > b.min[0] && x - P.halfW < b.max[0] &&
         y + P.height > b.min[1] && y < b.max[1] &&
         z + P.halfW > b.min[2] && z - P.halfW < b.max[2];
}

export function collidesAny(x, y, z, boxes) {
  for (let i = 0; i < boxes.length; i++) if (overlaps(x, y, z, boxes[i])) return true;
  return false;
}

function tryStep(s, b, boxes) {
  const rise = b.max[1] - s.y;
  if (rise <= 0 || rise > P.step) return false;
  if (collidesAny(s.x, b.max[1], s.z, boxes)) return false;
  s.y = b.max[1];
  return true;
}

// Avança um jogador em um tick. mul ajusta a velocidade (arma, mira).
export function step(s, keys, yaw, boxes, mul = 1) {
  let f = 0, r = 0;
  if (keys & KEY.FORWARD) f += 1;
  if (keys & KEY.BACK) f -= 1;
  if (keys & KEY.RIGHT) r += 1;
  if (keys & KEY.LEFT) r -= 1;
  const sn = Math.sin(yaw), cs = Math.cos(yaw);
  let wx = -sn * f + cs * r;
  let wz = -cs * f - sn * r;
  const l = Math.sqrt(wx * wx + wz * wz);
  if (l > 0) {
    const sp = P.speed * mul;
    wx = wx / l * sp;
    wz = wz / l * sp;
  }
  const k = Math.min(1, (s.onGround ? P.groundAccel : P.airAccel) * DT);
  s.vx += (wx - s.vx) * k;
  s.vz += (wz - s.vz) * k;
  if ((keys & KEY.JUMP) && s.onGround) s.vy = P.jump;
  s.vy -= P.gravity * DT;

  const canStep = s.onGround;
  s.onGround = false;

  const dx = s.vx * DT;
  if (dx !== 0) {
    s.x += dx;
    for (const b of boxes) {
      if (!overlaps(s.x, s.y, s.z, b)) continue;
      if (canStep && tryStep(s, b, boxes)) continue;
      s.x = dx > 0 ? b.min[0] - P.halfW - EPS : b.max[0] + P.halfW + EPS;
      s.vx = 0;
    }
  }
  const dz = s.vz * DT;
  if (dz !== 0) {
    s.z += dz;
    for (const b of boxes) {
      if (!overlaps(s.x, s.y, s.z, b)) continue;
      if (canStep && tryStep(s, b, boxes)) continue;
      s.z = dz > 0 ? b.min[2] - P.halfW - EPS : b.max[2] + P.halfW + EPS;
      s.vz = 0;
    }
  }
  const dy = s.vy * DT;
  if (dy !== 0) {
    s.y += dy;
    for (const b of boxes) {
      if (!overlaps(s.x, s.y, s.z, b)) continue;
      if (dy < 0) { s.y = b.max[1]; s.onGround = true; }
      else s.y = b.min[1] - P.height - EPS;
      s.vy = 0;
    }
  }
  if (s.y < 0) { s.y = 0; s.vy = 0; s.onGround = true; }
}

export const clampPitch = (p) => Math.max(-1.55, Math.min(1.55, p));

// ------------------------------------------------------------ armas

export function stepWeapon(w, keys, seq) {
  const d = WEAPONS[w.kind];
  if (!(keys & KEY.AIM)) w.aimSince = 0;
  else if (w.aimSince === 0) w.aimSince = seq;
  if (w.reloadEnd > 0 && seq >= w.reloadEnd) { w.ammo = d.mag; w.reloadEnd = 0; }
  if ((keys & KEY.NADE) && w.nades > 0 && seq >= w.nextNade && seq >= w.busy) {
    w.nades--; w.nextNade = seq + NADE.cooldown; w.busy = seq + NADE.busy;
    return A_NADE;
  }
  if ((keys & KEY.MELEE) && seq >= w.nextMelee && seq >= w.busy) {
    w.nextMelee = seq + KNIFE.cooldown; w.busy = seq + KNIFE.busy;
    return A_MELEE;
  }
  if (w.reloadEnd > 0 || seq < w.busy) return A_NONE;
  let reload = (keys & KEY.RELOAD) !== 0 && w.ammo < d.mag;
  if ((keys & KEY.FIRE) && w.ammo === 0) reload = true;
  if (reload) { w.reloadEnd = seq + d.reload; return A_RELOAD; }
  if ((keys & KEY.FIRE) && seq >= w.nextFire) {
    w.ammo--; w.nextFire = seq + d.fireInterval;
    return A_FIRE;
  }
  return A_NONE;
}

export const aimed = (w, keys, seq) => (keys & KEY.AIM) !== 0 && w.aimSince > 0 && seq - w.aimSince >= AIM_TIME;
export const moveMul = (w, keys) => ((keys & KEY.AIM) ? WEAPONS[w.kind].aimMoveMul : WEAPONS[w.kind].moveMul);
export const isMoving = (s) => !s.onGround || s.vx * s.vx + s.vz * s.vz > 6.25;

function hash32(a, b) {
  let h = Math.imul(a >>> 0, 0x9E3779B1) ^ Math.imul((b + 0x7F4A7C15) >>> 0, 0x85EBCA77);
  h ^= h >>> 15; h = Math.imul(h, 0x2C1B3C6D);
  h ^= h >>> 12; h = Math.imul(h, 0x297A2D39);
  h ^= h >>> 15;
  return h >>> 0;
}
const rand01 = (seq, i, salt) => hash32(seq, i * 7 + salt) / 4294967296;

// Direção de cada projétil (mesmo sorteio do servidor)
export function shotDirs(w, yaw, pitch, keys, seq, s) {
  const d = WEAPONS[w.kind];
  let spread = aimed(w, keys, seq) ? d.aimSpread : d.spread;
  if (isMoving(s)) spread += d.moveSpread;
  const out = [];
  for (let i = 0; i < d.pellets; i++) {
    const u1 = rand01(seq, i, 1), u2 = rand01(seq, i, 2);
    let a, r;
    if (d.pellets === 1) { a = 2 * Math.PI * u1; r = spread * Math.sqrt(u2); }
    else if (i === 0) { a = 2 * Math.PI * u1; r = spread * 0.2 * u2; }
    else { a = 2 * Math.PI * ((i - 1) / (d.pellets - 1) + u1 * 0.08); r = spread * (0.5 + 0.5 * u2); }
    out.push(aimDir(yaw + r * Math.cos(a), clampPitch(pitch + r * Math.sin(a))));
  }
  return out;
}

// ------------------------------------------------------------ granadas

export function nadeLaunch(s, yaw, pitch) {
  const d = aimDir(yaw, clampPitch(pitch));
  return {
    x: s.x + d.x * 0.3, y: s.y + P.eye - 0.1 + d.y * 0.3, z: s.z + d.z * 0.3,
    vx: d.x * NADE.speed + s.vx * 0.4, vy: d.y * NADE.speed + NADE.up, vz: d.z * NADE.speed + s.vz * 0.4,
    rest: false,
  };
}

function nadeHit(n, b) {
  const h = NADE.half;
  return n.x + h > b.min[0] && n.x - h < b.max[0] &&
         n.y + h > b.min[1] && n.y - h < b.max[1] &&
         n.z + h > b.min[2] && n.z - h < b.max[2];
}

// Avança a granada um tick. Devolve a velocidade do impacto (som de quique).
export function stepNade(n, boxes) {
  if (n.rest) return 0;
  let impact = 0;
  n.vy -= NADE.gravity * DT;
  if (n.vy < -25) n.vy = -25;
  const dx = n.vx * DT;
  if (dx !== 0) {
    n.x += dx;
    for (const b of boxes) {
      if (!nadeHit(n, b)) continue;
      n.x = dx > 0 ? b.min[0] - NADE.half - EPS : b.max[0] + NADE.half + EPS;
      impact = Math.max(impact, Math.abs(n.vx));
      n.vx = -n.vx * NADE.bounce;
      n.vz *= NADE.friction;
    }
  }
  const dz = n.vz * DT;
  if (dz !== 0) {
    n.z += dz;
    for (const b of boxes) {
      if (!nadeHit(n, b)) continue;
      n.z = dz > 0 ? b.min[2] - NADE.half - EPS : b.max[2] + NADE.half + EPS;
      impact = Math.max(impact, Math.abs(n.vz));
      n.vz = -n.vz * NADE.bounce;
      n.vx *= NADE.friction;
    }
  }
  let landed = false;
  const dy = n.vy * DT;
  if (dy !== 0) {
    n.y += dy;
    for (const b of boxes) {
      if (!nadeHit(n, b)) continue;
      impact = Math.max(impact, Math.abs(n.vy));
      if (dy < 0) { n.y = b.max[1] + NADE.half; landed = true; }
      else n.y = b.min[1] - NADE.half - EPS;
      n.vy = -n.vy * NADE.bounce;
    }
  }
  if (n.y < NADE.half) {
    impact = Math.max(impact, Math.abs(n.vy));
    n.y = NADE.half;
    n.vy = -n.vy * NADE.bounce;
    landed = true;
  }
  if (landed) {
    if (n.vy > 1.2) { n.vx *= NADE.friction; n.vz *= NADE.friction; }
    else {
      n.vy = 0; n.vx *= NADE.roll; n.vz *= NADE.roll;
      if (n.vx * n.vx + n.vz * n.vz < 0.04) { n.vx = 0; n.vz = 0; n.rest = true; }
    }
  }
  return impact;
}

// ------------------------------------------------------------ raios

export function aimDir(yaw, pitch) {
  const cp = Math.cos(pitch);
  return { x: -Math.sin(yaw) * cp, y: Math.sin(pitch), z: -Math.cos(yaw) * cp };
}

export function rayBox(o, d, min, max) {
  let tmin = 0, tmax = Infinity;
  const oa = [o.x, o.y, o.z], da = [d.x, d.y, d.z];
  for (let i = 0; i < 3; i++) {
    if (Math.abs(da[i]) < 1e-12) {
      if (oa[i] < min[i] || oa[i] > max[i]) return -1;
      continue;
    }
    const inv = 1 / da[i];
    let t1 = (min[i] - oa[i]) * inv, t2 = (max[i] - oa[i]) * inv;
    if (t1 > t2) { const t = t1; t1 = t2; t2 = t; }
    if (t1 > tmin) tmin = t1;
    if (t2 < tmax) tmax = t2;
    if (tmin > tmax) return -1;
  }
  return tmin;
}

export function rayWorld(o, d, boxes, maxDist) {
  let best = maxDist;
  if (d.y < -1e-9) {
    const t = -o.y / d.y;
    if (t < best) best = t;
  }
  for (const b of boxes) {
    const t = rayBox(o, d, b.min, b.max);
    if (t >= 0 && t < best) best = t;
  }
  return best;
}

export function rayPlayer(o, d, x, y, z, maxT) {
  let hit = null;
  const tb = rayBox(o, d, [x - P.halfW, y, z - P.halfW], [x + P.halfW, y + P.headBase, z + P.halfW]);
  if (tb >= 0 && tb < maxT) { hit = { t: tb, head: false }; maxT = tb; }
  const th = rayBox(o, d,
    [x - P.headHalfW, y + P.headBase, z - P.headHalfW],
    [x + P.headHalfW, y + P.height + 0.05, z + P.headHalfW]);
  if (th >= 0 && th < maxT) hit = { t: th, head: true };
  return hit;
}
