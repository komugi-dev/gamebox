package gamebox

import (
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
func (c *client) writePump() {
	for msg := range c.outbox {
		err := c.conn.WriteMessage(websocket.TextMessage, msg.payload)
		if err != nil {
			log.Errorf("ws write failed [%v]; %v", c.playerGUID, err)
			break
		}
	}
	log.Infof("writePump exits [%v]", c.playerGUID)
	c.conn.Close()
}

// readPump receives messages from the networs and writes them to inbox
func (c *client) readPump(inbox chan msgPlayer) error {
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

func (c *client) run(inbox chan msgPlayer) error {
	go c.writePump()
	return c.readPump(inbox)
}

func (c *client) dispose() {
	c.conn.Close()
}
