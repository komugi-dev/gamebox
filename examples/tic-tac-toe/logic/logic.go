package logic

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/komugi-dev/gamebox"
	log "github.com/sirupsen/logrus"
)

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

type T3State struct {
	Board   []int          `json:"grid"`
	Players map[int]string `json:"players,omitempty"` // map player index -> public name (to support "the winner is..." feature)
	Winner  int            `json:"winner,omitempty"`  // board id of the winner
}

type T3Logic struct {
	State      T3State
	players    map[int]T3Player
	currPlayer int
	mu         sync.RWMutex
}

func CreateT3() gamebox.GameRules {
	t3Logic := T3Logic{
		State: T3State{
			Board:   make([]int, 9),
			Players: make(map[int]string, 2),
			Winner:  0,
		},
		players:    make(map[int]T3Player, 2),
		currPlayer: 1,
	}

	return &t3Logic
}

func (t *T3Logic) nextPlayer() (playerIdx int) {
	t.currPlayer = t.currPlayer * -1
	return t.currPlayer
}

func (t *T3Logic) updatedStatus() (updatedStatus map[string]json.RawMessage, err error) {
	// get the status
	jsonState, err := json.Marshal(t.State)
	if err != nil {
		return
	}

	// update status
	// in this game, the status can be the same for all the players;
	// but every player can receive a tailored view of the status if needed
	updatedStatus = make(map[string]json.RawMessage, len(t.players))
	for _, v := range t.players {
		updatedStatus[v.Secret] = jsonState
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
	if t.State.Board[move] != 0 {
		return fmt.Errorf("cell already occupied")
	}
	t.State.Board[move] = playerIdx
	return nil
}

// is GameOver() checks if a game can continue; it returns the winner in case it's done
func (t *T3Logic) isGameOver() (gameOver bool, winnerSecret string, winnerId int) {

	// check winner
	winCond := 3 * t.currPlayer
	for _, i := range winChecks {
		partial := 0
		for _, cell := range i {
			partial += t.State.Board[cell]
		}
		if partial == winCond {
			return true, t.players[t.currPlayer].Secret, t.currPlayer
		}
	}

	// check draw
	gameOver = true
	for _, cell := range t.State.Board {
		if cell == 0 {
			gameOver = false
			return
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
	t.State.Players[t.currPlayer] = playerName
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
	log.Debugf("board:%v", t.State)

	// check winner / draw
	gameOver, winnerSecret, winnerBoardId := t.isGameOver()
	if len(winnerSecret) > 0 {
		nextPlayers = append(nextPlayers, winnerSecret)
	}
	t.State.Winner = winnerBoardId

	// update the status
	updatedStatus, err = t.updatedStatus()
	if err != nil || gameOver {
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
