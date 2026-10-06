package main

import (
	"math"
	"sort"
)

// Tipos do mapa e ferramentas para montar mapas com caixas.

type Spawn struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	Z   float64 `json:"z"`
	Yaw float64 `json:"yaw"`
}

// Zone é uma área do chão com acabamento próprio (só visual).
type Zone struct {
	X0 float64 `json:"x0"`
	Z0 float64 `json:"z0"`
	X1 float64 `json:"x1"`
	Z1 float64 `json:"z1"`
	K  string  `json:"k"`
}

// World é o mapa. O servidor monta e envia inteiro no "welcome", então
// todos os clientes colidem e desenham exatamente os mesmos blocos.
type World struct {
	Name   string  `json:"name"`
	HalfX  float64 `json:"hx"`
	HalfZ  float64 `json:"hz"`
	Boxes  []Box   `json:"boxes"`
	Spawns []Spawn `json:"spawns"`
	Zones  []Zone  `json:"zones,omitempty"`

	Stairs []Stairway `json:"-"` // só para os testes de navegação
	Doors  []Doorway  `json:"-"`

	// Objetivos dos modos (o cliente recebe as posições pelo snapshot)
	TeamSpawns [3][]Spawn `json:"-"` // [1] vermelho, [2] azul
	DomPoints  []Spot     `json:"-"`
	Hills      []Spot     `json:"-"`
	FlagHome   [3]Spot    `json:"-"`
	keep       []Spot     // áreas que o gerador da arena deixa livres
}

// Spot: um lugar com raio (ponto de captura, colina, base da bandeira).
type Spot struct {
	Name       string
	X, Y, Z, R float64
}

// Stairway: ponto de partida de uma escada (os testes sobem cada uma).
type Stairway struct {
	Name         string
	X, Y, Z, Yaw float64
	Top, Len     float64
}

// Doorway: centro de uma porta, no piso (os testes atravessam cada uma).
type Doorway struct {
	X, Y, Z float64
	AlongX  bool // parede corre ao longo de X (atravessa-se em Z)
}

func (w *World) box(x0, y0, z0, x1, y1, z1 float64, k string, v int) {
	x0, x1 = math.Min(x0, x1), math.Max(x0, x1)
	y0, y1 = math.Min(y0, y1), math.Max(y0, y1)
	z0, z1 = math.Min(z0, z1), math.Max(z0, z1)
	if x1-x0 < 1e-6 || y1-y0 < 1e-6 || z1-z0 < 1e-6 {
		return
	}
	w.Boxes = append(w.Boxes, Box{Min: [3]float64{x0, y0, z0}, Max: [3]float64{x1, y1, z1}, Kind: k, V: v})
}

// blk: bloco centrado em (cx, cz), tamanho sx × sz, de y0 até y0+h.
func (w *World) blk(cx, cz, sx, sz, y0, h float64, k string, v int) {
	w.box(cx-sx/2, y0, cz-sz/2, cx+sx/2, y0+h, cz+sz/2, k, v)
}

// op: abertura em [a, b] ao longo da parede, de lo até hi acima da base.
type op struct{ a, b, lo, hi float64 }

func door(c, width float64) op         { return op{c - width/2, c + width/2, 0, 2.5} }
func tallDoor(c, width, h float64) op  { return op{c - width/2, c + width/2, 0, h} }
func win(c, width float64) op          { return op{c - width/2, c + width/2, 1.0, 2.1} }
func hole(c, width, lo, hi float64) op { return op{c - width/2, c + width/2, lo, hi} }

func spans(a0, a1, h float64, ops []op, emit func(a, b, lo, hi float64)) {
	sorted := append([]op(nil), ops...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].a < sorted[j].a })
	cur := a0
	for _, o := range sorted {
		if o.a > cur {
			emit(cur, o.a, 0, h)
		}
		if o.lo > 0 {
			emit(o.a, o.b, 0, math.Min(o.lo, h))
		}
		if o.hi < h {
			emit(o.a, o.b, o.hi, h)
		}
		cur = o.b
	}
	if cur < a1 {
		emit(cur, a1, 0, h)
	}
}

// wallX: parede ao longo de X (de x0 a x1), centrada em z, espessura t.
func (w *World) wallX(x0, x1, z, t, y0, h float64, k string, v int, ops ...op) {
	spans(x0, x1, h, ops, func(a, b, lo, hi float64) { w.box(a, y0+lo, z-t/2, b, y0+hi, z+t/2, k, v) })
	for _, o := range ops {
		if o.lo == 0 && o.hi >= 2.2 {
			w.Doors = append(w.Doors, Doorway{X: (o.a + o.b) / 2, Y: y0, Z: z, AlongX: true})
		}
	}
}

// wallZ: parede ao longo de Z (de z0 a z1), centrada em x.
func (w *World) wallZ(z0, z1, x, t, y0, h float64, k string, v int, ops ...op) {
	spans(z0, z1, h, ops, func(a, b, lo, hi float64) { w.box(x-t/2, y0+lo, a, x+t/2, y0+hi, b, k, v) })
	for _, o := range ops {
		if o.lo == 0 && o.hi >= 2.2 {
			w.Doors = append(w.Doors, Doorway{X: x, Y: y0, Z: (o.a + o.b) / 2})
		}
	}
}

// sides: aberturas de cada parede (norte = z menor, sul = z maior).
type sides struct{ n, s, w, e []op }

// room: quatro paredes por dentro do retângulo.
func (w *World) room(x0, z0, x1, z1, y0, h, t float64, k string, v int, sd sides) {
	w.wallX(x0, x1, z0+t/2, t, y0, h, k, v, sd.n...)
	w.wallX(x0, x1, z1-t/2, t, y0, h, k, v, sd.s...)
	w.wallZ(z0+t, z1-t, x0+t/2, t, y0, h, k, v, sd.w...)
	w.wallZ(z0+t, z1-t, x1-t/2, t, y0, h, k, v, sd.e...)
}

// slab: laje com topo em y.
func (w *World) slab(x0, z0, x1, z1, y, th float64, k string, v int) {
	w.box(x0, y-th, z0, x1, y, z1, k, v)
}

// parapet: mureta na borda de uma laje (vãos não contam como portas).
func (w *World) parapet(x0, z0, x1, z1, y, h float64, k string, v int, gaps sides) {
	n := len(w.Doors)
	w.room(x0, z0, x1, z1, y, h, 0.3, k, v, gaps)
	w.Doors = w.Doors[:n]
}

// casa: térrea, paredes de 3,7 m, laje com topo em 4,0 m e mureta.
func (w *World) casa(x0, z0, x1, z1 float64, v int, walls, gaps sides) {
	w.room(x0, z0, x1, z1, 0, 3.7, 0.4, "house", v, walls)
	w.slab(x0, z0, x1, z1, 4.0, 0.3, "roof", v)
	w.parapet(x0, z0, x1, z1, 4.0, 0.6, "house", v, gaps)
}

// sobrado: dois andares (piso de cima em 4,0 m), telhado em 7,5 m.
func (w *World) sobrado(x0, z0, x1, z1 float64, v int, ground, upper sides) {
	w.room(x0, z0, x1, z1, 0, 3.7, 0.4, "house", v, ground)
	w.slab(x0, z0, x1, z1, 4.0, 0.3, "roof", v)
	w.room(x0, z0, x1, z1, 4.0, 3.2, 0.4, "house", v, upper)
	w.slab(x0, z0, x1, z1, 7.5, 0.3, "roof", v)
	w.parapet(x0, z0, x1, z1, 7.5, 0.6, "house", v, sides{})
}

// stairs: escada maciça. Começa no canto (x, z) e sobe na direção dir
// ('E' = +x, 'W' = −x, 'S' = +z, 'N' = −z), com largura para +z (E/W)
// ou +x (N/S). Os degraus vão de fill até h0 + rise·(i+1).
func (w *World) stairs(name string, x, z float64, dir byte, width float64, n int, rise, run, fill, h0 float64, k string, v int) {
	for i := 0; i < n; i++ {
		top := h0 + rise*float64(i+1)
		a, b := float64(i)*run, float64(i+1)*run
		switch dir {
		case 'E':
			w.box(x+a, fill, z, x+b, top, z+width, k, v)
		case 'W':
			w.box(x-b, fill, z, x-a, top, z+width, k, v)
		case 'S':
			w.box(x, fill, z+a, x+width, top, z+b, k, v)
		case 'N':
			w.box(x, fill, z-b, x+width, top, z-a, k, v)
		}
	}
	sw := Stairway{Name: name, Y: h0, Top: h0 + rise*float64(n), Len: run * float64(n)}
	switch dir {
	case 'E':
		sw.X, sw.Z, sw.Yaw = x-0.8, z+width/2, -math.Pi/2
	case 'W':
		sw.X, sw.Z, sw.Yaw = x+0.8, z+width/2, math.Pi/2
	case 'S':
		sw.X, sw.Z, sw.Yaw = x+width/2, z-0.8, math.Pi
	case 'N':
		sw.X, sw.Z, sw.Yaw = x+width/2, z+0.8, 0
	}
	w.Stairs = append(w.Stairs, sw)
}
