package gamebox

import "encoding/json"

// A game satisfying this interface supports GameBox
type GameRules interface {
	// the game is aware of the players
	AddPlayer(playerId string) error

	// start the game
	Start() (updatedStatus map[string]json.RawMessage, err error)

	// the next player in turn
	NextPlayer() (playerId string, err error)

	// A player moves.
	// The game returns a map with the view status for each player.
	// Hidden information natively supported.
	Play(playerId string, move json.RawMessage) (
		updatedStatus map[string]json.RawMessage,
		isGameOver bool,
		err error)
}
