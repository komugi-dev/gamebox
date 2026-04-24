package gamebox

import (
	"sync"

	"github.com/beevik/guid"
	log "github.com/sirupsen/logrus"
)

func getGUID() string {
	return guid.NewString()
}

type table struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
	rules     GameRules
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
	// TODO
	log.Debugf("creating table %v", tableName)
	return ""
}

func (g *gameRegistry) listTables() []table {
	// TODO
	log.Debugf("listing tables")
	return []table{}
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
	return nil
}

func (g *gameRegistry) listPlayers(tableGUID string) ([]string, error) {
	// TODO
	return []string{}, nil
}

func (g *gameRegistry) startTable(tableGUID string) error {
	// TODO
	return nil
}
