package main

import (
	"container/heap"
	"math"
)

// Nav é uma grade de 0,5 m sobre o chão do mapa: cada célula diz se um
// jogador cabe ali em pé. Os bots acham caminho nela com A*.
// (Os bots andam pelo térreo; não sobem escadas.)
type Nav struct {
	cell   float64
	nx, nz int
	x0, z0 float64
	free   []bool

	// buffers reaproveitados entre buscas (tudo roda sob o mutex do jogo)
	gcost  []float32
	parent []int32
	stamp  []int32
	gen    int32
}

func BuildNav(w *World) *Nav {
	n := &Nav{cell: 0.5, x0: -w.HalfX, z0: -w.HalfZ}
	n.nx, n.nz = int(2*w.HalfX/n.cell), int(2*w.HalfZ/n.cell)
	size := n.nx * n.nz
	n.free = make([]bool, size)
	n.gcost = make([]float32, size)
	n.parent = make([]int32, size)
	n.stamp = make([]int32, size)
	for k := 0; k < size; k++ {
		x, z := n.center(k)
		n.free[k] = !collidesAny(x, 0, z, w.Boxes)
	}
	return n
}

func (n *Nav) center(k int) (float64, float64) {
	i, j := k/n.nz, k%n.nz
	return n.x0 + (float64(i)+0.5)*n.cell, n.z0 + (float64(j)+0.5)*n.cell
}

func (n *Nav) index(x, z float64) int {
	i := int((x - n.x0) / n.cell)
	j := int((z - n.z0) / n.cell)
	i = max(0, min(n.nx-1, i))
	j = max(0, min(n.nz-1, j))
	return i*n.nz + j
}

func (n *Nav) Free(x, z float64) bool { return n.free[n.index(x, z)] }

// nearestFree procura a célula livre mais perto (anéis de até 6 m).
func (n *Nav) nearestFree(k int) int {
	if n.free[k] {
		return k
	}
	ci, cj := k/n.nz, k%n.nz
	for r := 1; r <= 12; r++ {
		best, bestD := -1, math.MaxInt
		for di := -r; di <= r; di++ {
			for dj := -r; dj <= r; dj++ {
				if max(abs(di), abs(dj)) != r {
					continue
				}
				i, j := ci+di, cj+dj
				if i < 0 || j < 0 || i >= n.nx || j >= n.nz || !n.free[i*n.nz+j] {
					continue
				}
				if d := di*di + dj*dj; d < bestD {
					best, bestD = i*n.nz+j, d
				}
			}
		}
		if best >= 0 {
			return best
		}
	}
	return -1
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// clearLine: dá para ir em linha reta de a até b só por células livres?
func (n *Nav) clearLine(ax, az, bx, bz float64) bool {
	dx, dz := bx-ax, bz-az
	steps := int(math.Hypot(dx, dz)/(n.cell*0.5)) + 1
	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps)
		if !n.free[n.index(ax+dx*t, az+dz*t)] {
			return false
		}
	}
	return true
}

type pqItem struct {
	k int32
	f float32
}
type pq []pqItem

func (q pq) Len() int           { return len(q) }
func (q pq) Less(i, j int) bool { return q[i].f < q[j].f }
func (q pq) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x any)        { *q = append(*q, x.(pqItem)) }
func (q *pq) Pop() any          { o := *q; it := o[len(o)-1]; *q = o[:len(o)-1]; return it }

var dirs8 = [8][3]int{{1, 0, 10}, {-1, 0, 10}, {0, 1, 10}, {0, -1, 10}, {1, 1, 14}, {1, -1, 14}, {-1, 1, 14}, {-1, -1, 14}}

// Path devolve pontos de passagem (já simplificados) de from até to.
func (n *Nav) Path(from, to Vec3) []Vec3 {
	s := n.nearestFree(n.index(from.X, from.Z))
	t := n.nearestFree(n.index(to.X, to.Z))
	if s < 0 || t < 0 {
		return nil
	}
	n.gen++
	ti, tj := t/n.nz, t%n.nz
	h := func(k int) float32 {
		di, dj := abs(k/n.nz-ti), abs(k%n.nz-tj)
		return float32(10*max(di, dj) + 4*min(di, dj))
	}
	open := &pq{{int32(s), h(s)}}
	n.stamp[s], n.gcost[s], n.parent[s] = n.gen, 0, -1
	found := false
	for expanded := 0; open.Len() > 0 && expanded < 60000; expanded++ {
		cur := heap.Pop(open).(pqItem)
		k := int(cur.k)
		if k == t {
			found = true
			break
		}
		if cur.f-h(k) > n.gcost[k]+0.01 { // entrada velha da fila
			continue
		}
		ci, cj := k/n.nz, k%n.nz
		for _, d := range dirs8 {
			i, j := ci+d[0], cj+d[1]
			if i < 0 || j < 0 || i >= n.nx || j >= n.nz {
				continue
			}
			nk := i*n.nz + j
			if !n.free[nk] {
				continue
			}
			if d[2] == 14 && (!n.free[ci*n.nz+j] || !n.free[i*n.nz+cj]) {
				continue // sem cortar quina
			}
			gc := n.gcost[k] + float32(d[2])
			if n.stamp[nk] != n.gen || gc < n.gcost[nk] {
				n.stamp[nk], n.gcost[nk], n.parent[nk] = n.gen, gc, int32(k)
				heap.Push(open, pqItem{int32(nk), gc + h(nk)})
			}
		}
	}
	if !found {
		return nil
	}
	var cells []int
	for k := t; k >= 0; k = int(n.parent[k]) {
		cells = append(cells, k)
	}
	pts := make([]Vec3, len(cells))
	for i, k := range cells {
		x, z := n.center(k)
		pts[len(cells)-1-i] = Vec3{x, 0, z}
	}
	if len(pts) == 1 { // já está no destino
		return pts
	}
	// Simplifica: pula pontos enquanto der para ir reto
	var out []Vec3
	for i := 0; i < len(pts)-1; {
		j := min(len(pts)-1, i+24)
		for j > i+1 && !n.clearLine(pts[i].X, pts[i].Z, pts[j].X, pts[j].Z) {
			j--
		}
		out = append(out, pts[j])
		i = j
	}
	return out
}
