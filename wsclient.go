package gamebox

import (
	"context"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type client struct {
	conn       *websocket.Conn
	playerGUID string
	outbox     chan msgPlayer // app -> websocket
}

type ClientMsg struct {
}

func newClient(wsConn *websocket.Conn, playerGUID string) *client {
	return &client{
		conn:       wsConn,
		playerGUID: playerGUID,
		outbox:     make(chan msgPlayer, 256),
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
				return
			}

			err := c.conn.WriteMessage(websocket.TextMessage, msg.payload)
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
		if msg.playerGUID != c.playerGUID {
			log.Errorf("guid check failed exp:[%v] recv:[%v]", c.playerGUID, msg.playerGUID)
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
		select {
		case <-ctx.Done():
			log.Infof("Context done, forcing close for client %s", c.playerGUID)
			c.conn.Close()
		}
	}()

	go c.writePump(ctx, outbox)

	return c.readPump(inbox)
}

func (c *client) dispose() {
	c.conn.Close()
}
