package gamebox

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/beevik/guid"
	log "github.com/sirupsen/logrus"
)

const (
	TableNotFound  = "table [%v] not found"
	PlayerNotFound = "player [%v] not found"
	TicketNotFound = "ticket [%v] not found"
	StartError     = "game start error; table guid: %v; err: %v"
)

const ticketCleanUpAfter = 30 * time.Second

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
	ticketCleanUp  time.Duration         // registration ticket clean up timeout
	wg             sync.WaitGroup        // games graceful shutdown
}

func newGameRegistry(f GameFactory) *gameRegistry {
	return &gameRegistry{
		registry:       make(map[string]*table),
		pendingTickets: make(map[string]joinTicket),
		factory:        f,
		ticketCleanUp:  ticketCleanUpAfter,
	}
}

func (g *gameRegistry) createTable(tableName string) string {
	t := createTable(tableName, g.factory())

	g.mu.Lock()
	g.registry[t.summary.TableGUID] = t
	g.mu.Unlock()

	return t.summary.TableGUID
}

func (g *gameRegistry) listTables() []tableSummary {
	g.mu.RLock()
	defer g.mu.RUnlock()

	log.Debugf("listing tables")
	ret := []tableSummary{}
	for _, t := range g.registry {
		ret = append(ret, t.getSummary())
	}

	return ret
}

func (g *gameRegistry) joinTable(tableGUID string, playerName string) (wsSecret string, err error) {
	g.mu.RLock()
	_, ok := g.registry[tableGUID]
	g.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf(TableNotFound, tableGUID)
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

	g.mu.Lock()
	g.pendingTickets[ticket.secret] = ticket
	g.mu.Unlock()

	log.Debugf("ticket %+v", ticket)
	g.cleanUpTicketAfter(ticket.secret, g.ticketCleanUp)

	return ticket.secret, nil
}

func (g *gameRegistry) rejoinTable(tableGUID string, playerGUID string) (wsSecret string, err error) {
	g.mu.RLock()
	t, ok := g.registry[tableGUID]
	g.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf(TableNotFound, tableGUID)
	}

	p := t.getPlayer(playerGUID)
	if p == nil {
		return "", fmt.Errorf(PlayerNotFound, playerGUID)
	}

	log.Infof("player %+v rejoining game %v", p, tableGUID)

	ticket := joinTicket{
		secret:    getGUID(),
		tableGUID: tableGUID,
		player:    *p,
	}

	g.mu.Lock()
	g.pendingTickets[ticket.secret] = ticket
	g.mu.Unlock()
	log.Debugf("ticket %+v", ticket)

	g.cleanUpTicketAfter(ticket.secret, g.ticketCleanUp)

	return ticket.secret, nil
}

func (g *gameRegistry) consumeTicket(secret string) (joinTicket, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	t, ok := g.pendingTickets[secret]
	if !ok {
		return joinTicket{}, fmt.Errorf(TicketNotFound, secret)
	}

	// remove the ticket nevertheless
	defer delete(g.pendingTickets, secret)

	// check the table exists
	_, ok = g.registry[t.tableGUID]
	if !ok {
		return joinTicket{}, fmt.Errorf(TableNotFound, t.tableGUID)
	}

	return t, nil
}

func (g *gameRegistry) seatPlayer(p player, c *client) (tableInbox chan<- msgPlayer, playerOutbox <-chan msgPlayer, err error) {

	// check the table first
	g.mu.RLock()
	table, ok := g.registry[p.tableGUID]
	g.mu.RUnlock()
	if !ok {
		err = fmt.Errorf(TableNotFound, p.tableGUID)
		return nil, nil, err
	}
	tableInbox = table.inbox

	err = table.rules.AddPlayer(p.guid, p.name)
	if err != nil {
		err = fmt.Errorf("AddPlayer failed; table [%v]; player [%v]; err [%w]", p.tableGUID, p.guid, err)
		return nil, nil, err
	}

	playerOutbox = table.setPlayer(p, c)
	return tableInbox, playerOutbox, nil
}

func (g *gameRegistry) quitTable(p player) error {
	g.mu.RLock()
	t, ok := g.registry[p.tableGUID]
	g.mu.RUnlock()
	if !ok {
		return fmt.Errorf(TableNotFound, p.tableGUID)
	}

	player := t.getPlayer(p.guid)
	if player == nil {
		return fmt.Errorf(PlayerNotFound, p.guid)
	}

	if player.client != nil {
		player.client.dispose()
	}

	t.deletePlayer(p.guid)

	return nil
}

func (g *gameRegistry) disconnectPlayer(playerGUID string, tableGUID string) {
	err := g.quitTable(player{guid: playerGUID, tableGUID: tableGUID})
	if err != nil {
		log.Warningf("disconnectPlayer failed for %s at table %s: %v", playerGUID, tableGUID, err)
	}
}

func (g *gameRegistry) listPlayers(tableGUID string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ret := []string{}

	t, ok := g.registry[tableGUID]
	if !ok {
		return ret, fmt.Errorf(TableNotFound, tableGUID)
	}

	for _, p := range t.getPlayers() {
		ret = append(ret, p.guid)
	}

	return ret, nil
}

func (g *gameRegistry) startTable(tableGUID string) error {

	g.mu.RLock()
	t, ok := g.registry[tableGUID]
	g.mu.RUnlock()
	if !ok {
		log.Errorf("table %v not fonud; aborting", tableGUID)
		return fmt.Errorf(TableNotFound, tableGUID)
	}

	log.Infof("startTable: Table %s starting...", t.summary.TableGUID)

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

// cleanUpTicketAfter() deletes a ticket after a timeout to prevent proliferation of unused tickets
func (g *gameRegistry) cleanUpTicketAfter(secret string, timeout time.Duration) {
	time.AfterFunc(timeout, func() {
		g.mu.Lock()
		delete(g.pendingTickets, secret)
		g.mu.Unlock()
	})
}
