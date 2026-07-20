package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

const ClientTimeout = 15 * time.Second

type msgPlayer struct {
	PlayerGUID string          `json:"player_guid"`
	TurnID     string          `json:"turn_id"`
	Payload    json.RawMessage `json:"payload"`
}

type Session struct {
	cli *http.Client
	url url.URL
}

type client struct {
	// websocket client (not exported)
	inbox  chan json.RawMessage
	outbox chan json.RawMessage
}

type TableSummary struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
}

type Table struct {
	table TableSummary
	ss    *Session
}

type Player struct {
	Name  string
	Guid  string
	Table Table

	turId string
	ws    *client
	ss    *Session
}

func CreateSession(cli *http.Client, gameboxURL url.URL) *Session {
	if cli == nil {
		cli = &http.Client{Timeout: ClientTimeout}
	}
	return &Session{
		cli: cli,
		url: gameboxURL,
	}
}

// Table() creates a table with the given name.
// It returns the table id on success.
func (c *Session) Table(ctx context.Context, name string) (Table, error) {

	url := c.url.JoinPath("tables")
	ts := TableSummary{
		TableName: name,
	}
	buf, err := json.Marshal(ts)
	if err != nil {
		return Table{}, fmt.Errorf("failed to encode table [%v][%v]", ts, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewReader(buf))
	if err != nil {
		return Table{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.cli.Do(req)
	if err != nil {
		return Table{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		return Table{}, fmt.Errorf("wrong return code %v", resp.StatusCode)
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&ts)
	if err != nil {
		return Table{}, err
	}

	return Table{table: ts, ss: c}, nil
}

// Tables() returns a list of tables.
func (c *Session) Tables(ctx context.Context) ([]Table, error) {
	return nil, nil
}

func (t *Table) Start(ctx context.Context) error {
	return nil
}

func CreatePlayer(name string, s *Session) *Player {
	return &Player{
		Name: name,
		ss:   s,
	}
}

// Join() joins a table.
func (c *Player) Join(ctx context.Context, t Table) error {
	// table join
	// instantiate web socket
	return nil
}

// Rejoin() rejoins a table.
func (c *Player) Rejoin(ctx context.Context) error {
	return nil
}

func (c *Player) Quit(ctx context.Context) error {
	return nil
}

// GetMsg() get messages from the gamebox service
func (c *Player) GetMsg(ctx context.Context) (json.RawMessage, error) {
	select {
	case msg := <-c.ws.inbox:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendMsg() sends messages to the gamebox service
func (c *Player) SendMsg(ctx context.Context, msg json.RawMessage) error {
	c.ss.
		c.ws.outbox <- msg
	return nil
}

// writePump receives messages from outbox and writes them to the network
func (c *client) writePump(ctx context.Context, outbox <-chan msgPlayer) {
	defer c.conn.Close()

	for {
		select {
		case <-ctx.Done():
			log.Infof("writePump stopping due to context [%v]", c.playerGUID)
			return

		case msg, ok := <-outbox:
			if !ok {
				log.Infof("outbox closed, writePump exiting [%v]", c.playerGUID)
				return
			}

			err := c.conn.WriteJSON(msg)
			if err != nil {
				log.Errorf("ws write failed [%v]; %v", c.playerGUID, err)
				return
			}
		}
	}
}

// readPump receives messages from the networs and writes them to inbox
func (c *client) readPump(inbox chan<- msgPlayer) error {
	msg := msgPlayer{}
	var err error
	for {
		err = c.conn.ReadJSON(&msg)
		if err != nil {
			log.Errorf("ws read failed [%v]; %v", c.playerGUID, err)
			break
		}
		if msg.PlayerGUID != c.playerGUID {
			log.Errorf("guid check failed exp:[%v] recv:[%v]", c.playerGUID, msg.PlayerGUID)
			continue
		}
		inbox <- msg
	}
	log.Infof("readPump exits [%v]", c.playerGUID)
	return err
}

// run() starts the read and write routines to exchange messages between the game engine and the pleayer.
// inbox receives messages from the player.
// outbox sends messages to the player.
// Both chans are managed externally.
func (c *client) run(ctx context.Context, inbox chan<- msgPlayer, outbox <-chan msgPlayer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		log.Infof("Context done, forcing close for client %s", c.playerGUID)
		c.conn.Close()
	}()

	go c.writePump(ctx, outbox)

	return c.readPump(inbox)
}
