package logic

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/komugi-dev/gamebox"
	log "github.com/sirupsen/logrus"
)

// TODO : write example

const numPlayers = 2

var winChecks = [8][3]int{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // cols
	{0, 4, 8}, {2, 4, 6}, // diags
}

type T3Player struct {
	Secret string
	Public string
}

type T3Logic struct {
	Board      []int `json:"grid"`
	players    map[int]T3Player
	currPlayer int
	mu         sync.RWMutex
}

func CreateT3() gamebox.GameRules {
	t3Logic := T3Logic{
		Board:      make([]int, 9),
		players:    make(map[int]T3Player, 0),
		currPlayer: 1,
	}

	return &t3Logic
}

func (t *T3Logic) nextPlayer() (playerIdx int) {
	t.currPlayer = t.currPlayer * -1
	return t.currPlayer
}

func (t *T3Logic) updatedStatus() (updatedStatus map[string]json.RawMessage, err error) {
	// get the board
	jsonGrid, err := json.Marshal(t.Board)
	if err != nil {
		return
	}

	// update status
	updatedStatus = make(map[string]json.RawMessage, len(t.players))
	for _, v := range t.players {
		updatedStatus[v.Secret] = jsonGrid
	}
	return
}

func (t *T3Logic) move(playerIdx int, move int) error {
	if move < 0 || move > 8 {
		return fmt.Errorf("wrong move: %v", move)
	}
	if playerIdx != -1 && playerIdx != 1 {
		return fmt.Errorf("wrong playerIdx: %v", playerIdx)
	}
	if t.Board[move] != 0 {
		return fmt.Errorf("cell already occupied")
	}
	t.Board[move] = playerIdx
	return nil
}

func (t *T3Logic) isGameOver() (gameOver bool, winner string) {

	// check winner
	winCond := 3 * t.currPlayer
	for _, i := range winChecks {
		partial := 0
		for _, cell := range i {
			partial += t.Board[cell]
		}
		if partial == winCond {
			return true, t.players[t.currPlayer].Secret
		}
	}

	// check draw
	gameOver = true
	for _, cell := range t.Board {
		if cell == 0 {
			gameOver = false
			break
		}
	}
	return
}

func (t *T3Logic) AddPlayer(playerId string, playerName string) error {
	log.Infof("ttt request to add player [%v][%v]", playerId, playerName)
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.players) == numPlayers {
		return fmt.Errorf("board full")
	}

	t.players[t.currPlayer] = T3Player{
		Secret: playerId,
		Public: playerName,
	}
	t.nextPlayer()

	return nil
}

func (t *T3Logic) Start() (updatedStatus map[string]json.RawMessage,
	nextPlayers []string,
	err error) {

	t.mu.Lock()
	defer t.mu.Unlock()

	updatedStatus, err = t.updatedStatus()
	if err != nil {
		return nil, nil, err
	}

	// get next player
	nextPlayers = append(nextPlayers, t.players[t.currPlayer].Secret)
	log.Debugf("ttt start - players[%v]; next[%v]", t.players, nextPlayers)

	return
}

func (t *T3Logic) Play(playerId string, move json.RawMessage) (
	updatedStatus map[string]json.RawMessage,
	nextPlayers []string,
	gameOver bool,
	err error) {

	t.mu.Lock()
	defer t.mu.Unlock()

	// check the player
	pId := t.players[t.currPlayer].Secret
	if pId != playerId {
		err = fmt.Errorf("wrong player, expected %v, got %v", pId, playerId)
		return
	}

	// apply the move
	var m int
	err = json.Unmarshal(move, &m)
	if err != nil {
		err = fmt.Errorf("unmarshal failed; player [%v]; %w", playerId, err)
		return
	}
	log.Debugf("pl:%v; move:%v", playerId, m)
	err = t.move(t.currPlayer, m)
	if err != nil {
		return
	}
	log.Debugf("board:%v", t.Board)

	// update the status
	updatedStatus, err = t.updatedStatus()
	if err != nil {
		return
	}

	// check winner / draw
	gameOver, winner := t.isGameOver()
	if len(winner) > 0 {
		nextPlayers = append(nextPlayers, winner)
	}
	if gameOver {
		return
	}

	// continue the game
	nextPlayers = append(nextPlayers, t.players[t.nextPlayer()].Secret)

	return
}

func (t *T3Logic) Info() gamebox.InstanceStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	i := gamebox.InstanceStatus{
		Players: len(t.players),
	}

	i.State = "lobby"
	if i.Players == 2 {
		i.State = "full"
	}

	return i
}
