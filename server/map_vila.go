package main

import (
	"math"
	"math/rand"
)

// Vila Poente: mapa feito à mão. Cidade murada no fim de tarde.
//
//	norte: igreja com torre do sino (sniper), armazém com telhado (sniper)
//	centro: praça aberta com fonte, carros e barracas (fuzil)
//	oeste: pátio de containers com corredores longos (fuzil / sniper)
//	leste: becos e casas com interior, sobrados com janelas (escopeta)
//	sul: ruínas e escombros (escopeta), jardim de oliveiras
//	cantos NE e SO: bastiões da muralha a 7 m (sniper)
//
// Norte = z negativo, leste = x positivo. Tudo é feito de caixas.
func BuildVila() *World {
	w := &World{Name: "Vila Poente", HalfX: 52, HalfZ: 42}
	vilaMuralha(w)
	vilaPraca(w)
	vilaIgreja(w)
	vilaNordeste(w)
	vilaArmazem(w)
	vilaContainers(w)
	vilaCasasOeste(w)
	vilaBecos(w)
	vilaRuinas(w)
	vilaJardim(w)
	vilaSpawns(w)
	vilaObjetivos(w)
	w.Zones = []Zone{
		{-15, -21, 15, 12, "paving"},    // praça e adro da igreja
		{-28, -42, 44, -38, "cobble"},   // rua de trás
		{15, -22, 19, 27, "cobble"},     // rua leste
		{-18, -21, -15, 16, "cobble"},   // rua oeste
		{-24, 12, 19, 16, "cobble"},     // rua sul
		{-52, -22, -24, 22, "concrete"}, // pátio de containers
		{20, 27, 52, 42, "grass"},       // jardim
		{-38, 16, 14, 42, "dirt"},       // ruínas
	}
	return w
}

// ---------------------------------------------------------------- pecinhas

func (w *World) car(cx, cz float64, alongZ bool, v int) {
	if alongZ {
		w.blk(cx, cz, 1.9, 4.2, 0, 1.0, "car", v)
		w.blk(cx, cz-0.3, 1.7, 2.2, 1.0, 0.65, "carcab", v)
	} else {
		w.blk(cx, cz, 4.2, 1.9, 0, 1.0, "car", v)
		w.blk(cx-0.3, cz, 2.2, 1.7, 1.0, 0.65, "carcab", v)
	}
}

// stall: barraca de feira com toldo (o toldo também é sólido).
func (w *World) stall(cx, cz float64, alongZ bool, v int) {
	sx, sz := 2.4, 1.1
	if alongZ {
		sx, sz = sz, sx
	}
	w.blk(cx, cz, sx, sz, 0, 0.95, "stall", v)
	w.blk(cx-sx/2+0.06, cz-sz/2+0.06, 0.12, 0.12, 0, 2.5, "post", 0)
	w.blk(cx+sx/2-0.06, cz+sz/2-0.06, 0.12, 0.12, 0, 2.5, "post", 0)
	w.blk(cx, cz, sx+0.5, sz+0.5, 2.5, 0.08, "awning", v)
}

func (w *World) palm(x, z float64) {
	w.blk(x, z, 1.6, 1.6, 0, 0.5, "planter", 0)
	w.blk(x, z, 0.35, 0.35, 0.5, 4.0, "trunk", 0)
}

func (w *World) olive(x, z float64) { w.blk(x, z, 0.35, 0.35, 0, 2.4, "trunk", 1) }

func (w *World) sandbags(x0, z0, x1, z1 float64) {
	w.box(x0, 0, z0, x1, 1.15, z1, "sandbag", 0)
}

func (w *World) container(x0, z0, x1, z1, y0 float64, v int) {
	w.box(x0, y0, z0, x1, y0+2.6, z1, "container", v)
}

// mound: monte de escombros em camadas de 0,5 m (dá pra subir andando).
func (w *World) mound(cx, cz, sx, sz float64, layers int, seed int64) {
	r := rand.New(rand.NewSource(seed))
	for i := 0; i < layers; i++ {
		k := 1 - 0.24*float64(i)
		jx, jz := (r.Float64()-0.5)*0.6, (r.Float64()-0.5)*0.6
		w.blk(cx+jx*float64(i), cz+jz*float64(i), sx*k, sz*k, 0.5*float64(i), 0.5, "rubble", i%2)
	}
}

// ruinX / ruinZ: parede quebrada, topo irregular, com rombos.
func (w *World) ruinX(x0, x1, z, t, hmin, hmax float64, seed int64, gaps ...op) {
	r := rand.New(rand.NewSource(seed))
	spans(x0, x1, 1, gaps, func(a, b, lo, hi float64) {
		if lo > 0 || hi < 1 {
			return
		}
		for x := a; x < b-0.01; x += 1.1 {
			h := hmin + (hmax-hmin)*r.Float64()
			w.box(x, 0, z-t/2, math.Min(x+1.1, b), h, z+t/2, "ruin", 0)
		}
	})
}

func (w *World) ruinZ(z0, z1, x, t, hmin, hmax float64, seed int64, gaps ...op) {
	r := rand.New(rand.NewSource(seed))
	spans(z0, z1, 1, gaps, func(a, b, lo, hi float64) {
		if lo > 0 || hi < 1 {
			return
		}
		for z := a; z < b-0.01; z += 1.1 {
			h := hmin + (hmax-hmin)*r.Float64()
			w.box(x-t/2, 0, z, x+t/2, h, math.Min(z+1.1, b), "ruin", 0)
		}
	})
}

// ---------------------------------------------------------------- áreas

func vilaMuralha(w *World) {
	// 10 m de altura: ninguém sobe, ninguém sai
	w.box(-54, 0, -44, 54, 10, -42, "rampart", 0)
	w.box(-54, 0, 42, 54, 10, 44, "rampart", 0)
	w.box(-54, 0, -42, -52, 10, 42, "rampart", 0)
	w.box(52, 0, -42, 54, 10, 42, "rampart", 0)

	// Bastião NE: plataforma a 7 m, escada encostada na muralha norte
	w.box(44, 0, -42, 52, 7, -34, "rampart", 1)
	w.box(44, 7, -40.6, 44.35, 8, -34, "rampart", 1)
	w.box(44, 7, -34.35, 52, 8, -34, "rampart", 1)
	w.stairs("bastião NE", 35.6, -42, 'E', 1.4, 28, 0.25, 0.3, 0, 0, "stair", 1)

	// Bastião SO: espelhado, escada encostada na muralha sul
	w.box(-52, 0, 34, -44, 7, 42, "rampart", 1)
	w.box(-44.35, 7, 34, -44, 8, 40.6, "rampart", 1)
	w.box(-52, 7, 34, -44, 8, 34.35, "rampart", 1)
	w.stairs("bastião SO", -35.6, 40.6, 'W', 1.4, 28, 0.25, 0.3, 0, 0, "stair", 1)
}

func vilaPraca(w *World) {
	// Fonte no centro: a borda tem 0,5 m (sobe andando)
	w.blk(0, 1, 5, 5, 0, 0.5, "basin", 0)
	w.blk(0, 1, 0.9, 0.9, 0.5, 1.6, "stone", 1)
	w.blk(0, 1, 2.2, 2.2, 2.1, 0.35, "stone", 0)

	w.car(-8, -5, false, 0)
	w.car(8.5, 6, true, 1)

	// Trincheiras de sacos de areia em L
	w.sandbags(4.4, -6.85, 7.6, -6.15)
	w.sandbags(6.9, -6.15, 7.6, -4.0)
	w.sandbags(-7.6, 6.15, -4.4, 6.85)
	w.sandbags(-7.6, 4.0, -6.9, 6.15)

	// Barracas de feira nas bordas
	w.stall(-12.5, -6, true, 0)
	w.stall(-12.5, 0, true, 1)
	w.stall(-12.5, 6, true, 2)
	w.stall(12.5, -3, true, 1)
	w.stall(12.5, 3, true, 0)

	// Postes e palmeiras
	for _, p := range [][2]float64{{-13.6, -9.2}, {13.6, -9.2}, {-13.6, 11.2}, {13.6, 11.2}} {
		w.blk(p[0], p[1], 0.25, 0.25, 0, 4.2, "lamp", 0)
	}
	w.palm(-4.5, -8.5)
	w.palm(4.5, 10.5)
	w.palm(-4.5, 10.5)

	// Adro da igreja: oliveiras em canteiros e um degrau largo na porta
	for _, p := range [][2]float64{{-9, -15}, {9, -15}, {-4, -18.5}, {4, -18.5}} {
		w.blk(p[0], p[1], 2.0, 2.0, 0, 0.5, "planter", 0)
		w.blk(p[0], p[1], 0.35, 0.35, 0.5, 2.4, "trunk", 1)
	}
	w.blk(0, -21.2, 6, 1.6, 0, 0.25, "stone", 1)
	w.blk(-12.5, -14, 0.6, 3.0, 0, 1.1, "stone", 0) // muretas
	w.blk(12.5, -14, 0.6, 3.0, 0, 1.1, "stone", 0)
}

func vilaIgreja(w *World) {
	// Nave: 18 × 16 m, paredes de 7 m, janelas altas (só para tiro)
	high := func(c float64) op { return hole(c, 1.4, 3.6, 5.4) }
	w.room(-9, -38, 9, -22, 0, 7, 0.6, "stone", 2, sides{
		n: []op{high(-4), high(4)},
		s: []op{tallDoor(0, 3.2, 3.8), high(-5), high(5)},
		w: []op{door(-27.2, 1.6), high(-33)},
		e: []op{door(-25.2, 1.6), high(-33)},
	})
	w.slab(-9, -38, 9, -22, 7.4, 0.4, "roof", 2)
	w.box(-4, 0, -37.4, 4, 0.5, -34.5, "stone", 1) // altar
	for r := 0; r < 5; r++ {                       // bancos
		z := -32.5 + 2.0*float64(r)
		w.box(-7.2, 0, z, -1.4, 0.9, z+0.6, "pew", 0)
		w.box(1.4, 0, z, 7.2, 0.9, z+0.6, "pew", 0)
	}

	// Torre do sino: 7 × 7 m, campanário a 7,5 m. Dois lances internos.
	w.room(9, -38, 16, -31, 0, 7.5, 0.5, "stone", 1, sides{s: []op{tallDoor(10.25, 1.4, 3.4)}}) // porta alta: a escada começa logo atrás
	w.stairs("torre (1º lance)", 9.5, -31.5, 'N', 1.5, 15, 0.25, 0.3, 0, 0, "stair", 0)
	w.box(9.5, 0, -37.5, 12.5, 3.75, -36.0, "stair", 0) // patamar
	w.stairs("torre (2º lance)", 11.0, -36.0, 'S', 1.5, 15, 0.25, 0.3, 0, 3.75, "stair", 0)
	w.box(12.5, 0, -37.5, 15.5, 7.5, -31.5, "stone", 1) // miolo maciço
	w.box(9.5, 7.2, -37.5, 11.0, 7.5, -31.5, "stone", 0)
	w.box(11.0, 7.2, -37.5, 12.5, 7.5, -36.0, "stone", 0)
	// campanário: pilares, mureta de 1 m (dá cobertura) e teto
	for _, c := range [][2]float64{{9, -38}, {15.2, -38}, {9, -31.8}, {15.2, -31.8}} {
		w.box(c[0], 7.5, c[1], c[0]+0.8, 10, c[1]+0.8, "stone", 1)
	}
	w.box(9.8, 7.5, -38, 15.2, 8.5, -37.7, "stone", 1)
	w.box(9.8, 7.5, -31.3, 15.2, 8.5, -31, "stone", 1)
	w.box(9, 7.5, -37.2, 9.3, 8.5, -31.8, "stone", 1)
	w.box(15.7, 7.5, -37.2, 16, 8.5, -31.8, "stone", 1)
	w.slab(9, -38, 16, -31, 10.4, 0.4, "belfry", 1)
}

func vilaNordeste(w *World) {
	// Sobrado NE: janelas do 1º andar olham o adro e a praça
	w.sobrado(20, -37, 30, -27, 1,
		sides{n: []op{win(25, 1.2)}, s: []op{door(28.5, 1.4)}, w: []op{door(-31, 1.4), win(-34, 1.2)}, e: []op{win(-32, 1.2)}},
		sides{n: []op{win(23, 1.2), win(27, 1.2)}, s: []op{door(21.5, 1.4), win(25, 1.4), win(28, 1.4)},
			w: []op{win(-33, 1.2), win(-30, 1.2)}, e: []op{win(-34, 1.2), win(-30, 1.2)}})
	w.stairs("sobrado NE", 27.0, -27, 'W', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)
	w.box(20.6, 0, -27, 22.2, 4.0, -25.6, "stair", 0)

	// Casinha NE com terraço (escada externa no lado leste)
	w.casa(33, -30, 40, -22, 3,
		sides{n: []op{win(36.5, 1.2)}, s: []op{door(36.5, 1.4)}, w: []op{door(-26, 1.4)}, e: []op{win(-28.5, 1.0)}},
		sides{e: []op{hole(-26.7, 1.6, 0, 1)}})
	w.stairs("casa NE (terraço)", 40, -22.0, 'N', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)

	// Caixotes e barris na rua de trás
	w.blk(-12, -40.5, 1.2, 1.2, 0, 1.2, "crate", 0)
	w.blk(18, -40.2, 1.4, 1.4, 0, 1.25, "crate", 0)
	w.blk(31.5, -40.4, 1.0, 1.0, 0, 1.0, "crate", 0)
	w.blk(39, -32.5, 1.3, 1.3, 0, 1.25, "crate", 0)
}

func vilaArmazem(w *World) {
	// Galpão de 20 × 14 m, 7 m de altura, telhado acessível por fora
	w.room(-48, -38, -28, -24, 0, 7.0, 0.5, "metal", 0, sides{
		s: []op{tallDoor(-32, 4.0, 4.5)},
		w: []op{door(-27.5, 1.6)},
		e: []op{door(-30, 2.0), win(-35, 2.0)},
	})
	w.slab(-48, -38, -28, -24, 7.25, 0.25, "roof", 3)
	w.parapet(-48, -38, -28, -24, 7.25, 0.8, "metal", 1, sides{s: []op{hole(-39.0, 1.6, 0, 1)}})
	w.stairs("armazém (telhado)", -47.5, -24, 'E', 1.4, 29, 0.25, 0.3, 0, 0, "stair", 1)

	// Mezanino ao longo da parede norte, com escada e guarda-corpo
	w.box(-47.5, 3.25, -37.5, -28.5, 3.5, -35.0, "metal", 1)
	w.box(-46.0, 3.5, -35.1, -28.5, 4.5, -35.0, "rail", 0)
	w.stairs("armazém (mezanino)", -47.5, -30.8, 'N', 1.5, 14, 0.25, 0.3, 0, 0, "stair", 1)

	// Prateleiras e caixotes: corredores de combate de perto
	w.box(-42.5, 0, -33.5, -41.5, 3.0, -27.5, "shelf", 0)
	w.box(-36.5, 0, -33.5, -35.5, 3.0, -27.5, "shelf", 0)
	w.blk(-39, -26, 1.2, 1.2, 0, 1.2, "crate", 0)
	w.blk(-32, -33.5, 1.4, 1.4, 0, 1.4, "crate", 0)
	w.blk(-44.5, -26.5, 1.3, 1.3, 0, 1.25, "crate", 0)
	w.blk(-29.8, -27.5, 1.1, 1.1, 0, 1.1, "crate", 0)
}

func vilaContainers(w *World) {
	// Quatro colunas de containers com corredores de 4,2 m (linhas longas)
	c1, c2, c3, c4 := [2]float64{-50.0, -47.6}, [2]float64{-43.4, -41.0}, [2]float64{-36.8, -34.4}, [2]float64{-30.2, -27.8}
	w.container(c1[0], -20, c1[1], -8, 0, 0)
	w.container(c1[0], -4, c1[1], 2, 0, 3)
	w.container(c1[0], -4, c1[1], 2, 2.6, 1)
	w.container(c1[0], 6, c1[1], 18, 0, 2)

	w.container(c2[0], -19, c2[1], -13, 0, 4)
	w.container(c2[0], -9, c2[1], 3, 0, 1)
	w.container(c2[0], 7, c2[1], 13, 0, 0)
	w.container(c2[0], 7, c2[1], 13, 2.6, 3)
	w.container(c2[0], 15, c2[1], 21, 0, 2)

	w.container(c3[0], -20, c3[1], -8, 0, 2)
	w.container(c3[0], -20, c3[1], -14, 2.6, 0) // pilha: mirante a 5,2 m
	w.container(c3[0], -4, c3[1], 2, 0, 4)
	w.container(c3[0], 5, c3[1], 17, 0, 3)

	w.container(c4[0], -18, c4[1], -12, 0, 1)
	w.container(c4[0], -7, c4[1], 5, 0, 0)
	w.container(c4[0], 9, c4[1], 15, 0, 2)
	w.container(c4[0], 9, c4[1], 15, 2.6, 4)

	// Escada de ferro até o mirante da pilha (com patamar ao lado dela)
	w.stairs("containers (mirante)", -38.3, -8.0, 'N', 1.5, 20, 0.26, 0.3, 0, 0, "stair", 1)
	w.box(-38.3, 0, -16.0, -36.8, 5.2, -14.0, "stair", 1)

	// Caixotes para pular em cima dos containers (1,25 m → 2,6 m)
	w.blk(-44.0, -5.0, 1.2, 1.2, 0, 1.25, "crate", 0)
	w.blk(-33.8, 8.0, 1.2, 1.2, 0, 1.25, "crate", 0)
	w.blk(-48.8, 20.0, 1.2, 1.2, 0, 1.25, "crate", 0)
	w.blk(-31.0, -10.0, 1.2, 1.2, 0, 1.25, "crate", 0)
}

func vilaCasasOeste(w *World) {
	// Três casas de frente para a praça, com becos de 4 m entre elas
	w.casa(-24, -19, -18, -11, 0,
		sides{s: []op{door(-21, 1.4)}, w: []op{door(-15, 1.4)}, e: []op{win(-17, 1.2), win(-13, 1.2)}},
		sides{n: []op{hole(-23.2, 1.6, 0, 1)}})
	w.stairs("casa O1 (terraço)", -18.6, -20.4, 'W', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)

	w.sobrado(-24, -7, -18, 4, 4,
		sides{n: []op{door(-21, 1.4)}, s: []op{door(-21, 1.4)}, w: []op{win(-1, 1.2)}, e: []op{door(-1.5, 1.6), win(-5, 1.2), win(2, 1.2)}},
		sides{n: []op{win(-21, 1.2)}, s: []op{win(-21, 1.2)}, w: []op{door(-5.5, 1.4), win(1, 1.2)}, e: []op{win(-5, 1.4), win(-1.5, 1.4), win(2, 1.4)}})
	w.stairs("sobrado O", -25.4, 0.1, 'N', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)
	w.box(-25.4, 0, -6.3, -24, 4.0, -4.7, "stair", 0)

	w.casa(-24, 8, -18, 16, 2,
		sides{n: []op{door(-21, 1.4)}, s: []op{win(-21, 1.2)}, w: []op{win(12, 1.2)}, e: []op{door(10, 1.4), win(14, 1.2)}},
		sides{s: []op{hole(-19.1, 1.6, 0, 1)}})
	w.stairs("casa O3 (terraço)", -23.6, 16, 'E', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)
}

func vilaBecos(w *World) {
	// Colunas de casas x ∈ [19,27], [30,38], [41,50]; becos de 3 m entre elas.
	// A1: casa com parede interna (dois cômodos)
	w.casa(19, -18, 27, -10, 2,
		sides{n: []op{win(23, 1.2)}, s: []op{door(21, 1.4)}, w: []op{door(-14, 1.4)}, e: []op{door(-15.5, 1.4), win(-12, 1.2)}},
		sides{})
	w.wallZ(-17.6, -10.4, 23, 0.3, 0, 3.7, "house", 2, door(-14, 1.2))
	w.blk(21, -12, 1.4, 0.9, 0, 0.8, "wood", 0)

	// B1: quintal murado (2,4 m: não dá pra ver nem pular por cima)
	w.room(30, -18, 38, -10, 0, 2.4, 0.4, "yard", 0, sides{n: []op{door(34, 2.0)}, s: []op{door(32, 1.6)}, e: []op{door(-13, 1.6)}})
	w.blk(34, -14, 1.6, 1.6, 0, 0.9, "stone", 0) // poço
	w.blk(36.2, -11.8, 1.1, 1.1, 0, 1.1, "crate", 0)

	// C1: sobrado com escada externa no lado norte
	w.sobrado(41, -18, 50, -7, 4,
		sides{n: []op{door(42.0, 1.4)}, s: []op{win(45.5, 1.2)}, w: []op{door(-12.5, 1.4), win(-16, 1.2)}},
		sides{n: []op{door(44.0, 1.4)}, s: []op{win(43.5, 1.2), win(47.5, 1.2)}, w: []op{win(-15.5, 1.2), win(-10, 1.2)}})
	w.stairs("sobrado L1", 49.6, -19.4, 'W', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)
	w.box(43.2, 0, -19.4, 44.8, 4.0, -18, "stair", 0)

	// A2: casa de passagem (porta dos dois lados) com terraço
	w.casa(19, -7, 27, 1, 0,
		sides{n: []op{win(23, 1.2)}, w: []op{door(-3, 1.6), win(-5.5, 1.0)}, e: []op{door(-3, 1.6)}},
		sides{s: []op{hole(24.0, 1.6, 0, 1)}})
	w.stairs("casa L2 (terraço)", 19.4, 1.0, 'E', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)

	// B2: casa grande com dois cômodos e corredor
	w.casa(30, -7, 38, 4, 3,
		sides{n: []op{door(32, 1.4)}, s: []op{door(36, 1.4)}, w: []op{win(-4, 1.2), win(1, 1.2)}, e: []op{door(-1.5, 1.4)}},
		sides{})
	w.wallX(30.4, 37.6, 1.0, 0.3, 0, 3.7, "house", 3, door(34, 1.2))
	w.blk(32, 1.8, 1.2, 1.2, 0, 1.2, "crate", 0)

	// C2: casa em ruínas, sem teto
	w.ruinX(41, 50, -4.2, 0.4, 1.2, 3.2, 21, door(45.5, 2.0))
	w.ruinX(41, 50, 5.8, 0.4, 0.8, 2.6, 22, door(43.5, 1.6))
	w.ruinZ(-3.8, 5.4, 41.2, 0.4, 1.0, 3.0, 23, door(1.0, 1.8))
	w.mound(47, 1.5, 3.5, 3.0, 3, 24)

	// A3: quintal com mureta de 1 m (pula por cima) e oliveira
	w.room(19, 4, 27, 12, 0, 1.0, 0.4, "yard", 1, sides{w: []op{door(8, 1.6)}, e: []op{door(8, 1.6)}})
	w.olive(23, 8)

	// B3: sobrado com janelas para a rua leste e escada no beco
	w.sobrado(30, 7, 38, 15, 1,
		sides{n: []op{door(34, 1.4)}, s: []op{win(32, 1.2), win(36, 1.2)}, w: []op{door(10, 1.4)}, e: []op{win(13.5, 1.0)}},
		sides{n: []op{win(32, 1.2), win(36, 1.2)}, s: []op{win(34, 1.4)}, w: []op{win(9, 1.2), win(12.5, 1.2)}, e: []op{door(11, 1.4)}})
	w.stairs("sobrado L3", 38, 16.6, 'N', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)
	w.box(38, 0, 10.2, 39.4, 4.0, 11.8, "stair", 0)

	// C3: casa com caixotes dentro
	w.casa(41, 9, 50, 17, 2,
		sides{n: []op{door(45.5, 1.4)}, s: []op{win(45.5, 1.2)}, w: []op{door(13, 1.4)}},
		sides{})
	w.blk(47.5, 11.5, 1.2, 1.2, 0, 1.2, "crate", 0)
	w.blk(44, 15, 1.3, 1.0, 0, 0.9, "wood", 0)

	// A4: ruína em L entre os becos e o jardim
	w.ruinX(19, 29, 18, 0.5, 1.0, 2.8, 31, door(24, 2.0))
	w.ruinZ(18.25, 24, 19.25, 0.5, 1.2, 3.0, 32)
	w.mound(26, 21.5, 3.0, 2.6, 3, 33)

	// B4: pracinha com caixotes; C4: barracão
	w.blk(33, 19, 1.2, 1.2, 0, 1.2, "crate", 0)
	w.blk(35.5, 21.5, 1.3, 1.3, 0, 1.25, "crate", 0)
	w.olive(31.5, 22.5)
	w.room(42, 19, 49, 25, 0, 2.8, 0.3, "wood", 0, sides{w: []op{door(22, 1.4)}, n: []op{win(45.5, 1.2)}})
	w.slab(42, 19, 49, 25, 3.1, 0.3, "wood", 1)
}

func vilaRuinas(w *World) {
	// R1: sobrado desabado. Só metade do 1º andar ficou de pé; dá pra
	// subir pela rampa de escombros (degraus de 0,5 m).
	w.ruinX(-12, 4, 25.2, 0.4, 2.0, 3.7, 41, door(-8, 1.6), hole(0, 2.6, 0, 3))
	w.wallX(-12, 4, 36.8, 0.4, 0, 3.7, "ruin", 0, win(-8, 1.2), door(1.5, 1.6))
	w.wallZ(25.4, 36.6, -11.8, 0.4, 0, 3.7, "ruin", 0, win(30, 1.2))
	w.ruinZ(25.4, 36.6, 3.8, 0.4, 0.9, 2.2, 42, door(33, 1.8))
	w.slab(-12, 25, -4, 37, 4.0, 0.3, "ruin", 1)
	w.ruinX(-11.6, -4, 25.35, 0.3, 4.4, 5.4, 43)
	w.ruinZ(25.6, 36.4, -11.65, 0.3, 4.4, 5.6, 44)
	w.stairs("ruína (rampa de escombros)", 2.0, 28.0, 'W', 3.0, 8, 0.5, 0.75, 0, 0, "rubble", 1)

	// R2: casa sem teto, uma parede caída
	w.wallX(-34, -24, 24.2, 0.4, 0, 3.4, "ruin", 0, door(-29, 1.6))
	w.wallZ(24.4, 33.6, -33.8, 0.4, 0, 3.4, "ruin", 0, win(29, 1.2))
	w.ruinX(-34, -24, 33.8, 0.4, 0.6, 2.4, 51, door(-27, 2.0))
	w.mound(-24.5, 29, 2.6, 4.5, 3, 52)

	// Montes de escombros, paredes soltas e um carro queimado
	w.mound(-18, 30, 6, 5, 4, 61)
	w.mound(9, 21, 4, 4, 3, 62)
	w.mound(-27, 37.5, 4, 3.5, 3, 63)
	w.ruinX(-36, -28, 18.5, 0.5, 0.8, 2.6, 64)
	w.ruinZ(19, 23, 13, 0.5, 1.0, 2.4, 65)
	w.car(-3, 19.5, false, 2)
	w.blk(-14, 22, 0.8, 0.7, 0, 0.6, "rubble", 0)
	w.blk(-8.5, 39.5, 1.0, 0.9, 0, 0.7, "rubble", 1)
	w.blk(-36, 26, 0.9, 1.1, 0, 0.8, "rubble", 0)
}

func vilaJardim(w *World) {
	// Mureta de 1 m separando o jardim da rua, oliveiras e um galpão
	w.wallX(20, 52, 27.2, 0.4, 0, 1.0, "yard", 1, door(25, 2.0), door(35, 2.0), door(46, 2.0))
	for _, p := range [][2]float64{{24, 31}, {30, 31.5}, {36, 31}, {42, 31.5}, {27, 36}, {33, 36.5}, {39, 36}} {
		w.olive(p[0], p[1])
	}
	for _, p := range [][2]float64{{27, 33.5}, {39, 33.5}} {
		w.blk(p[0], p[1], 4.0, 1.2, 0, 0.3, "bed", 0)
	}
	w.room(44, 33, 50, 39, 0, 2.8, 0.3, "wood", 0, sides{n: []op{door(46.5, 1.2)}, w: []op{win(36, 1.0)}})
	w.slab(44, 33, 50, 39, 3.1, 0.3, "wood", 1)
	w.blk(22, 38.5, 2.6, 2.6, 0, 3.0, "tank", 0)
}

func vilaSpawns(w *World) {
	for _, p := range [][2]float64{
		{-46, -28}, {-51, -10}, {-39, 2}, {-40, 30}, {-20, 39}, {6, 39.5},
		{31, 39.5}, {51, 4}, {46, -26}, {25, -40}, {-5, -40}, {0, -29},
	} {
		w.Spawns = append(w.Spawns, Spawn{X: p[0], Z: p[1], Yaw: math.Atan2(p[0], p[1])})
	}
}

// Objetivos: vermelhos no oeste (containers), azuis no leste (becos e jardim).
func vilaObjetivos(w *World) {
	sp := func(x, z float64) Spawn { return Spawn{X: x, Z: z, Yaw: math.Atan2(x, z)} }
	w.TeamSpawns[TeamRed] = []Spawn{sp(-46, -28), sp(-51, -10), sp(-39, 2), sp(-45, 20), sp(-27, -16), sp(-40, 30)}
	w.TeamSpawns[TeamBlue] = []Spawn{sp(51, 4), sp(46, -26), sp(31, 39.5), sp(45, 30), sp(40, -1), sp(17, 22)}
	w.DomPoints = []Spot{
		{"A", -16.5, 0, 0, 3.5}, // rua oeste
		{"B", 0, 0, 1, 4.5},     // fonte da praça
		{"C", 17, 0, 2, 3.5},    // rua leste
	}
	w.Hills = []Spot{
		{"Praça", 0, 0, -6, 5},
		{"Igreja", 0, 0, -29, 5},
		{"Ruína", -8, 0, 31, 4.5},
		{"Containers", -39, 0, -10, 4},
		{"Becos", 28.5, 0, -8.5, 4},
	}
	w.FlagHome[TeamRed] = Spot{"Vermelha", -39, 0, 4, 1.6}
	w.FlagHome[TeamBlue] = Spot{"Azul", 34, 0, 24, 1.6}
}
