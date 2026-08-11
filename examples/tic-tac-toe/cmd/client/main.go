package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/guid"
	log "github.com/sirupsen/logrus"

	client "github.com/komugi-dev/gamebox/client"
)

func setupPlayer(ctx context.Context, pName string) (*client.Player, error) {
	var joinedTable client.Table

	gbURL, err := url.Parse("http://localhost:8181/gamebox/v1")
	if err != nil {
		return nil, fmt.Errorf("cannot parse gamebox URL; %w", err)
	}
	cli := http.Client{
		Timeout: 10 * time.Second,
	}

	// connect to gamebox
	ss := client.CreateSession(&cli, *gbURL)

	// create a player
	pl := client.CreatePlayer(pName, ss)

	// get tables
	existingTables, err := ss.Tables(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot get tables; %w", err)
	}
	log.Infof("found %v tables", len(existingTables))

	// as there are 2 players per table,
	// if joining an existing table works, start playing...
	for _, t := range existingTables {
		log.Infof("joining table %+v", t)
		err = pl.Join(ctx, t)
		if err != nil {
			log.Warningf("cannot join table %v (%v)", t.Summary.TableGUID, err)
			continue
		}
		joinedTable = t
		err = joinedTable.Start(ctx)
		if err != nil {
			log.Warningf("cannot start table; %v", err)
			continue
		}
		log.Infof("joined and started table %+v", joinedTable.Summary)
		break
	}
	// ...otherwise, create a new table and wait for another player
	if joinedTable.Summary.TableGUID == "" {
		joinedTable, err = ss.Table(ctx, "tic-tac-toe-"+guid.NewString())
		if err != nil {
			return nil, fmt.Errorf("cannot create new table; %w", err)
		}
		log.Infof("created new table %+v", joinedTable.Summary)

		log.Infof("joining table %+v", joinedTable)
		err = pl.Join(ctx, joinedTable)
		if err != nil {
			return nil, fmt.Errorf("cannot join table %v (%v)", joinedTable.Summary.TableGUID, err)
		}
	}

	return pl, nil
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

func main() {
	log.SetLevel(log.DebugLevel)

	ctx := context.Background()
	pl, err := setupPlayer(ctx, "player1")
	if err != nil {
		log.Fatalf("error setup player; %v", err)
	}
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
			move := readUserInput(board)
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
