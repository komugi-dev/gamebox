package client

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const ClientTimeout = 15 * time.Second

type client struct {
	playerGUID string
	conn       *websocket.Conn // websocket
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// writePump() receives messages from outbox and writes them to the network
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
func (c *client) readPump(ctx context.Context, inbox chan<- msgPlayer) {
	var err error
	//defer close(inbox) // TODO: clean disconnection when the service goes down
	defer c.conn.Close()
	for {
		msg := msgPlayer{}
		err = c.conn.ReadJSON(&msg)
		if err != nil {
			log.Errorf("ws read failed [%v]; %v", c.playerGUID, err)
			break
		}
		if msg.PlayerGUID != c.playerGUID {
			log.Errorf("guid check failed exp:[%v] recv:[%v]", c.playerGUID, msg.PlayerGUID)
			continue
		}
		select {
		case inbox <- msg:
		case <-ctx.Done():
			log.Infof("readPump stopping due to context [%v]", c.playerGUID)
			return
		}
	}
	log.Infof("readPump exits [%v]", c.playerGUID)
}

// run() starts the read and write routines to exchange messages between the game engine and the pleayer.
// inbox receives messages from the player.
// outbox sends messages to the player.
// Both chans are managed externally.
func (c *client) run(ctx context.Context, inbox chan<- msgPlayer, outbox <-chan msgPlayer) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.writePump(ctx, outbox)
	}()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.readPump(ctx, inbox)
		c.dispose()
	}()
}

// dispose() blocks until the routines controlled by the client have been terminated.
func (c *client) dispose() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.conn != nil {
		// unblock the read pump
		c.conn.Close()
	}
	c.wg.Wait()
	log.Infof("Client [%v] completely disposed", c.playerGUID)
}
