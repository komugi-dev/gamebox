package gamebox

import (
	"encoding/json"
	"sync"

	log "github.com/sirupsen/logrus"
)

type table struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
	rules     GameRules
	players   map[string]player
	msgs      chan json.RawMessage
	wg        *sync.WaitGroup
}

func (t *table) startLoop() {
	log.Infof("Table %s starting...", t.TableGUID)

	// init the game
	initialState, err := t.rules.Start()
	if err != nil {
		log.Errorf("Failed to start game. Guid: %v, err: %v", t.TableGUID, err)
		return
	}

	// send information to the right channels
	t.broadcastState(initialState)

	// infinite loop
	for {
		// ask the rules who's next
		nextPlayerGUID, err := t.rules.NextPlayer()
		if err != nil {
			log.Errorf("Engine error (guid: %v): %v", t.TableGUID, err)
			return
		}

		// listen for messages
		select {
		case msg := <-t.msgs:

			// security check
			if msg.playerGUID != nextPlayerGUID {
				t.sendErrorTo(msg.playerGUID, "Not your turn!")
				continue
			}

			// play
			newState, isGameOver, err := t.rules.Play(nextPlayerGUID, msg.payload)
			if err != nil {
				t.sendErrorTo(msg.playerGUID, err.Error())
				continue
			}

			// update new state
			t.broadcastState(newState)

			if isGameOver {
				log.Infof("Game Over at table %s", t.TableGUID)
				return
			}
		}
	}
}
