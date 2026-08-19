package logic

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/komugi-dev/gamebox"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func createGame() (gamebox.GameRules, *T3Logic) {
	tt := CreateT3()
	ptt, ok := tt.(*T3Logic)
	if !ok {
		return tt, nil
	}
	return tt, ptt
}

func createGameWithPlayers() (gamebox.GameRules, *T3Logic) {
	tt, ptt := createGame()
	tt.AddPlayer("1", "Alice")
	tt.AddPlayer("2", "Bob")
	return tt, ptt
}

func TestCreate(t *testing.T) {
	r, tt := createGame()
	assert.NotNil(t, r)
	assert.NotNil(t, tt)
	assert.Equal(t, tt.currPlayer, 1)
}

func TestAddPlayer(t *testing.T) {
	r, tt := createGame()
	curr := tt.currPlayer

	err := r.AddPlayer("1", "Alice")
	assert.Nil(t, err)
	assert.Equal(t, "1", tt.players[curr].Secret) // Aggiornato: ora accede a .Secret
	assert.Equal(t, "Alice", tt.players[curr].Public)
	curr = tt.currPlayer

	err = r.AddPlayer("2", "Bob")
	assert.Nil(t, err)
	assert.Equal(t, "2", tt.players[curr].Secret)

	err = r.AddPlayer("3", "Charlie")
	assert.NotNil(t, err)
}

func TestUtils(t *testing.T) {
	t.Run("next player", func(t *testing.T) {
		_, tt := createGame()
		idx := tt.nextPlayer()
		assert.Equal(t, idx, -1)
	})

	t.Run("update status", func(t *testing.T) {
		_, tt := createGameWithPlayers()
		tt.State.Board = []int{-1, -1, -1, 0, 0, 0, 1, 1, 1} // Aggiornato: assegna a .Board
		upd, err := tt.updatedStatus()
		assert.Nil(t, err)

		var exp T3State // Aggiornato: usa T3State per l'unmarshal
		for _, stat := range upd {
			err = json.Unmarshal(stat, &exp)
			assert.Nil(t, err)
			assert.Zero(t, slices.Compare(exp.Board, tt.State.Board))
		}
	})

	t.Run("move", func(t *testing.T) {
		_, tt := createGameWithPlayers()
		err := tt.move(tt.currPlayer, 0)
		assert.Nil(t, err)
		assert.Zero(t, slices.Compare(tt.State.Board, []int{tt.currPlayer, 0, 0, 0, 0, 0, 0, 0, 0}))

		// cell already occupied
		err = tt.move(tt.currPlayer, 0)
		assert.NotNil(t, err)

		// invalid player
		err = tt.move(0, 0)
		assert.NotNil(t, err)

		// invalid cell
		err = tt.move(tt.currPlayer, 9)
		assert.NotNil(t, err)

		// invalid cell
		err = tt.move(tt.currPlayer, -1)
		assert.NotNil(t, err)
	})

	t.Run("winner", func(t *testing.T) {
		_, tt := createGameWithPlayers()
		emptyBoard := slices.Repeat([]int{0}, 9)

		for _, i := range winChecks {
			board := slices.Clone(emptyBoard)
			for _, cell := range i {
				board[cell] = tt.currPlayer
			}
			tt.State.Board = board                              // Aggiornato: assegna a .Board
			gameOver, winnerSecret, winnerId := tt.isGameOver() // Aggiornato: gestisce i 3 ritorni
			assert.True(t, gameOver)
			assert.Equal(t, tt.players[tt.currPlayer].Secret, winnerSecret)
			assert.Equal(t, tt.currPlayer, winnerId)
		}
	})
}

func TestStart(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	r, _ := createGameWithPlayers()
	stat := slices.Repeat([]int{0}, 9)
	var upd T3State // Aggiornato: usa T3State

	updatedStatus, nextPlayers, err := r.Start()
	assert.Nil(t, err)

	for _, p := range []string{"1", "2"} {
		err = json.Unmarshal(updatedStatus[p], &upd)
		assert.Nil(t, err)
		assert.Zero(t, slices.Compare(stat, upd.Board)) // Compara .Board
	}
	assert.Equal(t, "1", nextPlayers[0])
}

func TestGame(t *testing.T) {
	r, _ := createGameWithPlayers()

	game := []struct {
		p     string
		moves []int
	}{
		{
			moves: []int{4, 0, 8},
			p:     "1",
		},
		{
			moves: []int{1, 7},
			p:     "2",
		},
	}
	expBoard := []int{
		1, -1, 0,
		0, 1, 0,
		0, -1, 1,
	}

	upd, pl, err := r.Start()
	assert.Nil(t, err)
	log.Debugf("idx:%v will start", pl)

	isGameOver := false
	turn := 0
	for !isGameOver {
		for i := 0; i < 2 && !isGameOver; i++ {
			moveJson, err := json.Marshal(game[i].moves[turn])
			log.Debugf("idx:%v; move:%v", i, game[i].moves[turn])
			assert.Nil(t, err)
			upd, pl, isGameOver, err = r.Play(game[i].p, moveJson)
			assert.Nil(t, err)
		}
		turn++
	}
	assert.Equal(t, 1, len(pl))
	assert.Equal(t, "1", pl[0])

	var endState T3State // Aggiornato: usa T3State
	json.Unmarshal(upd["1"], &endState)
	assert.Zero(t, slices.Compare(expBoard, endState.Board))
	json.Unmarshal(upd["2"], &endState)
	assert.Zero(t, slices.Compare(expBoard, endState.Board))
}
