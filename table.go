package gamebox

import (
	"encoding/json"
	"sync"

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
	inbox   chan msgPlayer // incoming messages from players
	mu      sync.RWMutex
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

// TODO : check
func (t *table) setPlayer(p player) (clientToClose *client) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if oldPlayer, exists := t.players[p.guid]; exists && oldPlayer.client != nil {
		clientToClose = oldPlayer.client
	}
	t.players[p.guid] = p
	return clientToClose
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
	for _, p := range t.players {
		if p.client != nil {
			close(p.client.outbox)
		}
	}
	t.mu.Unlock()
}

func (t *table) Dispose() {
	close(t.inbox)
}
