package main

import (
	"math"
	"math/rand"
)

// GenerateArena monta a arena pequena e aleatória (MAPA=arena): muros, plataforma central com escadas e
// obstáculos aleatórios espelhados pelo centro (o mapa fica justo para
// qualquer spawn). A mesma seed sempre gera o mesmo mapa.
func GenerateArena(seed int64) *World {
	rng := rand.New(rand.NewSource(seed))
	const half = 30.0
	w := &World{Name: "Arena", HalfX: half, HalfZ: half}

	// 1) Muros externos
	const wallH, th = 6.0, 1.0
	w.box(-half-th, 0, -half-th, half+th, wallH, -half, "wall", 0)
	w.box(-half-th, 0, half, half+th, wallH, half+th, "wall", 0)
	w.box(-half-th, 0, -half, -half, wallH, half, "wall", 0)
	w.box(half, 0, -half, half+th, wallH, half, "wall", 0)

	// 2) Plataforma central (2,5 m) com escadas ao norte e ao sul.
	//    Cada degrau sobe 0,5 m, abaixo do StepHeight, então dá pra subir andando.
	const ph, pw = 2.5, 4.0
	w.box(-pw, 0, -pw, pw, ph, pw, "platform", 0)
	for i := 0; i < 4; i++ {
		h := 0.5 * float64(i+1)
		z0 := pw + float64(3-i)*0.9
		w.box(-1.5, 0, z0, 1.5, h, z0+0.9, "stair", 0)
		w.box(-1.5, 0, -z0-0.9, 1.5, h, -z0, "stair", 0)
	}
	w.box(-3.3, ph, -0.7, -2.1, ph+1.1, 0.5, "crate", 0) // cobertura em cima da plataforma
	w.box(2.1, ph, -0.5, 3.3, ph+1.1, 0.7, "crate", 0)

	// 3) Spawns em círculo, olhando para o centro
	for i := 0; i < 8; i++ {
		a := float64(i)/8*2*math.Pi + math.Pi/8
		x, z := math.Cos(a)*(half-4), math.Sin(a)*(half-4)
		w.Spawns = append(w.Spawns, Spawn{X: x, Z: z, Yaw: math.Atan2(x, z)})
	}

	// 4) Objetivos (espelhados) e as áreas que os obstáculos precisam deixar livres
	for _, sp := range w.Spawns {
		if sp.X < 0 {
			w.TeamSpawns[TeamRed] = append(w.TeamSpawns[TeamRed], sp)
		} else {
			w.TeamSpawns[TeamBlue] = append(w.TeamSpawns[TeamBlue], sp)
		}
	}
	w.DomPoints = []Spot{{"A", -18, 0, 0, 3.5}, {"B", 0, ph, 0, 4}, {"C", 18, 0, 0, 3.5}}
	w.Hills = []Spot{{"Plataforma", 0, ph, 0, 4}, {"Noroeste", -18, 0, -18, 4}, {"Sudeste", 18, 0, 18, 4},
		{"Sudoeste", -18, 0, 18, 4}, {"Nordeste", 18, 0, -18, 4}}
	w.FlagHome[TeamRed] = Spot{"Vermelha", -25, 0, 0, 1.6}
	w.FlagHome[TeamBlue] = Spot{"Azul", 25, 0, 0, 1.6}
	w.keep = append(append(append([]Spot{}, w.DomPoints...), w.Hills...), w.FlagHome[1], w.FlagHome[2])

	// 5) Obstáculos aleatórios, cada um com um gêmeo espelhado em (-x, -z)
	reserved := Box{Min: [3]float64{-6, 0, -9.5}, Max: [3]float64{6, 9, 9.5}}
	placed := 0
	for tries := 0; placed < 14 && tries < 800; tries++ {
		var sx, sz, h float64
		var kind string
		switch r := rng.Float64(); {
		case r < 0.45: // caixote (os baixos dá pra subir pulando)
			e := 1.0 + rng.Float64()*1.3
			sx, sz, h, kind = e/2, e/2, e, "crate"
		case r < 0.7: // pilar alto
			s := 0.6 + rng.Float64()*0.5
			sx, sz, h, kind = s, s, 3.5+rng.Float64()*2.5, "pillar"
		default: // mureta de cobertura
			l := 2.0 + rng.Float64()*1.8
			if rng.Intn(2) == 0 {
				sx, sz = l, 0.3
			} else {
				sx, sz = 0.3, l
			}
			h, kind = 1.2+rng.Float64()*0.4, "cover"
		}
		cx := (rng.Float64()*2 - 1) * (half - 3 - sx)
		cz := (rng.Float64()*2 - 1) * (half - 3 - sz)
		a := Box{Min: [3]float64{cx - sx, 0, cz - sz}, Max: [3]float64{cx + sx, h, cz + sz}, Kind: kind}
		b := Box{Min: [3]float64{-cx - sx, 0, -cz - sz}, Max: [3]float64{-cx + sx, h, -cz + sz}, Kind: kind}
		if !w.free(a, reserved) || !w.free(b, reserved) || near(a, b, 1.5) {
			continue
		}
		w.Boxes = append(w.Boxes, a, b)

		// Às vezes empilha um caixote menor em cima (vira um mirante)
		if kind == "crate" && h <= 1.4 && rng.Float64() < 0.5 {
			e2 := sx * 2 * 0.55
			ox := (sx*2 - e2) / 2 * float64(1-2*rng.Intn(2))
			oz := (sz*2 - e2) / 2 * float64(1-2*rng.Intn(2))
			w.box(cx+ox-e2/2, h, cz+oz-e2/2, cx+ox+e2/2, h+e2, cz+oz+e2/2, "crate", 0)
			w.box(-cx-ox-e2/2, h, -cz-oz-e2/2, -cx-ox+e2/2, h+e2, -cz-oz+e2/2, "crate", 0)
		}
		placed++
	}
	return w
}

// near diz se duas caixas estão a menos de m metros uma da outra (no plano XZ).
func near(a, b Box, m float64) bool {
	return a.Max[0]+m > b.Min[0] && a.Min[0]-m < b.Max[0] &&
		a.Max[2]+m > b.Min[2] && a.Min[2]-m < b.Max[2]
}

// free garante corredores livres: longe do centro, dos outros blocos e dos spawns.
func (w *World) free(b Box, reserved Box) bool {
	const m = 1.4 // largura mínima de corredor
	if near(b, reserved, m) {
		return false
	}
	for _, o := range w.Boxes {
		if o.Kind != "wall" && near(b, o, m) {
			return false
		}
	}
	for _, s := range w.Spawns {
		dx := math.Max(0, math.Max(b.Min[0]-s.X, s.X-b.Max[0]))
		dz := math.Max(0, math.Max(b.Min[2]-s.Z, s.Z-b.Max[2]))
		if dx*dx+dz*dz < 9 {
			return false
		}
	}
	for _, k := range w.keep {
		dx := math.Max(0, math.Max(b.Min[0]-k.X, k.X-b.Max[0]))
		dz := math.Max(0, math.Max(b.Min[2]-k.Z, k.Z-b.Max[2]))
		if k.Y == 0 && dx*dx+dz*dz < (k.R+1)*(k.R+1) {
			return false
		}
	}
	return true
}
