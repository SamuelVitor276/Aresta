import { playerColor } from './models.js';
import { TEAM_COLORS } from './objectives.js';

const $ = (id) => document.getElementById(id);

// Só mexe no DOM quando o valor muda (o HUD é atualizado todo frame).
export class Hud {
  constructor() {
    this.root = $('hud');
    this.cache = {};
    this.spread = 0;
  }

  set(key, value, apply) {
    if (this.cache[key] === value) return;
    this.cache[key] = value;
    apply(value);
  }

  show(on) {
    this.root.hidden = !on;
  }

  setAccent(color) {
    document.documentElement.style.setProperty('--me', `#${color.getHexString()}`);
  }

  setHealth(hp) {
    if (hp > (this.cache.hp ?? hp)) this.regenUntil = performance.now() + 600;
    this.set('regen', performance.now() < (this.regenUntil ?? 0), (v) => $('health').classList.toggle('regen', v));
    this.set('hp', hp, (v) => {
      $('hp-num').textContent = v;
      $('hp-fill').style.transform = `scaleX(${v / 100})`;
      $('health').classList.toggle('low', v <= 30);
    });
  }

  setWeapon(name, nades) {
    this.set('wname', name, (v) => { $('weapon-name').textContent = v; });
    this.set('nades', nades, (v) => {
      [...$('nades').children].forEach((el, i) => el.classList.toggle('on', i < v));
    });
  }

  setScope(on) {
    this.set('scope', on, (v) => { $('scope').hidden = !v; });
  }

  setNextWeapon(kind) {
    this.set('next', kind, (v) => {
      document.querySelectorAll('#next-weapon [data-w]').forEach((b) => b.classList.toggle('on', Number(b.dataset.w) === v));
    });
  }

  toast(text) {
    const el = $('toast');
    el.textContent = text;
    this.replay(el, 'show');
    clearTimeout(this.toastT);
    this.toastT = setTimeout(() => el.classList.remove('show'), 2200);
  }

  setAmmo(ammo, mag, reloading) {
    this.set('ammo', `${ammo}|${mag}|${reloading}`, () => {
      $('ammo-num').textContent = reloading ? '··' : ammo;
      $('ammo-max').textContent = `/${mag}`;
      $('ammo').classList.toggle('low', !reloading && ammo <= 5);
      $('reload-note').hidden = !reloading;
    });
  }

  setStatus(players, ping) {
    this.set('status', `${players}|${ping}`, () => {
      $('status').textContent = `${players} ${players === 1 ? 'jogador' : 'jogadores'}   ${ping === null ? '–' : ping} ms`;
    });
  }

  // A mira abre um pouco a cada tiro e volta sozinha
  kickCrosshair() {
    this.spread = Math.min(1, this.spread + 0.35);
  }

  // basePx: abertura real da arma em pixels (muda com mira e movimento)
  update(dt, basePx = 0, dot = false) {
    this.spread = Math.max(0, this.spread - dt * 4);
    this.set('spread', Math.round(basePx + this.spread * 14), (v) =>
      document.documentElement.style.setProperty('--spread', `${v}px`));
    this.set('dot', dot, (v) => $('crosshair').classList.toggle('dot', v));
  }

  replay(el, cls) {
    el.classList.remove(cls);
    void el.offsetWidth; // reinicia a animação CSS
    el.classList.add(cls);
  }

  hitmarker(head, kill) {
    const el = $('hitmarker');
    el.classList.toggle('head', !!head);
    el.classList.toggle('kill', !!kill);
    this.replay(el, 'show');
  }

  hurt() {
    this.replay($('vignette'), 'show');
  }

  feed(html, mine = false) {
    const box = $('killfeed');
    const row = document.createElement('div');
    row.className = mine ? 'row mine' : 'row';
    row.innerHTML = html;
    box.prepend(row);
    while (box.children.length > 6) box.lastChild.remove();
    setTimeout(() => row.classList.add('out'), 5000);
    setTimeout(() => row.remove(), 5600);
  }

  kill(killer, victim, head, kColor, vColor, mine, w = 0) {
    const k = `<b style="color:#${kColor.getHexString()}">${esc(killer)}</b>`;
    const v = `<b style="color:#${vColor.getHexString()}">${esc(victim)}</b>`;
    const mark = head ? '<span class="head" title="Na cabeça"></span>' : '';
    this.feed(`${k}<span class="weapon w${w}"></span>${mark}${v}`, mine);
  }

  notice(text) {
    this.feed(`<span class="notice">${esc(text)}</span>`);
  }

  setDeath(dead, killer, seconds) {
    this.set('dead', dead, (v) => { $('death').hidden = !v; });
    if (!dead) return;
    this.set('killer', killer, (v) => { $('death-by').textContent = v ? `Você foi abatido por ${v}` : 'Você foi abatido'; });
    this.set('respawn', seconds < 0 ? -1 : Math.ceil(seconds), (v) => {
      $('death-timer').textContent = v < 0 ? 'Você volta na próxima rodada' : v > 0 ? `Voltando em ${v}` : 'Voltando…';
    });
  }

  setPaused(on) {
    this.set('paused', on, (v) => { $('pause').hidden = !v; });
  }

  setBoard(show, players, myId, lms = false, score = null) {
    this.set('board', show, (v) => { $('scoreboard').hidden = !v; });
    this.set('lms', lms, (v) => $('scoreboard').classList.toggle('lms', v));
    if (!show || !players) return;
    const teams = players.some((p) => p.tm > 0);
    const sorted = [...players].sort((a, b) => (teams ? a.tm - b.tm : 0) || (lms ? b.vt - a.vt : 0) || b.k - a.k || a.d - b.d);
    const key = sorted.map((p) => `${p.id}:${p.k}:${p.d}:${p.vt}:${p.a}:${p.in}:${p.tm}`).join(',') + `|${score}`;
    this.set('boardRows', key, () => {
      let html = '', lastTeam = -1;
      for (const p of sorted) {
        if (teams && p.tm !== lastTeam) {
          lastTeam = p.tm;
          const sc = score ? `  ${score[p.tm - 1]}` : '';
          html += `<tr class="team t${p.tm}"><td colspan="4">Equipe ${p.tm === 1 ? 'vermelha' : 'azul'}${sc}</td></tr>`;
        }
        const color = p.tm ? `#${TEAM_COLORS[p.tm].toString(16).padStart(6, '0')}` : cssColor(p.id);
        html += `
        <tr class="${p.id === myId ? 'me' : ''}${lms && p.in && !p.a ? ' out' : ''}">
          <td><i style="background:${color}"></i>${esc(p.n)}${p.b ? '<span class="bot">bot</span>' : ''}</td>
          <td class="wins">${p.vt ?? 0}</td><td>${p.k}</td><td>${p.d}</td>
        </tr>`;
      }
      $('board-rows').innerHTML = html;
    });
  }

  // ---------- chat
  chat(name, color, text, system = false) {
    const log = $('chat-log');
    const row = document.createElement('div');
    row.className = system ? 'msg system' : 'msg';
    row.innerHTML = system ? esc(text) : `<b style="color:#${color.getHexString()}">${esc(name)}</b>${esc(text)}`;
    log.append(row);
    while (log.children.length > 40) log.firstChild.remove();
    log.scrollTop = log.scrollHeight;
    setTimeout(() => row.classList.add('old'), 10000);
  }

  chatOpen(on) {
    $('chat').classList.toggle('open', on);
    const input = $('chat-input');
    input.hidden = !on;
    if (on) {
      input.value = '';
      input.focus();
      $('chat-log').scrollTop = $('chat-log').scrollHeight;
    } else input.blur();
  }

  // ---------- partida
  // ctx: { myId, myTeam, nameOf(id), players }
  setMatch(mt, ctx) {
    let bar = '', end = '', sub = '', endCls = '';
    if (mt) {
      const t = Math.ceil(mt.t ?? 0), clock = `${Math.floor(t / 60)}:${String(t % 60).padStart(2, '0')}`;
      const mode = `<span class="mode">${esc(mt.nm)}</span>`;
      if (mt.f === 'espera') bar = `${mode}<span>Aguardando mais um jogador</span>`;
      else if (mt.f === 'contagem') bar = `${mode}<span>Começa em ${t}</span>`;
      else if (mt.f === 'votacao') bar = `<span>Votação</span><span class="clock">${t}</span>`;
      else {
        let mid = '';
        if (mt.sc) {
          mid = `<span class="score"><b class="t1${ctx.myTeam === 1 ? ' mine' : ''}">${mt.sc[0]}</b><b class="t2${ctx.myTeam === 2 ? ' mine' : ''}">${mt.sc[1]}</b></span>`;
        } else if (mt.m === 'lms' && mt.rd) {
          mid = mt.rd.s === 'intervalo' ? '<span>Próxima rodada</span>' : `<span>Rodada ${mt.rd.n}</span><span>${mt.rd.v} de ${mt.rd.p} vivos</span>`;
        } else if (ctx.players?.length) {
          const lead = [...ctx.players].sort((a, b) => b.k - a.k)[0];
          const me = ctx.players.find((p) => p.id === ctx.myId);
          mid = `<span>${esc(lead.n)} ${lead.k}</span><span class="mode">você ${me?.k ?? 0} de ${mt.lim}</span>`;
        }
        if (mt.pts) mid += `<span class="obj">${mt.pts.map((p) => `<i class="o${p.o}${p.c ? ' c' : ''}">${esc(p.n)}</i>`).join('')}</span>`;
        if (mt.hill) mid += `<span class="obj"><i class="o${mt.hill.o}${mt.hill.c ? ' c' : ''}">▲</i></span>`;
        const showClock = mt.f === 'jogo' && mt.t > 0;
        bar = `${mode}${mid}${showClock ? `<span class="clock">${clock}</span>` : ''}`;
        if (mt.f === 'fim') {
          if (mt.wt) {
            end = `Vitória da equipe ${mt.wt === 1 ? 'vermelha' : 'azul'}`;
            endCls = mt.wt === ctx.myTeam ? 'won' : `t${mt.wt}`;
            sub = mt.wt === ctx.myTeam ? 'Sua equipe venceu.' : 'Fica para a próxima.';
          } else if (mt.w) {
            end = mt.w === ctx.myId ? 'Você venceu a partida' : `${ctx.nameOf(mt.w)} venceu a partida`;
            endCls = mt.w === ctx.myId ? 'won' : '';
          } else end = 'Empate';
          sub = `${sub} Votação em ${t}.`.trim();
        }
      }
    }
    this.set('bar', bar, (v) => { $('match-bar').hidden = !v; $('match-bar').innerHTML = v; });
    this.set('end', `${end}|${sub}|${endCls}`, () => {
      $('match-end').hidden = !end;
      $('match-end-title').textContent = end;
      $('match-end-sub').textContent = sub;
      $('match-end').className = endCls;
    });
  }

  // Votação: três modos; vota com 1/2/3 ou clicando
  setVote(vote, myVote, left, onPick) {
    this.set('voteOn', !!vote, (v) => { $('vote').hidden = !v; });
    if (!vote) return;
    const key = `${vote.o.join()}|${vote.c.join()}|${myVote}`;
    this.set('voteCards', key, () => {
      $('vote-cards').innerHTML = vote.n.map((name, i) => `
        <button type="button" data-v="${i}" class="${i === myVote ? 'mine' : ''}">
          <b><kbd>${i + 1}</kbd> ${esc(name)}</b>
          <small>${esc(vote.d[i])}</small>
          <span class="count">${vote.c[i]} ${vote.c[i] === 1 ? 'voto' : 'votos'}</span>
        </button>`).join('');
      $('vote-cards').querySelectorAll('button').forEach((b) => b.addEventListener('click', () => onPick(Number(b.dataset.v))));
    });
    this.set('voteHint', Math.ceil(left), (v) => {
      $('vote-hint').textContent = `Aperte 1, 2 ou 3 para votar. Faltam ${v} s.`;
    });
  }

  // Marcadores dos objetivos: [{x, y, label, sub, color}] em pixels
  setMarkers(list) {
    const box = $('markers');
    while (box.children.length < list.length) {
      const el = document.createElement('div');
      el.className = 'mk';
      el.innerHTML = '<i></i><span></span>';
      box.append(el);
    }
    [...box.children].forEach((el, i) => {
      const m = list[i];
      el.hidden = !m;
      if (!m) return;
      el.style.left = `${m.x}px`;
      el.style.top = `${m.y}px`;
      el.style.color = m.color;
      el.firstChild.textContent = m.label;
      el.lastChild.textContent = m.sub;
    });
  }

  setCapture(text, frac, color) {
    this.set('cap', text, (v) => { $('capture').hidden = !v; $('capture-label').textContent = v; });
    if (!text) return;
    $('capture-fill').style.transform = `scaleX(${Math.max(0, Math.min(1, frac))})`;
    $('capture-fill').style.background = color;
  }

  setFlagNote(text) {
    this.set('flagNote', text, (v) => { $('flag-note').hidden = !v; $('flag-note').textContent = v; });
  }

  setSpectate(name) {
    this.set('spec', name, (v) => {
      $('spectate').hidden = !v;
      $('spectate').textContent = v ? `Assistindo ${v}. Clique para trocar.` : '';
    });
  }
}

function cssColor(id) { return `#${playerColor(id).getHexString()}`; }

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}
