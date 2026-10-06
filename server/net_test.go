package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Cliente de teste: conecta de verdade por WebSocket e guarda o que chega.
type testClient struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	id     int
	last   map[string]any
	events []map[string]any
	seq    int
}

func dialTest(t *testing.T, url, name string, arma int) *testClient {
	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s?name=%s&arma=%d", url, name, arma), nil)
	if err != nil {
		t.Fatalf("conexão de %s: %v", name, err)
	}
	c := &testClient{conn: conn}
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m map[string]any
			if json.Unmarshal(data, &m) != nil {
				continue
			}
			c.mu.Lock()
			switch m["t"] {
			case "welcome":
				c.id = int(m["id"].(float64))
			case "snap":
				c.last = m
				if evs, ok := m["ev"].([]any); ok {
					for _, e := range evs {
						c.events = append(c.events, e.(map[string]any))
					}
				}
			}
			c.mu.Unlock()
		}
	}()
	return c
}

func (c *testClient) chat(text string) {
	msg, _ := json.Marshal(map[string]any{"t": "chat", "m": text})
	_ = c.conn.WriteMessage(websocket.TextMessage, msg)
}

func (c *testClient) ID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.id
}

func (c *testClient) input(keys int, yaw, pitch float64) {
	c.seq++
	msg, _ := json.Marshal(map[string]any{"t": "i", "s": c.seq, "k": keys, "y": yaw, "p": pitch})
	_ = c.conn.WriteMessage(websocket.TextMessage, msg)
}

func (c *testClient) position(id int) (Vec3, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last == nil {
		return Vec3{}, false
	}
	for _, p := range c.last["players"].([]any) {
		pm := p.(map[string]any)
		if int(pm["id"].(float64)) == id {
			return Vec3{pm["x"].(float64), pm["y"].(float64), pm["z"].(float64)}, true
		}
	}
	return Vec3{}, false
}

func (c *testClient) find(kind string) map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events {
		if e["t"] == kind {
			return e
		}
	}
	return nil
}

// Combate completo pela rede: Ana (sniper) mira na cabeça de Beto e atira.
func TestRedeTiroDeSniperNaCabeca(t *testing.T) {
	g := NewGame(GenerateArena(1), "ffa", 0)
	g.world.Boxes = nil // arena vazia: sempre há linha de visada
	go g.Run()
	srv := httptest.NewServer(http.HandlerFunc(g.ServeWS))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	ana := dialTest(t, url, "Ana", WSniper)
	beto := dialTest(t, url, "Beto", WAR)
	defer ana.conn.Close()
	defer beto.conn.Close()

	var pa, pb Vec3
	deadline := time.Now().Add(2 * time.Second)
	for {
		var okA, okB bool
		if ana.ID() > 0 && beto.ID() > 0 {
			pa, okA = ana.position(ana.ID())
			pb, okB = ana.position(beto.ID())
		}
		if okA && okB {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("snapshots com os dois jogadores não chegaram")
		}
		time.Sleep(20 * time.Millisecond)
	}

	eye := Vec3{pa.X, pa.Y + EyeHeight, pa.Z}
	dx, dy, dz := pb.X-eye.X, pb.Y+1.65-eye.Y, pb.Z-eye.Z
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	yaw, pitch := math.Atan2(-dx, -dz), math.Asin(dy/dist)

	for i := 0; i <= AimTime+2; i++ { // segura a mira até ficar precisa
		ana.input(KeyAim, yaw, pitch)
		time.Sleep(time.Second / TickRate)
	}
	ana.input(KeyAim|KeyFire, yaw, pitch)

	deadline = time.Now().Add(2 * time.Second)
	for beto.find("kill") == nil {
		if time.Now().After(deadline) {
			t.Fatalf("o tiro a %.1f m não derrubou (acerto: %v)", dist, beto.find("hit"))
		}
		time.Sleep(20 * time.Millisecond)
	}
	k := beto.find("kill")
	if int(k["id"].(float64)) != ana.ID() || int(k["tg"].(float64)) != beto.ID() || k["head"] != true || int(k["w"].(float64)) != WSniper {
		t.Fatalf("abate errado: %v", k)
	}
	t.Logf("headshot de sniper a %.1f m confirmado pelos dois clientes", dist)
}

// Chat pela rede: o que um manda, o outro recebe (limpo e com o nome certo).
func TestRedeChat(t *testing.T) {
	g := NewGame(GenerateArena(1), "ffa", 0)
	go g.Run()
	srv := httptest.NewServer(http.HandlerFunc(g.ServeWS))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ana := dialTest(t, url, "Ana", WAR)
	beto := dialTest(t, url, "Beto", WAR)
	defer ana.conn.Close()
	defer beto.conn.Close()
	time.Sleep(200 * time.Millisecond)
	ana.chat("  bom jogo\x07 pessoal  ")
	deadline := time.Now().Add(2 * time.Second)
	for beto.find("chat") == nil {
		if time.Now().After(deadline) {
			t.Fatal("a mensagem não chegou")
		}
		time.Sleep(20 * time.Millisecond)
	}
	m := beto.find("chat")
	if m["msg"] != "bom jogo pessoal" || m["name"] != "Ana" {
		t.Fatalf("mensagem errada: %v", m)
	}
}
