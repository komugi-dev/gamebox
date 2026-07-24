package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type Player struct {
	Name  string // player name
	Guid  string // player unique id
	Table Table  // table the player joins

	turnId string // turn identifier; prevents message spoofing

	ss *Session // a game session

	cli    *client        // exchange messages with the service
	inbox  chan msgPlayer // network -> client
	outbox chan msgPlayer // client -> network

}

type msgPlayer struct {
	PlayerGUID string          `json:"player_guid"`
	TurnID     string          `json:"turn_id"`
	Payload    json.RawMessage `json:"payload"`
}

// CreatePlayer() creates a new player that interacts with the game.
func CreatePlayer(name string, s *Session) *Player {
	return &Player{
		Name:   name,
		ss:     s,
		inbox:  make(chan msgPlayer, 256),
		outbox: make(chan msgPlayer, 256),
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
	req.Header.Set("Accept", "application/json")
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

type rejoinTableRequest struct {
	PlayerGUID string `json:"player_guid"`
}

type rejoinTableResponse struct {
	WebsocketSecret string `json:"websocket_secret"`
}

func (p *Player) rejoin(ctx context.Context) (string, error) {
	s := p.ss
	url := s.url.JoinPath("tables", p.Table.Summary.TableGUID, "rejoin")
	jr := rejoinTableRequest{
		PlayerGUID: p.Guid,
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
	req.Header.Set("Accept", "application/json")
	resp, err := s.cli.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wrong return code %v", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	jresp := rejoinTableResponse{}
	err = dec.Decode(&jresp)
	if err != nil {
		return "", err
	}

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
		return fmt.Errorf("websocket handshake failed: %w (status: %v)", err, status)
	}

	// get the guid
	var welcomeMsg msgPlayer
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	err = conn.ReadJSON(&welcomeMsg)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to read welcome message: %w", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	p.Guid = welcomeMsg.PlayerGUID

	// run the client
	if p.cli != nil {
		p.cli.dispose()
	}
	wsCtx, canc := context.WithCancel(context.Background())
	p.cli = &client{
		conn:       conn,
		cancel:     canc,
		playerGUID: p.Guid,
	}
	p.cli.run(wsCtx, p.inbox, p.outbox)

	return nil

}

// Join() - The player joins a table and instantiate the web socket.
// Note: Join, Rejoin and Quit are not thread-safe.
// Do not call them from different go routines in the same instance of PLayer.
func (p *Player) Join(ctx context.Context, t Table) error {
	secret, err := p.join(ctx, t)
	if err != nil {
		return fmt.Errorf("failed to join; %w", err)
	}
	err = p.wsConn(ctx, secret)
	if err != nil {
		return fmt.Errorf("failed to establish ws connection; %w", err)
	}
	return nil
}

// Rejoin() - The player rejoins a table.
// Note: Join, Rejoin and Quit are not thread-safe.
// Do not call them from different go routines in the same instance of PLayer.
func (p *Player) Rejoin(ctx context.Context) error {
	secret, err := p.rejoin(ctx)
	if err != nil {
		return fmt.Errorf("failed to rejoin; %w", err)
	}
	err = p.wsConn(ctx, secret)
	if err != nil {
		return fmt.Errorf("failed to establish ws connection; %w", err)
	}
	return nil
}

type quitTableRequest struct {
	PlayerGUID string `json:"player_guid"`
}

// Quit() - The player quits the table.
// Note: Join, Rejoin and Quit are not thread-safe.
// Do not call them from different go routines in the same instance of PLayer.
func (p *Player) Quit(ctx context.Context) error {
	s := p.ss
	url := s.url.JoinPath("tables", p.Table.Summary.TableGUID, "quit")
	jr := quitTableRequest{
		PlayerGUID: p.Guid,
	}
	buf, err := json.Marshal(jr)
	if err != nil {
		return fmt.Errorf("failed to encode request [%v][%v]", jr, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewReader(buf))
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

	if p.cli != nil {
		p.cli.dispose()
		p.cli = nil
	}

	return nil
}

// GetMsg() get messages from the gamebox service.
// The function blocs until a message is received or when the context is canceled.
func (p *Player) GetMsg(ctx context.Context) (json.RawMessage, error) {
	select {
	case msg, ok := <-p.inbox:
		if !ok {
			log.Infof("inbox chan closed, exiting")
			return nil, nil
		}
		p.turnId = msg.TurnID
		return msg.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendMsg() sends messages to the gamebox service.
func (p *Player) SendMsg(ctx context.Context, msg json.RawMessage) error {
	mp := msgPlayer{
		Payload:    msg,
		PlayerGUID: p.Guid,
		TurnID:     p.turnId,
	}
	select {
	case p.outbox <- mp:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
