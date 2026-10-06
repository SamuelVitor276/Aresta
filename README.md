# Aresta

FPS multiplayer low poly que roda no navegador. Seis modos de jogo com votação entre partidas, bots que completam a sala, vida que se recupera com o tempo, chat, três armas (fuzil, sniper e escopeta), faca e granadas, num mapa feito à mão: a Vila Poente, uma cidade murada com praça, becos, ruínas e torres. O servidor é autoritativo e escrito em Go, o cliente usa Three.js, e tudo (mapa, modelos, efeitos e sons) é gerado por código. Sobe com um comando via Docker.

## Rodando

Você só precisa do Docker (no Windows e no Mac, o Docker Desktop).

```bash
docker compose up --build
```

Abra http://localhost:8080, digite um nome e clique em **Entrar na arena**.

Para jogar com amigos na mesma rede, eles abrem `http://IP-DA-SUA-MÁQUINA:8080` (no Windows, `ipconfig` mostra o IP). Libere a porta 8080 no firewall.

Para parar, use `Ctrl+C`, ou `docker compose down` se tiver subido com `-d`. Se a porta 8080 estiver ocupada, troque `"8080:80"` no `docker-compose.yml` por outra, por exemplo `"3000:80"`.

## Jogar com quem está fora da sua rede

Dá para gerar um link público temporário, sem domínio e sem abrir porta no roteador (funciona até com CGNAT):

```bash
docker compose --profile tunel up --build
```

No meio do log aparece um link `https://algumas-palavras-aleatorias.trycloudflare.com`. Espere uns 30 segundos antes de abrir ou mandar para alguém: o endereço demora um pouco para existir no DNS, e quem tenta cedo demais pode ficar um bom tempo vendo "site não encontrado". Se acontecer, derrube e suba de novo para ganhar um link novo.

Para achar o link depois: `docker compose logs tunel` e procure a linha com `trycloudflare.com`.

O link muda toda vez que o túnel sobe e para de funcionar quando você derruba. Quem tiver o link entra na partida, então derrube quando terminar. O tráfego passa pela Cloudflare, o que acrescenta um pouco de latência; o ping aparece no canto da tela.

Se o atraso incomodar, a alternativa é o Tailscale: vocês dois instalam e criam conta (é grátis para uso pessoal), você compartilha a sua máquina com o seu amigo pelo painel (Machines, depois "Share") e ele abre `http://IP-DO-TAILSCALE-DA-SUA-MÁQUINA:8080`. Normalmente a conexão é direta entre os dois computadores.

## Controles

| Tecla | Ação |
|---|---|
| W A S D | andar |
| Espaço | pular |
| Botão esquerdo | atirar |
| Botão direito | mirar (zoom; na sniper, a luneta) |
| V ou botão do meio | faca (sem trocar de arma) |
| G | granada (duas por vida) |
| R | recarregar (também recarrega sozinho quando o pente acaba) |
| 1, 2, 3 | arma da próxima vida: fuzil, sniper, escopeta (na votação, votam) |
| Tab | placar |
| Enter | chat (Enter envia, Esc cancela) |
| Esc | soltar o mouse (a partida continua) |

## Modos de jogo

| Modo | Como vence |
|---|---|
| Todos contra todos (`ffa`) | 25 abates primeiro (ou quem tiver mais em 6 min) |
| Mata-mata em equipe (`tdm`) | equipe que fizer 50 abates (ou mais em 6 min) |
| Dominação (`dom`) | três pontos (A na rua oeste, B na fonte, C na rua leste). Cada ponto da equipe soma 1 por segundo; 200 vencem |
| Rei da colina (`koth`) | uma colina que muda de lugar a cada minuto. Só pontua quem está nela sem inimigos; 120 vencem |
| Captura a bandeira (`ctf`) | pegue a bandeira inimiga e leve até a sua, com a sua em casa. 3 capturas vencem. Quem morre deixa a bandeira cair; tocar a própria bandeira caída devolve, e sozinha ela volta em 20 s |
| Sobrevivente (`lms`) | ninguém renasce durante a rodada; o último de pé vence a rodada. Quem vencer 3 rodadas leva a partida. Quem morre assiste aos vivos (clique troca) |

Nos modos de equipe não existe fogo amigo: as balas atravessam os colegas (a sua própria granada ainda machuca você). As equipes são equilibradas no começo de cada partida, distribuindo primeiro as pessoas e depois os bots.

**Votação:** quando a partida acaba, o resultado aparece por 7 s e abre a votação com três modos diferentes do atual. Vote com 1, 2 ou 3 (ou clicando no cartão, com o mouse solto). O mais votado vence; empate ou ninguém votando é sorteado. Depois vem uma contagem de 10 s com aquecimento e a partida nova começa.

O primeiro modo é o `MODO` do `docker-compose.yml`.

## Bots

`BOTS` define o tamanho da sala: com `BOTS: "6"` e duas pessoas, entram quatro bots. Quando chega gente, sai um bot; quando a última pessoa sai, os bots saem também.

Os bots não trapaceiam na física: geram teclas e mira como uma pessoa. Eles acham caminho pelo mapa (A* numa grade de 0,5 m) e só enxergam quem está no campo de visão, com parede bloqueando. Reagem com atraso (0,33 a 0,63 s ao ver alguém; 0,25 a 0,45 s quando o alvo some e reaparece) e com um erro de mira que diminui enquanto acompanham o alvo. Também miram onde você estava ~70 ms antes, como alguém tentando acompanhar: correr de lado e mudar de direção faz eles errarem. Com escopeta, miram no peito.

Para medir a dificuldade, `go test -v -run TestBotHabilidade` roda cenários fixos (duelo a 20 m, peek, janela, escopeta) e mostra os números. Os valores ficam no topo de `bot.go`. Cada um escolhe uma arma por vida e se posiciona conforme ela: escopeta chega perto, sniper fica longe e mira. Também usam faca e granada. Em cada modo jogam o objetivo: vão aos pontos sem dono, sobem na colina, atacam ou defendem a bandeira.

O que eles ainda não fazem: subir escadas (andam só pelo térreo) e jogar a bandeira com muita esperteza.

## Cura

Como no Phantom Forces: depois de 5 s sem tomar dano, a vida volta sozinha, 20 por segundo. Qualquer dano recomeça a contagem. A barra de vida pulsa enquanto cura.

## Chat

Enter abre o campo, Enter envia e Esc cancela; enquanto você digita, o boneco fica parado. As mensagens somem da tela depois de 10 s e voltam a aparecer quando você abre o chat. O servidor tira caracteres de controle, corta em 120 letras e segura quem manda mais de quatro mensagens seguidas (libera uma a cada 1,5 s).

## Armas

Você escolhe a arma principal no menu e pode trocar com 1/2/3. A troca vale a partir da próxima vida, então cada escolha pesa. A faca e as granadas estão sempre à mão.

| Arma | Dano (corpo / cabeça) | Pente | Onde brilha |
|---|---|---|---|
| Fuzil | 24 / 55, perde força depois de 25 m | 30, ~10 tiros/s | praça, ruas, pátio de containers |
| Sniper | 80 / 200 (derruba) | 5, um tiro a cada 1,2 s | torre do sino, bastiões, telhado do armazém |
| Escopeta | 9 projéteis de 13 / 18, cai muito com a distância; só marca tiro na cabeça quando a maioria dos projéteis pega nela | 6 | becos, casas, ruínas |
| Faca | 55 de frente, derruba pelas costas | — | quem chega pelas costas |
| Granada | até 120, raio de 7 m; parede protege, e fere você também | 2 | tirar alguém de trás da cobertura |

A mira com o botão direito leva 0,2 s para ficar precisa (não existe "quickscope"), e mirando você anda mais devagar. Correndo ou pulando, os tiros abrem mais. A mira da tela mostra a abertura real da arma no momento.

## O mapa: Vila Poente

Cidade murada de 104 × 84 m no fim de tarde. Norte é para onde o sol se põe.

- **Praça (centro):** aberta, com fonte, carros, barracas e sacos de areia. Terreno do fuzil.
- **Igreja e torre do sino (norte):** a nave tem janelas altas; o campanário, a 7,5 m, vê a praça inteira.
- **Becos (leste):** casas com interior, quintais murados, sobrados com escada e uma casa em ruínas. Escopeta e faca.
- **Ruínas (sul):** um sobrado desabado com rampa de escombros até o andar de cima, montes de entulho e paredes quebradas.
- **Pátio de containers (oeste):** corredores longos e uma pilha com mirante a 5,2 m.
- **Armazém (noroeste):** prateleiras por dentro, mezanino e telhado acessível por fora.
- **Bastiões (cantos nordeste e sudoeste):** plataformas a 7 m na muralha, para sniper.
- **Jardim (sudeste):** oliveiras e muretas baixas.

A arena pequena da primeira versão continua disponível: troque `MAPA: "vila"` por `MAPA: "arena"` no `docker-compose.yml`.

## Estrutura

```
aresta/
├── docker-compose.yml   server (Go), client (nginx) e o túnel opcional
├── server/              servidor autoritativo
│   ├── main.go          escolhe o mapa e sobe o HTTP (/ws, /map, /health)
│   ├── game.go          tick de 60 Hz, tiros, faca, granadas, dano, cura, respawn
│   ├── match.go         modos, fases da partida, votação, equipes, pontos e bandeiras
│   ├── bot.go           o "cérebro" dos bots
│   ├── nav.go           grade de navegação e A*
│   ├── net.go           WebSocket: entrada/saída, inputs, escolha de arma
│   ├── physics.go       física, armas e granadas (ESPELHO de physics.js)
│   ├── world.go         ferramentas de mapa: paredes com portas e janelas, casas, escadas
│   ├── map_vila.go      a Vila Poente, bairro por bairro
│   ├── map_arena.go     a arena pequena e sorteada
│   ├── protocol.go      formato das mensagens JSON
│   ├── *_test.go        testes: física, armas, modos, votação, bots, mapa e rede
│   └── Dockerfile       compila, roda os testes e gera uma imagem distroless
└── client/
    ├── Dockerfile       baixa o three.js do npm e monta o nginx
    ├── nginx.conf       serve a página e repassa /ws e /map para o servidor
    └── public/
        ├── index.html   canvas, menu (com escolha de arma) e HUD
        ├── style.css
        └── js/
            ├── main.js      loop do jogo: predição, mira, granadas, eventos, interpolação
            ├── physics.js   física, armas e granadas (ESPELHO de physics.go)
            ├── net.js       conexão WebSocket
            ├── input.js     teclado, mouse e pointer lock
            ├── world.js     céu, terreno, cores por tipo de bloco, pisos e detalhes
            ├── models.js    boneco, as três armas, faca e granada
            ├── effects.js   partículas, rastros, clarões e explosões
            ├── audio.js     todos os sons, sintetizados com Web Audio
            └── hud.js       vida, munição, granadas, mira, luneta, feed, placar
```

## Como funciona

**O servidor manda.** O navegador só envia "quais teclas estão apertadas e para onde estou olhando". O servidor Go simula todo mundo e decide quem acertou quem. Isso impede trapaça básica e resolve empates (dois tiros no mesmo instante).

**Ritmo fixo.** O servidor roda 60 ticks por segundo e manda o estado do jogo (snapshot) 30 vezes por segundo. Cada input do cliente vale exatamente 1/60 s de movimento.

**Predição e reconciliação.** Para não esperar o servidor, o cliente roda a mesma física e move o seu boneco na hora. Quando chega um snapshot, ele volta para o estado oficial e reaplica os inputs que o servidor ainda não processou. Como a física é idêntica, normalmente nada muda na tela.

**Interpolação.** Os outros jogadores são desenhados 100 ms no passado, deslizando entre dois snapshots. Isso esconde a variação de latência da rede.

**Compensação de lag.** Cada tiro informa o instante que o atirador estava vendo. O servidor guarda cerca de 1 s de posições e testa o tiro contra onde o alvo estava na tela de quem atirou. Assim você mira no inimigo, não na frente dele.

**Tiros e granadas iguais nos dois lados.** A dispersão de cada projétil é sorteada a partir do número do input, com a mesma conta no Go e no JavaScript. Então os rastros que você vê saindo da sua arma são exatamente os que o servidor testa. As granadas também: o servidor manda só o lançamento, e cada cliente simula os quiques sozinho, chegando ao mesmo ponto da explosão.

**Mensagens** (JSON via WebSocket em `/ws?name=Fulano&arma=0`):

- cliente → servidor: `i` (input: sequência, teclas, yaw, pitch e, ao atirar ou esfaquear, o instante visto), `arma` (arma da próxima vida), `chat`, `voto` e `ping`
- servidor → cliente: `welcome` (seu id, o modo e o mapa), `snap` (estado de todos, seu estado completo, a partida com placar, objetivos e votação, último input confirmado e eventos como tiro, granada, acerto, abate, chat, captura e bandeira), `aviso` e `pong`

## A regra de ouro

`server/physics.go` e `client/public/js/physics.js` precisam ser idênticos em comportamento. Mudou velocidade, pulo, gravidade, colisão, arma ou granada em um? Mude no outro. Se divergirem, o seu boneco vai "dar tranco" sempre que o servidor corrigir a posição, e os rastros dos tiros deixam de bater com os acertos.

## Ajustando o jogo

| O quê | Onde |
|---|---|
| Velocidade, pulo, gravidade, degrau | `P` em `physics.js` e as variáveis no topo de `physics.go` |
| Dano, pente, cadência, abertura, alcance de cada arma | `WEAPONS` em `physics.js` e `Weapons` em `physics.go` (o dano só existe no Go) |
| Faca e granadas | `KNIFE` / `NADE` em `physics.js` e as variáveis `Knife…` / `Nade…` em `physics.go` |
| Vida, cura e tempo de respawn | constantes no topo de `game.go` (`RegenDelay`, `RegenPerSec`) |
| Limites e tempo de cada modo | `Modes` em `match.go` |
| Contagem, resultado, votação | `initMatch` em `match.go` |
| Pontos, colinas, bandeiras e spawns das equipes | `vilaObjetivos` em `map_vila.go` |
| Dificuldade dos bots (reação, atraso da mira, tiros na cabeça) | constantes `Bot…` no topo de `bot.go` |
| Limite do chat | `ChatBurst` e `ChatEvery` em `game.go` |
| Zoom de cada arma | `ADS_FOV` em `main.js` |
| Cores dos blocos e pisos | `KIND` e `ZONE` em `world.js`; `PLAYER_COLORS` em `models.js` |
| Sons | `audio.js` (cada som é uma função curta) |
| Sensibilidade do mouse | `sens` em `input.js` |

### Mexendo no mapa

O mapa é código em `server/map_vila.go`, montado com algumas ferramentas de `world.go`:

```go
// casa térrea de 8 × 8 m, porta ao sul, janela a leste, terraço com mureta
w.casa(19, -18, 27, -10, 2,
    sides{s: []op{door(21, 1.4)}, e: []op{win(-12, 1.2)}},
    sides{})
// escada de 16 degraus subindo para leste até 4 m
w.stairs("minha escada", 19.4, 1.0, 'E', 1.4, 16, 0.25, 0.3, 0, 0, "stair", 0)
```

Os testes conferem o mapa por você: `go test` sobe cada escada até o topo, atravessa cada porta e garante que todos os spawns estão ligados pelo chão. Se alguma coisa ficar bloqueada, o teste diz qual.

## Desenvolvimento

**Editar o JS sem rebuildar:** descomente o bloco `volumes` do serviço `client` no `docker-compose.yml`, suba de novo e é só dar F5 depois de salvar.

**Testes do servidor:** com Go instalado, `cd server && go test ./...`. Sem Go, `docker compose build server`, porque o build roda os testes e falha se algum quebrar. Os testes cobrem física, armas, cura, os seis modos, a votação, o chat, o mapa inteiro (escadas, portas, objetivos alcançáveis) e partidas de 90 s só de bots, além de combate e chat de verdade pela rede.

**Logs:** `docker compose logs -f server` mostra quem entra e sai.

## Próximos passos sugeridos

1. Bots subindo escadas (navegação em vários andares) e jogando a bandeira melhor
2. Zona que fecha no modo sobrevivente
3. Agachar e deitar (a física e as hitboxes mudam: lembre da regra de ouro)
4. Indicador de direção do dano
5. Mensagens binárias no lugar de JSON (menos banda com muitos jogadores)
6. Ranking persistente com um banco (Postgres ou Redis) em outro container

## Colocando na internet

- Num servidor com Docker, o mesmo `docker compose up -d` funciona.
- Coloque um proxy com HTTPS na frente (Caddy ou Traefik resolvem o certificado sozinhos). O cliente passa a usar `wss://` automaticamente quando a página é HTTPS.
- Em `server/net.go`, troque o `CheckOrigin` que aceita tudo por uma comparação com o seu domínio.
