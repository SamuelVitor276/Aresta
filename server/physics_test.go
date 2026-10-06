package main

import (
	"math"
	"testing"
)

func run(s *PhysState, keys int, yaw float64, boxes []Box, ticks int) {
	for i := 0; i < ticks; i++ {
		Step(s, keys, yaw, boxes, 1)
	}
}

func TestCaiAteOChao(t *testing.T) {
	s := PhysState{Y: 5}
	run(&s, 0, 0, nil, 120)
	if s.Y != 0 || !s.OnGround {
		t.Fatalf("esperava no chão, ficou y=%.3f onGround=%v", s.Y, s.OnGround)
	}
}

func TestParedeBloqueia(t *testing.T) {
	wall := []Box{{Min: [3]float64{-5, 0, -3}, Max: [3]float64{5, 4, -2}}}
	s := PhysState{OnGround: true}
	run(&s, KeyForward, 0, wall, 180)
	if s.Z-PlayerHalfW < -2-1e-3 {
		t.Fatalf("atravessou a parede: z=%.3f", s.Z)
	}
}

func TestSobeEscadaDaArena(t *testing.T) {
	w := GenerateArena(1)
	s := PhysState{Z: 12, OnGround: true}
	run(&s, KeyForward, 0, w.Boxes, 100)
	if math.Abs(s.Y-2.5) > 1e-9 {
		t.Fatalf("não chegou ao topo da plataforma: y=%.3f z=%.3f", s.Y, s.Z)
	}
}

func TestAlturaDoPulo(t *testing.T) {
	s := PhysState{OnGround: true}
	maxY := 0.0
	Step(&s, KeyJump, 0, nil, 1)
	for i := 0; i < 120; i++ {
		Step(&s, 0, 0, nil, 1)
		maxY = math.Max(maxY, s.Y)
	}
	if maxY < 1.3 || maxY > 1.6 {
		t.Fatalf("altura do pulo fora do esperado: %.3f", maxY)
	}
}

func TestSniperMirandoAndaMaisDevagar(t *testing.T) {
	sn := NewWeapon(WSniper)
	a, b := PhysState{OnGround: true}, PhysState{OnGround: true}
	for i := 0; i < 120; i++ {
		Step(&a, KeyForward, 0, nil, MoveMul(&sn, KeyForward))
		Step(&b, KeyForward|KeyAim, 0, nil, MoveMul(&sn, KeyForward|KeyAim))
	}
	if -b.Z > -a.Z*0.6 {
		t.Fatalf("mirando deveria andar bem menos: %.1f m contra %.1f m", -b.Z, -a.Z)
	}
}

func TestPenteERecargaAutomatica(t *testing.T) {
	for kind := range Weapons {
		w := NewWeapon(kind)
		d := Weapons[kind]
		shots, seq := 0, 1
		for ; shots < d.Mag && seq < 10000; seq++ {
			if StepWeapon(&w, KeyFire, seq) == ActFire {
				shots++
			}
		}
		if w.Ammo != 0 {
			t.Fatalf("%s: esperava pente vazio, sobrou %d", d.Name, w.Ammo)
		}
		if r := StepWeapon(&w, KeyFire, seq); r != ActReload {
			t.Fatalf("%s: esperava recarga automática, veio %d", d.Name, r)
		}
		for i := 0; i <= d.Reload; i++ {
			seq++
			StepWeapon(&w, 0, seq)
		}
		if w.Ammo != d.Mag {
			t.Fatalf("%s: recarga não completou: %d", d.Name, w.Ammo)
		}
	}
}

func TestFacaEGranadaBloqueiamOTiroPorUmInstante(t *testing.T) {
	w := NewWeapon(WAR)
	if StepWeapon(&w, KeyMelee|KeyFire, 1) != ActMelee {
		t.Fatal("faca deveria ter prioridade")
	}
	if StepWeapon(&w, KeyFire, 2) != ActNone {
		t.Fatal("logo depois da facada não deveria atirar")
	}
	if StepWeapon(&w, KeyFire, 1+KnifeBusy) != ActFire {
		t.Fatal("depois do intervalo deveria atirar")
	}
	thrown := 0
	for seq := 100; seq < 400; seq++ {
		if StepWeapon(&w, KeyNade, seq) == ActNade {
			thrown++
		}
	}
	if thrown != NadeCount {
		t.Fatalf("esperava %d granadas, jogou %d", NadeCount, thrown)
	}
}

func TestSniperSemQuickscope(t *testing.T) {
	w := NewWeapon(WSniper)
	s := PhysState{OnGround: true}
	StepWeapon(&w, KeyAim, 10)
	early := ShotDirs(&w, 0, 0, KeyAim, 10, &s)[0]
	for seq := 11; seq <= 10+AimTime; seq++ {
		StepWeapon(&w, KeyAim, seq)
	}
	late := ShotDirs(&w, 0, 0, KeyAim, 10+AimTime, &s)[0]
	if math.Abs(late.X) > 1e-12 || math.Abs(late.Y) > 1e-12 {
		t.Fatalf("mirado há %d ticks deveria ser perfeito: %+v", AimTime, late)
	}
	if math.Abs(early.X)+math.Abs(early.Y) < 1e-4 {
		t.Fatal("mirada instantânea não deveria ser perfeita")
	}
}

func TestGranadaQuicaEPara(t *testing.T) {
	s := PhysState{OnGround: true}
	n := NadeLaunch(&s, 0, 0.3)
	for i := 0; i < NadeFuse; i++ {
		StepNade(&n, nil)
	}
	if n.Y > NadeHalf+1e-9 || n.Z > -5 || n.Z < -30 || math.Hypot(n.VX, n.VZ) > 0.3 {
		t.Fatalf("granada deveria ter parado no chão longe: %+v", n)
	}
}
