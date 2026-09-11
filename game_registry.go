package gamebox

import (
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

	// start the event loop
	// prepare for a cleanup after the routine exits
	go func() {
		t.eventLoop()

		g.mu.Lock()
		delete(g.registry, t.summary.TableGUID)
		g.mu.Unlock()

		log.Infof("Table %s removed from registry (Game Over)", t.summary.TableGUID)
	}()

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

	// check join / rejoin
	table.mu.RLock()
	_, isRejoin := table.players[p.guid]
	table.mu.RUnlock()

	if !isRejoin {
		replyChan := make(chan error, 1)
		tableInbox <- msgPlayer{
			Type:       MsgTypeJoin,
			PlayerGUID: p.guid,
			Payload:    []byte(p.name),
			reply:      replyChan,
		}

		err = <-replyChan
		if err != nil {
			err = fmt.Errorf("AddPlayer rejected by game logic for player [%v]: %w", p.guid, err)
			return nil, nil, err
		}
	}

	// eventually, complete the initialization
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

	// delete wsocket, readPump and writePump
	if player.client != nil {
		player.client.dispose()
	}
	t.deletePlayer(p.guid)

	// send the answer to the reply chan
	replyChan := make(chan error, 1)

	select {
	case t.inbox <- msgPlayer{
		Type:       MsgTypeQuit,
		PlayerGUID: p.guid,
		reply:      replyChan,
	}:
		err := <-replyChan
		if err != nil {
			return fmt.Errorf("game logic error on quit: %w", err)
		}

	default:
		log.Warnf("inbox full, dropping quit message for %s (table %s)", p.guid, p.tableGUID)
	}

	return nil
}

func (g *gameRegistry) disconnectPlayer(playerGUID string, tableGUID string) {
	err := g.quitTable(player{guid: playerGUID, tableGUID: tableGUID})
	if err != nil {
		log.Debugf("disconnectPlayer failed for %s at table %s: %v", playerGUID, tableGUID, err)
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

	replyChan := make(chan error, 1)

	select {
	case t.inbox <- msgPlayer{
		Type:  MsgTypeStart,
		reply: replyChan,
	}:
		err := <-replyChan
		if err != nil {
			return fmt.Errorf("start failed: %w", err)
		}

	default:
		return fmt.Errorf("table inbox full, cannot start table %s", tableGUID)
	}

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
