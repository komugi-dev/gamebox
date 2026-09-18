package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/komugi-dev/gamebox"
)

// --- dominium constants ---

// Rock -> Scissor -> Paper -> Rock

const (
	Rock = iota
	Paper
	Scissor
)

// player's choice
type choice int

// --- payloads ---

type Move struct {
	Choice int `json:"choice"`
}

type TurnResult struct {
	Self      string   `json:"my-id"`     // secret id if the player passed the turn, empty otherwise
	Survivors []string `json:"survivors"` // the public names of the survivors
}

type MsgPlayerPayload struct {
	Joiner string     `json:"joiner"`
	Leaver string     `json:"leaver"`
	Turn   TurnResult `json:"turn"`
}

// --- internal structures ---

type player struct {
	public string
	secret string
}

const StatusLobby = "lobby"
const StatusPlay = "play"

// --- ctor and state machine ---

type RPSLogic struct {
	state         string
	currTurn      int
	players       map[string]player // starting players
	inGamePlayers map[string]player // still actively playng
	moves         map[string]choice // turn's moves
}

func CreateRPSLogic() *RPSLogic {

	rps := RPSLogic{
		state:   StatusLobby,
		players: make(map[string]player),
		moves:   make(map[string]choice),
	}
	return &rps
}

// --- game functions ---

func (rps *RPSLogic) Info() gamebox.InstanceStatus {
	return gamebox.InstanceStatus{
		State:   rps.state,
		Players: len(rps.players),
	}
}

func (rps *RPSLogic) AddPlayer(secret string, public string) (gamebox.GameUpdate, error) {
	if rps.state != StatusLobby {
		return gamebox.GameUpdate{}, fmt.Errorf("cannot add player if not lobbying")
	}

	// add new player to the internal status
	p := player{
		public: public,
		secret: secret,
	}
	rps.players[secret] = p

	// notify players (only the public names will be sent)
	basePayload := MsgPlayerPayload{
		Joiner: p.public,
	}
	return rps.buildGameUpdate(basePayload, false)
}

func (rps *RPSLogic) RemovePlayer(secret string) (gamebox.GameUpdate, error) {

	// remove player from the internal status
	p, found := rps.players[secret]
	if !found {
		// no players to remove, no updates
		return gamebox.GameUpdate{}, nil
	}

	basePayload := MsgPlayerPayload{
		Leaver: p.public,
	}

	delete(rps.players, secret)
	delete(rps.inGamePlayers, secret)

	// avoid potential deadlock: check if the leaving player was the last one in the turn
	isComplete := rps.isTurnComplete()
	if isComplete {
		rps.resolveTurn()
	}

	return rps.buildGameUpdate(basePayload, isComplete)
}

// start() notifies all players they can play.
// It cannot be called twice
func (rps *RPSLogic) Start() (gamebox.GameUpdate, error) {
	gu := gamebox.GameUpdate{}
	if rps.state == StatusPlay {
		return gu, errors.New("already playing")
	}

	rps.state = StatusPlay // the game don't admit more players
	rps.currTurn = 0
	rps.inGamePlayers = maps.Clone(rps.players) // the original players are in game

	return rps.buildGameUpdate(MsgPlayerPayload{}, true)
}

func (rps *RPSLogic) Play(playerId string, move json.RawMessage) (gamebox.GameUpdate, error) {
	gu := gamebox.GameUpdate{
		Status: make(map[string]json.RawMessage),
	}

	// check player
	_, exist := rps.inGamePlayers[playerId]
	if !exist {
		return gu, fmt.Errorf("player does not exist [%v]", playerId)
	}

	// check already moved
	ch, exist := rps.moves[playerId]
	if exist {
		return gu, fmt.Errorf("player [%v] already played [%v]", playerId, ch)
	}

	// apply the move
	m := Move{}
	err := json.Unmarshal(move, &m)
	if err != nil {
		return gu, fmt.Errorf("unmarshal failed; player [%v]; payload [%v]", playerId, string(move))
	}
	rps.moves[playerId] = choice(m.Choice)

	// not all the players have moved yet;
	// to keep the logic simple there is no notification here now, but
	// it's possible to notify the players about who did the last move and who's left to play
	isComplete := rps.isTurnComplete()
	if !isComplete {
		return gu, nil
	}

	// all the players moved, resolve the turn
	// redo the turn if all players are losers
	rps.resolveTurn()

	basePayload := MsgPlayerPayload{}
	gu, err = rps.buildGameUpdate(basePayload, isComplete)
	if err != nil {
		return gu, fmt.Errorf("failed to build game update; %w", err)
	}

	return gu, nil
}

// --- helpers ---

func getLosers(players map[string]choice) []string {

	var rock, paper, scissor bool
	rockPlayers := []string{}
	paperPlayers := []string{}
	scissorPlayers := []string{}

	losers := []string{}

	for k, v := range players {
		switch v {
		case Rock:
			rockPlayers = append(rockPlayers, k)
			rock = true
		case Paper:
			paperPlayers = append(paperPlayers, k)
			paper = true
		case Scissor:
			scissorPlayers = append(scissorPlayers, k)
			scissor = true
		}
	}
	if rock {
		losers = append(losers, scissorPlayers...)
	}
	if paper {
		losers = append(losers, rockPlayers...)
	}
	if scissor {
		losers = append(losers, paperPlayers...)
	}

	return losers
}

// isTurnComplete() evaluates if the turn can be resolved
func (rps *RPSLogic) isTurnComplete() bool {
	return rps.state == StatusPlay && len(rps.moves) >= len(rps.inGamePlayers)
}

// resolveTurn() resolves a turn.
// Call it when all the players played.
// Return turn draw
func (rps *RPSLogic) resolveTurn() {

	losers := getLosers(rps.moves)
	if len(losers) < len(rps.inGamePlayers) {
		for _, p := range losers {
			delete(rps.inGamePlayers, p)
		}
	}

	rps.moves = make(map[string]choice)
	rps.currTurn++
}

// buildGameUpdate() returns a GameUpdate struct based on the game status
func (rps *RPSLogic) buildGameUpdate(basePayload MsgPlayerPayload, turnResolved bool) (gamebox.GameUpdate, error) {
	gu := gamebox.GameUpdate{
		Status: make(map[string]json.RawMessage),
	}

	// determine game over
	if rps.state == StatusPlay {
		if turnResolved {
			gu.NextPlayers = slices.Collect(maps.Keys(rps.inGamePlayers))
		}
		gu.IsGameOver = len(rps.inGamePlayers) == 1
	}

	survivors := []string{}
	for _, p := range rps.inGamePlayers {
		survivors = append(survivors, p.public)
	}

	for secret := range rps.players {

		// dedicated payload
		playerPayload := basePayload

		// players still in game sees their own secret
		if _, isAlive := rps.inGamePlayers[secret]; isAlive {
			playerPayload.Turn.Self = secret
		}
		playerPayload.Turn.Survivors = survivors

		// pack the info
		raw, err := json.Marshal(playerPayload)
		if err != nil {
			return gu, err
		}
		gu.Status[secret] = raw
	}

	return gu, nil
}
