package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/komugi-dev/gamebox"
)

const (
	Rock = iota
	Paper
	Scissor
)

// Rock -> Scissor -> Paper -> Rock

// player's choice
type choice int

// play result
const (
	win = iota
	lose
	draw
)

type result int

func (c choice) isWinner(other choice) result {
	if c == other {
		return draw
	}
	if c == Rock && other == Scissor ||
		c == Scissor && other == Paper ||
		c == Paper && other == Rock {
		return win
	}
	return lose
}

type player struct {
	public string
	secret string
}

type NewPlayer struct {
	Name string `json:"name"`
}

// "R", "S", "P"
type Move struct {
	Choice int `json:"choice"`
}

// message after turn
type TurnResult struct {
	Self      string   `json:"my-id"`     // secret id if the player passed the turn, empty otherwise
	Survivors []string `json:"survivors"` // the public names of the survivors
}

const StatusLobby = "lobby"
const StatusPlay = "play"

type RPSLogic struct {
	state    string
	currTurn int
	players  map[string]player
	moves    map[string]choice
}

func CreateRPSLogic() *RPSLogic {

	rps := RPSLogic{
		state:   StatusLobby,
		players: make(map[string]player),
		moves:   make(map[string]choice),
	}
	return &rps
}

func (rps *RPSLogic) Info() gamebox.InstanceStatus {
	return gamebox.InstanceStatus{
		State:   rps.state,
		Players: len(rps.players),
	}
}

func (rps *RPSLogic) updateCurrPlayers() (gamebox.GameUpdate, error) {
	currPlayers := []NewPlayer{}
	for _, v := range rps.players {
		currPlayers = append(currPlayers, NewPlayer{Name: v.public})
	}

	gu := gamebox.GameUpdate{
		Status:      make(map[string]json.RawMessage),
		NextPlayers: nil,
		IsGameOver:  false,
	}

	for _, p := range rps.players {
		pl, err := json.Marshal(currPlayers)
		if err != nil {
			return gu, err
		}
		gu.Status[p.secret] = pl
	}
	return gu, nil
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
	return rps.updateCurrPlayers()
}

func (rps *RPSLogic) RemovePlayer(secret string) (gamebox.GameUpdate, error) {
	// remove player from the internal status
	_, found := rps.players[secret]
	if !found {
		// no players to remove, no updates
		return gamebox.GameUpdate{}, nil
	}
	delete(rps.players, secret)

	// notify players (only the public names will be sent)
	return rps.updateCurrPlayers()
}

// start() notifies all players they can play.
// It cannot be called twice
func (rps *RPSLogic) Start() (gamebox.GameUpdate, error) {
	gu := gamebox.GameUpdate{}
	if rps.state == StatusPlay {
		return gu, errors.New("already playing")
	}

	rps.state = StatusPlay
	rps.currTurn = 0
	gu.Status = nil
	gu.NextPlayers = slices.Collect(maps.Keys(rps.players))
	gu.IsGameOver = false

	return gu, nil
}

func getSurvivors(players map[string]choice) []string {

	var rock, paper, scissor bool
	rockSurv := []string{}
	paperSurv := []string{}
	scissorSurv := []string{}

	surv := []string{}

	for k, v := range players {
		switch v {
		case Rock:
			rockSurv = append(rockSurv, k)
			rock = true
		case Paper:
			paperSurv = append(paperSurv, k)
			paper = true
		case Scissor:
			scissorSurv = append(scissorSurv, k)
			scissor = true
		}
	}
	if !rock {
		surv = append(surv, scissorSurv...)
	}
	if !paper {
		surv = append(surv, rockSurv...)
	}
	if !scissor {
		surv = append(surv, paperSurv...)
	}

	return surv
}

func (rps *RPSLogic) Play(playerId string, move json.RawMessage) (gamebox.GameUpdate, error) {
	gu := gamebox.GameUpdate{
		Status: make(map[string]json.RawMessage),
	}

	// check player
	_, exist := rps.players[playerId]
	if !exist {
		return gu, fmt.Errorf("player does not exist [%v]", playerId)
	}

	// check already moved
	ch, exist := rps.moves[playerId]
	if exist {
		return gu, fmt.Errorf("player [%v] alredy played [%v]", playerId, ch)
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
	// it's possible to notify the players who did the last move and who's left to play
	if len(rps.moves) < len(rps.players) {
		return gu, nil
	}

	// all the players moved, resolve the turn
	surv := getSurvivors(rps.moves)

	// if at least one survived, update the status,
	// otherwise send nothing to redo the turn
	if len(surv) > 0 {
		survNames := []string{}
		for _, k := range surv {
			survNames = append(survNames, rps.players[k].public)
		}
		for _, k := range surv {
			s := TurnResult{
				Self:      k,
				Survivors: survNames,
			}
			j, _ := json.Marshal(s)
			gu.Status[k] = j
		}
	}

	gu.NextPlayers = slices.Collect(maps.Keys(rps.moves))

	// only one survivor, we have a winner
	if len(surv) == 1 {
		gu.IsGameOver = true
	}

	// reset the moves
	rps.moves = make(map[string]choice)

	return gu, nil
}
