package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// Em desenvolvimento aceitamos qualquer origem. Em produção, compare
	// r.Header.Get("Origin") com o seu domínio.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWS atende /ws?name=Fulano. Cada conexão vira um jogador.
func (g *Game) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	kind, _ := strconv.Atoi(r.URL.Query().Get("arma"))
	p := g.join(cleanName(r.URL.Query().Get("name")), kind)
	go writePump(conn, p.send)
	g.readPump(conn, p) // bloqueia até a conexão cair
	g.leave(p)
}

// ServeMap devolve o mapa da partida. O menu usa para mostrar a arena
// antes de você entrar (o mapa nunca muda depois de gerado, então não
// precisa de lock).
func (g *Game) ServeMap(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	_ = json.NewEncoder(w).Encode(g.world)
}

func (g *Game) join(name string, kind int) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()

	p := &Player{ID: g.nextID, Name: name, NextKind: validKind(kind), Vote: -1,
		send: make(chan []byte, 64), chatTokens: ChatBurst, chatTick: g.tick}
	g.nextID++
	g.players[p.ID] = p
	g.assignTeam(p)
	if g.respawnAllowed() {
		g.spawn(p)
	} // no sobrevivente com a rodada rolando, entra assistindo

	welcome, _ := json.Marshal(WelcomeMsg{
		T: "welcome", ID: p.ID, Tick: g.tick,
		TickRate: TickRate, SnapEvery: SnapshotEvery, Mode: g.modeDef().ID, Map: g.world,
	})
	p.send <- welcome
	g.emit(Event{T: "join", ID: p.ID, Name: name})
	g.balanceBots()
	log.Printf("+ %s (#%d) entrou — %d na sala", name, p.ID, len(g.players))
	return p
}

func (g *Game) leave(p *Player) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.players[p.ID]; !ok {
		return
	}
	g.removePlayer(p)
	g.balanceBots()
	log.Printf("- %s (#%d) saiu — %d na sala", p.Name, p.ID, len(g.players))
}

func (g *Game) readPump(conn *websocket.Conn, p *Player) {
	conn.SetReadLimit(1024)
	extend := func() { _ = conn.SetReadDeadline(time.Now().Add(60 * time.Second)) }
	extend()
	conn.SetPongHandler(func(string) error { extend(); return nil })

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		extend()
		var m ClientMsg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.T {
		case "i":
			in := Input{Seq: m.S, Keys: m.K & 0x3ff, Yaw: m.Y, Pitch: m.P, RT: m.RT}
			g.mu.Lock()
			if len(p.inputs) < MaxQueued {
				p.inputs = append(p.inputs, in)
			}
			g.mu.Unlock()
		case "voto":
			g.mu.Lock()
			if g.phase == "votacao" && m.V >= 0 && m.V < len(g.voteOpts) {
				p.Vote = m.V
			}
			g.mu.Unlock()
		case "arma": // vale no próximo respawn
			g.mu.Lock()
			p.NextKind = validKind(m.W)
			g.mu.Unlock()
		case "chat":
			text := cleanChat(m.M)
			if text == "" {
				continue
			}
			g.mu.Lock()
			ok := g.allowChat(p)
			if ok {
				g.emit(Event{T: "chat", ID: p.ID, Name: p.Name, Msg: text})
			}
			g.mu.Unlock()
			if !ok {
				warn, _ := json.Marshal(map[string]any{"t": "aviso", "m": "Espere um instante para mandar outra mensagem."})
				select {
				case p.send <- warn:
				default:
				}
			}
		case "ping":
			pong, _ := json.Marshal(map[string]any{"t": "pong", "c": m.C})
			select {
			case p.send <- pong:
			default:
			}
		}
	}
}

// writePump é o único lugar que escreve no socket (o gorilla não aceita
// escritas concorrentes).
func writePump(conn *websocket.Conn, send <-chan []byte) {
	ping := time.NewTicker(25 * time.Second)
	defer func() {
		ping.Stop()
		conn.Close()
	}()
	for {
		select {
		case msg, ok := <-send:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func validKind(k int) int {
	if k < 0 || k >= len(Weapons) {
		return WAR
	}
	return k
}

// cleanChat: sem caracteres de controle, sem espaços nas pontas, até 120 letras.
func cleanChat(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if rs := []rune(s); len(rs) > 120 {
		s = strings.TrimSpace(string(rs[:120]))
	}
	return s
}

func cleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if rs := []rune(s); len(rs) > 16 {
		s = string(rs[:16])
	}
	if s == "" {
		s = "Jogador"
	}
	return s
}
