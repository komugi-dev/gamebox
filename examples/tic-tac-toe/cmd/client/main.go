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
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"

	client "github.com/komugi-dev/gamebox/client"
	"github.com/komugi-dev/gamebox/examples/clientutil"
)

const gameboxURL = "http://localhost:8181/gamebox/v1"

const (
	PlayerJoined = "player_joined"
)

type T3State struct {
	Board   []int          `json:"grid"`
	Players map[int]string `json:"players,omitempty"` // map player index -> public name (to support "the winner is..." feature)
	Winner  int            `json:"winner,omitempty"`  // board id of the winner
	Tag     string         `json:"tag,omitempty"`
}

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
	fmt.Println("└───┴───┴───┘")
	fmt.Println()
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

// setupGame() instantiates a new tic-tac-toe player
func setupGame(ctx context.Context, gameboxURL string, playerName string) (*client.Player, error) {

	// setup player
	gbURL, err := url.Parse(gameboxURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse gamebox URL; %w", err)
	}
	pl, err := clientutil.SetupPlayer(ctx, gbURL, "tic-tac-toe", playerName)
	if err != nil {
		return nil, fmt.Errorf("error setup player; %w", err)

	}

	// handle graceful disconnection
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nquitting the table...")
		pl.Quit(ctx)
		os.Exit(0)
	}()

	return pl, nil
}

// the default is human player.
// "-bot" flag runs in auto mode.
func main() {
	log.SetLevel(log.ErrorLevel)

	// human or bot
	isBot := flag.Bool("bot", false, "Play randomly as a bot.")
	flag.Parse()
	switch *isBot {
	case true:
		fmt.Println("--- BOT player ---")
	case false:
		fmt.Println("--- Human player ---")
	}
	playerName := clientutil.RandomPlayerName()
	fmt.Printf("My name is %v\n", playerName)

	// setup game
	ctx := context.Background()
	pl, err := setupGame(ctx, gameboxURL, playerName)
	if err != nil {
		log.Fatalf("failed to setup the game, quitting... [%v]", err)
	}

	// game loop
	state := T3State{}
	justPlayed := false // enhance the UX
	for {
		typ, msg, winners, err := pl.GetMsg(ctx)
		if err != nil {
			log.Errorf("error getting msg; %v; unrecoverable, exiting...", err)
			return
		}
		log.Infof("recv msg %v", string(msg))

		// error
		if typ == client.MsgTypeError {
			log.Errorf("error received; %v", string(msg))
			continue
		}

		// move
		err = json.Unmarshal(msg, &state)
		if err != nil {
			log.Errorf("wrong payload; %v", err)
			continue
		}

		board := state.Board

		switch typ {

		case client.MsgTypeYourTurn:
			log.Info("playing my turn!")

			var move int
			if *isBot {
				move = randomBotMove(board)
				fmt.Printf("BOT moves to cell %v\n", move)
			} else {
				move = readUserInput(board)
			}

			moveJSON, _ := json.Marshal(move)
			err = pl.SendMsg(ctx, moveJSON)
			if err != nil {
				log.Errorf("Error sending the move: %v", err)
			}
			justPlayed = true

		case client.MsgTypeState:
			if state.Tag == PlayerJoined {
				fmt.Println("Opponent joined...")
				break
			}
			if !justPlayed {
				fmt.Println("Updating the board...")
			} else {
				justPlayed = false
			}
			drawBoard(board)

		case client.MsgTypeGameOver:
			log.Info("updating the board!")
			if !justPlayed {
				fmt.Println("Updating the board...")
			}
			drawBoard(board)

			if len(winners) == 0 {
				fmt.Print("\n--- DRAW! ---\n")
				return
			}
			if winners[0] == pl.Guid {
				fmt.Print("\n=== You win! ===\n")
			} else {
				fmt.Print("\n+++ You lose! +++\n")
				fmt.Printf("\nThe winner is %v\n", state.Players[state.Winner])
			}
			return
		}
	}
}
