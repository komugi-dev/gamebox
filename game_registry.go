package gamebox

import (
	"errors"
	"fmt"
	"sync"

	"github.com/beevik/guid"
	log "github.com/sirupsen/logrus"
)

const (
	TableNotFound  = "table not found"
	PlayerNotFound = "player not found"
	TicketNotFound = "ticket not found"
	StartError     = "game start error; table guid: %v; err: %v"
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

type gameRegistry struct {
	mu             sync.RWMutex
	registry       map[string]*table     // game tables available
	pendingTickets map[string]joinTicket // join requests
	factory        GameFactory           // factory of new games
	wg             sync.WaitGroup        // games graceful shutdown
}

func newGameRegistry(f GameFactory) *gameRegistry {
	return &gameRegistry{
		registry:       make(map[string]*table),
		pendingTickets: make(map[string]joinTicket),
		factory:        f,
	}
}

func (g *gameRegistry) createTable(tableName string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	log.Infof("creating table %v", tableName)
	t := table{
		summary: tableSummary{
			TableName: tableName,
			TableGUID: guid.NewString(),
		},
		rules:   g.factory(),
		players: make(map[string]player),
		inbox:   make(chan msgPlayer),
		outbox:  make(map[string]chan msgPlayer),
	}
	log.Debugf("%+v", t.summary)
	g.registry[t.summary.TableGUID] = &t

	return t.summary.TableGUID
}

func (g *gameRegistry) listTables() []tableSummary {
	g.mu.RLock()
	defer g.mu.RUnlock()

	log.Debugf("listing tables")
	ret := []tableSummary{}
	for _, t := range g.registry {
		ret = append(ret, t.summary)
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

func (g *gameRegistry) seatPlayer(p player, c *client) (tableInbox chan<- msgPlayer, playerOutbox <-chan msgPlayer, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// check the table first
	table, ok := g.registry[p.tableGUID]
	if !ok {
		err = errors.New(TableNotFound)
		return nil, nil, err
	}

	// TODO : check

	// in case this is a rejoin, close the existing connection
	oldPlayer := setPlayer(p)

	if oldPlayer, exists := table.players[p.guid]; exists && oldPlayer.client != nil {
		log.Warningf("kicking old connection for player %s", p.guid)
		_ = oldPlayer.client.conn.Close()
	}

	// assign the communication client
	p.client = c
	table.players[p.guid] = p

	return table.inbox, nil
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

	// player
	t.deletePlayer(p.guid)

	// clean up
	p.client.dispose()

	return nil
}

func (g *gameRegistry) disconnectPlayer(playerGUID string, tableGUID string) {
	// TODO
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

	g.mu.RLock()
	t, ok := g.registry[tableGUID]
	g.mu.RUnlock()
	if !ok {
		log.Errorf("table %v not fonud; aborting", tableGUID)
		return errors.New(TableNotFound)
	}

	log.Infof("Table %s starting...", t.summary.TableGUID)

	// init the game
	updatedStatus, nextPlayers, err := t.rules.Start()
	if err != nil {
		errMsg := fmt.Sprintf(StartError, t.summary.TableGUID, err)
		log.Error(errMsg)
		return errors.New(errMsg)
	}

	go func() {
		t.startLoop(updatedStatus, nextPlayers)

		// the game is done, clean up the table
		t.Dispose()

		g.mu.Lock()
		delete(g.registry, tableGUID)
		g.mu.Unlock()

	}()
	return nil
}
