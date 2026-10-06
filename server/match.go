package main

import (
	"math"
)

// ---------------------------------------------------------------- modos

type ModeDef struct {
	ID, Name, Desc string
	Teams          bool
	Limit          int // pontuação para vencer: abates, pontos, capturas ou rodadas
	Time           int // segundos de jogo (0 = sem limite)
}

var Modes = []ModeDef{
	{"ffa", "Todos contra todos", "Cada um por si. 25 abates vencem.", false, 25, 360},
	{"tdm", "Mata-mata em equipe", "Vermelhos contra azuis. 50 abates da equipe vencem.", true, 50, 360},
	{"dom", "Dominação", "Três pontos no mapa. Cada ponto da sua equipe soma 1 por segundo.", true, 200, 420},
	{"koth", "Rei da colina", "A colina muda de lugar a cada minuto. Fique nela sem inimigos para pontuar.", true, 120, 420},
	{"ctf", "Captura a bandeira", "Traga a bandeira inimiga até a sua. 3 capturas vencem.", true, 3, 480},
	{"lms", "Sobrevivente", "Ninguém renasce na rodada. Quem vencer 3 rodadas leva a partida.", false, 3, 0},
}

const (
	TeamRed  = 1
	TeamBlue = 2
)

func modeIndex(id string) int {
	switch id { // nomes antigos continuam valendo
	case "sobrevivente":
		id = "lms"
	case "livre":
		id = "ffa"
	}
	for i, m := range Modes {
		if m.ID == id {
			return i
		}
	}
	return 1 // padrão: mata-mata em equipe
}

// CapPoint é um ponto de captura (dominação) ou a colina.
// Prog vai de −1 (vermelho) a +1 (azul).
type CapPoint struct {
	Name      string  `json:"n"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Z         float64 `json:"z"`
	R         float64 `json:"r"`
	Owner     int     `json:"o"`
	Prog      float64 `json:"p"`
	Contested bool    `json:"c,omitempty"`
	in        [3]int
}

type Flag struct {
	Team     int     `json:"t"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Z        float64 `json:"z"`
	State    string  `json:"s"` // base | carried | dropped
	Carrier  int     `json:"c,omitempty"`
	home     Vec3
	returnAt uint64
}

// Match guarda o estado da partida (fica embutido no Game).
type Match struct {
	mode       int
	phase      string // espera | contagem | jogo | fim | votacao
	phaseEnd   uint64
	matchEnd   uint64
	score      [3]float64
	winner     int
	winTeam    int
	lmsState   string // rodada | intervalo
	lmsRound   int
	lmsNext    uint64
	points     []*CapPoint
	hill       *CapPoint
	hillIdx    int
	hillMoveAt uint64
	flags      [3]*Flag
	voteOpts   []int
	targetBots int
	botNames   int

	countdown, results, voteTime, interval uint64
}

func (g *Game) initMatch(mode, bots int) {
	g.mode = mode
	g.phase = "espera"
	g.targetBots = bots
	g.countdown = 10 * TickRate
	g.results = 7 * TickRate
	g.voteTime = 15 * TickRate
	g.interval = 5 * TickRate
	g.setupObjectives()
}

func (g *Game) modeDef() *ModeDef { return &Modes[g.mode] }
func (g *Game) teamMode() bool    { return Modes[g.mode].Teams }

func (g *Game) respawnAllowed() bool {
	return g.phase != "jogo" || g.modeDef().ID != "lms"
}

func (g *Game) setPhase(ph string, ticks uint64) {
	g.phase = ph
	g.phaseEnd = g.tick + ticks
	e := Event{T: "match", S: ph, Name: g.modeDef().Name}
	if ph == "fim" {
		e.ID, e.Tm = g.winner, g.winTeam
		if w := g.players[g.winner]; w != nil {
			e.Msg = w.Name
		}
	}
	g.emit(e)
}

func (g *Game) updateMatch() {
	n := len(g.players)
	switch g.phase {
	case "espera":
		if n >= 2 {
			g.setPhase("contagem", g.countdown)
		}
	case "contagem":
		if n < 2 {
			g.setPhase("espera", 0)
		} else if g.tick >= g.phaseEnd {
			g.startMatch()
		}
	case "jogo":
		g.updateObjectives()
		if g.phase == "jogo" {
			g.checkEnd()
		}
	case "fim":
		if g.tick >= g.phaseEnd {
			g.startVote()
		}
	case "votacao":
		if g.tick >= g.phaseEnd {
			g.applyVote()
		}
	}
}

// startMatch: placar zerado, equipes equilibradas e todo mundo renasce junto.
func (g *Game) startMatch() {
	g.score = [3]float64{}
	g.winner, g.winTeam = 0, 0
	g.nades = nil
	for _, p := range g.players {
		p.Kills, p.Deaths, p.Wins = 0, 0, 0
		p.InRound = false
	}
	g.balanceTeams()
	g.setupObjectives()
	d := g.modeDef()
	g.matchEnd = g.tick + uint64(d.Time)*TickRate
	g.setPhase("jogo", 0)
	if d.ID == "lms" {
		g.lmsRound = 0
		g.startRound()
		return
	}
	g.respawnAll()
}

func (g *Game) respawnAll() {
	for _, p := range g.players {
		p.Alive = false
	}
	for _, p := range g.players {
		g.spawn(p)
	}
}

func (g *Game) checkEnd() {
	d := g.modeDef()
	switch d.ID {
	case "ffa":
		for _, p := range g.players {
			if p.Kills >= d.Limit {
				g.endMatch(p.ID, 0)
				return
			}
		}
	case "lms":
		return // as rodadas decidem
	default:
		for t := TeamRed; t <= TeamBlue; t++ {
			if g.score[t] >= float64(d.Limit) {
				g.endMatch(0, t)
				return
			}
		}
	}
	if d.Time > 0 && g.tick >= g.matchEnd {
		g.endByScore()
	}
}

// endByScore: acabou o tempo, vence quem estiver na frente (ou empate).
func (g *Game) endByScore() {
	if g.teamMode() {
		switch {
		case g.score[TeamRed] > g.score[TeamBlue]:
			g.endMatch(0, TeamRed)
		case g.score[TeamBlue] > g.score[TeamRed]:
			g.endMatch(0, TeamBlue)
		default:
			g.endMatch(0, 0)
		}
		return
	}
	best, bestScore, tie := 0, -1, false
	for _, p := range g.players {
		s := p.Kills
		if g.modeDef().ID == "lms" {
			s = p.Wins
		}
		if s > bestScore {
			best, bestScore, tie = p.ID, s, false
		} else if s == bestScore {
			tie = true
		}
	}
	if tie {
		best = 0
	}
	g.endMatch(best, 0)
}

func (g *Game) endMatch(winner, team int) {
	g.winner, g.winTeam = winner, team
	g.setPhase("fim", g.results)
}

// ---------------------------------------------------------------- votação

func (g *Game) startVote() {
	var others []int
	for i := range Modes {
		if i != g.mode {
			others = append(others, i)
		}
	}
	g.rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	if len(others) > 3 {
		others = others[:3]
	}
	g.voteOpts = others
	for _, p := range g.players {
		p.Vote = -1
	}
	g.setPhase("votacao", g.voteTime)
}

func (g *Game) voteCounts() []int {
	counts := make([]int, len(g.voteOpts))
	for _, p := range g.players {
		if p.Vote >= 0 && p.Vote < len(counts) {
			counts[p.Vote]++
		}
	}
	return counts
}

// applyVote: o modo mais votado vence; empate (ou ninguém votou) é sorteado.
func (g *Game) applyVote() {
	counts := g.voteCounts()
	best, top := -1, []int{}
	for i, c := range counts {
		if c > best {
			best, top = c, []int{i}
		} else if c == best {
			top = append(top, i)
		}
	}
	if len(top) > 0 {
		g.mode = g.voteOpts[top[g.rng.Intn(len(top))]]
	}
	g.voteOpts = nil
	g.winner, g.winTeam = 0, 0
	g.score = [3]float64{}
	g.balanceTeams()
	g.setupObjectives()
	for _, p := range g.players { // aquecimento já com as equipes novas
		p.Vote = -1
		p.Alive = false
		p.RespawnAt = g.tick
	}
	g.setPhase("contagem", g.countdown)
}

// ---------------------------------------------------------------- equipes

func (g *Game) teamCount() [3]int {
	var c [3]int
	for _, p := range g.players {
		c[p.Team]++
	}
	return c
}

func (g *Game) assignTeam(p *Player) {
	if !g.teamMode() {
		p.Team = 0
		return
	}
	c := g.teamCount()
	switch {
	case c[TeamRed] < c[TeamBlue]:
		p.Team = TeamRed
	case c[TeamBlue] < c[TeamRed]:
		p.Team = TeamBlue
	default:
		p.Team = TeamRed + g.rng.Intn(2)
	}
}

// balanceTeams: gente de verdade primeiro, alternando; bots completam.
func (g *Game) balanceTeams() {
	var humans, bots []*Player
	for _, p := range g.players {
		p.Team = 0
		if p.Bot == nil {
			humans = append(humans, p)
		} else {
			bots = append(bots, p)
		}
	}
	if !g.teamMode() {
		return
	}
	g.rng.Shuffle(len(humans), func(i, j int) { humans[i], humans[j] = humans[j], humans[i] })
	for _, p := range append(humans, bots...) {
		g.assignTeam(p)
	}
}

// ---------------------------------------------------------------- objetivos

func (g *Game) setupObjectives() {
	w := g.world
	g.points = nil
	for _, s := range w.DomPoints {
		g.points = append(g.points, &CapPoint{Name: s.Name, X: s.X, Y: s.Y, Z: s.Z, R: s.R})
	}
	g.hill = nil
	if len(w.Hills) > 0 {
		g.hillIdx = g.rng.Intn(len(w.Hills))
		g.placeHill()
	}
	for t := TeamRed; t <= TeamBlue; t++ {
		h := w.FlagHome[t]
		g.flags[t] = &Flag{Team: t, X: h.X, Y: h.Y, Z: h.Z, State: "base", home: Vec3{h.X, h.Y, h.Z}}
	}
}

func (g *Game) placeHill() {
	s := g.world.Hills[g.hillIdx]
	g.hill = &CapPoint{Name: s.Name, X: s.X, Y: s.Y, Z: s.Z, R: s.R}
	g.hillMoveAt = g.tick + 60*TickRate
}

func (g *Game) updateObjectives() {
	second := g.tick%TickRate == 0
	switch g.modeDef().ID {
	case "dom":
		for _, cp := range g.points {
			g.tickPoint(cp, 1.0/8)
			if second && cp.Owner != 0 {
				g.score[cp.Owner]++
			}
		}
	case "koth":
		if g.tick >= g.hillMoveAt && len(g.world.Hills) > 1 {
			next := g.hillIdx
			for next == g.hillIdx {
				next = g.rng.Intn(len(g.world.Hills))
			}
			g.hillIdx = next
			g.placeHill()
			g.emit(Event{T: "hill", Name: g.hill.Name})
		}
		g.tickPoint(g.hill, 1.0/4)
		if o := g.hill.Owner; second && o != 0 && g.hill.in[o] > 0 && g.hill.in[3-o] == 0 {
			g.score[o]++
		}
	case "ctf":
		g.tickFlags()
	case "lms":
		g.updateRounds()
	}
}

// tickPoint: quem está dentro empurra o progresso; com as duas equipes
// dentro, fica disputado e nada muda.
func (g *Game) tickPoint(cp *CapPoint, rate float64) {
	var n [3]int
	for _, p := range g.players {
		if !p.Alive || p.Team == 0 {
			continue
		}
		dx, dz := p.State.X-cp.X, p.State.Z-cp.Z
		if dx*dx+dz*dz <= cp.R*cp.R && math.Abs(p.State.Y-cp.Y) <= 4 {
			n[p.Team]++
		}
	}
	cp.in = n
	cp.Contested = n[TeamRed] > 0 && n[TeamBlue] > 0
	if cp.Contested {
		return
	}
	step := rate / TickRate
	switch {
	case n[TeamRed] > 0:
		g.pushPoint(cp, TeamRed, -step*math.Min(2, float64(n[TeamRed])))
	case n[TeamBlue] > 0:
		g.pushPoint(cp, TeamBlue, step*math.Min(2, float64(n[TeamBlue])))
	default: // vazio: volta devagar para o lado de quem é dono
		target := [3]float64{0, -1, 1}[cp.Owner]
		cp.Prog += math.Max(-0.1/TickRate, math.Min(0.1/TickRate, target-cp.Prog))
	}
}

func (g *Game) pushPoint(cp *CapPoint, team int, delta float64) {
	cp.Prog = math.Max(-1, math.Min(1, cp.Prog+delta))
	if (cp.Owner == TeamBlue && cp.Prog <= 0) || (cp.Owner == TeamRed && cp.Prog >= 0) {
		cp.Owner = 0
		g.emit(Event{T: "point", S: "neutro", Name: cp.Name, Tm: team})
	}
	if cp.Owner != team && math.Abs(cp.Prog) >= 1 {
		cp.Owner = team
		g.emit(Event{T: "point", S: "capturado", Name: cp.Name, Tm: team})
	}
}

func flagNear(p *Player, x, y, z, r float64) bool {
	dx, dz := p.State.X-x, p.State.Z-z
	return dx*dx+dz*dz <= r*r && math.Abs(p.State.Y-y) <= 2
}

func (g *Game) tickFlags() {
	for t := TeamRed; t <= TeamBlue; t++ {
		f := g.flags[t]
		switch f.State {
		case "carried":
			if c := g.players[f.Carrier]; c != nil && c.Alive {
				f.X, f.Y, f.Z = c.State.X, c.State.Y, c.State.Z
			} else {
				g.dropFlag(f)
			}
		case "dropped":
			if g.tick >= f.returnAt {
				g.returnFlag(f, 0)
			}
		}
	}
	for _, p := range g.players {
		if !p.Alive || p.Team == 0 {
			continue
		}
		own, enemy := g.flags[p.Team], g.flags[3-p.Team]
		if enemy.State != "carried" && flagNear(p, enemy.X, enemy.Y, enemy.Z, 1.6) {
			enemy.State, enemy.Carrier = "carried", p.ID
			g.emit(Event{T: "flag", S: "pegou", ID: p.ID, Tm: enemy.Team})
		}
		if own.State == "dropped" && flagNear(p, own.X, own.Y, own.Z, 1.6) {
			g.returnFlag(own, p.ID)
		}
		if enemy.State == "carried" && enemy.Carrier == p.ID && own.State == "base" &&
			flagNear(p, own.home.X, own.home.Y, own.home.Z, 2.5) {
			g.score[p.Team]++
			g.emit(Event{T: "flag", S: "capturou", ID: p.ID, Tm: enemy.Team})
			g.returnFlag(enemy, -1)
		}
	}
}

func (g *Game) dropFlag(f *Flag) {
	f.State, f.Carrier = "dropped", 0
	f.returnAt = g.tick + 20*TickRate
	g.emit(Event{T: "flag", S: "caiu", Tm: f.Team})
}

// returnFlag: by > 0 quem devolveu; 0 voltou sozinha; −1 depois de captura.
func (g *Game) returnFlag(f *Flag, by int) {
	f.State, f.Carrier = "base", 0
	f.X, f.Y, f.Z = f.home.X, f.home.Y, f.home.Z
	if by >= 0 {
		g.emit(Event{T: "flag", S: "voltou", ID: by, Tm: f.Team})
	}
}

func (g *Game) dropFlagOf(p *Player) {
	for t := TeamRed; t <= TeamBlue; t++ {
		if f := g.flags[t]; f != nil && f.State == "carried" && f.Carrier == p.ID {
			f.X, f.Y, f.Z = p.State.X, p.State.Y, p.State.Z
			g.dropFlag(f)
		}
	}
}

// onKill: regras de cada modo quando alguém cai.
func (g *Game) onKill(v, a *Player) {
	g.dropFlagOf(v)
	if g.phase == "jogo" && g.modeDef().ID == "tdm" && a != v && a.Team != v.Team {
		g.score[a.Team]++
	}
}

// ---------------------------------------------------------------- sobrevivente

func (g *Game) startRound() {
	g.lmsRound++
	g.nades = nil
	for _, p := range g.players {
		p.Alive = false
	}
	for _, p := range g.players {
		p.InRound = true
		g.spawn(p)
	}
	g.lmsState = "rodada"
	g.emit(Event{T: "round", S: "rodada", N: g.lmsRound})
}

func (g *Game) updateRounds() {
	switch g.lmsState {
	case "rodada":
		alive := 0
		var last *Player
		for _, p := range g.players {
			if p.InRound && p.Alive {
				alive++
				last = p
			}
		}
		if alive > 1 {
			return
		}
		e := Event{T: "round", S: "fim", N: g.lmsRound}
		if last != nil {
			last.Wins++
			e.ID, e.Name = last.ID, last.Name
		}
		g.emit(e)
		if last != nil && last.Wins >= g.modeDef().Limit {
			g.endMatch(last.ID, 0)
			return
		}
		g.lmsState = "intervalo"
		g.lmsNext = g.tick + g.interval
	case "intervalo":
		if g.tick >= g.lmsNext {
			if len(g.players) < 2 {
				g.endByScore()
				return
			}
			g.startRound()
		}
	}
}

// ---------------------------------------------------------------- snapshot

func (g *Game) matchInfo() *MatchInfo {
	d := g.modeDef()
	mi := &MatchInfo{Mode: d.ID, Name: d.Name, Phase: g.phase, Limit: d.Limit, Winner: g.winner, WinTeam: g.winTeam}
	end := g.phaseEnd
	if g.phase == "jogo" {
		end = 0
		if d.Time > 0 {
			end = g.matchEnd
		}
		if d.ID == "lms" && g.lmsState == "intervalo" {
			end = g.lmsNext
		}
	}
	if end > g.tick {
		mi.Left = r2(float64(end-g.tick) / TickRate)
	}
	if d.Teams {
		mi.Score = []int{int(g.score[TeamRed]), int(g.score[TeamBlue])}
	}
	switch d.ID {
	case "dom":
		mi.Points = g.points
	case "koth":
		mi.Hill = g.hill
	case "ctf":
		mi.Flags = []*Flag{g.flags[TeamRed], g.flags[TeamBlue]}
	case "lms":
		if g.phase == "jogo" {
			rd := &RoundInfo{State: g.lmsState, Num: g.lmsRound}
			for _, p := range g.players {
				if p.InRound {
					rd.Total++
					if p.Alive {
						rd.Alive++
					}
				}
			}
			mi.Round = rd
		}
	}
	if g.phase == "votacao" {
		v := &VoteInfo{Counts: g.voteCounts()}
		for _, i := range g.voteOpts {
			v.Opts = append(v.Opts, Modes[i].ID)
			v.Names = append(v.Names, Modes[i].Name)
			v.Descs = append(v.Descs, Modes[i].Desc)
		}
		mi.Vote = v
	}
	return mi
}
