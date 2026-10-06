import * as THREE from 'three';
import {
  TICK_RATE, DT, P, WEAPONS, NADE,
  W_SNIPER, W_SHOTGUN, W_KNIFE,
  A_FIRE, A_RELOAD, A_MELEE, A_NADE,
  newState, newWeapon, weaponFromServer, step, stepWeapon, moveMul, isMoving, aimed,
  shotDirs, nadeLaunch, stepNade, rayWorld, rayPlayer,
} from './physics.js';
import { Net } from './net.js';
import { Input } from './input.js';
import { Sfx } from './audio.js';
import { World } from './world.js';
import { PlayerModel, ViewModel, playerColor, buildNade } from './models.js';
import { Effects } from './effects.js';
import { Hud } from './hud.js';
import { Objectives, TEAM_COLORS } from './objectives.js';

// Os outros jogadores são desenhados 100 ms "no passado", interpolando entre
// dois snapshots. Isso esconde a variação de latência da rede (jitter).
const INTERP_DELAY = 0.1;
const BASE_FOV = 78;
const ADS_FOV = [60, 22, 68]; // fuzil, sniper (luneta), escopeta
const ADS_TIME = 0.2;         // igual ao AIM_TIME da física (12 ticks)
const RECOIL = [0.011, 0.045, 0.03];
const reduceMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;

// ------------------------------------------------------------ renderização

const canvas = document.getElementById('game');
const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, powerPreference: 'high-performance' });
renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
renderer.setSize(innerWidth, innerHeight);
renderer.shadowMap.enabled = true;
renderer.shadowMap.type = THREE.PCFSoftShadowMap;
renderer.autoClear = false;

const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(BASE_FOV, innerWidth / innerHeight, 0.05, 900);
camera.rotation.order = 'YXZ';

// A arma fica numa cena própria, desenhada por cima: nunca atravessa parede
const fpScene = new THREE.Scene();
const fpCam = new THREE.PerspectiveCamera(62, innerWidth / innerHeight, 0.01, 10);
const fpSun = new THREE.DirectionalLight(0xffe0b5, 2.2);
fpSun.position.set(-1, 2, 1);
fpScene.add(fpCam, fpSun, new THREE.HemisphereLight(0xdfe8ff, 0x8a6a4a, 1.6));

const world = new World(scene);
const objectives = new Objectives(scene);
const fx = new Effects(scene);
const sfx = new Sfx();
const hud = new Hud();
const input = new Input(canvas);
const net = new Net();
const muzzleLight = new THREE.PointLight(0xffc070, 0, 10, 2);
scene.add(muzzleLight);
let viewModel = null;

addEventListener('resize', () => {
  renderer.setSize(innerWidth, innerHeight);
  camera.aspect = fpCam.aspect = innerWidth / innerHeight;
  camera.updateProjectionMatrix();
  fpCam.updateProjectionMatrix();
});

// ------------------------------------------------------------ estado

const G = {
  phase: 'menu',      // menu | connecting | playing
  ready: false,       // já chegou o primeiro snapshot
  myId: 0,
  boxes: [],
  mapHX: 52,
  mapHZ: 42,
  me: newState(),     // meu estado previsto
  prev: newState(),   // estado do tick anterior (suaviza o desenho entre ticks)
  weapon: newWeapon(),
  nextKind: 0,
  alive: false,
  hp: 100,
  respawnIn: 0,
  killedBy: '',
  seq: 0,
  pending: [],        // inputs enviados que o servidor ainda não confirmou
  acc: 0,
  snaps: [],
  remotes: new Map(),
  nades: [],          // granadas em voo (minhas previstas + as dos outros)
  offset: null,       // relógio do servidor − relógio local (segundos)
  delayed: [],        // eventos visuais esperando o tempo de renderização
  names: new Map(),
  players: [],
  ping: null,
  stepDist: 0,
  stepSmooth: 0,
  deadT: 0,
  menuT: 0,
  ads: 0,             // 0 = arma na cintura, 1 = mirando
  mode: 'tdm',
  match: null,        // situação da partida (modo, fase, placar, objetivos, votação)
  myTeam: 0,
  everAlive: false,   // já nasceu nesta conexão (quem entra no meio da rodada só assiste)
  specId: 0,          // jogador que estou assistindo
  specNext: false,
  lastFire: false,
};

// Em modo de equipe, todo mundo usa a cor da equipe
const colorFor = (id, team) => (team ? new THREE.Color(TEAM_COLORS[team]) : playerColor(id));
const teamOf = (id) => G.players.find((p) => p.id === id)?.tm ?? 0;

function rebuildViewModel() {
  const color = colorFor(G.myId, G.myTeam);
  hud.setAccent(color);
  if (viewModel) fpCam.remove(viewModel.root);
  viewModel = new ViewModel(color);
  fpCam.add(viewModel.root);
}

const nowSec = () => performance.now() / 1000;
const renderTick = () => (nowSec() + G.offset - INTERP_DELAY) * TICK_RATE;

function syncClock(tick) {
  const off = tick / TICK_RATE - nowSec();
  if (G.offset === null || Math.abs(off - G.offset) > 0.5) G.offset = off;
  else G.offset += (off - G.offset) * 0.05;
}

function setMap(map) {
  G.mapHX = map.hx ?? 30;
  G.mapHZ = map.hz ?? 30;
  world.buildArena(map);
}

// ------------------------------------------------------------ rede

net.on('welcome', (m) => {
  G.myId = m.id;
  G.mode = m.mode ?? 'livre';
  G.boxes = m.map.boxes;
  setMap(m.map);
  syncClock(m.tick);
  G.myTeam = -1; // força montar a arma com a cor certa no primeiro snapshot
  rebuildViewModel();
  G.phase = 'playing';
  setMenu(false);
  hud.show(true);
});

net.on('snap', (s) => {
  syncClock(s.tick);
  const players = new Map();
  for (const p of s.players) {
    players.set(p.id, p);
    G.names.set(p.id, p.n);
  }
  G.players = s.players;
  G.match = s.mt ?? null;
  if (s.you.tm !== G.myTeam) { // mudou de equipe (partida nova)
    G.myTeam = s.you.tm;
    G.myVote = -1;
    rebuildViewModel();
  }
  G.myVote = s.you.vo ?? -1;
  G.snaps.push({ tick: s.tick, players });
  if (G.snaps.length > 40) G.snaps.shift();
  reconcile(s.you, s.ack);
  if (s.ev) for (const e of s.ev) onEvent(e, s.tick);
  G.ready = true;
});

net.on('pong', (m) => { G.ping = Math.round(performance.now() - m.c); });
net.on('aviso', (m) => hud.chat('', null, m.m, true));

net.on('close', () => {
  const wasPlaying = G.phase === 'playing';
  resetMatch();
  setMenu(true, wasPlaying
    ? 'A conexão com o servidor caiu. Entre de novo quando ele voltar.'
    : 'Não foi possível conectar. Confira se o servidor está no ar (docker compose up).');
  if (document.pointerLockElement) document.exitPointerLock();
});

setInterval(() => { if (G.phase === 'playing') net.send({ t: 'ping', c: performance.now() }); }, 2000);

// O servidor é a verdade: volta para o estado dele e reaplica os inputs que
// ele ainda não processou. Com a mesma física dos dois lados, normalmente o
// resultado é idêntico ao que já estava na tela.
function reconcile(you, ack) {
  const wasAlive = G.alive;
  G.alive = you.a;
  if (G.alive) G.everAlive = true;
  G.hp = you.hp;
  G.respawnIn = you.rs || 0;
  G.nextKind = you.nk ?? G.nextKind;
  const me = G.me, w = G.weapon;
  me.x = you.x; me.y = you.y; me.z = you.z;
  me.vx = you.vx; me.vy = you.vy; me.vz = you.vz;
  me.onGround = you.og;
  weaponFromServer(you, w);

  G.pending = G.pending.filter((c) => c.s > ack);
  if (G.alive) {
    for (const c of G.pending) {
      step(me, c.k, c.y, G.boxes, moveMul(w, c.k));
      stepWeapon(w, c.k, c.s);
    }
  }
  if (wasAlive !== G.alive) G.deadT = 0;
  if (!wasAlive && G.alive) Object.assign(G.prev, me); // renasceu: sem "arrasto"
}

function resetMatch() {
  G.phase = 'menu';
  G.ready = false;
  G.alive = false;
  G.snaps = [];
  G.pending = [];
  G.delayed = [];
  G.offset = null;
  G.seq = 0;
  G.acc = 0;
  for (const id of [...G.remotes.keys()]) removeRemote(id);
  for (const g of G.nades) removeNade(g);
  G.nades = [];
  G.everAlive = false;
  G.match = null;
  G.myTeam = 0;
  objectives.update(0, null, () => null);
  closeChat();
  hud.show(false);
  hud.setScope(false);
}

// ------------------------------------------------------------ meu jogador

const _obj = new THREE.Object3D();
const _v = new THREE.Vector3();

// Onde o cano aparece na tela (cena da arma) → ponto equivalente no mundo,
// para o rastro da bala sair visualmente da arma.
function muzzleWorld(yaw, pitch) {
  viewModel.flash.getWorldPosition(_v).project(fpCam);
  const th = Math.tan(THREE.MathUtils.degToRad(camera.fov / 2));
  const local = new THREE.Vector3(_v.x * th * camera.aspect, _v.y * th, -1);
  _obj.position.set(G.me.x, G.me.y + P.eye, G.me.z);
  _obj.rotation.set(pitch, yaw, 0, 'YXZ');
  _obj.updateMatrixWorld(true);
  return local.applyMatrix4(_obj.matrixWorld);
}

// Um tick fixo de 1/60 s: lê o input, prevê o resultado e manda pro servidor
function localTick() {
  const keys = input.locked ? input.bits() : 0;
  const cmd = { t: 'i', s: ++G.seq, k: keys, y: input.yaw, p: input.pitch };
  const me = G.me, w = G.weapon;
  Object.assign(G.prev, me);

  if (G.alive) {
    const wasGround = me.onGround, fallSpeed = me.vy, y0 = me.y;
    step(me, keys, cmd.y, G.boxes, moveMul(w, keys));
    if (wasGround && !me.onGround && me.vy > 0) sfx.jump();
    if (!wasGround && me.onGround && fallSpeed < -7) sfx.land(-fallSpeed / 16);
    if (wasGround && me.onGround && me.y - y0 > 0.1) G.stepSmooth -= me.y - y0; // subiu degrau
    if (me.onGround) {
      G.stepDist += Math.hypot(me.vx, me.vz) * DT;
      if (G.stepDist > 2.4) {
        G.stepDist = 0;
        sfx.step(null, 0.16);
      }
    }
    const act = stepWeapon(w, keys, cmd.s);
    if (act === A_FIRE || act === A_MELEE) cmd.rt = Math.round(renderTick() * 100) / 100;
    if (act === A_FIRE) localShot(cmd);
    else if (act === A_RELOAD) sfx.reload(null, 0.45, w.kind);
    else if (act === A_MELEE) {
      input.trig.melee = 0;
      viewModel.stab();
      sfx.knife(null);
    } else if (act === A_NADE) {
      input.trig.nade = 0;
      addNade(nadeLaunch(me, cmd.y, cmd.p), 0, G.myId); // prevista: sai na hora
      viewModel.toss();
      sfx.nadeThrow();
    }
  }

  stepNades();
  G.pending.push(cmd);
  if (G.pending.length > 300) G.pending.shift();
  net.send(cmd);
}

// Efeito imediato do tiro. As direções dos projéteis são sorteadas a partir
// da sequência do input, igual ao servidor: os rastros na sua tela são os
// mesmos que o servidor usa. Quem decide o acerto é ele.
function localShot(cmd) {
  const w = G.weapon, kind = w.kind, d = WEAPONS[kind];
  const eye = { x: G.me.x, y: G.me.y + P.eye, z: G.me.z };
  const muzzle = muzzleWorld(cmd.y, cmd.p);
  shotDirs(w, cmd.y, cmd.p, cmd.k, cmd.s, G.me).forEach((dir, i) => {
    let t = rayWorld(eye, dir, G.boxes, d.range), victim = null;
    for (const r of G.remotes.values()) {
      if (!r.alive) continue;
      const hit = rayPlayer(eye, dir, r.pos.x, r.pos.y, r.pos.z, t);
      if (hit) { t = hit.t; victim = r; }
    }
    const end = { x: eye.x + dir.x * t, y: eye.y + dir.y * t, z: eye.z + dir.z * t };
    if (kind !== W_SHOTGUN || i % 2 === 0) fx.tracer(muzzle, end, kind === W_SNIPER ? 0xfff3d0 : 0xffe3a3);
    if (victim) fx.blood(end, victim.color, victim.pos.y);
    else if (t < d.range && (kind !== W_SHOTGUN || i % 3 === 0)) fx.impact(end);
  });
  sfx.shot(null, 0.9, kind);
  viewModel.kick();
  muzzleLight.position.copy(muzzle);
  muzzleLight.intensity = kind === W_SHOTGUN ? 20 : 14;
  hud.kickCrosshair();

  // Coice: o próximo tiro sai mais alto. A mira vai junto no input, então o
  // servidor vê exatamente o mesmo coice.
  input.pitch += RECOIL[kind];
  input.yaw += (Math.random() - 0.5) * RECOIL[kind] * 0.5;
  input.clampAngles();
}

// ------------------------------------------------------------ granadas

function addNade(n, sid, owner) {
  const mesh = buildNade(1.6);
  mesh.position.set(n.x, n.y, n.z);
  scene.add(mesh);
  const g = { n, sid, owner, mesh, age: 0, prev: { x: n.x, y: n.y, z: n.z } };
  G.nades.push(g);
  return g;
}

function removeNade(g) {
  scene.remove(g.mesh);
  g.mesh.traverse((o) => { o.geometry?.dispose(); o.material?.dispose(); });
}

// Todos os clientes simulam a mesma trajetória (mesma física do servidor)
function stepNades() {
  for (let i = G.nades.length - 1; i >= 0; i--) {
    const g = G.nades[i];
    g.prev.x = g.n.x; g.prev.y = g.n.y; g.prev.z = g.n.z;
    const impact = stepNade(g.n, G.boxes);
    if (impact > 2.5) sfx.nadeBounce(g.n, impact / 10);
    g.age++;
    // granada prevista que o servidor nunca confirmou (ex.: morri antes)
    if (g.age > NADE.fuse + 120) {
      removeNade(g);
      G.nades.splice(i, 1);
    }
  }
}

function drawNades(dt, alpha) {
  for (const g of G.nades) {
    const p = g.prev, n = g.n;
    g.mesh.position.set(p.x + (n.x - p.x) * alpha, p.y + (n.y - p.y) * alpha, p.z + (n.z - p.z) * alpha);
    const sp = Math.hypot(n.vx, n.vz);
    g.mesh.rotation.x += sp * dt * 3;
    g.mesh.rotation.z += sp * dt * 1.7;
  }
}

function explodeNade(e) {
  const i = G.nades.findIndex((g) => g.sid === e.n);
  if (i >= 0) {
    removeNade(G.nades[i]);
    G.nades.splice(i, 1);
  }
  const pos = { x: e.from[0], y: e.from[1], z: e.from[2] };
  fx.explosion(pos);
  sfx.boom(pos);
  // tremor na câmera, mais forte perto
  const d = Math.hypot(pos.x - G.me.x, pos.y - G.me.y, pos.z - G.me.z);
  G.shake = Math.max(G.shake ?? 0, Math.max(0, 1 - d / 22) * 0.9);
}

// ------------------------------------------------------------ eventos

function onEvent(e, tick) {
  const mine = e.id === G.myId;
  switch (e.t) {
    case 'shot':
    case 'reload':
    case 'melee':
      // Dos outros: acontece na tela quando o desenho deles chegar nesse instante
      if (!mine) G.delayed.push({ tick, e });
      break;
    case 'nade':
      if (mine) { // a minha já está voando (prevista): só anota o id do servidor
        const g = G.nades.find((x) => x.owner === G.myId && x.sid === 0);
        if (g) g.sid = e.n;
      } else G.delayed.push({ tick: e.tk || tick, e });
      break;
    case 'boom':
      explodeNade(e);
      break;
    case 'hit':
      if (mine && e.tg !== G.myId) {
        hud.hitmarker(e.head, false);
        if (e.w === W_KNIFE) sfx.knifeHit();
        else sfx.hit(e.head);
      }
      if (e.tg === G.myId) {
        hud.hurt();
        sfx.hurt();
      }
      break;
    case 'kill': {
      const killer = G.names.get(e.id) ?? '?', victim = G.names.get(e.tg) ?? '?';
      hud.kill(killer, victim, e.head, colorFor(e.id, teamOf(e.id)), colorFor(e.tg, teamOf(e.tg)), mine || e.tg === G.myId, e.w ?? 0);
      if (mine && e.tg !== G.myId) {
        sfx.kill();
        hud.hitmarker(e.head, true);
        if (e.back) hud.toast('Pelas costas');
      }
      if (e.tg === G.myId) {
        G.killedBy = e.id === G.myId ? '' : killer;
        sfx.death(null);
        fx.explode(G.me, playerColor(G.myId));
      } else G.delayed.push({ tick, e });
      break;
    }
    case 'spawn':
      if (mine) {
        input.yaw = e.yaw || 0;
        input.pitch = 0;
        G.ads = 0;
        sfx.spawn();
      }
      break;
    case 'join':
      if (!mine) hud.notice(`${e.name} entrou na partida`);
      break;
    case 'chat':
      hud.chat(e.name, playerColor(e.id), e.msg);
      if (!mine) sfx.chatBlip();
      break;
    case 'round': // sobrevivente
      if (e.s === 'rodada') {
        for (const g of G.nades) removeNade(g);
        G.nades = [];
        sfx.roundStart();
        hud.toast(`Rodada ${e.n}: o último de pé vence`);
        hud.chat('', null, `Rodada ${e.n} começou.`, true);
      } else if (e.s === 'fim') {
        if (e.id === G.myId) sfx.victory();
        hud.chat('', null, e.id ? `${e.name} venceu a rodada ${e.n}.` : `Ninguém sobreviveu à rodada ${e.n}.`, true);
      }
      break;
    case 'match':
      if (e.s === 'jogo') {
        for (const g of G.nades) removeNade(g);
        G.nades = [];
        sfx.roundStart();
        hud.toast(`${e.name}: valendo`);
        hud.chat('', null, `Começou: ${e.name}.`, true);
      } else if (e.s === 'fim') {
        const won = e.id === G.myId || (e.tm && e.tm === G.myTeam);
        if (won) sfx.victory(); else sfx.objective(false);
        const who = e.tm ? `a equipe ${e.tm === 1 ? 'vermelha' : 'azul'}` : e.msg || 'ninguém';
        hud.chat('', null, `Fim de partida. Venceu ${who}.`, true);
      } else if (e.s === 'votacao') hud.chat('', null, 'Vote no próximo modo com 1, 2 ou 3.', true);
      else if (e.s === 'contagem') hud.chat('', null, `Próximo modo: ${e.name}.`, true);
      break;
    case 'point': {
      const good = e.tm === G.myTeam;
      if (e.s === 'capturado') {
        sfx.objective(good);
        hud.toast(good ? `Sua equipe capturou o ponto ${e.name}` : `Os inimigos capturaram o ponto ${e.name}`);
      }
      break;
    }
    case 'hill':
      sfx.objective(true);
      hud.toast(`A colina mudou: ${e.name}`);
      break;
    case 'flag': { // tm é a equipe DONA da bandeira
      const ours = e.tm === G.myTeam, who = G.names.get(e.id) ?? '';
      const text = {
        pegou: ours ? `${who} pegou a sua bandeira` : `${who} pegou a bandeira inimiga`,
        caiu: ours ? 'A sua bandeira caiu' : 'A bandeira inimiga caiu',
        voltou: ours ? 'A sua bandeira voltou para a base' : 'A bandeira inimiga voltou para a base',
        capturou: ours ? `${who} capturou a sua bandeira` : `${who} capturou a bandeira inimiga`,
      }[e.s];
      if (text) {
        hud.toast(text);
        hud.chat('', null, `${text}.`, true);
        sfx.objective(ours ? e.s === 'voltou' : e.s !== 'voltou');
      }
      break;
    }
  }
}

function flushDelayed(rt) {
  while (G.delayed.length && G.delayed[0].tick <= rt) {
    const { e } = G.delayed.shift();
    const r = G.remotes.get(e.id);
    if (e.t === 'shot') {
      const from = r ? r.model.muzzleWorld(new THREE.Vector3()) : { x: e.from[0], y: e.from[1], z: e.from[2] };
      const range = WEAPONS[e.w ?? 0].range;
      for (let i = 0; i < e.pts.length; i += 3) {
        const k = i / 3, to = { x: e.pts[i], y: e.pts[i + 1], z: e.pts[i + 2] };
        if (e.w !== W_SHOTGUN || k % 2 === 0) fx.tracer(from, to, e.w === W_SNIPER ? 0xfff3d0 : 0xffe3a3);
        const travel = Math.hypot(to.x - e.from[0], to.y - e.from[1], to.z - e.from[2]);
        const nearMe = Math.hypot(to.x - G.me.x, to.z - G.me.z) < 1;
        if (travel < range - 1 && !nearMe && (e.w !== W_SHOTGUN || k % 3 === 0)) fx.impact(to);
      }
      fx.flash(from);
      sfx.shot(from, 0.9, e.w ?? 0);
    } else if (e.t === 'reload' && r) {
      sfx.reload(r.pos, 0.35, e.w ?? 0);
    } else if (e.t === 'melee' && r) {
      r.model.strike();
      sfx.knife(r.pos);
    } else if (e.t === 'nade') {
      addNade({ x: e.from[0], y: e.from[1], z: e.from[2], vx: e.v[0], vy: e.v[1], vz: e.v[2], rest: false }, e.n, e.id);
      r?.model.strike();
    } else if (e.t === 'kill') {
      const v = G.remotes.get(e.tg);
      if (v) {
        fx.explode(v.pos, v.color);
        sfx.death(v.pos);
      }
    }
  }
}

// ------------------------------------------------------------ outros jogadores

function addRemote(id, name, team = 0) {
  const color = colorFor(id, team);
  const model = new PlayerModel(color, name);
  scene.add(model.root);
  G.remotes.set(id, { id, model, color, team, pos: new THREE.Vector3(), speed: 0, stepAcc: 0, alive: false });
}

function removeRemote(id) {
  const r = G.remotes.get(id);
  if (!r) return;
  scene.remove(r.model.root);
  r.model.dispose();
  G.remotes.delete(id);
}

function lerpAngle(a, b, t) {
  let d = b - a;
  while (d > Math.PI) d -= Math.PI * 2;
  while (d < -Math.PI) d += Math.PI * 2;
  return a + d * t;
}

function updateRemotes(rt, dt) {
  const snaps = G.snaps;
  if (!snaps.length) return;
  const latest = snaps[snaps.length - 1];
  for (const [id, p] of latest.players) {
    if (id === G.myId) continue;
    const r = G.remotes.get(id);
    if (r && r.team !== p.tm) removeRemote(id); // trocou de equipe: remonta com a cor nova
    if (!G.remotes.has(id)) addRemote(id, p.n, p.tm);
  }
  for (const id of [...G.remotes.keys()]) if (!latest.players.has(id)) removeRemote(id);

  let a = snaps[0], b = snaps[0];
  if (rt >= latest.tick) a = b = latest;
  else {
    for (let i = snaps.length - 1; i > 0; i--) {
      if (snaps[i - 1].tick <= rt) { a = snaps[i - 1]; b = snaps[i]; break; }
    }
  }
  const span = b.tick - a.tick;
  const f = span > 0 ? Math.min(1, Math.max(0, (rt - a.tick) / span)) : 0;

  for (const r of G.remotes.values()) {
    const pa = a.players.get(r.id) ?? b.players.get(r.id);
    const pb = b.players.get(r.id) ?? pa;
    if (!pa) {
      r.alive = false;
      r.model.root.visible = false;
      continue;
    }
    const teleported = Math.abs(pb.x - pa.x) + Math.abs(pb.z - pa.z) > 4; // renasceu
    const t = teleported ? 0 : f;
    const x = pa.x + (pb.x - pa.x) * t, y = pa.y + (pb.y - pa.y) * t, z = pa.z + (pb.z - pa.z) * t;
    const moved = Math.hypot(x - r.pos.x, z - r.pos.z);
    const speed = dt > 0 && moved < 2 ? moved / dt : 0;
    r.speed += (speed - r.speed) * Math.min(1, dt * 10);
    const onFloor = Math.abs(y - r.pos.y) < 0.02;

    r.alive = pa.a;
    r.model.root.visible = r.alive;
    r.model.setWeapon(pa.w ?? 0);
    r.model.root.position.set(x, y, z);
    r.yaw = lerpAngle(pa.yw, pb.yw, t);
    r.pitch = pa.pt + (pb.pt - pa.pt) * t;
    r.model.root.rotation.y = r.yaw;
    r.model.animate(dt, r.speed, r.pitch);

    if (r.alive && onFloor && r.speed > 2) { // passos dos outros, em 3D
      r.stepAcc += r.speed * dt;
      if (r.stepAcc > 2.4) {
        r.stepAcc = 0;
        sfx.step({ x, y, z }, 0.32);
      }
    }
    r.pos.set(x, y, z);
  }
}

// ------------------------------------------------------------ troca de arma

function chooseWeapon(kind) {
  if (G.phase !== 'playing') return;
  G.nextKind = kind;
  net.send({ t: 'arma', w: kind });
  hud.setNextWeapon(kind);
  if (G.alive && G.weapon.kind !== kind) hud.toast(`${WEAPONS[kind].name} na próxima vida`);
}

input.onKey = (code) => {
  if (code === 'Enter' || code === 'NumpadEnter') return openChat();
  const k = { Digit1: 0, Digit2: 1, Digit3: 2, Numpad1: 0, Numpad2: 1, Numpad3: 2 }[code];
  if (k === undefined) return;
  if (G.match?.f === 'votacao') vote(k);
  else chooseWeapon(k);
};

function vote(i) {
  if (G.match?.f !== 'votacao') return;
  G.myVote = i;
  net.send({ t: 'voto', v: i });
  sfx.chatBlip();
}

// Marcadores na tela, captura e aviso de bandeira
const _p = new THREE.Vector3();
function objectiveHud() {
  const mt = G.match, list = [];
  const mark = (x, y, z, label, color, h = 3) => {
    _p.set(x, y + h, z).project(camera);
    if (_p.z > 1 || Math.abs(_p.x) > 1.05 || Math.abs(_p.y) > 1.05) return;
    const d = Math.hypot(x - camera.position.x, z - camera.position.z);
    list.push({ x: (_p.x + 1) / 2 * innerWidth, y: (1 - _p.y) / 2 * innerHeight, label, sub: `${Math.round(d)} m`, color });
  };
  const css = (t) => `#${TEAM_COLORS[t].toString(16).padStart(6, '0')}`;
  let capture = '', frac = 0, capColor = '#fff', note = '';
  const inside = (cp) => G.alive && Math.hypot(G.me.x - cp.x, G.me.z - cp.z) <= cp.r && Math.abs(G.me.y - cp.y) <= 4;
  const zone = (cp, label) => {
    mark(cp.x, cp.y, cp.z, label, css(cp.o));
    if (inside(cp)) {
      const mineSide = G.myTeam === 1 ? -cp.p : cp.p; // progresso a favor da minha equipe
      capture = cp.c ? `Disputando ${label}` : cp.o === G.myTeam ? `${label} é da sua equipe` : `Capturando ${label}`;
      frac = cp.c ? Math.abs(cp.p) : (mineSide + 1) / 2;
      capColor = cp.c ? 'var(--peach)' : css(G.myTeam);
    }
  };
  if (mt?.f === 'jogo') {
    mt.pts?.forEach((cp) => zone(cp, cp.n));
    if (mt.hill) zone(mt.hill, '▲');
    mt.fl?.forEach((f) => {
      const pos = f.s === 'carried' ? G.remotes.get(f.c)?.pos : f;
      if (pos && f.c !== G.myId) mark(pos.x, pos.y, pos.z, '⚑', css(f.t), 3.2);
      if (f.s === 'carried' && f.c === G.myId) note = 'Você está com a bandeira. Leve até a base da sua equipe.';
    });
  }
  hud.setMarkers(list);
  hud.setCapture(capture, frac, capColor);
  hud.setFlagNote(note);
}

// ------------------------------------------------------------ chat

const chatInput = document.getElementById('chat-input');

function openChat() {
  if (G.phase !== 'playing' || input.typing) return;
  input.setTyping(true);
  hud.chatOpen(true);
}

function closeChat() {
  if (!input.typing) return;
  input.setTyping(false);
  hud.chatOpen(false);
}

chatInput.addEventListener('keydown', (e) => {
  e.stopPropagation();
  if (e.key === 'Enter') {
    const text = chatInput.value.trim();
    if (text) net.send({ t: 'chat', m: text });
    closeChat();
  } else if (e.key === 'Escape') closeChat();
});
input.onLockChange = (locked) => { if (!locked) closeChat(); };

// ------------------------------------------------------------ espectador

// Quem está morto no modo sobrevivente assiste aos vivos (clique troca).
function pickSpectate() {
  const alive = [...G.remotes.values()].filter((r) => r.alive).sort((a, b) => a.id - b.id);
  if (!alive.length) return null;
  let i = Math.max(0, alive.findIndex((r) => r.id === G.specId));
  if (G.specNext) {
    i = (i + 1) % alive.length;
    G.specNext = false;
  }
  G.specId = alive[i].id;
  return alive[i];
}

// ------------------------------------------------------------ quadros

function playFrame(dt, alpha) {
  const rt = renderTick();
  updateRemotes(rt, dt);
  flushDelayed(rt);
  drawNades(dt, alpha);

  const me = G.me, pv = G.prev, w = G.weapon, kind = w.kind, d = WEAPONS[kind];

  // Mira: rampa de 0,2 s (a precisão total chega junto, na física)
  const wantAim = G.alive && input.aim && w.reloadEnd === 0 && G.seq >= w.busy;
  G.ads = Math.max(0, Math.min(1, G.ads + (wantAim ? dt : -dt * 1.5) / ADS_TIME));
  const e = G.ads * G.ads * (3 - 2 * G.ads);
  const scoped = kind === W_SNIPER && e > 0.85;
  const fov = BASE_FOV + (ADS_FOV[kind] - BASE_FOV) * e;
  if (Math.abs(camera.fov - fov) > 0.01) {
    camera.fov = fov;
    camera.updateProjectionMatrix();
  }
  input.sensMul = Math.tan(THREE.MathUtils.degToRad(fov / 2)) / Math.tan(THREE.MathUtils.degToRad(BASE_FOV / 2));

  let spec = null;
  G.stepSmooth *= Math.exp(-dt * 14);
  G.shake = Math.max(0, (G.shake ?? 0) - dt * 2);
  const sx = (Math.random() - 0.5) * G.shake * 0.05, sy = (Math.random() - 0.5) * G.shake * 0.05;
  if (G.alive) {
    camera.position.set(
      pv.x + (me.x - pv.x) * alpha,
      pv.y + (me.y - pv.y) * alpha + P.eye + G.stepSmooth,
      pv.z + (me.z - pv.z) * alpha,
    );
    camera.rotation.set(input.pitch + sy, input.yaw + sx, 0);
  } else {
    G.deadT += dt;
    if (input.fire && !G.lastFire) G.specNext = true;
    spec = G.respawnIn < 0 && (G.deadT > 2 || !G.everAlive) ? pickSpectate() : null;
    if (spec) { // assistindo: câmera nos olhos de quem está vivo
      camera.position.set(spec.pos.x, spec.pos.y + P.eye, spec.pos.z);
      camera.rotation.set(spec.pitch ?? 0, spec.yaw ?? 0, 0);
      spec.model.root.visible = false;
    } else if (!G.everAlive) {
      menuFrame(dt); // entrou com a rodada rolando e ninguém para assistir
    } else { // câmera de morte: sobe devagar olhando o lugar onde você caiu
      const k = 1 - Math.exp(-G.deadT * 1.5);
      camera.position.set(me.x, me.y + P.eye + k * 3, me.z);
      camera.rotation.set(input.pitch - k * 0.6, input.yaw, 0);
    }
  }
  G.lastFire = input.fire;

  const look = input.consumeLook();
  if (viewModel) {
    viewModel.setWeapon(kind);
    viewModel.root.visible = G.alive && !scoped;
    viewModel.update(dt, {
      moving: Math.hypot(me.vx, me.vz) > 1,
      onGround: me.onGround,
      reloading: w.reloadEnd > 0,
      look,
      ads: e,
    });
  }
  muzzleLight.intensity *= Math.exp(-dt * 40);
  sfx.setListener(camera.position.x, camera.position.y, camera.position.z, input.yaw);

  // Abertura real da arma na tela: a mira mostra o cone dos tiros
  const keysNow = input.locked ? input.bits() : 0;
  let spread = aimed(w, keysNow, G.seq) ? d.aimSpread : d.spread;
  if (isMoving(me)) spread += d.moveSpread;
  const px = (Math.tan(spread) / Math.tan(THREE.MathUtils.degToRad(fov / 2))) * (innerHeight / 2);

  hud.setHealth(G.hp);
  hud.setWeapon(d.name, w.nades);
  hud.setAmmo(w.ammo, d.mag, w.reloadEnd > 0);
  hud.setStatus(G.players.length, G.ping);
  hud.setDeath(!G.alive && G.everAlive && !spec, G.killedBy, G.respawnIn);
  hud.setSpectate(spec ? G.names.get(spec.id) ?? '?' : '');
  const nameOf = (id) => G.names.get(id) ?? '?';
  hud.setMatch(G.match, { myId: G.myId, myTeam: G.myTeam, nameOf, players: G.players });
  hud.setVote(G.match?.f === 'votacao' ? G.match.vote : null, G.myVote, G.match?.t ?? 0, vote);
  objectives.update(dt, G.match, (id) => (id === G.myId ? null : G.remotes.get(id)?.pos ?? null));
  objectiveHud();
  hud.setNextWeapon(G.nextKind);
  hud.setScope(scoped && G.alive);
  hud.setPaused(!input.locked);
  hud.setBoard(input.has('Tab') || G.match?.f === 'fim', G.players, G.myId, G.match?.m === 'lms', G.match?.sc ?? null);
  hud.update(dt, Math.max(0, px - 5), e > 0.6 && kind !== W_SNIPER);
}

function menuFrame(dt) {
  if (!reduceMotion) G.menuT += dt * 0.04;
  const a = G.menuT + 0.6, R = Math.max(G.mapHX, G.mapHZ) * 1.25 + 10;
  camera.position.set(Math.cos(a) * R, R * 0.38, Math.sin(a) * R);
  camera.lookAt(0, 3, 0);
  if (camera.aspect > 1.3) camera.rotateY(0.3); // mapa à direita, fora de trás do cartão do menu
}

let last = performance.now();
function frame(now) {
  requestAnimationFrame(frame);
  const dt = Math.min(0.1, (now - last) / 1000);
  last = now;

  if (G.phase === 'playing' && G.ready) {
    // Simulação em passos fixos de 1/60 s, independente do FPS do monitor
    G.acc += dt;
    while (G.acc >= DT) {
      localTick();
      G.acc -= DT;
    }
    playFrame(dt, G.acc / DT);
  } else {
    if (camera.fov !== BASE_FOV) {
      camera.fov = BASE_FOV;
      camera.updateProjectionMatrix();
    }
    menuFrame(dt);
  }

  fx.update(dt);
  world.update(dt, camera);
  renderer.clear();
  renderer.render(scene, camera);
  if (G.phase === 'playing' && G.ready && G.alive && viewModel && viewModel.root.visible) {
    renderer.clearDepth();
    renderer.render(fpScene, fpCam);
  }
}
requestAnimationFrame(frame);

// ------------------------------------------------------------ menu

const menu = document.getElementById('menu');
const nameInput = document.getElementById('name');
const playBtn = document.getElementById('play');
const menuMsg = document.getElementById('menu-msg');
const pickedWeapon = () => Number(document.querySelector('input[name="arma"]:checked')?.value ?? 0);

try {
  nameInput.value = localStorage.getItem('aresta:name') || '';
  const saved = localStorage.getItem('aresta:arma');
  if (saved !== null) document.querySelector(`input[name="arma"][value="${Number(saved)}"]`)?.click();
} catch { /* sem storage */ }

function setMenu(show, msg = '') {
  menu.hidden = !show;
  menuMsg.textContent = msg;
  playBtn.disabled = false;
  if (show) nameInput.focus();
}

function join() {
  if (G.phase !== 'menu') return;
  const name = nameInput.value.trim() || 'Jogador';
  const kind = pickedWeapon();
  try {
    localStorage.setItem('aresta:name', name);
    localStorage.setItem('aresta:arma', String(kind));
  } catch { /* sem storage */ }
  sfx.init();     // áudio e pointer lock só podem começar num clique
  input.lock();
  playBtn.disabled = true;
  menuMsg.textContent = 'Entrando na partida…';
  G.phase = 'connecting';
  G.nextKind = kind;
  net.connect(name, kind);
}

// O menu já mostra o mapa da partida e avisa se o servidor não responde
fetch('/map')
  .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
  .then((map) => { if (!G.myId) setMap(map); })
  .catch(() => {
    if (G.phase === 'menu') menuMsg.textContent = 'O servidor não respondeu. Confira se o docker compose está rodando.';
  });

playBtn.addEventListener('click', join);
nameInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') join(); });
document.getElementById('pause').addEventListener('click', () => input.lock());
