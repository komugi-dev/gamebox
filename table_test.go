package gamebox

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// mock logic for testing

type StressLogic struct {
	counter int
	players int
}

func (l *StressLogic) Info() InstanceStatus {
	return InstanceStatus{State: "playing", Players: 1000}
}

func (l *StressLogic) AddPlayer(secret string, public string) (GameUpdate, error) {
	l.players++
	return GameUpdate{}, nil
}

func (l *StressLogic) RemovePlayer(secret string) (GameUpdate, error) {
	l.players--
	return GameUpdate{}, nil
}

func (l *StressLogic) Start() (GameUpdate, error) {
	return GameUpdate{}, nil
}

func (l *StressLogic) Play(playerId string, move json.RawMessage) (GameUpdate, error) {
	l.counter++
	return GameUpdate{}, nil
}

// unit test

func TestTable_StressConcurrency(t *testing.T) {

	tmp := log.GetLevel()
	log.SetLevel(log.ErrorLevel)
	defer func() { log.SetLevel(tmp) }()

	const numClients = 1000

	logic := &StressLogic{counter: 0}
	table := createTable("stress_table", logic)

	go table.eventLoop()

	// prepare 1000 clients to run simultaneously
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func(playerID int) {
			defer wg.Done()

			msg := msgPlayer{
				Type:       MsgTypePlay,
				PlayerGUID: fmt.Sprintf("player-%d", playerID),
				Payload:    []byte(`"1"`),
			}

			table.inbox <- msg
		}(i)
	}

	wg.Wait()

	// check the event loop processed them all
	replyChan := make(chan error, 1)
	table.inbox <- msgPlayer{
		Type:  MsgTypeStart,
		reply: replyChan,
	}

	select {
	case <-replyChan:
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock!")
	}

	assert.Equal(t, numClients, logic.counter, "Race condition or message lost")

	table.Dispose()
}

func TestTable_StressConcurrencyPlayers(t *testing.T) {

	tmp := log.GetLevel()
	log.SetLevel(log.ErrorLevel)
	defer func() { log.SetLevel(tmp) }()

	// barrier to synchronize the test start
	startLobby := make(chan struct{})
	var wg sync.WaitGroup
	var wgReady sync.WaitGroup

	// single table
	logic := &StressLogic{counter: 0}
	table := createTable("stress_table", logic)

	const numClients = 10000

	// the players join the table
	lobby := func(playerId int) {
		defer wg.Done()
		strPlayer := strconv.Itoa(playerId)

		// subscribe and count the opponents
		p := player{
			name:      strPlayer,
			guid:      strPlayer,
			tableGUID: table.summary.TableGUID,
			client:    nil, // no need to set the client yet
		}

		// wait for the synchronized start
		wgReady.Done()
		<-startLobby

		// join the game
		table.setPlayer(p, nil)
		reply := make(chan error)
		table.inbox <- msgPlayer{
			Type:       MsgTypeJoin,
			PlayerGUID: p.guid,
			Payload:    []byte(p.name),
			reply:      reply,
		}

		// wait for the answer
		err := <-reply
		assert.Nil(t, err)
	}

	// event loop
	go table.eventLoop()

	// set the players simultaneously
	wg.Add(numClients)
	wgReady.Add(numClients)
	for i := 0; i < numClients; i++ {
		go lobby(i)
	}

	// clients ready to join
	wgReady.Wait()

	// fire them all...
	close(startLobby)

	// ...and wait for the result
	wg.Wait()
	assert.Equal(t, numClients, logic.players)
	table.Dispose()
}
