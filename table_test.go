package gamebox

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mock logic for testing

type StressLogic struct {
	counter int
}

func (l *StressLogic) Info() InstanceStatus {
	return InstanceStatus{State: "playing", Players: 1000}
}

func (l *StressLogic) AddPlayer(secret string, public string) (GameUpdate, error) {
	return GameUpdate{}, nil
}

func (l *StressLogic) RemovePlayer(secret string) (GameUpdate, error) {
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
