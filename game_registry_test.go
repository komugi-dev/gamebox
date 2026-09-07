package gamebox

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- MOCK LOGIC PER IL REGISTRY ---
type MockLogic struct{}

func (m *MockLogic) Info() InstanceStatus                             { return InstanceStatus{} }
func (m *MockLogic) AddPlayer(string, string) (GameUpdate, error)     { return GameUpdate{}, nil }
func (m *MockLogic) RemovePlayer(string) (GameUpdate, error)          { return GameUpdate{}, nil }
func (m *MockLogic) Start() (GameUpdate, error)                       { return GameUpdate{}, nil }
func (m *MockLogic) Play(string, json.RawMessage) (GameUpdate, error) { return GameUpdate{}, nil }

func mockFactory() GameRules {
	return &MockLogic{}
}

func TestRegistry_TableLifecycle(t *testing.T) {
	reg := newGameRegistry(mockFactory)

	tableGUID := reg.createTable("Lifecycle_Table")

	reg.mu.RLock()
	_, exists := reg.registry[tableGUID]
	reg.mu.RUnlock()

	assert.True(t, exists, "table exists after creation")

	reg.mu.RLock()
	table := reg.registry[tableGUID]
	reg.mu.RUnlock()

	table.Dispose()

	isDeleted := false
	for i := 0; i < 10; i++ {
		reg.mu.RLock()
		_, exists = reg.registry[tableGUID]
		reg.mu.RUnlock()

		if !exists {
			isDeleted = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	assert.True(t, isDeleted, "table not removed after closing game loop")
}

func TestRegistry_TicketSystem(t *testing.T) {
	reg := newGameRegistry(mockFactory)
	tableGUID := reg.createTable("Ticket_Table")

	secret, err := reg.joinTable(tableGUID, "Mario")
	assert.Nil(t, err)
	assert.NotEmpty(t, secret)

	ticket, err := reg.consumeTicket(secret)
	assert.Nil(t, err)
	assert.Equal(t, "Mario", ticket.player.name)
	assert.Equal(t, tableGUID, ticket.tableGUID)

	_, err = reg.consumeTicket(secret)
	assert.NotNil(t, err, "ticket consumed twice")
	assert.Contains(t, err.Error(), "not found", "expected error TicketNotFound")

	_, err = reg.consumeTicket("ticket_false_123")
	assert.NotNil(t, err)

	tableInfo := reg.registry[tableGUID]
	tableInfo.Dispose()
}
