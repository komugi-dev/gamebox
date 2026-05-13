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
	TicketNotFound = "ticket not found"
)

func getGUID() string {
	return guid.NewString()
}

type player struct {
	name      string
	guid      string
	tableGUID string
	client    *client
}

type joinTicket struct {
	secret    string
	tableGUID string
	player    player
}

type table struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
	rules     GameRules
	players   map[string]player
	msgs      chan json.RawMessage
}

type gameRegistry struct {
	mu             sync.RWMutex
	registry       map[string]*table     // game tables available
	pendingTickets map[string]joinTicket // join requests
	factory        GameFactory
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
		rules:     g.factory(),
		players:   make(map[string]player),
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
	g.mu.Lock()
	defer g.mu.Unlock()

	_, ok := g.registry[tableGUID]
	if !ok {
		return "", errors.New(TableNotFound)
	}

	p := player{
		name:      playerName,
		guid:      getGUID(),
		tableGUID: tableGUID,
	}
	log.Infof("player %+v joining game %v", p, tableGUID)

	ticket := joinTicket{
		secret:    getGUID(),
		tableGUID: tableGUID,
		player:    p,
	}
	g.pendingTickets[ticket.secret] = ticket
	log.Debugf("ticket %+v", ticket)

	return ticket.secret, nil
}

func (g *gameRegistry) rejoinTable(tableGUID string, playerGUID string) (wsSecret string, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	t, ok := g.registry[tableGUID]
	if !ok {
		return "", errors.New(TableNotFound)
	}

	p, ok := t.players[playerGUID]
	if !ok {
		return "", errors.New(PlayerNotFound)
	}

	log.Infof("player %+v rejoining game %v", p, tableGUID)

	ticket := joinTicket{
		secret:    getGUID(),
		tableGUID: tableGUID,
		player:    p,
	}
	g.pendingTickets[ticket.secret] = ticket
	log.Debugf("ticket %+v", ticket)

	return ticket.secret, nil
}

func (g *gameRegistry) consumeTicket(secret string) (joinTicket, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	t, ok := g.pendingTickets[secret]
	if !ok {
		return joinTicket{}, errors.New(TicketNotFound)
	}

	// remove the ticket nevertheless
	defer delete(g.pendingTickets, secret)

	// check the table exists
	_, ok = g.registry[t.tableGUID]
	if !ok {
		return joinTicket{}, errors.New(TableNotFound)
	}

	return t, nil
}

func (g *gameRegistry) seatPlayer(p player, c *client) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// check the table first
	table, ok := g.registry[p.tableGUID]
	if !ok {
		return errors.New(TableNotFound)
	}

	// in case this is a rejoin, close the existing connection
	if oldPlayer, exists := table.players[p.guid]; exists && oldPlayer.client != nil {
		log.Infof("kicking old connection for player %s", p.guid)
		oldPlayer.client.conn.Close()
	}

	// assign the communication client
	p.client = c
	table.players[p.guid] = p

	return nil
}

func (g *gameRegistry) quitTable(p player) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	t, ok := g.registry[p.tableGUID]
	if !ok {
		return errors.New(TableNotFound)
	}

	_, ok = t.players[p.guid]
	if !ok {
		return errors.New(PlayerNotFound)
	}

	delete(t.players, p.guid)

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
