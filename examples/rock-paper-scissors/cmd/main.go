package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/komugi-dev/gamebox"
	"github.com/komugi-dev/gamebox/client"
	"github.com/komugi-dev/gamebox/examples/rock-paper-scissors/logic"
)

const GameboxPort = 8182

func main() {
	log.SetLevel(log.ErrorLevel)

	// server setup
	meta := gamebox.EngineMeta{
		Name:       "rock-paper-scissor",
		Version:    "v0.0.1",
		Desc:       "An example of hidden information, asynchronous game built on gamebox",
		MinPlayers: 2,
	}
	engine := gamebox.NewEngine(meta, logic.CreateRPSLogic)

	go func() {
		fmt.Printf("Starting gamebox on port %v\n", GameboxPort)
		err := engine.Run(GameboxPort)
		if err != nil {
			log.Fatalf("Gamebox error: %v", err)
		}
	}()

	time.Sleep(time.Second)

	// client setup and session
	gameboxURL, _ := url.Parse(fmt.Sprintf("http://localhost:%v/gamebox/v1", GameboxPort))
	session := client.CreateSession(nil, *gameboxURL)
	ctx := context.Background()

	// table
	table, err := session.Table(ctx, "bot-simluator-table")
	if err != nil {
		log.Fatalf("error creating table: %v", err)
	}
	fmt.Printf("Table ready! Guid: %s\n", table.Summary.TableGUID)

	// bots creation
	botNames := []string{"Alice", "Bob", "Charlie", "Dave", "Eve"}
	players := make([]*client.Player, len(botNames))

	fmt.Println("Connecting bots...")
	for i, name := range botNames {
		p := client.CreatePlayer(name, session)
		err := p.Join(ctx, table)
		if err != nil {
			log.Fatalf("join error for %s: %v", name, err)
		}
		players[i] = p
		fmt.Printf("   -> %s joined the table (GUID: %s)\n", p.Name, p.Guid)
	}

	// game over
	gameOverChan := make(chan string, 1)

	choicesName := map[int]string{
		logic.Rock:    "Rock",
		logic.Paper:   "Paper",
		logic.Scissor: "Scissor",
	}

	// start bots
	for _, p := range players {
		go func(bot *client.Player) {
			turn := 1
			rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

			for {
				msgType, payload, _, err := bot.GetMsg(ctx)
				if err != nil {
					return
				}

				switch msgType {

				case client.MsgTypeYourTurn:

					// my turn to play
					choice := rnd.Intn(3)
					moveJSON := []byte(fmt.Sprintf(`{"choice": %d}`, choice))

					fmt.Printf("Turn: %v - [%s] my turn: %s\n", turn, bot.Name, choicesName[choice])

					err := bot.SendMsg(ctx, moveJSON)
					if err != nil {
						log.Printf("error sending moves for %s: %v", bot.Name, err)
					}
					turn++

				case client.MsgTypeGameOver:

					// game over, let's see who won
					var rpsPayload logic.MsgPlayerPayload
					json.Unmarshal(payload, &rpsPayload)

					winnerName := ""
					if len(rpsPayload.Turn.Survivors) == 1 {
						winnerName = rpsPayload.Turn.Survivors[0]
					}

					select {
					case gameOverChan <- winnerName:
					default:
					}

					return
				}
			}
		}(p)
	}

	time.Sleep(1 * time.Second)

	// the first player starts the game
	err = players[0].Table.Start(ctx)
	if err != nil {
		log.Fatalf("error starting the table: %v", err)
	}

	// waiting for the game end
	winner := <-gameOverChan

	fmt.Println("\n==============================================")
	fmt.Println("GAME OVER!")
	fmt.Printf("And the winner is: %s\n", winner)
	fmt.Println("==============================================")
}
