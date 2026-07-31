package gamebox

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/beevik/guid"
	log "github.com/sirupsen/logrus"
)

// outbox buffer gives a room to handle network jitter
const outboxBufferSize = 256

type MsgType string

const (
	MsgTypeWelcome  MsgType = "welcome"
	MsgTypePlay     MsgType = "play"
	MsgTypeState    MsgType = "state"
	MsgTypeYourTurn MsgType = "yourturn"
	MsgTypeGameOver MsgType = "gameover"
	MsgTypeError    MsgType = "error"
)

type msgPlayer struct {
	Type       MsgType         `json:"type"`
	PlayerGUID string          `json:"player_guid"`
	Winners    []string        `json:"winners,omitempty"`
	TurnID     string          `json:"turn_id"`
	Payload    json.RawMessage `json:"payload"`
}

type tableSummary struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
}

type table struct {
	summary  tableSummary
	rules    GameRules
	players  map[string]player
	inbox    chan msgPlayer            // incoming messages from players
	outbox   map[string]chan msgPlayer // outgoing messages fo players
	turnGUID map[string]string         // check the message ids in every turn
	mu       sync.RWMutex
}

func createTable(tableName string, r GameRules) *table {
	log.Infof("creating table %v", tableName)
	t := table{
		summary: tableSummary{
			TableName: tableName,
			TableGUID: guid.NewString(),
		},
		rules:    r,
		players:  make(map[string]player),
		inbox:    make(chan msgPlayer),
		outbox:   make(map[string]chan msgPlayer),
		turnGUID: make(map[string]string),
	}
	log.Debugf("%+v", t.summary)
	return &t
}

func (t *table) getSummary() tableSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	log.Debugf("listing tables")
	return tableSummary{
		TableName: t.summary.TableName,
		TableGUID: t.summary.TableGUID,
	}
}

func (t *table) getPlayer(guid string) *player {
	t.mu.RLock()
	p, ok := t.players[guid]
	t.mu.RUnlock()
	if !ok {
		return nil
	}
	return &p
}

func (t *table) getPlayers() []player {
	players := []player{}
	t.mu.RLock()
	for _, p := range t.players {
		players = append(players, p)
	}
	t.mu.RUnlock()
	return players
}

func (t *table) broadcastState(state map[string]json.RawMessage) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for guid, rawState := range state {
		outbox, ok := t.outbox[guid]
		if !ok {
			continue
		}

		msg := msgPlayer{
			Type:       MsgTypeState,
			PlayerGUID: guid,
			Payload:    rawState,
		}

		select {
		case outbox <- msg:
		default:
			log.Warningf("outbox buffer full for player %s during broadcast", guid)
			// go t.deletePlayer(playerGUID) // think more on this
		}
	}
	return nil
}

func (t *table) sendYourTurn(playerGUID string, state json.RawMessage) error {
	turnGuid := guid.NewString()
	t.mu.Lock()
	outbox, ok := t.outbox[playerGUID]
	t.turnGUID[playerGUID] = turnGuid
	t.mu.Unlock()

	if !ok {
		return fmt.Errorf("sendErrorTo - player [%s] not found or disconnected", playerGUID)
	}

	msg := msgPlayer{
		Type:       MsgTypeYourTurn,
		PlayerGUID: playerGUID,
		TurnID:     turnGuid,
		Payload:    state,
	}

	select {
	case outbox <- msg:
		return nil
	default:
		log.Warningf("outbox buffer full for player %s during sendYourTurn", playerGUID)
		// go t.deletePlayer(playerGUID) // think more on this
		return fmt.Errorf("buffer full for player %s", playerGUID)
	}
}

// TODO: check
func (t *table) notifyGameOver(updatedStatus map[string]json.RawMessage, winners []string) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for pID, statePayload := range updatedStatus {
		msg := msgPlayer{
			Type:       MsgTypeGameOver,
			PlayerGUID: pID,
			Winners:    winners,
			Payload:    statePayload,
		}

		if out, ok := t.outbox[pID]; ok {
			out <- msg
		}
	}
}

func (t *table) sendErrorTo(playerGUID string, errMsg string) error {
	log.Warningf("sending error to %v; %v", playerGUID, errMsg)
	t.mu.RLock()
	outbox, ok := t.outbox[playerGUID]
	t.mu.RUnlock()

	if !ok {
		return fmt.Errorf("sendErrorTo - player [%s] not found or disconnected", playerGUID)
	}

	// send a valid json to represent the error
	errPayload, _ := json.Marshal(map[string]string{
		"type":  "error",
		"error": errMsg,
	})

	msg := msgPlayer{
		Type:       MsgTypeError,
		PlayerGUID: playerGUID,
		Payload:    errPayload,
	}

	select {
	case outbox <- msg:
		return nil
	default:
		log.Warningf("outbox buffer full for player %s during sendError", playerGUID)
		// go t.deletePlayer(playerGUID) // think more on this
		return fmt.Errorf("buffer full for player %s", playerGUID)
	}
}

func (t *table) updatePlayers(updatedStatus map[string]json.RawMessage, nextPlayers []string) {

	var err error
	if len(updatedStatus) > 0 {
		err = t.broadcastState(updatedStatus)
		if err != nil {
			log.Errorf("broadcast failed; table:%v; err:%v", t.summary.TableGUID, err)
		}
	}

	log.Debugf("updatePlayers - next players: %v", nextPlayers)
	if len(nextPlayers) == 0 {
		log.Warningf("updatePlayers - next players empty; table [%v][%v]", t.summary.TableName, t.summary.TableGUID)
	}

	for _, nextPlayerGUID := range nextPlayers {
		err = t.sendYourTurn(nextPlayerGUID, updatedStatus[nextPlayerGUID])
		if err != nil {
			log.Errorf("send your turn failed; table:%v; player:%v; err:%v", t.summary.TableGUID, nextPlayerGUID, err)
		}
	}
}

// setPlayer() checks if the player is rejoining, then creates a channel to handle outgoing messages
// clientToClose is not nil, close it
func (t *table) setPlayer(p player, c *client) (playerOutbox <-chan msgPlayer) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// this can be either a join or a rejoin;
	// join (new player): create a new outbox chan
	// rejoin: close the previous client and recycle the outbox chan
	oldPlayer, exists := t.players[p.guid]
	if exists && oldPlayer.client != nil && oldPlayer.client.conn != nil {
		log.Infof("closing old player [%v] connection", oldPlayer.guid)
		_ = oldPlayer.client.conn.Close()
	}
	p.client = c

	playerChan, exists := t.outbox[p.guid]
	if !exists {
		log.Debugf("create outbox for player [%v]", p.guid)
		playerChan = make(chan msgPlayer, outboxBufferSize)
		t.outbox[p.guid] = playerChan
	}
	playerOutbox = playerChan

	t.players[p.guid] = p
	return playerOutbox
}

func (t *table) deletePlayer(playerGUID string) {
	t.mu.Lock()
	delete(t.players, playerGUID)
	t.mu.Unlock()
}

func (t *table) isValidTurn(playerGUID string, turnGUID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	tg, ok := t.turnGUID[playerGUID]
	valid := ok && tg == turnGUID
	if valid {
		delete(t.turnGUID, turnGUID)
	}
	return valid
}

func (t *table) startLoop(updatedStatus map[string]json.RawMessage, nextPlayers []string) {
	log.Infof("startLoop: Table %s starting...", t.summary.TableGUID)

	t.updatePlayers(updatedStatus, nextPlayers)

	// event loop
	for {
		msg, ok := <-t.inbox
		if !ok {
			log.Warningf("no more msg; table: %v", t.summary.TableGUID)
			break
		}
		if !t.isValidTurn(msg.PlayerGUID, msg.TurnID) {
			log.Warningf("invalid id in msg; table: %v; player: %v; turnGuid: %v",
				t.summary.TableGUID, msg.PlayerGUID, msg.TurnID)
			continue
		}

		// player sent a valid message
		updatedStatus, nextPlayers, isGameOver, err := t.rules.Play(msg.PlayerGUID, msg.Payload)
		if err != nil {
			t.sendErrorTo(msg.PlayerGUID, err.Error())
			continue
		}

		// game over or continue
		// TODO: check
		if isGameOver {
			t.notifyGameOver(updatedStatus, nextPlayers)
			log.Infof("Game Over at table %s", t.summary.TableGUID)
			break
		} else {
			t.updatePlayers(updatedStatus, nextPlayers)
		}
	}

	// players clean up
	t.mu.Lock()
	for _, o := range t.outbox {
		close(o)
	}
	t.mu.Unlock()
}

func (t *table) Dispose() {
	close(t.inbox)
}
