package main

import (
	"math"
	"math/rand"
)

// Ajustes de dificuldade dos bots (em ticks: 60 = 1 s)
const (
	BotReactMin    = 20  // reação ao ver alguém pela primeira vez: 0,33 s ...
	BotReactSpread = 18  // ... até 0,63 s
	BotReacqMin    = 15  // reação quando o alvo reaparece: 0,25 s ...
	BotReacqSpread = 12  // ... até 0,45 s
	BotTrackLag    = 4   // mira onde o alvo estava há ~70 ms
	BotHeadshot    = 0.2 // chance de mirar na cabeça (nunca com escopeta)
)

var botNames = []string{"Zé", "Tião", "Nina", "Gugu", "Dida", "Lulu", "Kiko", "Bia", "Juca", "Mel", "Tuca", "Pipa"}

// Bot é o "cérebro" de um jogador controlado pelo servidor. Ele não trapaceia
// na física: gera inputs (teclas + mira) iguais aos de uma pessoa.
type Bot struct {
	seq        int
	path       []Vec3
	goal       Vec3
	goalUntil  uint64
	repathAt   uint64
	target     int
	seenSince  uint64
	lastSeen   Vec3
	lastSeenAt uint64
	aimYaw     float64
	aimPitch   float64
	errYaw     float64
	errPitch   float64
	headshot   bool
	reaction   uint64
	strafe     int
	strafeAt   uint64
	checkPos   Vec3
	checkAt    uint64
	stuck      int
	jumpUntil  uint64
	role       int // na bandeira: 0 defende, 1 e 2 atacam
	hurtBy     int
	hurtAt     uint64
	nadeAt     uint64
}

func (b *Bot) reset(yaw float64) {
	b.path, b.goalUntil, b.repathAt, b.target = nil, 0, 0, 0
	b.aimYaw, b.aimPitch = yaw, 0
	b.lastSeenAt, b.stuck = 0, 0
}

func (g *Game) addBot() *Player {
	name := "Bot " + botNames[g.botNames%len(botNames)]
	g.botNames++
	p := &Player{ID: g.nextID, Name: name, NextKind: g.rng.Intn(len(Weapons)),
		Bot: &Bot{role: g.rng.Intn(3)}, Vote: -1, chatTokens: ChatBurst}
	g.nextID++
	g.players[p.ID] = p
	g.assignTeam(p)
	if g.respawnAllowed() {
		g.spawn(p)
	}
	g.emit(Event{T: "join", ID: p.ID, Name: name})
	return p
}

// balanceBots completa a sala com bots até targetBots participantes e
// tira bots quando chega gente. Sem ninguém de verdade, sem bots.
func (g *Game) balanceBots() {
	humans := 0
	var bots []*Player
	for _, p := range g.players {
		if p.Bot == nil {
			humans++
		} else {
			bots = append(bots, p)
		}
	}
	want := g.targetBots - humans
	if humans == 0 || want < 0 {
		want = 0
	}
	for len(bots) < want {
		bots = append(bots, g.addBot())
	}
	for len(bots) > want {
		c := g.teamCount()
		k := len(bots) - 1
		for i, b := range bots { // sai bot da equipe maior
			if g.teamMode() && c[b.Team] > c[3-b.Team] {
				k = i
				break
			}
		}
		g.removePlayer(bots[k])
		bots = append(bots[:k], bots[k+1:]...)
	}
}

func (g *Game) botsThink() {
	for _, p := range g.players {
		if p.Bot != nil && p.Alive {
			p.inputs = append(p.inputs, g.botInput(p))
		}
	}
}

func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b+math.Pi, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return d - math.Pi
}

// moveKeys: teclas que levam na direção (dx, dz) olhando para yaw — o bot
// consegue andar para um lado enquanto mira para outro.
func moveKeys(yaw, dx, dz float64) int {
	l := math.Hypot(dx, dz)
	if l < 1e-6 {
		return 0
	}
	dx, dz = dx/l, dz/l
	f := dx*-math.Sin(yaw) + dz*-math.Cos(yaw)
	r := dx*math.Cos(yaw) + dz*-math.Sin(yaw)
	keys := 0
	if f > 0.38 {
		keys |= KeyForward
	} else if f < -0.38 {
		keys |= KeyBack
	}
	if r > 0.38 {
		keys |= KeyRight
	} else if r < -0.38 {
		keys |= KeyLeft
	}
	return keys
}

func (g *Game) objectiveMode() bool {
	switch g.modeDef().ID {
	case "dom", "koth", "ctf":
		return true
	}
	return false
}

func (b *Bot) turn(yaw, pitch, rate float64) {
	step := rate * DT
	b.aimYaw += math.Max(-step, math.Min(step, angleDiff(yaw, b.aimYaw)))
	b.aimYaw = angleDiff(b.aimYaw, 0)
	b.aimPitch += math.Max(-step, math.Min(step, pitch-b.aimPitch))
	b.aimPitch = ClampPitch(b.aimPitch)
}

func hdist(a, b Vec3) float64 { return math.Hypot(a.X-b.X, a.Z-b.Z) }

// canSee: há linha de visada do olho até o peito ou a cabeça do alvo?
func (g *Game) canSee(eye Vec3, q *Player) bool {
	for _, h := range [2]float64{1.1, 1.6} {
		t := Vec3{q.State.X, q.State.Y + h, q.State.Z}
		dx, dy, dz := t.X-eye.X, t.Y-eye.Y, t.Z-eye.Z
		dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if dist < 0.01 {
			return true
		}
		if RayWorld(eye, Vec3{dx / dist, dy / dist, dz / dist}, g.world.Boxes, dist) >= dist-0.3 {
			return true
		}
	}
	return false
}

// perceive: procura o inimigo visível mais perto (campo de visão de ~130°,
// mais quem está muito perto ou acabou de acertá-lo).
func (g *Game) perceive(p *Player, eye Vec3) {
	b := p.Bot
	var best *Player
	bestD := math.Inf(1)
	for _, q := range g.players {
		if q == p || !q.Alive || g.allies(p, q) {
			continue
		}
		dx, dz := q.State.X-p.State.X, q.State.Z-p.State.Z
		hd := math.Hypot(dx, dz)
		if hd > 110 || hd >= bestD {
			continue
		}
		ang := math.Abs(angleDiff(math.Atan2(-dx, -dz), b.aimYaw))
		hurt := b.hurtBy == q.ID && g.tick-b.hurtAt < 3*TickRate
		if ang > 1.15 && hd > 8 && !hurt {
			continue
		}
		if g.canSee(eye, q) {
			best, bestD = q, hd
		}
	}
	if best == nil {
		return
	}
	gap := g.tick - b.lastSeenAt
	switch {
	case best.ID != b.target || gap > TickRate:
		// alvo novo: tempo de reação e um erro de mira que vai diminuindo
		b.target = best.ID
		b.seenSince = g.tick
		b.reaction = uint64(BotReactMin + g.rng.Intn(BotReactSpread))
		b.errYaw = (g.rng.Float64()*2 - 1) * 0.22
		b.errPitch = (g.rng.Float64()*2 - 1) * 0.09
		b.headshot = p.Weapon.Kind != WShotgun && g.rng.Float64() < BotHeadshot
	case gap > 10:
		// o mesmo alvo reapareceu (passou por trás de algo): reage de novo,
		// um pouco mais rápido, e volta com algum erro de mira
		b.seenSince = g.tick
		b.reaction = uint64(BotReacqMin + g.rng.Intn(BotReacqSpread))
		b.errYaw += (g.rng.Float64()*2 - 1) * 0.12
		b.errPitch += (g.rng.Float64()*2 - 1) * 0.05
	}
	b.lastSeenAt = g.tick
	b.lastSeen = Vec3{best.State.X, best.State.Y, best.State.Z}
}

func (g *Game) botInput(p *Player) Input {
	b := p.Bot
	b.seq++
	me := Vec3{p.State.X, p.State.Y, p.State.Z}
	eye := Vec3{me.X, me.Y + EyeHeight, me.Z}
	kind := p.Weapon.Kind
	keys := 0
	pitchExtra := 0.0

	if b.seq%6 == 0 {
		g.perceive(p, eye)
	}
	tgt := g.players[b.target]
	visible := tgt != nil && tgt.Alive && g.tick-b.lastSeenAt <= 8
	objective := g.objectiveMode()

	// Caminho até o objetivo (nos modos de objetivo, mesmo durante o tiroteio)
	var pathDX, pathDZ float64
	hasPath := false
	if !visible || objective {
		goal := g.botGoal(p)
		if b.path == nil || g.tick >= b.repathAt {
			b.path = g.nav.Path(me, goal)
			b.repathAt = g.tick + uint64(2*TickRate+g.rng.Intn(TickRate))
		}
		for len(b.path) > 0 && hdist(me, b.path[0]) < 0.7 {
			b.path = b.path[1:]
		}
		if len(b.path) > 0 {
			pathDX, pathDZ, hasPath = b.path[0].X-me.X, b.path[0].Z-me.Z, true
		} else if g.tick >= b.repathAt-TickRate {
			b.goalUntil = 0 // chegou (ou não há caminho): escolhe outro objetivo
		}
	}

	if visible {
		aimH := 1.1
		if b.headshot {
			aimH = 1.62
		}
		if kind == WShotgun {
			aimH = 1.0 // escopeta: centro do corpo
		}
		// Acompanha o alvo com um pequeno atraso, como uma pessoa: mira onde
		// ele estava há BotTrackLag ticks. Quem corre de lado faz o bot errar.
		aimAt := Vec3{tgt.State.X, tgt.State.Y, tgt.State.Z}
		if g.tick > BotTrackLag {
			if old, ok := g.histPos(tgt.ID, g.tick-BotTrackLag); ok {
				aimAt = old
			}
		}
		dx, dy, dz := aimAt.X-eye.X, aimAt.Y+aimH-eye.Y, aimAt.Z-eye.Z
		hd := math.Hypot(dx, dz)
		dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
		wantYaw, wantPitch := math.Atan2(-dx, -dz), math.Atan2(dy, hd)
		decay := math.Exp(-DT / 0.45)
		b.errYaw *= decay
		b.errPitch *= decay
		b.turn(wantYaw+b.errYaw, wantPitch+b.errPitch, 9)

		if objective && hasPath && dist > 6 {
			keys |= moveKeys(b.aimYaw, pathDX, pathDZ) // segue para o objetivo atirando
		} else {
			pref := [3]float64{18, 32, 5}[kind]
			if dist > pref+6 {
				keys |= KeyForward
			} else if kind != WShotgun && dist < pref-8 {
				keys |= KeyBack
			}
			if g.tick >= b.strafeAt {
				b.strafe = g.rng.Intn(3) - 1
				b.strafeAt = g.tick + uint64(30+g.rng.Intn(60))
			}
			if b.strafe < 0 {
				keys |= KeyLeft
			} else if b.strafe > 0 {
				keys |= KeyRight
			}
		}
		needAim := (kind == WSniper && dist > 10) || (kind == WAR && dist > 22)
		if needAim {
			keys |= KeyAim
		}
		errAng := math.Abs(angleDiff(b.aimYaw, wantYaw)) + math.Abs(b.aimPitch-wantPitch)
		ready := g.tick-b.seenSince >= b.reaction
		inRange := dist < Weapons[kind].Range*0.8 && !(kind == WShotgun && dist > 22)
		if ready && inRange && errAng < math.Max(0.012, 0.32/dist) && (!needAim || Aimed(&p.Weapon, keys, b.seq)) {
			keys |= KeyFire
		}
		if dist < 2.0 {
			keys |= KeyMelee
		}
		if dist > 12 && dist < 28 && p.Weapon.Nades > 0 && g.tick >= b.nadeAt && g.rng.Float64() < 0.004 {
			keys |= KeyNade
			pitchExtra = 0.3
			b.nadeAt = g.tick + 10*TickRate
		}
	} else {
		if hasPath {
			wantYaw := math.Atan2(-pathDX, -pathDZ)
			b.turn(wantYaw, 0, 10)
			if math.Abs(angleDiff(b.aimYaw, wantYaw)) < 1.2 {
				keys |= KeyForward
			}
		}
		if p.Weapon.Ammo*2 < Weapons[kind].Mag && p.Weapon.ReloadEnd == 0 {
			keys |= KeyReload
		}
	}

	// Preso? Pula, desvia de lado e, se continuar, refaz o caminho.
	if keys&(KeyForward|KeyBack) != 0 {
		if g.tick >= b.checkAt {
			if hdist(me, b.checkPos) < 0.5 {
				b.stuck++
				b.jumpUntil = g.tick + 20
				b.strafe = 1 - 2*g.rng.Intn(2)
				b.strafeAt = g.tick + 40
				if b.stuck >= 2 {
					b.path, b.goalUntil, b.stuck = nil, 0, 0
				}
			} else {
				b.stuck = 0
			}
			b.checkPos, b.checkAt = me, g.tick+45
		}
	}
	if g.tick < b.jumpUntil {
		keys |= KeyJump
	}
	if !visible && g.tick < b.strafeAt && b.stuck > 0 {
		if b.strafe < 0 {
			keys |= KeyLeft
		} else {
			keys |= KeyRight
		}
	}
	return Input{Seq: b.seq, Keys: keys, Yaw: b.aimYaw, Pitch: ClampPitch(b.aimPitch + pitchExtra)}
}

func randAround(r *rand.Rand, c Vec3, rad float64) Vec3 {
	a, d := r.Float64()*2*math.Pi, r.Float64()*rad
	return Vec3{c.X + math.Cos(a)*d, c.Y, c.Z + math.Sin(a)*d}
}

// botGoal: para onde ir quando não há inimigo à vista, conforme o modo.
func (g *Game) botGoal(p *Player) Vec3 {
	b := p.Bot
	me := Vec3{p.State.X, p.State.Y, p.State.Z}
	if g.tick < b.goalUntil && hdist(me, b.goal) > 1.5 {
		return b.goal
	}
	b.goalUntil = g.tick + uint64(4+g.rng.Intn(4))*TickRate
	b.path = nil
	// Vai até onde viu o inimigo (nos modos de objetivo, só se ele estava perto)
	if b.target != 0 && g.tick-b.lastSeenAt < 4*TickRate && (!g.objectiveMode() || hdist(me, b.lastSeen) < 15) {
		b.goal = b.lastSeen
		return b.goal
	}
	switch g.modeDef().ID {
	case "dom":
		var best *CapPoint
		bestD := math.Inf(1)
		for _, cp := range g.points {
			d := hdist(me, Vec3{cp.X, 0, cp.Z})
			if cp.Owner == p.Team {
				d += 60 // prefere os pontos que ainda não são da equipe
			}
			if d < bestD {
				best, bestD = cp, d
			}
		}
		if best != nil {
			b.goal = randAround(g.rng, Vec3{best.X, best.Y, best.Z}, best.R*0.6)
			return b.goal
		}
	case "koth":
		if h := g.hill; h != nil {
			b.goal = randAround(g.rng, Vec3{h.X, h.Y, h.Z}, h.R*0.6)
			return b.goal
		}
	case "ctf":
		own, enemy := g.flags[p.Team], g.flags[3-p.Team]
		b.goalUntil = g.tick + 2*TickRate
		switch {
		case enemy.State == "carried" && enemy.Carrier == p.ID:
			b.goal = own.home
		case b.role == 0 && own.State != "base":
			b.goal = Vec3{own.X, own.Y, own.Z} // persegue quem levou / devolve
		case b.role == 0:
			b.goal = randAround(g.rng, own.home, 6)
			b.goalUntil = g.tick + 5*TickRate
		default:
			b.goal = Vec3{enemy.X, enemy.Y, enemy.Z} // ataca (ou escolta quem levou)
		}
		return b.goal
	}
	// Mata-mata: caça um inimigo qualquer, ou passeia por um spawn
	var enemies []*Player
	for _, q := range g.players {
		if q != p && q.Alive && !g.allies(p, q) {
			enemies = append(enemies, q)
		}
	}
	if len(enemies) > 0 && g.rng.Float64() < 0.7 {
		q := enemies[g.rng.Intn(len(enemies))]
		b.goal = Vec3{q.State.X, q.State.Y, q.State.Z}
	} else {
		s := g.world.Spawns[g.rng.Intn(len(g.world.Spawns))]
		b.goal = Vec3{s.X, 0, s.Z}
	}
	return b.goal
}
