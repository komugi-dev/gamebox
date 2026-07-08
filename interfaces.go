package gamebox

import "encoding/json"

// GameRules is the interface that a game engine must implement to be hosted by GameBox.
type GameRules interface {
	// AddPlayer() registers a player in the engine's internal state.
	AddPlayer(playerId string) error

	// Start() initializes the game.
	// It returns the initial state views for the players and the list of players
	// who are expected to make the first move.
	// The specific error returned depends on the game engine implementation.
	Start() (updatedStatus map[string]json.RawMessage,
		nextPlayers []string,
		err error)

	// Play() processes a move from a specific player.
	// It returns a map with the updated state view for each player, natively supporting hidden information.
	// GameBox will:
	// - broadcast the updated state only to the players present as keys in the updatedStatus map.
	// - send a "your turn" notification to every player listed in the nextPlayers slice.
	// It is the engine's responsibility to manage simultaneous turns (e.g., barrier synchronization)
	// and return nil/empty values until a new state is ready to be broadcasted.
	// Returning an error means a severe violation of the game's rules, a sign of a client misbehaving.
	// In such a case, the GameBox will ignore the other parameters.
	// It is the engine's responsibility to manage the follow-up.
	Play(playerId string, move json.RawMessage) (
		updatedStatus map[string]json.RawMessage,
		nextPlayers []string,
		isGameOver bool,
		err error)
}
