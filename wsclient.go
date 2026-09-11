package gamebox

import (
	"context"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type client struct {
	conn       *websocket.Conn
	playerGUID string
}

type ClientMsg struct {
}

func newClient(wsConn *websocket.Conn, playerGUID string) *client {
	return &client{
		conn:       wsConn,
		playerGUID: playerGUID,
	}
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
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
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
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
				websocket.CloseNoStatusReceived) {
				log.Errorf("ws read failed [%v]; %v", c.playerGUID, err)
			} else {
				log.Infof("ws read closed cleanly [%v]", c.playerGUID)
			}
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

func (c *client) dispose() {
	if c.conn != nil {
		c.conn.Close()
	}
}
