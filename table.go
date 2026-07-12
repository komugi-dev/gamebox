package gamebox

import (
	"encoding/json"
	"sync"

	"github.com/beevik/guid"
	log "github.com/sirupsen/logrus"
)

type msgPlayer struct {
	playerGUID string
	turnGUID   string
	payload    json.RawMessage
}

type tableSummary struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
}

type table struct {
	summary tableSummary
	rules   GameRules
	players map[string]player
	inbox   chan msgPlayer            // incoming messages from players
	outbox  map[string]chan msgPlayer // outgoing messages fo players
	mu      sync.RWMutex
}

func createTable(tableName string, r GameRules) *table {
	log.Infof("creating table %v", tableName)
	t := table{
		summary: tableSummary{
			TableName: tableName,
			TableGUID: guid.NewString(),
		},
		rules:   r,
		players: make(map[string]player),
		inbox:   make(chan msgPlayer),
		outbox:  make(map[string]chan msgPlayer),
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
	// TODO
	return nil
}

func (t *table) sendYourTurn(playerGUID string, state json.RawMessage) error {
	// TODO
	return nil
}

func (t *table) sendErrorTo(playerGUID string, errMsg string) error {
	// TODO
	return nil
}

func (t *table) updatePlayers(updatedStatus map[string]json.RawMessage, nextPlayers []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var err error
	if len(updatedStatus) > 0 {
		err = t.broadcastState(updatedStatus)
		if err != nil {
			log.Errorf("broadcast failed; table:%v; err:%v", t.summary.TableGUID, err)
		}
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
		playerChan = make(chan msgPlayer)
		t.outbox[p.guid] = playerChan
	}
	playerOutbox = playerChan

	t.players[p.guid] = p
	return
}

func (t *table) deletePlayer(playerGUID string) {
	t.mu.Lock()
	delete(t.players, playerGUID)
	t.mu.Unlock()
}

func (t *table) startLoop(updatedStatus map[string]json.RawMessage, nextPlayers []string) {
	log.Infof("Table %s starting...", t.summary.TableGUID)

	t.updatePlayers(updatedStatus, nextPlayers)

	// event loop
	for {
		msg, ok := <-t.inbox
		if !ok {
			log.Warningf("no more msg; table: %v", t.summary.TableGUID)
			break
		}

		// player sent a valid message
		updatedStatus, nextPlayers, isGameOver, err := t.rules.Play(msg.playerGUID, msg.payload)
		if err != nil {
			t.sendErrorTo(msg.playerGUID, err.Error())
			continue
		}

		t.updatePlayers(updatedStatus, nextPlayers)

		if isGameOver {
			log.Infof("Game Over at table %s", t.summary.TableGUID)
			break
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
