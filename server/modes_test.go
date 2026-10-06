package main

import (
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Jogo de teste num modo, arena vazia, fases curtas.
func newModeGame(mode string) *Game {
	g := NewGame(GenerateArena(1), mode, 0)
	g.world.Boxes = nil
	g.countdown, g.results, g.voteTime, g.interval = 10, 10, 10, 10
	return g
}

func ticks(g *Game, n int) {
	for i := 0; i < n; i++ {
		g.update()
	}
}

func place(p *Player, x, y, z float64) { p.State = PhysState{X: x, Y: y, Z: z, OnGround: true} }

func TestCuraComOTempo(t *testing.T) {
	g := newModeGame("ffa")
	a, b := g.join("Ana", WAR), g.join("Beto", WAR)
	ticks(g, 12) // a partida começa (todo mundo renasce) antes do dano
	g.damage(b, a, 60, false, WAR, false)
	ticks(g, RegenDelay-5)
	if b.HP != 40 {
		t.Fatalf("antes de 5 s não deveria curar: %d", b.HP)
	}
	ticks(g, TickRate) // 1 s depois do atraso: ~20 de vida
	if b.HP < 55 || b.HP > 65 {
		t.Fatalf("depois de 1 s curando esperava ~60, veio %d", b.HP)
	}
	g.damage(b, a, 10, false, WAR, false) // tomou dano: a cura para e recomeça a contagem
	hp := b.HP
	ticks(g, TickRate)
	if b.HP != hp {
		t.Fatal("tomar dano deveria interromper a cura")
	}
	ticks(g, RegenDelay+3*TickRate)
	if b.HP != MaxHP {
		t.Fatalf("deveria encher: %d", b.HP)
	}
}

func TestEquipeSemFogoAmigoEPlacar(t *testing.T) {
	g := newModeGame("tdm")
	ps := []*Player{g.join("Ana", WAR), g.join("Beto", WAR), g.join("Caio", WAR), g.join("Duda", WAR)}
	ticks(g, 12)
	if g.phase != "jogo" {
		t.Fatalf("fase %s", g.phase)
	}
	c := g.teamCount()
	if c[TeamRed] != 2 || c[TeamBlue] != 2 {
		t.Fatalf("equipes desequilibradas: %v", c)
	}
	var red, blue []*Player
	for _, p := range ps {
		if p.Team == TeamRed {
			red = append(red, p)
		} else {
			blue = append(blue, p)
		}
	}
	g.damage(red[1], red[0], 50, false, WAR, false)
	if red[1].HP != MaxHP {
		t.Fatal("colega de equipe não deveria tomar dano")
	}
	for i := 0; i < Modes[1].Limit; i++ {
		g.damage(blue[0], red[0], 200, false, WAR, false)
		blue[0].Alive, blue[0].HP = true, MaxHP
		ticks(g, 1)
		if g.phase != "jogo" {
			break
		}
	}
	if g.phase != "fim" || g.winTeam != TeamRed {
		t.Fatalf("vermelho deveria vencer com %d abates: fase %s, placar %v", Modes[1].Limit, g.phase, g.score)
	}
}

func TestDominacaoCapturaEPontua(t *testing.T) {
	g := newModeGame("dom")
	a, b := g.join("Ana", WAR), g.join("Beto", WAR)
	ticks(g, 12)
	a.Team, b.Team = TeamRed, TeamBlue
	cp := g.points[0]
	place(a, cp.X, cp.Y, cp.Z)
	place(b, 100, 0, 100)
	ticks(g, 8*TickRate+5)
	if cp.Owner != TeamRed {
		t.Fatalf("em ~8 s o ponto deveria ser vermelho: dono %d, progresso %.2f", cp.Owner, cp.Prog)
	}
	before := g.score[TeamRed]
	ticks(g, 3*TickRate)
	if g.score[TeamRed]-before < 2 {
		t.Fatalf("ponto dominado deveria somar 1 por segundo: %.0f → %.0f", before, g.score[TeamRed])
	}
	place(b, cp.X+1, cp.Y, cp.Z) // disputado: nada muda
	prog := cp.Prog
	ticks(g, 2*TickRate)
	if !cp.Contested || cp.Prog != prog {
		t.Fatal("com as duas equipes no ponto ele fica disputado")
	}
}

func TestReiDaColinaSoPontuaSozinho(t *testing.T) {
	g := newModeGame("koth")
	a, b := g.join("Ana", WAR), g.join("Beto", WAR)
	ticks(g, 12)
	a.Team, b.Team = TeamRed, TeamBlue
	h := g.hill
	place(a, h.X, h.Y, h.Z)
	place(b, 100, 0, 100)
	ticks(g, 5*TickRate)
	if h.Owner != TeamRed || g.score[TeamRed] < 1 {
		t.Fatalf("colina deveria ser vermelha e pontuar: dono %d, placar %v", h.Owner, g.score)
	}
	place(b, h.X+1, h.Y, h.Z)
	s := g.score[TeamRed]
	ticks(g, 3*TickRate)
	if g.score[TeamRed] != s {
		t.Fatal("com inimigo na colina ninguém pontua")
	}
}

func TestCapturaABandeira(t *testing.T) {
	g := newModeGame("ctf")
	a, b := g.join("Ana", WAR), g.join("Beto", WAR)
	ticks(g, 12)
	a.Team, b.Team = TeamRed, TeamBlue
	blue, red := g.flags[TeamBlue], g.flags[TeamRed]
	place(b, 100, 0, 100)
	place(a, blue.X, blue.Y, blue.Z) // Ana pega a bandeira azul
	ticks(g, 1)
	if blue.State != "carried" || blue.Carrier != a.ID {
		t.Fatal("deveria pegar a bandeira inimiga")
	}
	place(a, 0, 0, 0)
	g.damage(a, b, 200, false, WAR, false) // morreu carregando: a bandeira cai
	ticks(g, 1)
	if blue.State != "dropped" {
		t.Fatalf("bandeira deveria cair: %s", blue.State)
	}
	place(b, blue.X, blue.Y, blue.Z) // Beto devolve a própria bandeira
	ticks(g, 1)
	if blue.State != "base" {
		t.Fatal("tocar a própria bandeira caída devolve")
	}
	for i := 0; i < 3; i++ { // três capturas
		ticks(g, RespawnTicks+2)
		place(b, 100, 0, 100)
		place(a, blue.X, blue.Y, blue.Z)
		ticks(g, 1)
		place(a, red.home.X, red.home.Y, red.home.Z)
		ticks(g, 1)
	}
	if g.phase != "fim" || g.winTeam != TeamRed {
		t.Fatalf("3 capturas deveriam vencer: fase %s, placar %v", g.phase, g.score)
	}
}

func TestSobreviventeMelhorDeRodadas(t *testing.T) {
	g := newModeGame("lms")
	a, b, c := g.join("Ana", WAR), g.join("Beto", WAR), g.join("Caio", WAR)
	ticks(g, 12)
	if g.phase != "jogo" || g.lmsState != "rodada" || !b.InRound {
		t.Fatalf("a primeira rodada deveria começar: %s %s", g.phase, g.lmsState)
	}
	d := g.join("Duda", WAR) // entrou no meio: só assiste
	g.damage(b, a, 200, false, WAR, false)
	ticks(g, RespawnTicks+5)
	if b.Alive || d.Alive {
		t.Fatal("ninguém renasce durante a rodada")
	}
	for round := 1; round <= 3; round++ {
		for _, v := range []*Player{b, c, d} {
			if v.Alive {
				g.damage(v, a, 200, false, WAR, false)
			}
		}
		ticks(g, 1)
		if a.Wins != round {
			t.Fatalf("rodada %d: Ana deveria ter %d vitórias, tem %d", round, round, a.Wins)
		}
		if round < 3 {
			ticks(g, 12)
			if !d.InRound || !d.Alive {
				t.Fatal("quem entrou no meio joga a rodada seguinte")
			}
		}
	}
	if g.phase != "fim" || g.winner != a.ID {
		t.Fatalf("3 rodadas vencem a partida: fase %s, vencedor %d", g.phase, g.winner)
	}
}

func TestVotacaoEscolheOProximoModo(t *testing.T) {
	g := newModeGame("ffa")
	a, b, c := g.join("Ana", WAR), g.join("Beto", WAR), g.join("Caio", WAR)
	ticks(g, 12)
	g.endMatch(a.ID, 0)
	ticks(g, 12)
	if g.phase != "votacao" || len(g.voteOpts) != 3 {
		t.Fatalf("deveria abrir votação com 3 opções: %s %v", g.phase, g.voteOpts)
	}
	for _, o := range g.voteOpts {
		if o == modeIndex("ffa") {
			t.Fatal("o modo atual não entra na votação")
		}
	}
	a.Vote, b.Vote, c.Vote = 2, 2, 0
	want := g.voteOpts[2]
	ticks(g, 12)
	if g.mode != want || g.phase != "contagem" {
		t.Fatalf("o mais votado deveria vencer: modo %s, fase %s", Modes[g.mode].ID, g.phase)
	}
	if g.teamMode() {
		cnt := g.teamCount()
		if cnt[0] != 0 || math.Abs(float64(cnt[1]-cnt[2])) > 1 {
			t.Fatalf("equipes do modo novo: %v", cnt)
		}
	}
}

func TestChatLimiteELimpeza(t *testing.T) {
	g := newModeGame("ffa")
	p := g.join("Ana", WAR)
	ok := 0
	for i := 0; i < 10; i++ {
		if g.allowChat(p) {
			ok++
		}
	}
	if ok != ChatBurst {
		t.Fatalf("esperava %d mensagens seguidas, passaram %d", ChatBurst, ok)
	}
	ticks(g, int(ChatEvery*TickRate)+1)
	if !g.allowChat(p) {
		t.Fatal("depois de esperar, deveria liberar mais uma")
	}
	if got := cleanChat("  oi\x07 tudo bem?  "); got != "oi tudo bem?" {
		t.Fatalf("limpeza: %q", got)
	}
	if n := utf8.RuneCountInString(cleanChat(strings.Repeat("á", 300))); n != 120 {
		t.Fatalf("limite de 120 letras: %d", n)
	}
}

// Bots: uma pessoa entra e a sala completa; quando sai, os bots saem.
func TestBotsCompletamASala(t *testing.T) {
	g := NewGame(BuildVila(), "tdm", 6)
	ana := g.join("Ana", WAR)
	if len(g.players) != 6 {
		t.Fatalf("esperava 6 na sala, tem %d", len(g.players))
	}
	beto := g.join("Beto", WAR)
	if len(g.players) != 6 {
		t.Fatal("quando entra gente, sai um bot")
	}
	c := g.teamCount()
	if c[TeamRed] != 3 || c[TeamBlue] != 3 {
		t.Fatalf("equipes com bots desequilibradas: %v", c)
	}
	g.leave(ana)
	g.leave(beto)
	if len(g.players) != 0 {
		t.Fatalf("sem ninguém de verdade, sem bots: %d", len(g.players))
	}
}

// Uma partida só de bots na vila: eles precisam andar, achar uns aos outros,
// atirar e derrubar, sem travar o servidor.
func TestBotsJogamNaVila(t *testing.T) {
	for _, mode := range []string{"ffa", "dom", "ctf"} {
		g := NewGame(BuildVila(), mode, 8)
		g.countdown = 10
		human := g.join("Observador", WAR)
		human.State = PhysState{X: 0, Y: 0, Z: -200} // fora do mapa, só olhando
		start := map[int]Vec3{}
		walked := map[int]float64{}
		ticks(g, 12)
		for _, p := range g.players {
			start[p.ID] = Vec3{p.State.X, 0, p.State.Z}
		}
		t0 := time.Now()
		const n = 90 * TickRate
		for i := 0; i < n; i++ {
			human.Alive = false
			human.RespawnAt = 1 << 62
			prev := map[int]Vec3{}
			for _, p := range g.players {
				prev[p.ID] = Vec3{p.State.X, 0, p.State.Z}
			}
			g.update()
			for _, p := range g.players {
				if p.Bot != nil && p.Alive {
					walked[p.ID] += math.Min(1, hdist(prev[p.ID], Vec3{p.State.X, 0, p.State.Z}))
				}
			}
		}
		per := time.Since(t0) / n
		kills, lazy := 0, 0
		for _, p := range g.players {
			if p.Bot == nil {
				continue
			}
			kills += p.Kills
			if walked[p.ID] < 30 {
				lazy++
			}
		}
		t.Logf("%-4s: 90 s de jogo, %d abates entre 7 bots, %d bots andaram pouco, %v por tick, placar %v",
			mode, kills, lazy, per.Round(time.Microsecond), g.score)
		if kills < 3 {
			t.Errorf("%s: bots quase não se enfrentaram (%d abates)", mode, kills)
		}
		if lazy > 1 {
			t.Errorf("%s: %d bots ficaram parados", mode, lazy)
		}
		if per > 4*time.Millisecond {
			t.Errorf("%s: tick lento demais: %v", mode, per)
		}
		if mode == "dom" && g.score[TeamRed]+g.score[TeamBlue] == 0 {
			t.Error("dom: os bots não seguraram nenhum ponto") // o placar só sobe com ponto dominado
		}
		if mode == "ctf" && g.flags[1].State == "base" && g.flags[2].State == "base" && g.score[1]+g.score[2] == 0 {
			t.Log("ctf: nenhuma bandeira saiu da base em 90 s")
		}
	}
}
