package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
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
	conn *websocket.Conn // websocket
}

type TableSummary struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
}

type Table struct {
	Summary TableSummary
	ss      *Session
}

type Player struct {
	Name  string
	Guid  string
	Table Table

	turnId string

	ss *Session

	ws     *client
	inbox  chan msgPlayer // network -> client
	outbox chan msgPlayer // client -> network

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
func (s *Session) Table(ctx context.Context, name string) (Table, error) {

	url := s.url.JoinPath("tables")
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
	resp, err := s.cli.Do(req)
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

	return Table{Summary: ts, ss: s}, nil
}

// Tables() returns a list of tables.
func (s *Session) Tables(ctx context.Context) ([]Table, error) {
	url := s.url.JoinPath("tables")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return []Table{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.cli.Do(req)
	if err != nil {
		return []Table{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return []Table{}, fmt.Errorf("wrong return code %v", resp.StatusCode)
	}
	ts := []TableSummary{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&ts)
	if err != nil {
		return []Table{}, err
	}

	tables := []Table{}
	for _, t := range ts {
		tables = append(tables, Table{
			Summary: t,
			ss:      s,
		})
	}

	return tables, nil
}

func (t *Table) Start(ctx context.Context) error {
	s := t.ss
	url := s.url.JoinPath("tables", t.Summary.TableGUID, "start")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.cli.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("wrong return code %v", resp.StatusCode)
	}
	return nil
}

func CreatePlayer(name string, s *Session) *Player {
	return &Player{
		Name: name,
		ss:   s,
	}
}

type joinTableRequest struct {
	PlayerName string `json:"player_name"`
}

type joinTableResponse struct {
	WebsocketSecret string `json:"websocket_secret"`
}

func (p *Player) join(ctx context.Context, t Table) (string, error) {
	s := p.ss
	url := s.url.JoinPath("tables", t.Summary.TableGUID, "join")
	jr := joinTableRequest{
		PlayerName: p.Name,
	}
	buf, err := json.Marshal(jr)
	if err != nil {
		return "", fmt.Errorf("failed to encode request [%v][%v]", jr, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.cli.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wrong return code %v", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	jresp := joinTableResponse{}
	err = dec.Decode(&jresp)
	if err != nil {
		return "", err
	}

	p.Table = t
	return jresp.WebsocketSecret, nil
}

func (p *Player) wsConn(ctx context.Context, secret string) error {

	// prepare the request for websocket
	s := p.ss
	u := s.url.JoinPath("ws")
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	q := u.Query()
	q.Add("secret", secret)
	u.RawQuery = q.Encode()

	// dial websocket
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		status := "unknown"
		if resp != nil {
			status = fmt.Sprintf("%d", resp.StatusCode)
		}
		return fmt.Errorf("websocket handshake failed: %v (status: %v)", err, status)
	}

	// get the guid
	var welcomeMsg msgPlayer
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	err = conn.ReadJSON(&welcomeMsg)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to read welcome message: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	p.Guid = welcomeMsg.PlayerGUID

	// run the client
	p.inbox = make(chan msgPlayer, 256)
	p.outbox = make(chan msgPlayer, 256)
	p.ws = &client{
		conn: conn,
	}
	go func() {
		err := p.ws.run(ctx, p.inbox, p.outbox)
		if err != nil {
			log.Errorf("client run error; %v", err)
		}
	}()

	return nil

}

// Join() joins a table and instantiate the web socket.
func (p *Player) Join(ctx context.Context, t Table) error {
	secret, err := p.join(ctx, t)
	if err != nil {
		return err
	}
	err = p.wsConn(ctx, secret)
	if err != nil {
		return err
	}
	return nil
}

// Rejoin() rejoins a table.
func (p *Player) Rejoin(ctx context.Context) error {
	return nil
}

func (p *Player) Quit(ctx context.Context) error {
	return nil
}

// GetMsg() get messages from the gamebox service
func (p *Player) GetMsg(ctx context.Context) (json.RawMessage, error) {
	select {
	case msg := <-p.ws.inbox:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendMsg() sends messages to the gamebox service
func (p *Player) SendMsg(ctx context.Context, msg json.RawMessage) error {
	p.ss.
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
