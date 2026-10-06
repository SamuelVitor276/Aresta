package main

import (
	"encoding/json"
	"math"
)

// ---------------------------------------------------------------- cliente → servidor

// ClientMsg cobre as mensagens do cliente:
//
//	{"t":"i","s":123,"k":5,"y":1.2,"p":-0.1,"rt":4567.5}  input de um tick
//	{"t":"ping","c":123456.7}                             medição de latência
//	{"t":"arma","w":1}                                    arma do próximo respawn
//	{"t":"chat","m":"bom jogo"}                           mensagem no chat
//	{"t":"voto","v":2}                                    voto no próximo modo
type ClientMsg struct {
	T  string  `json:"t"`
	S  int     `json:"s"`
	K  int     `json:"k"`
	Y  float64 `json:"y"`
	P  float64 `json:"p"`
	RT float64 `json:"rt"`
	C  float64 `json:"c"`
	W  int     `json:"w"`
	M  string  `json:"m"`
	V  int     `json:"v"`
}

// ---------------------------------------------------------------- servidor → cliente

type WelcomeMsg struct {
	T         string `json:"t"`
	ID        int    `json:"id"`
	Tick      uint64 `json:"tick"`
	TickRate  int    `json:"tickRate"`
	SnapEvery int    `json:"snapEvery"`
	Mode      string `json:"mode"`
	Map       *World `json:"map"`
}

type SnapPlayer struct {
	ID     int     `json:"id"`
	Name   string  `json:"n"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	Yaw    float64 `json:"yw"`
	Pitch  float64 `json:"pt"`
	HP     int     `json:"hp"`
	Alive  bool    `json:"a"`
	Kills  int     `json:"k"`
	Deaths int     `json:"d"`
	W      int     `json:"w"`
	Wins   int     `json:"vt"`
	In     bool    `json:"in"`
	Team   int     `json:"tm"`
	Bot    bool    `json:"b,omitempty"`
}

// RoundInfo: rodada do modo sobrevivente.
type RoundInfo struct {
	State string `json:"s"` // rodada | intervalo
	Num   int    `json:"n"`
	Alive int    `json:"v"`
	Total int    `json:"p"`
}

type VoteInfo struct {
	Opts   []string `json:"o"`
	Names  []string `json:"n"`
	Descs  []string `json:"d"`
	Counts []int    `json:"c"`
}

// MatchInfo: situação da partida, mandada em todo snapshot.
type MatchInfo struct {
	Mode    string      `json:"m"`
	Name    string      `json:"nm"`
	Phase   string      `json:"f"` // espera | contagem | jogo | fim | votacao
	Left    float64     `json:"t,omitempty"`
	Score   []int       `json:"sc,omitempty"` // vermelho, azul
	Limit   int         `json:"lim"`
	Round   *RoundInfo  `json:"rd,omitempty"`
	Points  []*CapPoint `json:"pts,omitempty"`
	Hill    *CapPoint   `json:"hill,omitempty"`
	Flags   []*Flag     `json:"fl,omitempty"`
	Vote    *VoteInfo   `json:"vote,omitempty"`
	Winner  int         `json:"w,omitempty"`
	WinTeam int         `json:"wt,omitempty"`
}

// YouState é o estado completo do próprio jogador (para a reconciliação).
type YouState struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Z       float64 `json:"z"`
	VX      float64 `json:"vx"`
	VY      float64 `json:"vy"`
	VZ      float64 `json:"vz"`
	OG      bool    `json:"og"`
	HP      int     `json:"hp"`
	Alive   bool    `json:"a"`
	Respawn float64 `json:"rs"` // segundos até renascer (−1: só na próxima rodada)
	NK      int     `json:"nk"`
	Vote    int     `json:"vo"`
	Team    int     `json:"tm"`
	Weapon
}

type SnapshotMsg struct {
	T       string          `json:"t"`
	Tick    uint64          `json:"tick"`
	Ack     int             `json:"ack"`
	You     YouState        `json:"you"`
	Match   *MatchInfo      `json:"mt"`
	Players json.RawMessage `json:"players"`
	Events  json.RawMessage `json:"ev,omitempty"`
}

// Event: algo que aconteceu entre dois snapshots.
// t: shot | hit | kill | melee | nade | boom | spawn | reload | join | leave |
//
//	chat | match | round | point | hill | flag
type Event struct {
	T      string    `json:"t"`
	ID     int       `json:"id,omitempty"`
	Target int       `json:"tg,omitempty"`
	Dmg    int       `json:"dmg,omitempty"`
	Head   bool      `json:"head,omitempty"`
	Back   bool      `json:"back,omitempty"`
	W      int       `json:"w,omitempty"`
	N      int       `json:"n,omitempty"`
	Tk     uint64    `json:"tk,omitempty"`
	From   []float64 `json:"from,omitempty"`
	Pts    []float64 `json:"pts,omitempty"`
	V      []float64 `json:"v,omitempty"`
	Yaw    float64   `json:"yaw,omitempty"`
	Name   string    `json:"name,omitempty"`
	Msg    string    `json:"msg,omitempty"`
	S      string    `json:"s,omitempty"`
	Tm     int       `json:"tm,omitempty"` // equipe envolvida
}

func r3(v float64) float64 { return math.Round(v*1000) / 1000 }
func r2(v float64) float64 { return math.Round(v*100) / 100 }

func arr(v Vec3) []float64 { return []float64{r2(v.X), r2(v.Y), r2(v.Z)} }
