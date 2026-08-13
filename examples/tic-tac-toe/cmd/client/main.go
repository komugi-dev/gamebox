package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	client "github.com/komugi-dev/gamebox/client"
	"github.com/komugi-dev/gamebox/examples/clientutil"
)

const gameboxURL = "http://localhost:8181/gamebox/v1"

// drawBoard prints the game grid
func drawBoard(board []int) {
	// printing helper
	getSymbol := func(i int) string {
		switch board[i] {
		case 1:
			return "\033[32mX\033[0m" // green X
		case -1:
			return "\033[31mO\033[0m" // red O
		default:
			// print the index on empty cells
			return fmt.Sprintf("%d", i)
		}
	}

	fmt.Println("\n┌───┬───┬───┐")
	fmt.Printf("│ %s │ %s │ %s │\n", getSymbol(0), getSymbol(1), getSymbol(2))
	fmt.Println("├───┼───┼───┤")
	fmt.Printf("│ %s │ %s │ %s │\n", getSymbol(3), getSymbol(4), getSymbol(5))
	fmt.Println("├───┼───┼───┤")
	fmt.Printf("│ %s │ %s │ %s │\n", getSymbol(6), getSymbol(7), getSymbol(8))
	fmt.Println("└───┴───┴───┘\n")
}

// readUserInput reads the input.
// It accepts a number between 0-9 associated with an empty cell.
// It asks again if the input is not a number or the cell is not empty.
func readUserInput(board []int) int {
	reader := bufio.NewReader(os.Stdin)

	for {
		available := []string{}
		for idx, val := range board {
			if val == 0 {
				available = append(available, fmt.Sprintf("%d", idx))
			}
		}
		fmt.Printf("\nYour turn, choose the cell (available: %s): ", strings.Join(available, ","))

		// read until enter is pressed
		inputStr, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Read error, retry.")
			continue
		}

		// clean the string
		inputStr = strings.TrimSpace(inputStr)

		// convert to an integer
		move, err := strconv.Atoi(inputStr)
		if err != nil || move < 0 || move > 8 {
			fmt.Println("Invalid input, it must be a number between 0 and 8.")
			continue
		}

		// validate the game logic
		if board[move] != 0 {
			fmt.Println("Please, select an empty cell.")
			continue
		}

		return move
	}
}

// randomBotMove returns a random move.
func randomBotMove(board []int) int {
	available := []int{}
	for idx, val := range board {
		if val == 0 {
			available = append(available, idx)
		}
	}

	if len(available) == 0 {
		log.Error("bot cannot find the right move, board full")
		return -1
	}

	moveIdx := rand.Int32N((int32)(len(available)))
	move := available[moveIdx]
	log.Infof("bot moves to cell %v", move)
	return move
}

// the default is human player.
// "-bot" flag runs in auto mode.
func main() {
	log.SetLevel(log.DebugLevel)

	// human or bot
	isBot := flag.Bool("bot", false, "Play randomly as a bot.")
	flag.Parse()
	switch *isBot {
	case true:
		fmt.Print("--- BOT player ---")
	case false:
		fmt.Print("--- Human player ---")
	}

	// setup game
	ctx := context.Background()
	gbURL, err := url.Parse(gameboxURL)
	if err != nil {
		log.Fatalf("cannot parse gamebox URL; %v", err)
	}
	pl, err := clientutil.SetupPlayer(ctx, gbURL, "player1", "tic-tac-toe")
	if err != nil {
		log.Fatalf("error setup player; %v", err)
	}

	// game loop
	for {
		typ, msg, winners, err := pl.GetMsg(ctx)
		if err != nil {
			log.Fatalf("error getting msg; %v", err)
		}
		log.Infof("recv msg %v", string(msg))
		board := []int{}
		switch typ {
		case client.MsgTypeYourTurn:
			log.Info("playing my turn!")
			err = json.Unmarshal(msg, &board)
			if err != nil {
				log.Errorf("wrong payloed; %v", err)
			}

			var move int
			if *isBot {
				move = randomBotMove(board)
			} else {
				move = readUserInput(board)
			}

			moveJSON, _ := json.Marshal(move)
			err = pl.SendMsg(ctx, moveJSON)
			if err != nil {
				log.Errorf("Errore invio mossa: %v", err)
			}

		case client.MsgTypeState:
			log.Info("updating the board!")
			err = json.Unmarshal(msg, &board)
			if err != nil {
				log.Errorf("wrong payloed; %v", err)
			}
			drawBoard(board)

		case client.MsgTypeGameOver:
			log.Info("updating the board!")
			err = json.Unmarshal(msg, &board)
			if err != nil {
				log.Errorf("wrong payloed; %v", err)
			}
			drawBoard(board)

			if len(winners) == 0 {
				fmt.Print("\n--- DRAW! ---\n")
				return
			}
			if winners[0] == pl.Guid {
				fmt.Print("\n=== You win! ===\n")
			} else {
				fmt.Print("\n+++ You loose! +++\n")
			}
			return

		case client.MsgTypeError:
			log.Errorf("error received; %v", string(msg))
		}
	}
}
