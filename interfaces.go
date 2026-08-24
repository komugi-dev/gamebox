package gamebox

import "encoding/json"

// EngineMeta contains immutable static data of the game.
type EngineMeta struct {
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	Desc       string          `json:"description"`
	MinPlayers int             `json:"min_players"`
	MaxPlayers int             `json:"max_players"`
	Extended   json.RawMessage `json:"extended,omitempty"`
}

// InstanceStatus represents the dynamic state of the table.
type InstanceStatus struct {
	State    string          `json:"state"`              // e.g. "lobby", "playing", "done"
	Players  int             `json:"players"`            // the current players
	Extended json.RawMessage `json:"extended,omitempty"` // extended info
}

// GameUpdate groups the fields returned by the interface functions.
type GameUpdate struct {
	Status      map[string]json.RawMessage
	NextPlayers []string
	IsGameOver  bool
}

// GameRules is the interface that a game engine must implement to be hosted by GameBox.
type GameRules interface {
	// Info() returns information about the game.
	// Values and meanings of the fields are responsibility of the game creator.
	Info() InstanceStatus

	// AddPlayer() registers a player in the engine's internal state.
	// - secret is the secret identifier created by the service
	// - public contains public data about the player
	// Developers can use the public string at their own convenience.
	// If the game allows late-joins, it should return the updated state.
	AddPlayer(secret string, public string) (GameUpdate, error)

	// RemovePlayer() deletes from the game a player who explicitly quits the table.
	// The game logic decides how to handle this event, e.g.:
	// - continuing the game
	// - putting the game in stand-by and waiting for a new player to join
	// - declaring GameOver and assigning the victory to other player(s)
	RemovePlayer(secret string) (GameUpdate, error)

	// Start() initializes the game.
	// It returns the initial state views for the players and the list of players
	// who are expected to make the first move.
	// The specific error returned depends on the game engine implementation.
	Start() (GameUpdate, error)

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
	// Suggested semantics for nextPlayers and isGameOver:
	// - isGameOver is false: the game continues, nextPlayers contains the player(s) whose move is expected
	// - isGameOver is true: the game is over; nextPlayers contains the winner(s). If empty, the game ends in a draw.
	Play(playerId string, move json.RawMessage) (GameUpdate, error)
}
