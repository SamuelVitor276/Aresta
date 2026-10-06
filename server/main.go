package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	// MAPA=vila (padrão, feito à mão) ou MAPA=arena (pequena e aleatória;
	// MAP_SEED fixa o sorteio).
	var world *World
	if os.Getenv("MAPA") == "arena" {
		seed := time.Now().UnixNano()
		if v := os.Getenv("MAP_SEED"); v != "" {
			if s, err := strconv.ParseInt(v, 10, 64); err == nil {
				seed = s
			}
		}
		world = GenerateArena(seed)
	} else {
		world = BuildVila()
	}
	// MODO: primeiro modo (ffa, tdm, dom, koth, ctf, lms); depois os jogadores votam.
	// BOTS: quantos participantes a sala deve ter; bots completam o que faltar.
	bots := 6
	if v := os.Getenv("BOTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			bots = n
		}
	}
	game := NewGame(world, os.Getenv("MODO"), bots)
	go game.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", game.ServeWS)
	mux.HandleFunc("/map", game.ServeMap)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("servidor ARESTA ouvindo em %s (tick %d Hz, mapa %s, %d blocos, modo %s, sala de %d com bots)",
		addr, TickRate, world.Name, len(world.Boxes), game.modeDef().Name, bots)
	log.Fatal(http.ListenAndServe(addr, mux))
}
