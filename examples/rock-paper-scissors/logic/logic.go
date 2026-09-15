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
	return rps.updateCurrPlayers(public, "")
}

func (rps *RPSLogic) RemovePlayer(secret string) (gamebox.GameUpdate, error) {
	// remove player from the internal status
	p, found := rps.players[secret]
	if !found {
		// no players to remove, no updates
		return gamebox.GameUpdate{}, nil
	}
	delete(rps.players, secret)
	delete(rps.inGamePlayers, secret)

	// TODO: potential deadlock: check if the leaving player was still in game
	if len(rps.inGamePlayers) == len(rps.moves) {
		// ... WARNING - deadlock -> call play logic
	}

	// notify players (only the public names will be sent)
	return rps.updateCurrPlayers("", p.public)
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

	gu.Status = nil // no need to notify a status at start
	gu.NextPlayers = slices.Collect(maps.Keys(rps.inGamePlayers))
	gu.IsGameOver = false

	return gu, nil
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

	// parse the move
	m := Move{}
	err := json.Unmarshal(move, &m)
	if err != nil {
		return gu, fmt.Errorf("unmarshal failed; player [%v]; payload [%v]", playerId, string(move))
	}

	rps.moves[playerId] = choice(m.Choice)

	// not all the players have moved yet;
	// to keep the logic simple there is no notification here now, but
	// it's possible to notify the players about who did the last move and who's left to play
	if len(rps.moves) < len(rps.inGamePlayers) {
		return gu, nil
	}

	// all the players moved, resolve the turn
	// redo the turn if all players are losers
	losers := getLosers(rps.moves)
	if len(losers) < len(rps.inGamePlayers) {
		for _, p := range losers {
			delete(rps.inGamePlayers, p)
		}
	}

	// reset the moves
	rps.moves = make(map[string]choice)

	// send the outcome
	return rps.updatePlayersTurn()
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

func (rps *RPSLogic) updateCurrPlayers(joiner, leaver string) (gamebox.GameUpdate, error) {
	gu := gamebox.GameUpdate{
		Status:      make(map[string]json.RawMessage),
		NextPlayers: nil,
		IsGameOver:  false,
	}

	m := MsgPlayerPayload{
		Turn:   TurnResult{},
		Joiner: joiner,
		Leaver: leaver,
	}

	for _, p := range rps.players {
		pl, err := json.Marshal(m)
		if err != nil {
			return gu, err
		}
		gu.Status[p.secret] = pl
	}
	return gu, nil
}

func (rps *RPSLogic) updatePlayersTurn() (gamebox.GameUpdate, error) {
	gu := gamebox.GameUpdate{
		Status: make(map[string]json.RawMessage),
	}

	// only the players still in game can play
	gu.NextPlayers = slices.Collect(maps.Keys(rps.inGamePlayers))

	survivors := []string{}
	for _, v := range rps.inGamePlayers {
		survivors = append(survivors, v.public)
	}

	// all the players get an update
	// self is the player id if still playing, empty otherwise
	for _, p := range rps.players {

		self := ""
		if _, stillPlaying := rps.inGamePlayers[p.secret]; stillPlaying {
			self = p.secret
		}
		t := TurnResult{
			Self:      self,
			Survivors: survivors,
		}
		m := MsgPlayerPayload{
			Joiner: "",
			Leaver: "",
			Turn:   t,
		}
		rawj, err := json.Marshal(m)
		if err != nil {
			return gu, err
		}
		gu.Status[p.secret] = rawj
	}

	// only one can win the game
	gu.IsGameOver = len(rps.inGamePlayers) == 1

	return gu, nil
}
