package main

import (
	"math"
	"sort"
	"testing"
)

// Cenários para medir quão "humanos" os bots são. Os números aparecem com
// go test -v -run TestBotHabilidade. Cada função roda várias tentativas,
// porque os bots têm sorteio de reação e erro de mira.

func duel(w *World, kind int) (*Game, *Player, *Player) {
	g := NewGame(w, "ffa", 0)
	g.countdown = 1 << 40
	target := g.join("Alvo", WAR)
	bot := g.addBot()
	bot.NextKind = kind
	g.spawn(bot)
	bot.Weapon = NewWeapon(kind)
	return g, bot, target
}

func openWorld() *World {
	return &World{Name: "aberto", HalfX: 40, HalfZ: 40, Spawns: []Spawn{{X: -30, Z: -30}, {X: 30, Z: 30}}}
}

// hold mantém o bot parado no lugar (ele ainda mira e atira à vontade).
func hold(bot *Player, x, z float64) { bot.State = PhysState{X: x, Z: z, OnGround: true} }

// Janela: parede em z = 0 com uma janela em x = 0. O bot, a 6 m da parede,
// está olhando para ela e nunca viu o alvo. O alvo passa correndo 10 m atrás
// da parede e fica visível ~0,45 s. Quantas vezes toma tiro?
func windowHitRate(trials int) float64 {
	hits := 0
	for i := 0; i < trials; i++ {
		w := openWorld()
		w.wallX(-20, 20, 0, 0.4, 0, 3, "house", 0, win(0, 1.2))
		g, bot, tg := duel(w, WAR)
		bot.Bot.reset(math.Pi)
		for x := -8.0; x < 8; x += MoveSpeed * DT {
			tg.State = PhysState{X: x, Z: 10, OnGround: true, VX: MoveSpeed}
			if bot.Bot.lastSeenAt == 0 { // segurando o ângulo até ver alguém
				bot.Bot.aimYaw, bot.Bot.aimPitch = math.Pi, 0
			}
			g.update()
			hold(bot, 0, -6)
		}
		if tg.HP < MaxHP {
			hits++
		}
	}
	return float64(hits) / float64(trials)
}

// Pilar: o bot já está trocando tiro com o alvo a 15 m. O alvo corre de lado,
// some atrás de um pilar por ~0,4 s e reaparece. Quanto tempo até o
// primeiro acerto depois que ele reaparece?
func pillarReacquire(trials int) (times []float64, hidden []float64) {
	for i := 0; i < trials; i++ {
		w := openWorld()
		w.blk(0, 7.5, 1.0, 1.0, 0, 3, "pillar", 0) // no meio do caminho
		g, bot, tg := duel(w, WAR)
		bot.Bot.reset(math.Pi)
		eye := Vec3{-3, EyeHeight, 0}
		x, hiddenAt, back := -6.0, -1, -1
		for t := 0; t < 5*TickRate; t++ {
			if t >= 90 { // depois de 1,5 s na mira, corre de lado
				x += MoveSpeed * DT
			}
			tg.State = PhysState{X: x, Z: 15, OnGround: true}
			vis := g.canSee(eye, tg)
			if t >= 90 && hiddenAt < 0 && !vis {
				hiddenAt = t
			}
			if hiddenAt >= 0 && back < 0 && vis {
				back = t
				hidden = append(hidden, float64(t-hiddenAt)/TickRate)
			}
			if back < 0 {
				tg.HP, tg.Alive = MaxHP, true // só conta depois que reaparece
			}
			hp := tg.HP
			g.update()
			hold(bot, -3, 0)
			if back >= 0 && tg.HP < hp {
				times = append(times, float64(t-back)/TickRate)
				break
			}
			if back >= 0 && t-back > 2*TickRate {
				times = append(times, 2)
				break
			}
		}
	}
	return
}

// Peek: o alvo sai de trás de uma parede a 15 m e fica parado à vista.
// Quanto tempo até o primeiro acerto?
func peek(trials int) []float64 {
	peekShots, peekHits = 0, 0
	var out []float64
	for i := 0; i < trials; i++ {
		w := openWorld()
		w.box(-10, 0, 13.5, -1, 3, 14.5, "house", 0) // parede escondendo o alvo
		g, bot, tg := duel(w, WAR)
		g.onEvent = func(e Event) {
			if e.T == "shot" && e.ID == bot.ID {
				peekShots++
			}
			if e.T == "hit" && e.ID == bot.ID {
				peekHits++
			}
		}
		bot.Bot.reset(math.Pi)
		res := 3.0
		for t := 0; t < 4*TickRate; t++ {
			x := -3.0
			if t >= 60 {
				x = 0.8 // saiu para a vista e parou
			}
			tg.State = PhysState{X: x, Z: 16, OnGround: true}
			if t < 60 {
				tg.HP = MaxHP
			}
			if bot.Bot.lastSeenAt == 0 { // segurando o ângulo da saída da parede
				bot.Bot.aimYaw, bot.Bot.aimPitch = math.Atan2(-0.8, -16), 0
			}
			hp := tg.HP
			g.update()
			hold(bot, 0, 0)
			if t >= 60 && tg.HP < hp {
				res = float64(t-60) / TickRate
				break
			}
		}
		out = append(out, res)
	}
	return out
}

var duelShots, duelHits, peekShots, peekHits int

// Duelo no aberto a 20 m, alvo andando de um lado para o outro.
func openDuel(trials int) (firstHit, kill []float64) {
	duelShots, duelHits = 0, 0
	for i := 0; i < trials; i++ {
		g, bot, tg := duel(openWorld(), WAR)
		g.onEvent = func(e Event) {
			if e.T == "shot" && e.ID == bot.ID {
				duelShots++
			}
			if e.T == "hit" && e.ID == bot.ID {
				duelHits++
			}
		}
		bot.Bot.reset(math.Pi)
		x, dir, first, start := 0.0, 1.0, -1.0, g.tick
		for t := 0; t < 10*TickRate && tg.Alive; t++ {
			if t%30 == 0 && t > 0 {
				dir = -dir
			}
			x += dir * MoveSpeed * DT
			tg.State = PhysState{X: x, Z: 20, OnGround: true, VX: dir * MoveSpeed}
			g.update()
			hold(bot, 0, 0)
			if first < 0 && tg.HP < MaxHP {
				first = float64(t) / TickRate
			}
		}
		if first < 0 {
			first = 10
		}
		k := 10.0
		if !tg.Alive {
			k = float64(g.tick-start) / TickRate
		}
		firstHit, kill = append(firstHit, first), append(kill, k)
	}
	return
}

// Escopeta a 4 m: quantos abates saem marcados como tiro na cabeça.
func shotgunHeadRate(trials int) float64 {
	heads, kills := 0, 0
	for i := 0; i < trials; i++ {
		g, bot, tg := duel(openWorld(), WShotgun)
		g.onEvent = func(e Event) {
			if e.T == "kill" && e.Target == tg.ID {
				kills++
				if e.Head {
					heads++
				}
			}
		}
		bot.Bot.reset(math.Pi)
		for t := 0; t < 6*TickRate && tg.Alive; t++ {
			tg.State = PhysState{X: 0, Z: 4, OnGround: true}
			g.update()
			hold(bot, 0, 0)
		}
	}
	if kills == 0 {
		return 0
	}
	return float64(heads) / float64(kills)
}

var pillarHidden float64

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

type skill struct{ window, reacq, first, kill, head, peek, acc float64 }

func measureSkill() skill {
	first, kill := openDuel(60)
	acc := float64(duelHits) / math.Max(1, float64(duelShots))
	reacq, hidden := pillarReacquire(60)
	pillarHidden = median(hidden)
	return skill{
		window: windowHitRate(100),
		reacq:  median(reacq),
		first:  median(first),
		kill:   median(kill),
		head:   shotgunHeadRate(60),
		peek:   median(peek(60)),
		acc:    acc,
	}
}

func TestBotHabilidade(t *testing.T) {
	s := measureSkill()
	t.Logf("janela (passa correndo, ~0,45 s visível): toma tiro em %.0f%% das vezes", s.window*100)
	t.Logf("pilar (some %.2f s e reaparece): volta a acertar em %.2f s (mediana)", pillarHidden, s.reacq)
	t.Logf("duelo a 20 m: primeiro acerto em %.2f s, abate em %.2f s (medianas)", s.first, s.kill)
	t.Logf("peek a 15 m (sai da parede e para): primeiro acerto em %.2f s (mediana)", s.peek)
	t.Logf("pontaria no duelo a 20 m: %.0f%% dos tiros acertam", s.acc*100)
	t.Logf("escopeta a 4 m: %.0f%% dos abates marcados como tiro na cabeça", s.head*100)
}
