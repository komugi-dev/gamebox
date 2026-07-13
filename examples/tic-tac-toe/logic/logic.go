package logic

import (
	"encoding/json"

	"github.com/komugi-dev/gamebox"
)

// TODO : write example
type T3Logic struct{}

func CreateT3() gamebox.GameRules {
	return &T3Logic{}
}

func (t *T3Logic) AddPlayer(playerId string) error {
	return nil
}

func (t *T3Logic) Start() (updatedStatus map[string]json.RawMessage,
	nextPlayers []string,
	err error) {
	return nil, nil, nil
}

func (t *T3Logic) Play(playerId string, move json.RawMessage) (
	updatedStatus map[string]json.RawMessage,
	nextPlayers []string,
	isGameOver bool,
	err error) {
	return nil, nil, true, nil
}
