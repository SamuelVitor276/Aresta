package main

import (
	"fmt"
	"math"
	"testing"
)

// Arena vazia, sem obstáculos, para testar tiros em linha reta
func newTestGame() *Game {
	g := NewGame(GenerateArena(1), "ffa", 0)
	g.world.Boxes = nil
	g.countdown = 1 << 40 // a partida nunca começa: os testes controlam tudo
	return g
}

func addTestPlayer(g *Game, id, kind int, x, z float64) *Player {
	p := &Player{ID: id, Name: fmt.Sprint("p", id), NextKind: kind, Vote: -1, send: make(chan []byte, 4096)}
	g.players[id] = p
	g.spawn(p)
	p.State = PhysState{X: x, Z: z, OnGround: true}
	p.Yaw = 0
	return p
}

func aimAt(h, dist float64) float64 { return math.Atan2(h-EyeHeight, dist) }

func shoot(g *Game, p *Player, seq, keys int, pitch, rt float64) {
	p.inputs = append(p.inputs, Input{Seq: seq, Keys: keys, Pitch: pitch, RT: rt})
	g.update()
}

func TestFuzilCorpoECabeca(t *testing.T) {
	g := newTestGame()
	a := addTestPlayer(g, 1, WAR, 0, 10)
	b := addTestPlayer(g, 2, WAR, 0, 0)
	// mirando (sem dispersão relevante) a 10 m
	for s := 1; s <= AimTime; s++ {
		shoot(g, a, s, KeyAim, 0, 0)
	}
	shoot(g, a, AimTime+1, KeyFire|KeyAim, aimAt(1.0, 10), 0)
	if b.HP != MaxHP-24 {
		t.Fatalf("peito: esperava %d, ficou %d", MaxHP-24, b.HP)
	}
	shoot(g, a, AimTime+1+Weapons[WAR].FireInterval, KeyFire|KeyAim, aimAt(1.65, 10), 0)
	if b.HP != MaxHP-24-55 {
		t.Fatalf("cabeça: esperava %d, ficou %d", MaxHP-24-55, b.HP)
	}
}

func TestEscopetaDePertoMataDeLongeNao(t *testing.T) {
	g := newTestGame()
	a := addTestPlayer(g, 1, WShotgun, 0, 3)
	b := addTestPlayer(g, 2, WAR, 0, 0)
	shoot(g, a, 1, KeyFire, aimAt(1.0, 3), 0)
	if b.Alive {
		t.Fatalf("a 3 m a escopeta deveria derrubar; vida %d", b.HP)
	}

	g2 := newTestGame()
	a2 := addTestPlayer(g2, 1, WShotgun, 0, 25)
	b2 := addTestPlayer(g2, 2, WAR, 0, 0)
	shoot(g2, a2, 1, KeyFire, aimAt(1.0, 25), 0)
	if b2.HP < 75 {
		t.Fatalf("a 25 m a escopeta deveria fazer pouco dano; vida %d", b2.HP)
	}
}

func TestSniperNaCabecaDerruba(t *testing.T) {
	g := newTestGame()
	a := addTestPlayer(g, 1, WSniper, 0, 60)
	b := addTestPlayer(g, 2, WAR, 0, 0)
	for s := 1; s <= AimTime; s++ {
		shoot(g, a, s, KeyAim, 0, 0)
	}
	shoot(g, a, AimTime+1, KeyFire|KeyAim, aimAt(1.65, 60), 0)
	if b.Alive {
		t.Fatalf("headshot de sniper deveria derrubar; vida %d", b.HP)
	}
}

func TestFacaDeFrenteEPelasCostas(t *testing.T) {
	g := newTestGame()
	a := addTestPlayer(g, 1, WAR, 0, 1.5)
	b := addTestPlayer(g, 2, WAR, 0, 0)
	b.Yaw = math.Pi // B olha para +Z, de frente para A
	shoot(g, a, 1, KeyMelee, 0, 0)
	if b.HP != MaxHP-KnifeDamage {
		t.Fatalf("facada de frente: esperava %d, ficou %d", MaxHP-KnifeDamage, b.HP)
	}
	g2 := newTestGame()
	a2 := addTestPlayer(g2, 1, WAR, 0, 1.5)
	b2 := addTestPlayer(g2, 2, WAR, 0, 0)
	b2.Yaw = 0 // B olha para −Z: A está nas costas dele
	shoot(g2, a2, 1, KeyMelee, 0, 0)
	if b2.Alive {
		t.Fatalf("facada pelas costas deveria derrubar; vida %d", b2.HP)
	}
}

func TestGranadaParedeProtegeEDanoProprio(t *testing.T) {
	g := newTestGame()
	a := addTestPlayer(g, 1, WAR, 0, 0)
	b := addTestPlayer(g, 2, WAR, 3, 0)
	c := addTestPlayer(g, 3, WAR, -3, 0)
	// parede entre a granada e C
	g.world.Boxes = []Box{{Min: [3]float64{-2, 0, -3}, Max: [3]float64{-1.6, 3, 3}}}
	n := &Nade{ID: 1, Owner: a.ID, X: 0.5, Y: NadeHalf, Z: 0, Rest: true, Explode: g.tick + 1}
	g.nades = append(g.nades, n)
	g.update()
	if b.HP >= MaxHP || a.HP >= MaxHP {
		t.Fatalf("B e o próprio A deveriam tomar dano: A=%d B=%d", a.HP, b.HP)
	}
	if c.HP != MaxHP {
		t.Fatalf("a parede deveria proteger C; vida %d", c.HP)
	}
}

func TestCompensacaoDeLag(t *testing.T) {
	g := newTestGame()
	a := addTestPlayer(g, 1, WSniper, 0, 10)
	b := addTestPlayer(g, 2, WAR, 0, 0)
	for s := 1; s <= AimTime; s++ {
		shoot(g, a, s, KeyAim, 0, 0)
	}
	seen := float64(g.tick)
	b.State.X = 3
	for i := 0; i < 5; i++ {
		g.update()
	}
	shoot(g, a, AimTime+1, KeyFire|KeyAim, aimAt(1.0, 10), seen)
	if b.HP != MaxHP-80 {
		t.Fatalf("com compensação de lag deveria acertar; vida %d", b.HP)
	}
}

func TestAbateERespawnComArmaNova(t *testing.T) {
	g := newTestGame()
	a := addTestPlayer(g, 1, WShotgun, 0, 2)
	b := addTestPlayer(g, 2, WAR, 0, 0)
	b.NextKind = WSniper
	shoot(g, a, 1, KeyFire, aimAt(1.0, 2), 0)
	if b.Alive || a.Kills != 1 || b.Deaths != 1 {
		t.Fatalf("esperava abate: vivo=%v kills=%d deaths=%d", b.Alive, a.Kills, b.Deaths)
	}
	for i := 0; i < RespawnTicks; i++ {
		g.update()
	}
	if !b.Alive || b.HP != MaxHP || b.Weapon.Kind != WSniper || b.Weapon.Nades != NadeCount {
		t.Fatalf("respawn com a arma escolhida: vivo=%v vida=%d arma=%d granadas=%d", b.Alive, b.HP, b.Weapon.Kind, b.Weapon.Nades)
	}
}
