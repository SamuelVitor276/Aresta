package main

// Física, armas e granadas.
//
// ESTE ARQUIVO É ESPELHADO EM client/public/js/physics.js.
// O cliente roda a mesma simulação para prever o próprio movimento, a
// munição e as granadas sem esperar o servidor. Mudou algo aqui? Mude lá
// também, senão o boneco "dá tranco" quando o servidor corrige.

import "math"

const TickRate = 60

// Teclas (bitmask enviado pelo cliente a cada input)
const (
	KeyForward = 1 << iota
	KeyBack
	KeyLeft
	KeyRight
	KeyJump
	KeyFire
	KeyReload
	KeyAim
	KeyMelee
	KeyNade
)

// Parâmetros são variáveis (e não constantes) de propósito: o Go calcula
// expressões entre constantes com precisão infinita, e o JavaScript em
// float64. Como variáveis, as contas batem bit a bit dos dois lados.
var (
	DT = 1.0 / float64(TickRate)

	PlayerHalfW  = 0.4
	PlayerHeight = 1.8
	EyeHeight    = 1.6
	HeadBase     = 1.45
	HeadHalfW    = 0.25

	MoveSpeed   = 7.5
	GroundAccel = 14.0
	AirAccel    = 2.5
	Gravity     = 22.0
	JumpSpeed   = 8.0
	StepHeight  = 0.55

	eps = 1e-4
)

// ---------------------------------------------------------------- mundo

// Box é um bloco de colisão alinhado aos eixos (AABB).
type Box struct {
	Min  [3]float64 `json:"min"`
	Max  [3]float64 `json:"max"`
	Kind string     `json:"k"`
	V    int        `json:"v,omitempty"` // variação de cor, só visual
}

// PhysState é o estado físico de um jogador. (X, Y, Z) é a posição dos pés.
type PhysState struct {
	X, Y, Z    float64
	VX, VY, VZ float64
	OnGround   bool
}

func overlaps(x, y, z float64, b *Box) bool {
	return x+PlayerHalfW > b.Min[0] && x-PlayerHalfW < b.Max[0] &&
		y+PlayerHeight > b.Min[1] && y < b.Max[1] &&
		z+PlayerHalfW > b.Min[2] && z-PlayerHalfW < b.Max[2]
}

func collidesAny(x, y, z float64, boxes []Box) bool {
	for i := range boxes {
		if overlaps(x, y, z, &boxes[i]) {
			return true
		}
	}
	return false
}

func tryStep(s *PhysState, b *Box, boxes []Box) bool {
	rise := b.Max[1] - s.Y
	if rise <= 0 || rise > StepHeight {
		return false
	}
	if collidesAny(s.X, b.Max[1], s.Z, boxes) {
		return false
	}
	s.Y = b.Max[1]
	return true
}

// Step avança um jogador em um tick. mul ajusta a velocidade (arma, mira).
// Os float64(...) em volta de multiplicações impedem FMA em ARM64.
func Step(s *PhysState, keys int, yaw float64, boxes []Box, mul float64) {
	f, r := 0.0, 0.0
	if keys&KeyForward != 0 {
		f += 1
	}
	if keys&KeyBack != 0 {
		f -= 1
	}
	if keys&KeyRight != 0 {
		r += 1
	}
	if keys&KeyLeft != 0 {
		r -= 1
	}
	sn, cs := math.Sin(yaw), math.Cos(yaw)
	wx := float64(-sn*f) + float64(cs*r)
	wz := float64(-cs*f) - float64(sn*r)
	if l := math.Sqrt(float64(wx*wx) + float64(wz*wz)); l > 0 {
		sp := MoveSpeed * mul
		wx = wx / l * sp
		wz = wz / l * sp
	}
	accel := AirAccel
	if s.OnGround {
		accel = GroundAccel
	}
	k := math.Min(1, accel*DT)
	s.VX += float64((wx - s.VX) * k)
	s.VZ += float64((wz - s.VZ) * k)
	if keys&KeyJump != 0 && s.OnGround {
		s.VY = JumpSpeed
	}
	s.VY -= float64(Gravity * DT)

	canStep := s.OnGround
	s.OnGround = false

	if dx := float64(s.VX * DT); dx != 0 {
		s.X += dx
		for i := range boxes {
			b := &boxes[i]
			if !overlaps(s.X, s.Y, s.Z, b) {
				continue
			}
			if canStep && tryStep(s, b, boxes) {
				continue
			}
			if dx > 0 {
				s.X = b.Min[0] - PlayerHalfW - eps
			} else {
				s.X = b.Max[0] + PlayerHalfW + eps
			}
			s.VX = 0
		}
	}
	if dz := float64(s.VZ * DT); dz != 0 {
		s.Z += dz
		for i := range boxes {
			b := &boxes[i]
			if !overlaps(s.X, s.Y, s.Z, b) {
				continue
			}
			if canStep && tryStep(s, b, boxes) {
				continue
			}
			if dz > 0 {
				s.Z = b.Min[2] - PlayerHalfW - eps
			} else {
				s.Z = b.Max[2] + PlayerHalfW + eps
			}
			s.VZ = 0
		}
	}
	if dy := float64(s.VY * DT); dy != 0 {
		s.Y += dy
		for i := range boxes {
			b := &boxes[i]
			if !overlaps(s.X, s.Y, s.Z, b) {
				continue
			}
			if dy < 0 {
				s.Y = b.Max[1]
				s.OnGround = true
			} else {
				s.Y = b.Min[1] - PlayerHeight - eps
			}
			s.VY = 0
		}
	}
	if s.Y < 0 {
		s.Y = 0
		s.VY = 0
		s.OnGround = true
	}
}

func ClampPitch(p float64) float64 { return math.Max(-1.55, math.Min(1.55, p)) }

// ---------------------------------------------------------------- armas

const (
	WAR      = iota // fuzil
	WSniper         // sniper
	WShotgun        // escopeta
	WKnife          // faca (só aparece em eventos)
	WNade           // granada (só aparece em eventos)
)

type WeaponDef struct {
	Name         string
	Mag          int     // balas no pente
	FireInterval int     // inputs entre tiros (60 = 1 s)
	Reload       int     // inputs para recarregar
	Pellets      int     // projéteis por tiro
	Spread       float64 // abertura sem mirar (rad)
	AimSpread    float64 // abertura mirando
	MoveSpread   float64 // extra correndo ou no ar
	Body, Head   float64 // dano por projétil
	FalloffStart float64 // dano cheio até aqui (m)
	FalloffEnd   float64 // dano mínimo daqui em diante
	FalloffMin   float64 // fração do dano no mínimo
	Range        float64
	MoveMul      float64 // velocidade com a arma
	AimMoveMul   float64 // velocidade mirando
}

var Weapons = [3]WeaponDef{
	WAR: {Name: "Fuzil", Mag: 30, FireInterval: 6, Reload: 96, Pellets: 1,
		Spread: 0.018, AimSpread: 0.004, MoveSpread: 0.02, Body: 24, Head: 55,
		FalloffStart: 25, FalloffEnd: 60, FalloffMin: 0.6, Range: 200, MoveMul: 1, AimMoveMul: 0.7},
	WSniper: {Name: "Sniper", Mag: 5, FireInterval: 72, Reload: 150, Pellets: 1,
		Spread: 0.08, AimSpread: 0, MoveSpread: 0.05, Body: 80, Head: 200,
		FalloffStart: 1000, FalloffEnd: 2000, FalloffMin: 1, Range: 300, MoveMul: 0.93, AimMoveMul: 0.5},
	WShotgun: {Name: "Escopeta", Mag: 6, FireInterval: 48, Reload: 120, Pellets: 9,
		Spread: 0.075, AimSpread: 0.055, MoveSpread: 0.01, Body: 13, Head: 18,
		FalloffStart: 7, FalloffEnd: 22, FalloffMin: 0.25, Range: 45, MoveMul: 1.05, AimMoveMul: 0.85},
}

var (
	AimTime = 12 // inputs segurando a mira até a precisão total (sem "quickscope")

	KnifeRange    = 2.3
	KnifePad      = 0.3 // a faca perdoa um pouco a mira
	KnifeDamage   = 55
	KnifeBackstab = 200 // pelas costas derruba
	KnifeCooldown = 33
	KnifeBusy     = 21 // sem atirar logo depois da facada

	NadeCount    = 2
	NadeCooldown = 45
	NadeBusy     = 24
	NadeFuse     = 150 // 2,5 s
	NadeSpeed    = 15.0
	NadeUp       = 3.5
	NadeGravity  = 18.0
	NadeBounce   = 0.35
	NadeFriction = 0.75
	NadeRoll     = 0.9
	NadeHalf     = 0.09
	NadeRadius   = 7.0
	NadeDamage   = 120.0
)

// Weapon usa a sequência do input como relógio: cliente e servidor
// concordam exatamente sobre cadência, recarga, faca e granada.
type Weapon struct {
	Kind      int `json:"wk"`
	Ammo      int `json:"am"`
	NextFire  int `json:"nf"`
	ReloadEnd int `json:"re"`
	Busy      int `json:"bu"`
	NextMelee int `json:"nm"`
	NextNade  int `json:"nn"`
	Nades     int `json:"ng"`
	AimSince  int `json:"as"`
}

func NewWeapon(kind int) Weapon {
	if kind < 0 || kind >= len(Weapons) {
		kind = WAR
	}
	return Weapon{Kind: kind, Ammo: Weapons[kind].Mag, Nades: NadeCount}
}

const (
	ActNone = iota
	ActFire
	ActReload
	ActMelee
	ActNade
)

func StepWeapon(w *Weapon, keys, seq int) int {
	d := &Weapons[w.Kind]
	if keys&KeyAim == 0 {
		w.AimSince = 0
	} else if w.AimSince == 0 {
		w.AimSince = seq
	}
	if w.ReloadEnd > 0 && seq >= w.ReloadEnd {
		w.Ammo = d.Mag
		w.ReloadEnd = 0
	}
	if keys&KeyNade != 0 && w.Nades > 0 && seq >= w.NextNade && seq >= w.Busy {
		w.Nades--
		w.NextNade = seq + NadeCooldown
		w.Busy = seq + NadeBusy
		return ActNade
	}
	if keys&KeyMelee != 0 && seq >= w.NextMelee && seq >= w.Busy {
		w.NextMelee = seq + KnifeCooldown
		w.Busy = seq + KnifeBusy
		return ActMelee
	}
	if w.ReloadEnd > 0 || seq < w.Busy {
		return ActNone
	}
	reload := keys&KeyReload != 0 && w.Ammo < d.Mag
	if keys&KeyFire != 0 && w.Ammo == 0 {
		reload = true
	}
	if reload {
		w.ReloadEnd = seq + d.Reload
		return ActReload
	}
	if keys&KeyFire != 0 && seq >= w.NextFire {
		w.Ammo--
		w.NextFire = seq + d.FireInterval
		return ActFire
	}
	return ActNone
}

func Aimed(w *Weapon, keys, seq int) bool {
	return keys&KeyAim != 0 && w.AimSince > 0 && seq-w.AimSince >= AimTime
}

func MoveMul(w *Weapon, keys int) float64 {
	if keys&KeyAim != 0 {
		return Weapons[w.Kind].AimMoveMul
	}
	return Weapons[w.Kind].MoveMul
}

func IsMoving(s *PhysState) bool {
	return !s.OnGround || float64(s.VX*s.VX)+float64(s.VZ*s.VZ) > 6.25
}

// hash32 gera números "aleatórios" iguais no cliente e no servidor:
// a dispersão dos tiros é sorteada a partir da sequência do input.
func hash32(a, b uint32) uint32 {
	h := a*0x9E3779B1 ^ (b+0x7F4A7C15)*0x85EBCA77
	h ^= h >> 15
	h *= 0x2C1B3C6D
	h ^= h >> 12
	h *= 0x297A2D39
	h ^= h >> 15
	return h
}

func rand01(seq, i, salt int) float64 {
	return float64(hash32(uint32(seq), uint32(i*7+salt))) / 4294967296
}

// ShotDirs devolve a direção de cada projétil do tiro.
func ShotDirs(w *Weapon, yaw, pitch float64, keys, seq int, s *PhysState) []Vec3 {
	d := &Weapons[w.Kind]
	spread := d.Spread
	if Aimed(w, keys, seq) {
		spread = d.AimSpread
	}
	if IsMoving(s) {
		spread += d.MoveSpread
	}
	out := make([]Vec3, d.Pellets)
	for i := range out {
		u1, u2 := rand01(seq, i, 1), rand01(seq, i, 2)
		var a, r float64
		switch {
		case d.Pellets == 1:
			a, r = 2*math.Pi*u1, spread*math.Sqrt(u2)
		case i == 0:
			a, r = 2*math.Pi*u1, spread*0.2*u2
		default: // anel com leve variação: padrão justo e previsível
			a = 2 * math.Pi * (float64(i-1)/float64(d.Pellets-1) + float64(u1*0.08))
			r = spread * (0.5 + float64(0.5*u2))
		}
		out[i] = AimDir(yaw+float64(r*math.Cos(a)), ClampPitch(pitch+float64(r*math.Sin(a))))
	}
	return out
}

func Falloff(d *WeaponDef, dist float64) float64 {
	if dist <= d.FalloffStart {
		return 1
	}
	if dist >= d.FalloffEnd {
		return d.FalloffMin
	}
	t := (dist - d.FalloffStart) / (d.FalloffEnd - d.FalloffStart)
	return 1 - t*(1-d.FalloffMin)
}

// ---------------------------------------------------------------- granadas

type Nade struct {
	ID, Owner  int
	X, Y, Z    float64
	VX, VY, VZ float64
	Rest       bool
	Explode    uint64 // tick da explosão (só no servidor)
}

func NadeLaunch(s *PhysState, yaw, pitch float64) Nade {
	d := AimDir(yaw, ClampPitch(pitch))
	return Nade{
		X:  s.X + float64(d.X*0.3),
		Y:  s.Y + EyeHeight - 0.1 + float64(d.Y*0.3),
		Z:  s.Z + float64(d.Z*0.3),
		VX: float64(d.X*NadeSpeed) + float64(s.VX*0.4),
		VY: float64(d.Y*NadeSpeed) + NadeUp,
		VZ: float64(d.Z*NadeSpeed) + float64(s.VZ*0.4),
	}
}

func nadeHit(n *Nade, b *Box) bool {
	return n.X+NadeHalf > b.Min[0] && n.X-NadeHalf < b.Max[0] &&
		n.Y+NadeHalf > b.Min[1] && n.Y-NadeHalf < b.Max[1] &&
		n.Z+NadeHalf > b.Min[2] && n.Z-NadeHalf < b.Max[2]
}

// StepNade avança a granada um tick. Devolve a velocidade do impacto mais
// forte (o cliente usa para o som de quique).
func StepNade(n *Nade, boxes []Box) float64 {
	if n.Rest {
		return 0
	}
	impact := 0.0
	n.VY -= float64(NadeGravity * DT)
	if n.VY < -25 {
		n.VY = -25
	}
	if dx := float64(n.VX * DT); dx != 0 {
		n.X += dx
		for i := range boxes {
			b := &boxes[i]
			if !nadeHit(n, b) {
				continue
			}
			if dx > 0 {
				n.X = b.Min[0] - NadeHalf - eps
			} else {
				n.X = b.Max[0] + NadeHalf + eps
			}
			impact = math.Max(impact, math.Abs(n.VX))
			n.VX = -n.VX * NadeBounce
			n.VZ *= NadeFriction
		}
	}
	if dz := float64(n.VZ * DT); dz != 0 {
		n.Z += dz
		for i := range boxes {
			b := &boxes[i]
			if !nadeHit(n, b) {
				continue
			}
			if dz > 0 {
				n.Z = b.Min[2] - NadeHalf - eps
			} else {
				n.Z = b.Max[2] + NadeHalf + eps
			}
			impact = math.Max(impact, math.Abs(n.VZ))
			n.VZ = -n.VZ * NadeBounce
			n.VX *= NadeFriction
		}
	}
	landed := false
	if dy := float64(n.VY * DT); dy != 0 {
		n.Y += dy
		for i := range boxes {
			b := &boxes[i]
			if !nadeHit(n, b) {
				continue
			}
			impact = math.Max(impact, math.Abs(n.VY))
			if dy < 0 {
				n.Y = b.Max[1] + NadeHalf
				landed = true
			} else {
				n.Y = b.Min[1] - NadeHalf - eps
			}
			n.VY = -n.VY * NadeBounce
		}
	}
	if n.Y < NadeHalf {
		impact = math.Max(impact, math.Abs(n.VY))
		n.Y = NadeHalf
		n.VY = -n.VY * NadeBounce
		landed = true
	}
	if landed {
		if n.VY > 1.2 { // quique de verdade
			n.VX *= NadeFriction
			n.VZ *= NadeFriction
		} else { // rolando no chão
			n.VY = 0
			n.VX *= NadeRoll
			n.VZ *= NadeRoll
			if float64(n.VX*n.VX)+float64(n.VZ*n.VZ) < 0.04 {
				n.VX, n.VZ, n.Rest = 0, 0, true
			}
		}
	}
	return impact
}

// ---------------------------------------------------------------- raios

type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Scale(k float64) Vec3 { return Vec3{a.X * k, a.Y * k, a.Z * k} }
func (a Vec3) Lerp(b Vec3, t float64) Vec3 {
	return Vec3{a.X + (b.X-a.X)*t, a.Y + (b.Y-a.Y)*t, a.Z + (b.Z-a.Z)*t}
}

func AimDir(yaw, pitch float64) Vec3 {
	cp := math.Cos(pitch)
	return Vec3{-math.Sin(yaw) * cp, math.Sin(pitch), -math.Cos(yaw) * cp}
}

// RayBox: raio x AABB (slabs). Devolve a distância do acerto.
func RayBox(o, d Vec3, b Box) (float64, bool) {
	tmin, tmax := 0.0, math.Inf(1)
	oa := [3]float64{o.X, o.Y, o.Z}
	da := [3]float64{d.X, d.Y, d.Z}
	for i := 0; i < 3; i++ {
		if math.Abs(da[i]) < 1e-12 {
			if oa[i] < b.Min[i] || oa[i] > b.Max[i] {
				return 0, false
			}
			continue
		}
		inv := 1 / da[i]
		t1 := (b.Min[i] - oa[i]) * inv
		t2 := (b.Max[i] - oa[i]) * inv
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
		if tmin > tmax {
			return 0, false
		}
	}
	return tmin, true
}

func RayWorld(o, d Vec3, boxes []Box, maxDist float64) float64 {
	best := maxDist
	if d.Y < -1e-9 {
		if t := -o.Y / d.Y; t < best {
			best = t
		}
	}
	for i := range boxes {
		if t, ok := RayBox(o, d, boxes[i]); ok && t < best {
			best = t
		}
	}
	return best
}

func BodyBox(p Vec3) Box {
	return Box{
		Min: [3]float64{p.X - PlayerHalfW, p.Y, p.Z - PlayerHalfW},
		Max: [3]float64{p.X + PlayerHalfW, p.Y + HeadBase, p.Z + PlayerHalfW},
	}
}

func HeadBox(p Vec3) Box {
	return Box{
		Min: [3]float64{p.X - HeadHalfW, p.Y + HeadBase, p.Z - HeadHalfW},
		Max: [3]float64{p.X + HeadHalfW, p.Y + PlayerHeight + 0.05, p.Z + HeadHalfW},
	}
}

// PlayerBox é o corpo inteiro com folga (usado pela faca).
func PlayerBox(p Vec3, pad float64) Box {
	h := PlayerHalfW + pad
	return Box{
		Min: [3]float64{p.X - h, p.Y, p.Z - h},
		Max: [3]float64{p.X + h, p.Y + PlayerHeight + 0.05, p.Z + h},
	}
}
