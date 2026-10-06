package main

import (
	"math"
	"testing"
)

// Todas as escadas da vila sobem até o topo andando em linha reta.
func TestVilaEscadasSobem(t *testing.T) {
	w := BuildVila()
	for _, st := range w.Stairs {
		s := PhysState{X: st.X, Y: st.Y, Z: st.Z, OnGround: true}
		if collidesAny(s.X, s.Y, s.Z, w.Boxes) {
			t.Errorf("%s: ponto de partida dentro de um bloco", st.Name)
			continue
		}
		maxY := s.Y
		ticks := int((st.Len/MoveSpeed + 1.5) * TickRate)
		for i := 0; i < ticks; i++ {
			Step(&s, KeyForward, st.Yaw, w.Boxes, 1)
			maxY = math.Max(maxY, s.Y)
		}
		if maxY < st.Top-1e-6 {
			t.Errorf("%s: parou em %.2f m, o topo é %.2f m", st.Name, maxY, st.Top)
		}
	}
	t.Logf("%d escadas ok, %d blocos no mapa", len(w.Stairs), len(w.Boxes))
}

// Todas as portas dão passagem (de um lado para o outro).
func TestVilaPortasPassam(t *testing.T) {
	w := BuildVila()
	fails := 0
	for _, d := range w.Doors {
		ok := false
		for _, dir := range []float64{1, -1} {
			x, z, yaw := d.X, d.Z-1.3*dir, map[bool]float64{true: math.Pi, false: 0}[dir > 0]
			if !d.AlongX {
				x, z = d.X-1.3*dir, d.Z
				yaw = map[bool]float64{true: -math.Pi / 2, false: math.Pi / 2}[dir > 0]
			}
			s := PhysState{X: x, Y: d.Y, Z: z, OnGround: true}
			if collidesAny(s.X, s.Y, s.Z, w.Boxes) {
				continue
			}
			for i := 0; i < 40; i++ {
				Step(&s, KeyForward, yaw, w.Boxes, 1)
			}
			if (d.AlongX && (s.Z-d.Z)*dir > 0.6) || (!d.AlongX && (s.X-d.X)*dir > 0.6) {
				ok = true
				break
			}
		}
		if !ok {
			fails++
			t.Errorf("porta em (%.1f, %.1f, %.1f) não dá passagem", d.X, d.Y, d.Z)
		}
	}
	t.Logf("%d portas testadas", len(w.Doors))
}

// Todos os spawns e pés de escada estão ligados pelo chão (busca em grade).
func TestVilaTudoConectado(t *testing.T) {
	w := BuildVila()
	const cell = 0.5
	nx, nz := int(2*w.HalfX/cell), int(2*w.HalfZ/cell)
	free := make([]bool, nx*nz)
	pos := func(i, j int) (float64, float64) {
		return -w.HalfX + (float64(i)+0.5)*cell, -w.HalfZ + (float64(j)+0.5)*cell
	}
	for i := 0; i < nx; i++ {
		for j := 0; j < nz; j++ {
			x, z := pos(i, j)
			free[i*nz+j] = !collidesAny(x, 0, z, w.Boxes)
		}
	}
	idx := func(x, z float64) int {
		i, j := int((x+w.HalfX)/cell), int((z+w.HalfZ)/cell)
		return i*nz + j
	}
	seen := make([]bool, nx*nz)
	start := idx(w.Spawns[0].X, w.Spawns[0].Z)
	queue := []int{start}
	seen[start] = true
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		i, j := c/nz, c%nz
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			a, b := i+d[0], j+d[1]
			if a < 0 || b < 0 || a >= nx || b >= nz {
				continue
			}
			n := a*nz + b
			if free[n] && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	for k, s := range w.Spawns {
		if collidesAny(s.X, s.Y, s.Z, w.Boxes) {
			t.Errorf("spawn %d dentro de um bloco", k)
		} else if !seen[idx(s.X, s.Z)] {
			t.Errorf("spawn %d (%.0f, %.0f) isolado dos outros", k, s.X, s.Z)
		}
	}
	for _, st := range w.Stairs {
		if st.Y == 0 && !seen[idx(st.X, st.Z)] {
			t.Errorf("escada %q não é alcançável pelo chão", st.Name)
		}
	}
	n := 0
	for _, s := range seen {
		if s {
			n++
		}
	}
	t.Logf("%.0f m² de chão alcançável", float64(n)*cell*cell)
}

// Spawns das equipes, pontos, colinas e bandeiras: livres e alcançáveis.
func TestVilaObjetivosAlcancaveis(t *testing.T) {
	w := BuildVila()
	nav := BuildNav(w)
	from := Vec3{w.Spawns[0].X, 0, w.Spawns[0].Z}
	reach := func(name string, x, z float64, r float64) {
		if p := nav.Path(from, Vec3{x, 0, z}); p == nil || hdist(p[len(p)-1], Vec3{x, 0, z}) > r {
			t.Errorf("%s (%.1f, %.1f) não é alcançável pelo chão", name, x, z)
		}
	}
	for team := TeamRed; team <= TeamBlue; team++ {
		for i, s := range w.TeamSpawns[team] {
			if collidesAny(s.X, s.Y, s.Z, w.Boxes) {
				t.Errorf("spawn %d da equipe %d dentro de um bloco", i, team)
			}
			reach("spawn de equipe", s.X, s.Z, 1)
		}
		h := w.FlagHome[team]
		if collidesAny(h.X, h.Y, h.Z, w.Boxes) {
			t.Errorf("bandeira %s dentro de um bloco", h.Name)
		}
		reach("bandeira "+h.Name, h.X, h.Z, 1)
	}
	for _, s := range append(append([]Spot{}, w.DomPoints...), w.Hills...) {
		reach(s.Name, s.X, s.Z, s.R) // basta chegar na área
	}
}
