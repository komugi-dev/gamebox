package gamebox

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/beevik/guid"
	log "github.com/sirupsen/logrus"
)

const (
	TableNotFound  = "table not found"
	PlayerNotFound = "player not found"
)

func getGUID() string {
	return guid.NewString()
}

type table struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
	rules     GameRules
	players   map[string]struct{}
	msgs      chan json.RawMessage
}

type gameRegistry struct {
	mu       sync.RWMutex
	registry map[string]*table
	factory  GameFactory
}

func newGameRegistry(f GameFactory) *gameRegistry {
	return &gameRegistry{
		registry: make(map[string]*table),
		factory:  f,
	}
}

func (g *gameRegistry) createTable(tableName string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	log.Infof("creating table %v", tableName)
	t := table{
		TableName: tableName,
		TableGUID: guid.NewString(),
		rules:     g.factory,
		players:   make(map[string]struct{}),
		msgs:      make(chan json.RawMessage),
	}
	log.Debugf("%+v", t)
	g.registry[t.TableGUID] = &t

	return t.TableGUID
}

func (g *gameRegistry) listTables() []table {
	g.mu.RLock()
	defer g.mu.RUnlock()

	log.Debugf("listing tables")
	ret := []table{}
	for _, t := range g.registry {
		ret = append(ret, *t)
	}

	return ret
}

func (g *gameRegistry) joinTable(tableGUID string, playerName string) (wsSecret string, err error) {
	// TODO
	wsSecret = getGUID()
	err = nil
	return
}

func (g *gameRegistry) rejoinTable(tableGUID string, playerGUID string) (wsSecret string, err error) {
	// TODO
	wsSecret = getGUID()
	err = nil
	return
}

func (g *gameRegistry) quitTable(tableGUID string, playerGUID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	t, ok := g.registry[tableGUID]
	if !ok {
		return errors.New(TableNotFound)
	}

	_, ok = t.players[playerGUID]
	if !ok {
		return errors.New(PlayerNotFound)
	}

	delete(t.players, playerGUID)

	return nil
}

func (g *gameRegistry) listPlayers(tableGUID string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ret := []string{}

	t, ok := g.registry[tableGUID]
	if !ok {
		return ret, errors.New(TableNotFound)
	}

	for k := range t.players {
		ret = append(ret, k)
	}

	return ret, nil
}

func (g *gameRegistry) startTable(tableGUID string) error {
	// TODO
	return nil
}
