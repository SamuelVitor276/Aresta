package main

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const (
	SnapshotEvery = 2 // snapshot a cada 2 ticks → 30 Hz
	MaxHP         = 100
	RespawnTicks  = 3 * TickRate
	HistorySize   = 64 // ~1 s de posições para a compensação de lag
	MaxQueued     = 32
	MaxPerTick    = 4

	RegenDelay  = 5 * TickRate // 5 s sem tomar dano e a vida começa a voltar
	RegenPerSec = 20.0

	ChatBurst = 4   // mensagens seguidas permitidas
	ChatEvery = 1.5 // segundos para ganhar mais uma
)

type Input struct {
	Seq   int
	Keys  int
	Yaw   float64
	Pitch float64
	RT    float64
}

type Player struct {
	ID        int
	Name      string
	State     PhysState
	Yaw       float64
	Pitch     float64
	HP        int
	Alive     bool
	Kills     int
	Deaths    int
	Weapon    Weapon
	NextKind  int
	LastSeq   int
	RespawnAt uint64
	Wins      int  // rodadas vencidas (sobrevivente)
	InRound   bool // participando da rodada atual (sobrevivente)
	Team      int  // 0 sem equipe, 1 vermelho, 2 azul
	Vote      int  // −1: ainda não votou
	LastHurt  uint64
	Bot       *Bot // nil para gente de verdade

	heal       float64
	chatTokens float64
	chatTick   uint64
	inputs     []Input
	send       chan []byte
}

type histFrame struct {
	tick uint64
	pos  map[int]Vec3
}

// Game é o estado autoritativo. Tudo que mexe nele passa pelo mutex.
type Game struct {
	mu       sync.Mutex
	world    *World
	nav      *Nav
	players  map[int]*Player
	nades    []*Nade
	nextID   int
	nextNade int
	tick     uint64
	events   []Event
	history  [HistorySize]histFrame
	rng      *rand.Rand
	onEvent  func(Event) // só para testes
	Match
}

// NewGame cria o jogo. mode é o id do modo inicial (ffa, tdm, dom, koth,
// ctf, lms); bots é quantos participantes a sala deve ter (bots completam).
func NewGame(world *World, mode string, bots int) *Game {
	g := &Game{
		world:   world,
		players: map[int]*Player{},
		nextID:  1,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	g.nav = BuildNav(world)
	g.initMatch(modeIndex(mode), bots)
	return g
}

// Run é o coração do servidor: 60 vezes por segundo, sempre no mesmo ritmo.
func (g *Game) Run() {
	t := time.NewTicker(time.Second / TickRate)
	defer t.Stop()
	for range t.C {
		g.mu.Lock()
		g.update()
		g.mu.Unlock()
	}
}

func (g *Game) update() {
	g.tick++

	// 0) Bots decidem o que fazer (viram inputs, iguais aos de um jogador)
	g.botsThink()

	// 1) Inputs: cada um vale exatamente 1/60 s
	for _, p := range g.players {
		for n := 0; len(p.inputs) > 0 && n < MaxPerTick; n++ {
			in := p.inputs[0]
			p.inputs = p.inputs[1:]
			if in.Seq <= p.LastSeq {
				continue
			}
			p.LastSeq = in.Seq
			p.Yaw, p.Pitch = in.Yaw, ClampPitch(in.Pitch)
			if !p.Alive {
				continue
			}
			Step(&p.State, in.Keys, in.Yaw, g.world.Boxes, MoveMul(&p.Weapon, in.Keys))
			switch StepWeapon(&p.Weapon, in.Keys, in.Seq) {
			case ActFire:
				g.fire(p, in)
			case ActReload:
				g.emit(Event{T: "reload", ID: p.ID, W: p.Weapon.Kind})
			case ActMelee:
				g.melee(p, in)
			case ActNade:
				g.throwNade(p, in)
			}
		}
	}

	// 2) Granadas, cura, respawn, regras do modo
	g.stepNades()
	g.regen()
	if g.respawnAllowed() {
		for _, p := range g.players {
			if !p.Alive && g.tick >= p.RespawnAt {
				g.spawn(p)
			}
		}
	}
	g.updateMatch()

	// 3) Histórico de posições e snapshot
	g.recordHistory()
	if g.tick%SnapshotEvery == 0 {
		g.broadcast()
	}
}

func (g *Game) emit(e Event) {
	g.events = append(g.events, e)
	if g.onEvent != nil {
		g.onEvent(e)
	}
}

// regen: estilo Phantom Forces. Depois de RegenDelay sem tomar dano, a vida
// sobe RegenPerSec por segundo até encher.
func (g *Game) regen() {
	for _, p := range g.players {
		if !p.Alive || p.HP >= MaxHP || g.tick-p.LastHurt < RegenDelay {
			continue
		}
		p.heal += RegenPerSec / TickRate
		for p.heal >= 1 && p.HP < MaxHP {
			p.HP++
			p.heal--
		}
	}
}

type target struct {
	p   *Player
	pos Vec3
}

// targets: os inimigos na posição em que o atirador os VIA (rt). Colegas de
// equipe ficam de fora: as balas passam por eles.
func (g *Game) targets(self *Player, rt float64) []target {
	var out []target
	for _, p := range g.players {
		if p == self || !p.Alive || g.allies(p, self) {
			continue
		}
		if pos, ok := g.posAt(p, rt); ok {
			out = append(out, target{p, pos})
		}
	}
	return out
}

func (g *Game) allies(a, b *Player) bool { return g.teamMode() && a != b && a.Team == b.Team }

func (g *Game) fire(shooter *Player, in Input) {
	w := &shooter.Weapon
	d := &Weapons[w.Kind]
	eye := Vec3{shooter.State.X, shooter.State.Y + EyeHeight, shooter.State.Z}
	dirs := ShotDirs(w, in.Yaw, in.Pitch, in.Keys, in.Seq, &shooter.State)
	tgs := g.targets(shooter, in.RT)

	dmg := map[*Player]float64{}
	headPellets, hitPellets := map[*Player]int{}, map[*Player]int{}
	pts := make([]float64, 0, len(dirs)*3)
	for _, dir := range dirs {
		best := RayWorld(eye, dir, g.world.Boxes, d.Range)
		var hit *Player
		hitHead := false
		for _, tg := range tgs {
			if t, ok := RayBox(eye, dir, BodyBox(tg.pos)); ok && t < best {
				best, hit, hitHead = t, tg.p, false
			}
			if t, ok := RayBox(eye, dir, HeadBox(tg.pos)); ok && t < best {
				best, hit, hitHead = t, tg.p, true
			}
		}
		end := eye.Add(dir.Scale(best))
		pts = append(pts, r2(end.X), r2(end.Y), r2(end.Z))
		if hit != nil {
			base := d.Body
			hitPellets[hit]++
			if hitHead {
				base = d.Head
				headPellets[hit]++
			}
			dmg[hit] += base * Falloff(d, best)
		}
	}
	g.emit(Event{T: "shot", ID: shooter.ID, W: w.Kind, From: arr(eye), Pts: pts})
	for p, v := range dmg {
		// Conta como tiro na cabeça quando a maioria dos projéteis pegou nela
		// (na escopeta, um bago perdido na cabeça não vale a marca)
		head := headPellets[p]*2 > hitPellets[p]
		if amount := int(math.Round(v)); amount > 0 {
			g.damage(p, shooter, amount, head, w.Kind, false)
		}
	}
}

func (g *Game) melee(a *Player, in Input) {
	eye := Vec3{a.State.X, a.State.Y + EyeHeight, a.State.Z}
	dir := AimDir(in.Yaw, ClampPitch(in.Pitch))
	best := RayWorld(eye, dir, g.world.Boxes, KnifeRange)
	var victim *Player
	var vpos Vec3
	for _, tg := range g.targets(a, in.RT) {
		if t, ok := RayBox(eye, dir, PlayerBox(tg.pos, KnifePad)); ok && t <= best {
			best, victim, vpos = t, tg.p, tg.pos
		}
	}
	g.emit(Event{T: "melee", ID: a.ID})
	if victim == nil {
		return
	}
	fx, fz := -math.Sin(victim.Yaw), -math.Cos(victim.Yaw)
	dx, dz := vpos.X-a.State.X, vpos.Z-a.State.Z
	back := false
	if l := math.Hypot(dx, dz); l > 1e-6 {
		back = (fx*dx+fz*dz)/l > 0.5
	}
	dmg := KnifeDamage
	if back {
		dmg = KnifeBackstab
	}
	g.damage(victim, a, dmg, false, WKnife, back)
}

func (g *Game) throwNade(p *Player, in Input) {
	n := NadeLaunch(&p.State, in.Yaw, in.Pitch)
	g.nextNade++
	n.ID, n.Owner = g.nextNade, p.ID
	n.Explode = g.tick + uint64(NadeFuse)
	g.nades = append(g.nades, &n)
	g.emit(Event{T: "nade", ID: p.ID, N: n.ID, Tk: g.tick,
		From: []float64{n.X, n.Y, n.Z}, V: []float64{n.VX, n.VY, n.VZ}})
}

func (g *Game) stepNades() {
	alive := g.nades[:0]
	for _, n := range g.nades {
		if g.tick >= n.Explode {
			g.explode(n)
			continue
		}
		StepNade(n, g.world.Boxes)
		alive = append(alive, n)
	}
	g.nades = alive
}

func (g *Game) explode(n *Nade) {
	c := Vec3{n.X, n.Y, n.Z}
	g.emit(Event{T: "boom", ID: n.Owner, N: n.ID, From: arr(c)})
	owner := g.players[n.Owner]
	for _, p := range g.players {
		if !p.Alive {
			continue
		}
		f := 0.0
		for _, h := range [3]float64{0.3, 1.0, 1.6} {
			t := Vec3{p.State.X, p.State.Y + h, p.State.Z}
			dx, dy, dz := t.X-c.X, t.Y-c.Y, t.Z-c.Z
			dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if dist >= NadeRadius {
				continue
			}
			if dist > 0.01 {
				dir := Vec3{dx / dist, dy / dist, dz / dist}
				if RayWorld(c, dir, g.world.Boxes, dist) < dist-0.05 {
					continue
				}
			}
			f = math.Max(f, 1-dist/NadeRadius)
		}
		if dmg := int(math.Round(NadeDamage * f)); dmg > 0 {
			att := owner
			if att == nil {
				att = p
			}
			g.damage(p, att, dmg, false, WNade, false)
		}
	}
}

func (g *Game) posAt(p *Player, rt float64) (Vec3, bool) {
	cur := Vec3{p.State.X, p.State.Y, p.State.Z}
	now := float64(g.tick)
	if rt <= 0 || rt >= now {
		return cur, true
	}
	if oldest := now - float64(HistorySize-2); rt < oldest {
		rt = oldest
	}
	t0 := uint64(rt)
	frac := rt - float64(t0)
	a, okA := g.histPos(p.ID, t0)
	b, okB := g.histPos(p.ID, t0+1)
	switch {
	case okA && okB:
		return a.Lerp(b, frac), true
	case okA:
		return a, true
	case okB:
		return b, true
	}
	return Vec3{}, false
}

func (g *Game) histPos(id int, tick uint64) (Vec3, bool) {
	f := &g.history[tick%HistorySize]
	if f.tick != tick {
		return Vec3{}, false
	}
	v, ok := f.pos[id]
	return v, ok
}

func (g *Game) recordHistory() {
	f := &g.history[g.tick%HistorySize]
	f.tick = g.tick
	if f.pos == nil {
		f.pos = make(map[int]Vec3)
	} else {
		clear(f.pos)
	}
	for id, p := range g.players {
		if p.Alive {
			f.pos[id] = Vec3{p.State.X, p.State.Y, p.State.Z}
		}
	}
}

func (g *Game) damage(v, a *Player, dmg int, head bool, w int, back bool) {
	if !v.Alive || g.allies(v, a) { // sem fogo amigo (a própria granada ainda machuca)
		return
	}
	v.HP -= dmg
	v.LastHurt = g.tick
	v.heal = 0
	if v.Bot != nil {
		v.Bot.hurtBy, v.Bot.hurtAt = a.ID, g.tick
	}
	g.emit(Event{T: "hit", ID: a.ID, Target: v.ID, Dmg: dmg, Head: head, W: w})
	if v.HP > 0 {
		return
	}
	v.HP = 0
	v.Alive = false
	v.Deaths++
	v.RespawnAt = g.tick + RespawnTicks
	if a != v {
		a.Kills++
	}
	g.emit(Event{T: "kill", ID: a.ID, Target: v.ID, Head: head, W: w, Back: back})
	g.onKill(v, a)
}

func (g *Game) spawn(p *Player) {
	sp := g.pickSpawn(p)
	if p.Bot != nil {
		p.NextKind = g.rng.Intn(len(Weapons)) // bot sorteia a arma a cada vida
	}
	p.State = PhysState{X: sp.X, Y: sp.Y, Z: sp.Z, OnGround: true}
	p.Yaw, p.Pitch = sp.Yaw, 0
	p.HP = MaxHP
	p.heal = 0
	p.Alive = true
	p.Weapon = NewWeapon(p.NextKind)
	if p.Bot != nil {
		p.Bot.reset(sp.Yaw)
	}
	g.emit(Event{T: "spawn", ID: p.ID, Yaw: sp.Yaw})
}

// pickSpawn: o ponto (da equipe, se houver) mais longe dos inimigos vivos.
func (g *Game) pickSpawn(self *Player) Spawn {
	list := g.world.Spawns
	if g.teamMode() && self.Team > 0 && len(g.world.TeamSpawns[self.Team]) > 0 {
		list = g.world.TeamSpawns[self.Team]
	}
	best, bestScore := list[0], -1.0
	for _, s := range list {
		score := 1e9
		for _, p := range g.players {
			if p == self || !p.Alive || g.allies(p, self) {
				continue
			}
			dx, dz := p.State.X-s.X, p.State.Z-s.Z
			score = math.Min(score, dx*dx+dz*dz)
		}
		score += g.rng.Float64() * 40
		if score > bestScore {
			best, bestScore = s, score
		}
	}
	return best
}

// allowChat: até ChatBurst mensagens seguidas, depois uma a cada ChatEvery s.
func (g *Game) allowChat(p *Player) bool {
	p.chatTokens = math.Min(ChatBurst, p.chatTokens+float64(g.tick-p.chatTick)/(ChatEvery*TickRate))
	p.chatTick = g.tick
	if p.chatTokens < 1 {
		return false
	}
	p.chatTokens--
	return true
}

// removePlayer tira alguém do jogo (gente que saiu ou bot dispensado).
func (g *Game) removePlayer(p *Player) {
	if _, ok := g.players[p.ID]; !ok {
		return
	}
	g.dropFlagOf(p)
	delete(g.players, p.ID)
	if p.send != nil {
		close(p.send)
	}
	g.emit(Event{T: "leave", ID: p.ID, Name: p.Name})
}

func (g *Game) broadcast() {
	list := make([]SnapPlayer, 0, len(g.players))
	for _, p := range g.players {
		list = append(list, SnapPlayer{
			ID: p.ID, Name: p.Name,
			X: r3(p.State.X), Y: r3(p.State.Y), Z: r3(p.State.Z),
			Yaw: r3(p.Yaw), Pitch: r3(p.Pitch),
			HP: p.HP, Alive: p.Alive, Kills: p.Kills, Deaths: p.Deaths, W: p.Weapon.Kind,
			Wins: p.Wins, In: p.InRound, Team: p.Team, Bot: p.Bot != nil,
		})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	playersJSON, _ := json.Marshal(list)

	var eventsJSON json.RawMessage
	if len(g.events) > 0 {
		eventsJSON, _ = json.Marshal(g.events)
		g.events = g.events[:0]
	}
	mi := g.matchInfo()

	for _, p := range g.players {
		if p.send == nil {
			continue // bot
		}
		you := YouState{
			X: p.State.X, Y: p.State.Y, Z: p.State.Z,
			VX: p.State.VX, VY: p.State.VY, VZ: p.State.VZ,
			OG: p.State.OnGround, HP: p.HP, Alive: p.Alive, NK: p.NextKind,
			Vote: p.Vote, Team: p.Team, Weapon: p.Weapon,
		}
		if !p.Alive {
			if g.respawnAllowed() {
				you.Respawn = math.Max(0, float64(int64(p.RespawnAt)-int64(g.tick))/TickRate)
			} else {
				you.Respawn = -1
			}
		}
		data, err := json.Marshal(SnapshotMsg{
			T: "snap", Tick: g.tick, Ack: p.LastSeq, You: you, Match: mi,
			Players: playersJSON, Events: eventsJSON,
		})
		if err != nil {
			log.Println("snapshot:", err)
			continue
		}
		select {
		case p.send <- data:
		default:
		}
	}
}
